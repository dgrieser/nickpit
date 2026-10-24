### C# 10 on .NET 6: The Complete Production Developer Guide

> **Version**: C# 10 shipped with the .NET 6 SDK and Visual Studio 2022 on **November 8, 2021**. The final .NET 6 patch is **.NET 6.0.36 / SDK 6.0.428 (November 12, 2024)**; ASP.NET Core and EF Core 6 ended on the same 6.0.36 version.
> **Support status (October 2026)**: ⚠️ **.NET 6 is out of support.** It was a Long Term Support (LTS) release, and its support ended on **November 12, 2024**. Microsoft ships no fixes for it. Security fixes released since then (for example the October 2025 Kestrel request-smuggling fix, CVE-2025-55315) went only to supported versions. The current LTS release is **.NET 10 (C# 14)**, released November 11, 2025. Its latest patch is 10.0.13 (October 13, 2026), and it is supported until November 14, 2028. **.NET 8 (LTS) and .NET 9 (STS) both reach end of support on November 10, 2026.** **.NET 11 / C# 15 is at Release Candidate 2** (October 13, 2026) and is not a stable release. **Recommend migrating C# 10/.NET 6 code to .NET 10 LTS.** Do not recommend .NET 8 or 9 as the upgrade target, because both are about to go out of support, and do not recommend .NET 11 for production yet.
> **Scope**: This guide covers the C# 10 language, the .NET 6 base class library and runtime, ASP.NET Core 6, EF Core 6, and the surrounding SDK tooling. It covers idioms, pitfalls, security, performance, testing and review rules. Sections marked **New in C# 10** or **New in .NET 6** cover version-specific behaviour.
> **Note**: This document is self-contained. Everything from C# 7–9 and .NET Core 3.1/.NET 5 that is still correct is carried forward. Anything from C# 11 / .NET 7 or later is listed under "Do not use or recommend". Every code example was compiled with the C# compiler of SDK 6.0.428 and run on the .NET 6.0.36 runtime.

---

#### Overview

C# 10 is the language version that the .NET 6 SDK uses by default for projects that target `net6.0`. Its features:

- **record structs** (`record struct`, `readonly record struct`)
- **struct improvements**: parameterless constructors, field initializers, and `with` expressions on any struct and on anonymous types
- **`global using` directives** and SDK **implicit usings**
- **file-scoped namespaces**
- **extended property patterns** (`{ A.B.C: 1 }`)
- **lambda improvements**: a natural delegate type, attributes on lambdas, and explicit return types
- **constant interpolated strings**
- **`sealed` `ToString()` in records**
- **improved definite assignment**
- **mixed declaration and assignment in deconstruction**
- **`[AsyncMethodBuilder]` on individual methods**
- **`[CallerArgumentExpression]`**
- **interpolated string handlers**
- **the `#line (start) - (end)` span format**
- **warning wave 6**

Two more features shipped only as *previews* that need `LangVersion=preview` and `EnablePreviewFeatures`: generic attributes and static abstract interface members. Both became stable in C# 11, so treat them as unavailable.

.NET 6 itself adds, among others:

- `DateOnly`/`TimeOnly`, `PriorityQueue<TElement,TPriority>`, and the LINQ operators `MaxBy`/`MinBy`/`DistinctBy`/`UnionBy`/`IntersectBy`/`ExceptBy`, `Chunk`, `TryGetNonEnumeratedCount`, three-way `Zip`, and `Index`/`Range` overloads of `ElementAt`/`Take`
- `FirstOrDefault(defaultValue)`
- `Parallel.ForEachAsync`, `Task.WaitAsync`, `PeriodicTimer`, `CancellationTokenSource.TryReset`, `Random.Shared` and `RandomNumberGenerator.GetBytes(int)`
- `ArgumentNullException.ThrowIfNull`
- `System.Text.Json.Nodes` (`JsonNode`), System.Text.Json source generation, and `IAsyncEnumerable<T>` (de)serialization
- `ReferenceHandler.IgnoreCycles`
- `RandomAccess`, `File.OpenHandle`, `NativeMemory`, and `CollectionsMarshal.GetValueRefOrAddDefault`
- `System.Diagnostics.Metrics` (`Meter`), IANA↔Windows time-zone conversion, `Environment.ProcessPath`, and `string.ReplaceLineEndings`
- the `[LoggerMessage]` source generator and `Rfc2898DeriveBytes.Pbkdf2`

ASP.NET Core 6 introduces **minimal hosting** (`WebApplication.CreateBuilder`) and **minimal APIs** (`app.MapGet`), plus HTTP logging and `IResult`/`Results`. EF Core 6 adds temporal tables (SQL Server), compiled models, pre-convention configuration (`ConfigureConventions`), `[Precision]`/`[Unicode]`/`[EntityTypeConfiguration]` attributes, and migration bundles.

Global truths for a C# 10 / .NET 6 code base:

- **The runtime is unpatched.** Any CVE published after November 2024 that affects .NET 6 stays open. Flag new projects that target `net6.0` and production services still on it. The fix is a migration to `net10.0`, not a newer 6.0.x, because 6.0.36 is the last.
- **The language version follows the target framework.** For `net6.0` the default is C# 10. The SDK 6.0.4xx compiler (Roslyn 4.3) still says `latest` = 10.0. With `LangVersion=preview` it unlocks *unfinished* C# 11 features (see Version-aware review).
- **Nullable reference types are a static analysis, not a runtime guarantee.** New .NET 6 templates set `<Nullable>enable</Nullable>`, but `null` still arrives through deserialization, reflection, EF materialization and `!`.
- **`async`/`await` is the concurrency model.** Blocking on tasks (`.Result`, `.Wait()`) and `async void` are bugs outside narrow cases.
- **Minimal hosting is the default.** `Program.cs` uses top-level statements and `WebApplication`. The `Startup` class pattern still works but is optional.
- **Implicit usings are on in new templates.** `System`, `System.IO`, `System.Linq`, `System.Net.Http`, `System.Threading(.Tasks)` and `System.Collections.Generic` are in scope without a `using`. The Web SDK adds more.

***

#### Version-Aware Review

##### How the version is determined

- `<TargetFramework>net6.0</TargetFramework>` with no `<LangVersion>` means C# 10 on .NET 6. This is the normal case for this guide.
- `<LangVersion>10</LangVersion>` (or `10.0`) on another framework (`netstandard2.0`, `net48`, `netcoreapp3.1`) means C# 10 *syntax* with that framework's *library*. Features that need runtime or library types then fail or need polyfills:
  - `readonly record struct` and record classes need `System.Runtime.CompilerServices.IsExternalInit`.
  - `[CallerArgumentExpression]` needs the attribute type.
  - `[AsyncMethodBuilder(typeof(PoolingAsyncValueTaskMethodBuilder<>))]` needs .NET 6.
  - Interpolated strings fall back to `string.Format` without `DefaultInterpolatedStringHandler`.
  - Every .NET 6 API in this guide is missing.

  Compiling a `readonly record struct` against `netstandard2.1` fails with `CS0518: Predefined type 'System.Runtime.CompilerServices.IsExternalInit' is not defined or imported`. A plain `record struct` compiles, because its positional properties are `get; set;`.
- `global.json` pins the SDK. The final .NET 6 SDKs are 6.0.428 (6.0.4xx band) and 6.0.136 (6.0.1xx band).
- Multi-targeting (`<TargetFrameworks>net6.0;net48</TargetFrameworks>`) compiles the same source for each target. Code that uses .NET 6 APIs needs `#if NET6_0_OR_GREATER`.

##### Rules for the reviewer

1. **Verify before claiming something is unavailable.** Say "not available in C# 10 / .NET 6" only when the pinned language version or framework proves it. Many APIs that feel new are older: `ArgumentNullException.ThrowIfNull`, `Random.Shared`, `MaxBy`, `Chunk`, `DateOnly`, `PriorityQueue`, `Parallel.ForEachAsync` and `Task.WaitAsync` all exist in .NET 6.
2. **Never suggest C# 11+ syntax or .NET 7+ APIs** (see the next section). For example, do not "simplify" a guard to `ArgumentException.ThrowIfNullOrEmpty` (.NET 7) or a disposal check to `ObjectDisposedException.ThrowIf` (.NET 7). Do not replace a constructor with a primary constructor (C# 12) or a `new List<int> { … }` with a collection expression (C# 12).
3. **Treat `<LangVersion>preview</LangVersion>` or `latest` + `EnablePreviewFeatures` on `net6.0` as a finding.** On SDK 6.0.428 (Roslyn 4.3.1), `preview` compiles most of C# 11 in pre-release form: raw string literals, list patterns, UTF-8 literals, `>>>`, generic attributes, newlines in interpolation holes, auto-default structs, `ref` fields, `Span<char>` constant patterns, extended `nameof` scope, checked operators and `static abstract` interface members. These implementations are unsupported in production (the compiler itself reports them as "in Preview and *unsupported*"). Generic math additionally needs `EnablePreviewFeatures` and the `System.Runtime.Experimental` package.
4. **Flag the platform itself.** A pull request that adds features to a `net6.0` service should at least carry a ticket for the `net10.0` migration. A pull request that *creates* a `net6.0` project is a defect.
5. **Package versions matter.** Many 2025–2026 packages no longer support `net6.0`. Microsoft.Extensions.* 10.0 and System.Text.Json 9.0 restore on `net6.0` through their `netstandard2.0` assets. They print `warning : Microsoft.Extensions.Logging.Abstractions 10.0.0 doesn't support net6.0 and has not been tested with it. Consider upgrading your TargetFramework to net8.0 or later.` Setting `<SuppressTfmSupportBuildWarnings>true</SuppressTfmSupportBuildWarnings>` hides that warning. Flag the suppression: the combination is untested.

```xml
<!-- Typical C# 10 / .NET 6 project file (as generated by the 6.0 templates) -->
<Project Sdk="Microsoft.NET.Sdk.Web">
  <PropertyGroup>
    <TargetFramework>net6.0</TargetFramework>   <!-- ⚠️ out of support: plan net10.0 -->
    <Nullable>enable</Nullable>
    <ImplicitUsings>enable</ImplicitUsings>
    <TreatWarningsAsErrors>true</TreatWarningsAsErrors>
    <!-- Do NOT add <LangVersion>preview</LangVersion> or <EnablePreviewFeatures> -->
  </PropertyGroup>
</Project>
```

***

#### Do Not Use or Recommend (C# 11+ / .NET 7+)

The project's compiler and runtime do not have these features. Each item below was checked: it fails to compile with `LangVersion 10` on SDK 6.0.428 or against the `net6.0` reference assemblies. Where noted, it compiles on the next version.

##### Language features newer than C# 10

| Feature | Syntax | Introduced | C# 10 diagnostic |
|---|---|---|---|
| Raw string literals | `"""…"""` | C# 11 | CS8652 (preview feature) |
| List patterns | `x is [1, .., 3]` | C# 11 | CS8652 |
| Required members | `required string Name { get; init; }` | C# 11 | CS8652 + missing `RequiredMemberAttribute` |
| Generic math / static abstract interface members | `where T : INumber<T>`, `static abstract` | C# 11 + .NET 7 | CS8703; `INumber<T>` absent from `net6.0` |
| Generic attributes | `class MyAttr<T> : Attribute`, `[MyAttr<int>]` | C# 11 | CS8652 |
| UTF-8 string literals | `"abc"u8` | C# 11 | CS8652 |
| Newlines inside interpolation holes | `$"{a\n + b}"` | C# 11 | CS8967 |
| File-local types | `file class Helper` | C# 11 | syntax error |
| Auto-default structs | ctor may leave fields unassigned | C# 11 | CS0171 |
| `ref` fields, `scoped` | `ref struct R { ref int F; }` | C# 11 | CS8652 |
| Pattern match `Span<char>` on a constant string | `span is "abc"` | C# 11 | CS8652 |
| Extended `nameof` scope | `[Obsolete(nameof(param))]` on a method | C# 11 | CS0103 |
| Unsigned right shift | `x >>> 1` | C# 11 | CS8652 |
| Checked user-defined operators | `operator checked +` | C# 11 | CS8652 |
| Primary constructors on classes/structs | `class Svc(ILogger log)` | C# 12 | syntax error |
| Collection expressions | `int[] a = [1, 2];`, `[..xs]` | C# 12 | CS1525 |
| Alias any type | `using Pair = (int A, int B);` | C# 12 | syntax error |
| Default lambda parameters | `(int x = 2) => x` | C# 12 | CS1065 |
| `ref readonly` parameters, inline arrays, `[Experimental]` | — | C# 12 | — |
| `params` collections | `params List<int> xs`, `params ReadOnlySpan<T>` | C# 13 | CS0225 |
| `System.Threading.Lock` + `lock` integration | `private readonly Lock _gate = new();` | C# 13 / .NET 9 | type missing |
| `\e` escape | `'\e'` | C# 13 | CS1009 |
| `ref` locals / `unsafe` in async methods and iterators | `ref int x = ref …;` inside `async` | C# 13 | CS8177 |
| `ref struct` interfaces, `allows ref struct`, partial properties, `[OverloadResolutionPriority]` | — | C# 13 | — |
| `field` keyword | `get => field;` | C# 14 | CS0103 |
| Null-conditional assignment | `c?.X = 1;` | C# 14 | CS0131 |
| Extension members, implicit span conversions, `nameof(List<>)`, partial constructors/events, compound-assignment operators | — | C# 14 | — |

Every C# 11 row except file-local types and required members (which still lacks its attributes on `net6.0`) compiles on SDK 6.0.428 when `LangVersion=preview` is set (rule 3 above). The C# 12+ rows fail even there. Compiled with the .NET 7 SDK at `LangVersion 11`, raw strings, list patterns, `u8`, file-local types and newline holes work. Required members additionally need `RequiredMemberAttribute`/`CompilerFeatureRequiredAttribute`, which `net6.0` lacks. Generic math needs `net7.0`.

The C# 10 replacements:

- Instead of **raw strings**, use verbatim strings: `@"{""a"": 1}"`.
- Instead of **required members**, use constructor parameters or positional records, plus validation.
- Instead of **list patterns**, use `Length`/indexer checks.
- Instead of **primary constructors**, write an explicit constructor that assigns `private readonly` fields.
- Instead of **collection expressions**, use `new[] { … }`, `new List<T> { … }` or `Array.Empty<T>()`.
- Instead of **file-local types**, use `internal` or `private` nested types.

##### BCL APIs newer than .NET 6 (verified missing from the `net6.0` reference pack)

- **.NET 7**:
  - `ArgumentException.ThrowIfNullOrEmpty` and `ObjectDisposedException.ThrowIf`
  - `Enumerable.Order()`/`OrderDescending()` and `Stopwatch.GetElapsedTime`
  - `Int128`/`UInt128`, `DateTime.Microsecond`/`Nanosecond`, and `System.Formats.Tar`
  - `Stream.ReadExactly`/`ReadAtLeast`, `Regex.EnumerateMatches`, `RegexOptions.NonBacktracking` and `[GeneratedRegex]`
  - `UnreachableException`, `Path.Exists` and `Directory.CreateTempSubdirectory`
  - `SHA256.HashData(Stream)`
  - in System.Text.Json: `JsonSerializerOptions.TypeInfoResolver`, `[JsonDerivedType]`/`[JsonPolymorphic]`, `[JsonRequired]`, and `DateOnly`/`TimeOnly` support
  - generic math (`INumber<T>`, `IParsable<T>`)
- **.NET 8**:
  - `ArgumentException.ThrowIfNullOrWhiteSpace` and `ArgumentOutOfRangeException.ThrowIfNegative`/`ThrowIfZero`/`ThrowIfGreaterThan`…
  - `TimeProvider` and `System.Collections.Frozen` (`FrozenDictionary`, `FrozenSet`)
  - `Random.Shuffle`/`GetItems` and `SearchValues`
  - `CompositeFormat` and `CancellationTokenSource.CancelAsync`
  - `ConfigureAwaitOptions` and `Parallel.ForAsync`
  - `IPNetwork` and `SHA3_256`
- **.NET 9**:
  - `CountBy`, `AggregateBy` and `Index()` in LINQ, and `Task.WhenEach`
  - `System.Threading.Lock` and `JsonSerializerOptions.Web`
  - `X509CertificateLoader`, `Guid.CreateVersion7` and `Base64Url`
  - `BinaryFormatter` removed: it always throws in .NET 9+.

##### ASP.NET Core 7+ (verified missing from the `Microsoft.AspNetCore.App` 6.0 reference pack)

- **7**:
  - `app.MapGroup`, `TypedResults` and `Results<T1,T2>`, `[AsParameters]`, and endpoint filters (`AddEndpointFilter`)
  - `IFormFile` binding in minimal APIs (in .NET 6 such a parameter is treated as a JSON body, and a multipart upload returns **415**)
  - `AddProblemDetails`, rate limiting (`AddRateLimiter`/`UseRateLimiter`) and output caching (`AddOutputCache`)
- **8**:
  - keyed services (`AddKeyedSingleton`, `[FromKeyedServices]`) and `IExceptionHandler`/`AddExceptionHandler<T>`
  - `AddOptionsWithValidateOnStart` and `ShortCircuit()`
  - Native AOT for ASP.NET Core and Identity API endpoints
- **9**: `MapStaticAssets`, built-in OpenAPI document generation (`AddOpenApi`), and `HybridCache`.

##### EF Core 7+ (verified missing from EF Core 6.0.36)

`ExecuteUpdate`/`ExecuteDelete` (bulk operations), `FromSql` (the interpolated-only form), `Database.SqlQuery<T>`/`SqlQueryRaw<T>`, `Database.ExecuteSql`, JSON columns (`OwnsOne(...).ToJson()`), TPC mapping, `DateOnly`/`TimeOnly` mapping on SQL Server (EF Core 8), and complex types (EF Core 8). In EF Core 6, use `FromSqlInterpolated`, `ExecuteSqlInterpolated`, and load-modify-`SaveChanges` or raw SQL for bulk updates.

***

#### What's New in C# 10 (summary)

| Feature | Section | Gated by `LangVersion`? |
|---|---|---|
| Record structs | [Records](#records-and-record-structs) | yes (CS8773 on C# 9) |
| Parameterless struct constructors, struct field initializers | [Structs](#new-in-c-10-struct-improvements) | yes |
| `with` on structs and anonymous types | [Structs](#new-in-c-10-struct-improvements) | yes |
| `global using`, implicit usings | [Global usings](#new-in-c-10-global-and-implicit-usings) | yes |
| File-scoped namespaces | [Code style](#new-in-c-10-file-scoped-namespaces) | yes |
| Extended property patterns | [Pattern matching](#pattern-matching) | yes |
| Lambda natural type, attributes, explicit return type | [Lambdas](#new-in-c-10-lambda-improvements) | yes |
| Constant interpolated strings | [Strings](#new-in-c-10-constant-interpolated-strings) | yes |
| `sealed override ToString()` in records | [Records](#records-and-record-structs) | yes |
| Mixed declaration/assignment in deconstruction | [Deconstruction](#new-in-c-10-deconstruction-mixing) | yes |
| `[AsyncMethodBuilder]` on methods | [Async](#new-in-c-10-per-method-asyncmethodbuilder) | yes |
| `[CallerArgumentExpression]` | [Guards](#new-in-c-10-callerargumentexpression) | **no**: it also works with the C# 10 compiler at `LangVersion 9` |
| Interpolated string handlers | [Strings](#new-in-c-10-interpolated-string-handlers) | **yes**: at `LangVersion 9` the `string` overload is chosen and every hole is evaluated |
| Improved definite assignment | [Nullable](#new-in-c-10-improved-definite-assignment) | **no**: the 6.0.428 compiler applies it at `LangVersion 9` too |
| `#line (l,c)-(l,c) offset "file"` | [Tooling](#new-in-c-10-line-span-directive) | **no** |
| Warning wave 6 (CS8826) | [Analyzers](#analyzers-and-warnings) | `AnalysisLevel`/`-warn:6` |

The "gated" column was established by compiling each example with `LangVersion 9` and `10` on SDK 6.0.428. These checks show that `[CallerArgumentExpression]`, improved definite assignment and the new `#line` form are compiler behaviours, not language-version-gated features. Code that relies on them compiles with the SDK 6.0.428 compiler whatever the `LangVersion`. Do not treat them as proof that a project needs C# 10.

***

#### Project Setup and Build Hygiene

##### Recommended project settings

```xml
<!-- Directory.Build.props: shared by every project in the repository -->
<Project>
  <PropertyGroup>
    <Nullable>enable</Nullable>
    <ImplicitUsings>enable</ImplicitUsings>
    <TreatWarningsAsErrors>true</TreatWarningsAsErrors>
    <AnalysisLevel>6.0</AnalysisLevel>           <!-- or latest-recommended -->
    <EnforceCodeStyleInBuild>true</EnforceCodeStyleInBuild>
    <GenerateDocumentationFile>true</GenerateDocumentationFile>
    <NoWarn>$(NoWarn);CS1591</NoWarn>            <!-- missing XML docs: opt in per project -->
    <Deterministic>true</Deterministic>
  </PropertyGroup>
</Project>
```

`global.json` pins the SDK so that CI and developers build with the same compiler:

```json
{ "sdk": { "version": "6.0.428", "rollForward": "latestPatch" } }
```

Review points:

- **`TreatWarningsAsErrors` plus nullable warnings** is the only way nullable annotations stay meaningful. Use `<WarningsAsErrors>nullable</WarningsAsErrors>` if full warnings-as-errors is too strict.
- **`<Nullable>disable</Nullable>` in new code** is a finding. Prefer enabling it per file (`#nullable enable`) during a migration.
- **Package versions:** prefer a lock file (`<RestorePackagesWithLockFile>true</RestorePackagesWithLockFile>`, then `dotnet restore --locked-mode` in CI). Central package management (`Directory.Packages.props`, `ManagePackageVersionsCentrally`) restores correctly with SDK 6.0.428, but that SDK still prints "…is using CentralPackageVersionManagement, a NuGet preview feature". `GlobalPackageReference` needs NuGet 6.4 (SDK 7.0.100 or later).
- **`<SuppressTfmSupportBuildWarnings>`** hides "package X doesn't support net6.0" warnings. Flag it.
- **The SDK's `dotnet format`** is built into the .NET 6 SDK. Run `dotnet format --verify-no-changes` in CI.

##### New in C# 10: Global and implicit usings

`global using` applies a using directive to every file in the compilation. With `<ImplicitUsings>enable</ImplicitUsings>`, the SDK generates `obj/…/<Project>.GlobalUsings.g.cs`:

- **`Microsoft.NET.Sdk`**: `System`, `System.Collections.Generic`, `System.IO`, `System.Linq`, `System.Net.Http`, `System.Threading`, `System.Threading.Tasks`.
- **`Microsoft.NET.Sdk.Web`** adds `System.Net.Http.Json`, `Microsoft.AspNetCore.Builder`, `.Hosting`, `.Http`, `.Routing`, `Microsoft.Extensions.Configuration`, `.DependencyInjection`, `.Hosting` and `.Logging`.

These lists were taken from the generated file of SDK 6.0.428.

```csharp
// GlobalUsings.cs — one file per project, not scattered across the code base
global using System.Text;
global using static System.Math;

var sb = new StringBuilder();
sb.Append(Max(1, 2));          // Math.Max via global using static
Console.WriteLine(sb);         // System via implicit usings
```

Rules:

- **Put all `global using` directives in one file** (`GlobalUsings.cs`). A `global using` hidden in a random file changes name resolution everywhere.
- **`global using` must precede all non-global usings and namespace declarations** in a file (error CS8915 otherwise).
- **Ambiguities**: implicit usings put `System.Threading` in scope. A file that also imports `System.Timers` gets `CS0104: 'Timer' is an ambiguous reference`. Fix it with an alias (`using Timer = System.Timers.Timer;`), not by turning off implicit usings.
- Remove an unwanted implicit namespace in the project file: `<Using Remove="System.Net.Http" />`. Add one with `<Using Include="MyCompany.Common" />`.
- **Multi-targeting libraries**: `ImplicitUsings` also generates global usings for older targets such as `netstandard2.0`. Those targets then need `LangVersion` 10 or later, because `global using` is a C# 10 feature. Without it, `netstandard2.0` builds at its default C# 7.3 and fail with CS8370 in the generated `GlobalUsings.g.cs`.

```csharp
using System.Timers;
var t = new Timer(1000);   // CS0104 — ambiguous between System.Timers.Timer and System.Threading.Timer
```

```csharp
using System.Timers;
using Timer = System.Timers.Timer;   // alias resolves the ambiguity
using var t = new Timer(1000) { AutoReset = false };
Console.WriteLine(t.Interval);
```

##### New in C# 10: File-scoped namespaces

```csharp
namespace Contoso.Orders;          // applies to the whole file, saves one indentation level

public sealed class OrderNumber
{
    public OrderNumber(string value) => Value = value;
    public string Value { get; }
    public static void Main() => System.Console.WriteLine(new OrderNumber("A-1").Value);
}
```

- A file can contain **only one** file-scoped namespace, and it cannot be mixed with block namespaces (CS8954/CS8955).
- Prefer file-scoped namespaces in all new C# 10 code. Enforce them with `.editorconfig`: `csharp_style_namespace_declarations = file_scoped:warning`.
- A file with top-level statements cannot use a file-scoped namespace: statements after `namespace X;` are syntax errors, and `namespace X;` after statements is CS8956. Keep `Program.cs` without a namespace.

***

#### Naming and Code Style

##### General rules

- **PascalCase** for types, namespaces, methods, properties, events and constants (`MaxRetryCount`, not `MAX_RETRY_COUNT`). **camelCase** for parameters and locals. **`_camelCase`** for private fields (`private readonly IUserRepository _repository;`).
- Interfaces start with `I` (`IUserRepository`). Async methods end with `Async` and take a `CancellationToken ct = default` last. Boolean members read as predicates (`IsActive`, `HasDiscount`, `CanEdit`).
- **Type parameters**: `T`, or `TKey`/`TValue`/`TResult` with a `T` prefix.
- **Do not name types in all-lowercase ASCII.** C# 11 (warning wave 7, CS8981) warns about such names because future keywords are lowercase. The C# 10 compiler does not warn yet (verified), but `record`, `required`, `scoped` and `file` show the risk.
- **Local functions and lambdas**: PascalCase for local functions, camelCase for delegate variables.
- **Member order** within a type: constants, static fields, instance fields, constructors, properties, public methods, private methods. One top-level type per file, and the file name matches the type name.
- Keep the domain project free of infrastructure references (EF Core, HTTP clients). Put a `Directory.Build.props` and `global.json` at the repository root.

##### Expression style

- Use `var` when the type is obvious from the right-hand side (`var list = new List<int>();`). Spell out the type when it carries meaning (`decimal total = 0;`).
- Use **target-typed `new`** (C# 9) in fields and arguments: `private readonly List<Order> _orders = new();`.
- Use expression-bodied members for one-liners only.
- Use `is null` / `is not null` rather than `== null` when a type may overload `==`.
- Use `nameof(x)` instead of string literals for member and parameter names.

***

#### Records and Record Structs

##### Record classes (C# 9)

```csharp
public record User(int Id, string Name, string Email);   // positional: init-only properties, value equality

public record Order
{
    public int Id { get; init; }
    public string CustomerName { get; init; } = "";
    public decimal Total { get; init; }
    public bool IsHighValue => Total > 1000;
}

public static class Demo
{
    public static void Main()
    {
        var user = new User(1, "Ann", "ann@example.com");
        var renamed = user with { Name = "Anna" };           // non-destructive mutation (shallow copy)
        Console.WriteLine(renamed);                           // User { Id = 1, Name = Anna, Email = ann@example.com }
        Console.WriteLine(user == new User(1, "Ann", "ann@example.com"));   // True: value equality
        Console.WriteLine(new Order { Total = 2000m }.IsHighValue);          // True
    }
}
```

##### New in C# 10: `record struct` and `readonly record struct`

```csharp
var p = new Point(1, 2);
var q = p with { Y = 5 };
p.X = 10;                                  // ⚠️ record struct positional properties are MUTABLE (get; set;)
Console.WriteLine($"{p} {q} {p == new Point(10, 2)}");  // Point { X = 10, Y = 2 } Point { X = 1, Y = 5 } True

var r = new Money(12.5m, "EUR");
Console.WriteLine(r);                      // Money { Amount = 12.5, Currency = EUR }
// r.Amount = 1;                           // error CS8852: init-only property

public record struct Point(int X, int Y);                      // mutable value type with value equality
public readonly record struct Money(decimal Amount, string Currency);   // immutable: prefer this
```

Review rules:

- **Prefer `readonly record struct`** for small value objects. A plain `record struct` generates `get; set;` positional properties, unlike a record *class* (`get; init;`). Mutable structs cause lost updates when a copy is modified. `list[0].X = 1` does not compile (CS1612), but `var item = list[0]; item.X = 1;` or a copy returned from a property or method silently changes only the copy.
- **`default(Money)` bypasses the constructor**: `Currency` is `null` even though it is declared non-nullable. Validate in consumers, or use a class when "no invalid default" matters.
- **Record equality is shallow and member-wise.** A record holding a `List<T>` or an array compares by *reference* for that member. Two records with equal list contents are not equal. Use `ImmutableArray<T>` with a custom `Equals`, or avoid collections in value-equality records.
- **`with` is a shallow copy.** Reference-type members are shared between the original and the copy.
- **Records as EF Core entities** are a smell. Value equality conflicts with change tracking, and `with` creates untracked copies. Use classes for entities and records for DTOs and value objects.
- **Record structs do not support inheritance.** Record classes can inherit only from records.

```csharp
var a = new Basket("ann", new List<string> { "apple" });
var b = new Basket("ann", new List<string> { "apple" });
Console.WriteLine(a == b);                 // False: List<string> compared by reference

var c = a with { Owner = "bob" };
c.Items.Add("pear");                       // ⚠️ also visible through a: shallow copy
Console.WriteLine(a.Items.Count);          // 2

public record Basket(string Owner, List<string> Items);
```

##### New in C# 10: `sealed` `ToString()` in records

A base record can seal its `ToString()` override, so derived records no longer synthesize their own. This is useful when `ToString()` must not leak sensitive members.

```csharp
Console.WriteLine(new Customer("ann", "4111-1111-1111-1111"));   // Person(ann) — derived record keeps the sealed format

public record Person(string Name)
{
    public sealed override string ToString() => $"Person({Name})";
}
public record Customer(string Name, string CardNumber) : Person(Name);
```

Records print **all** public properties by default. A record with `Password`, `Token` or `CardNumber` properties leaks them into logs as soon as someone writes `logger.LogInformation("{Request}", request)`. Override `PrintMembers` or `ToString`, or keep secrets out of records.

***

#### New in C# 10: Struct Improvements

C# 10 allows explicit **parameterless constructors** and **field initializers** in structs, and allows `with` on any struct and on anonymous types.

```csharp
var a = new Settings();          // runs the parameterless constructor
var b = default(Settings);       // does NOT run it
var c = new Settings[1];         // array elements do NOT run it
Console.WriteLine($"{a.Port} {b.Port} {c[0].Port}");     // 8080 0 0

var d = new Endpoint();          // ⚠️ no parameterless ctor declared: field initializers are skipped!
Console.WriteLine($"{d.Host ?? "null"} {d.Port}");       // null 0
var e = new Endpoint(443);
Console.WriteLine($"{e.Host} {e.Port}");                 // localhost 443

var anon = new { Id = 1, Name = "x" };
Console.WriteLine(anon with { Id = 2 });                 // { Id = 2, Name = x }

public struct Settings
{
    public int Port;
    public Settings() { Port = 8080; }
}

public struct Endpoint
{
    public string Host = "localhost";        // field initializers run only in explicitly declared constructors
    public int Port = 80;
    public Endpoint(int port) { Port = port; }
}
```

Pitfalls a reviewer must catch (all verified on .NET 6.0.36):

- **`default(T)`, array elements, uninitialized fields and default parameter values never run the parameterless constructor.** Code that relies on a struct constructor to establish an invariant is wrong whenever such a value is reachable.
- **A struct with field initializers but no parameterless constructor**: `new S()` produces the all-zero value, and the initializers do **not** run. The example shows `Host` is `null` although it is declared `= "localhost"`.
- **A struct with field initializers must declare at least one constructor** (CS8983). If some fields have initializers and others do not, the constructor must assign the rest (CS0171). C# 11's auto-default structs are not available.
- `new T()` with `where T : new()` and `Activator.CreateInstance<T>()` **do** call an explicit parameterless struct constructor on .NET 6.

```csharp
public struct Broken     // error CS8983: a 'struct' with field initializers must include an explicitly declared constructor
{
    public int X = 1;
}
```

***

#### New in C# 10: Lambda Improvements

##### Natural type, attributes, explicit return types

```csharp
using System.Diagnostics.CodeAnalysis;

var parse = (string s) => int.Parse(s);           // natural type: Func<string, int>
Console.WriteLine(parse.GetType().Name);          // Func`2

var read = Console.ReadLine;                      // method group natural type: Func<string?>
Console.WriteLine(read.GetType().Name);           // Func`1

var choose = object (bool b) => b ? 1 : "one";    // explicit return type when branches differ
Console.WriteLine(choose(false));                 // one

var trim = [return: NotNullIfNotNull("s")] (string? s) => s?.Trim();   // attributes on lambdas
Console.WriteLine(trim(" x "));

Delegate d = () => 42;                            // lambdas convert to Delegate / object now
Console.WriteLine(d.DynamicInvoke());             // 42
```

- `var f = x => x;` still fails (CS8917): the parameter type must be explicit for a natural type.
- **Attributes on lambdas** require parenthesized parameters. They are how minimal APIs read `[FromBody]`, `[FromServices]` and similar, for example `app.MapPost("/x", ([FromBody] Dto d) => …)`.

##### Breaking change: overload resolution with natural types

Because lambdas and method groups now have a natural type, they convert to `Delegate`, `object` and `Expression`. That can change which overload is chosen when code moves from C# 9 to C# 10:

```csharp
// LangVersion 10 prints "C.M(Delegate)"; the same code at LangVersion 9 prints "E.M(Action)"
var c = new C();
c.M(Hello);
static void Hello() { }
class C { public void M(Delegate d) => Console.WriteLine("C.M(Delegate)"); }
static class E { public static void M(this C c, Action a) => Console.WriteLine("E.M(Action)"); }
```

When a pull request raises `LangVersion` or retargets to `net6.0`, check APIs that overload on `Delegate`/`object` and `Action`/`Func`/`Expression`, such as test helpers, mocking libraries and event buses.

***

#### Strings

##### New in C# 10: Constant interpolated strings

```csharp
Console.WriteLine(Routes.Orders);    // /api/v1/orders
Console.WriteLine(Routes.Version);

public static class Routes
{
    public const string Base = "/api/v1";
    public const string Orders = $"{Base}/orders";     // OK: every hole is a const string
    public const string Version = nameof(Version);
    // public const string Bad = $"{Base}/{42}";       // error CS0133: int hole is not a constant string
}
```

Constant interpolated strings work in attribute arguments, `switch` case labels and default parameter values. Only `string` constants may appear in the holes, because formatting numbers would depend on culture.

##### New in C# 10: Interpolated string handlers

Since C# 10, `$"…"` is not always `string.Format`. The compiler lowers interpolations to `DefaultInterpolatedStringHandler`, which avoids boxing and the format-string parse. It also targets **handler types** chosen by overload resolution: `StringBuilder.Append($"…")`, `Debug.Assert(bool, $"…")`, `string.Create(IFormatProvider, $"…")`, `Span<char>.TryWrite($"…")` and custom handlers.

```csharp
using System.Diagnostics;
using System.Globalization;
using System.Text;

var sb = new StringBuilder();
sb.Append($"total={42}");                   // appends directly into the builder, no temporary string
Console.WriteLine(sb);

int calls = 0;
int Expensive() { calls++; return 1; }
Debug.Assert(true, $"value {Expensive()}"); // handler sees condition == true and skips formatting
Console.WriteLine($"calls={calls}");        // calls=0 — the hole was NOT evaluated

// Culture: interpolation uses the CURRENT culture. Be explicit for machine-readable text.
CultureInfo.CurrentCulture = new CultureInfo("de-DE");
double price = 1234.5;
Console.WriteLine($"{price}");                                            // 1234,5
Console.WriteLine(string.Create(CultureInfo.InvariantCulture, $"{price}"));   // 1234.5
Console.WriteLine(FormattableString.Invariant($"{price}"));               // 1234.5
```

Review consequences:

- **Side effects in interpolation holes may not run.** With a handler that short-circuits (`Debug.Assert` with a true condition, or a custom logging handler when the level is disabled), the expressions inside `{…}` are not evaluated. (In Release builds `Debug.Assert` calls are removed completely by `[Conditional("DEBUG")]`. The example output is from a build with `DEBUG` defined.) Never put state changes (`{counter++}`, `{queue.Dequeue()}`) in holes.
- **Interpolation is culture-sensitive.** Use `string.Create(CultureInfo.InvariantCulture, $"…")` or `FormattableString.Invariant` for SQL fragments, file names, protocol text, cache keys and anything parsed later. Analyzer CA1305 flags this.
- **Handlers are selected only at `LangVersion 10`.** In the verification run, a custom handler overload of `Write` was ignored at `LangVersion 9`: the `string` overload won and every hole was evaluated. Library authors who ship handler overloads must document that callers need C# 10.
- `ILogger.LogInformation($"…")` is **not** a handler API in .NET 6. The string is fully formatted, structured properties are lost, and analyzer CA2254 warns. Use message templates (see Logging).

A custom handler, for example for a logger that skips formatting when disabled:

```csharp
using System.Runtime.CompilerServices;

var log = new AuditLog { Enabled = false };
int calls = 0;
int Expensive() { calls++; return 42; }
log.Write($"value {Expensive()}");      // handler overload chosen; Enabled == false → holes skipped
Console.WriteLine($"calls={calls}");    // calls=0
log.Enabled = true;
log.Write($"value {Expensive()}");      // value 42

public sealed class AuditLog
{
    public bool Enabled { get; set; }
    public void Write(string message) => Console.WriteLine(message);
    public void Write([InterpolatedStringHandlerArgument("")] ref AuditHandler handler)
    {
        if (Enabled) Console.WriteLine(handler.ToStringAndClear());
    }
}

[InterpolatedStringHandler]
public ref struct AuditHandler
{
    private DefaultInterpolatedStringHandler _inner;
    public AuditHandler(int literalLength, int formattedCount, AuditLog log, out bool shouldAppend)
    {
        shouldAppend = log.Enabled;
        _inner = new DefaultInterpolatedStringHandler(literalLength, formattedCount);
    }
    public void AppendLiteral(string s) => _inner.AppendLiteral(s);
    public void AppendFormatted<T>(T value) => _inner.AppendFormatted(value);
    public string ToStringAndClear() => _inner.ToStringAndClear();
}
```

##### String comparison and culture

```csharp
using System.Globalization;

CultureInfo.CurrentCulture = new CultureInfo("tr-TR");
Console.WriteLine("FILE".ToLower() == "file");                         // False: Turkish dotless ı
Console.WriteLine("FILE".ToLowerInvariant() == "file");                // True
Console.WriteLine(string.Equals("file", "FILE", StringComparison.OrdinalIgnoreCase));   // True

var headers = new Dictionary<string, string>(StringComparer.OrdinalIgnoreCase) { ["Content-Type"] = "json" };
Console.WriteLine(headers["content-type"]);                            // json

Console.WriteLine("a\r\nb".ReplaceLineEndings("\n") == "a\nb");        // True (.NET 6)
```

- Use `StringComparison.Ordinal`/`OrdinalIgnoreCase` for identifiers, keys, paths, protocol tokens and security decisions. Use culture-aware comparison only for text shown to users.
- On Linux, and on Windows since .NET 5, globalization uses **ICU**. Culture-sensitive `IndexOf`/`StartsWith` can differ from .NET Framework (embedded nulls and zero-width characters are ignored). Pass `StringComparison.Ordinal` explicitly. CA1307/CA1309/CA1310 flag implicit comparisons.
- In containers, `InvariantGlobalization=true` makes every culture behave like the invariant culture. Verify that this is intended before relying on culture formatting.
- Build strings in loops with `StringBuilder` or `string.Join`/`string.Concat`, not `+=`.

##### New in C# 10: Deconstruction mixing

```csharp
int x = 0;
(x, var y) = (1, 2);          // assignment to an existing variable + new declaration in one deconstruction
Console.WriteLine($"{x} {y}");   // 1 2
```

***

#### Nullable Reference Types

##### Rules

- New .NET 6 templates enable nullable reference types (`<Nullable>enable</Nullable>`). Treat every nullable warning (CS86xx) as a bug. A suppression needs a comment that explains why the value cannot be null.
- **`!` (null-forgiving) is a claim, not a check.** `= null!` / `= default!` on non-nullable properties is acceptable only for members that a framework sets, such as EF Core `DbSet<T>` or options classes bound from configuration. Everywhere else it hides bugs.
- **Annotations are not enforced at runtime.** System.Text.Json in .NET 6 happily assigns `null` to a non-nullable `string` property or record parameter. Model binding, EF materialization of nullable columns and reflection do the same. Validate input at the boundary.
- `string?` on a parameter means callers may pass `null`. Do not add `?` to silence a warning; fix the flow instead.
- **Generic `T?`** on an unconstrained type parameter means "default of T" (C# 9): for `int` it is `int`, not `int?`. Use `[MaybeNull]`/`[return: MaybeNull]` semantics consciously.
- **Initialize every non-nullable property**, either in a constructor, with an initializer, or through a positional record. C# 11 `required` is not available.

##### Guard clauses: `ArgumentNullException.ThrowIfNull` (New in .NET 6)

```csharp
public sealed class OrderService
{
    private readonly IOrderRepository _repository;

    public OrderService(IOrderRepository repository)
    {
        ArgumentNullException.ThrowIfNull(repository);     // .NET 6; ParamName = "repository" via CallerArgumentExpression
        _repository = repository;
    }

    public void Process(Order order)
    {
        ArgumentNullException.ThrowIfNull(order);
        if (order.Items.Count == 0)
            throw new ArgumentException("Order must have items", nameof(order));
        // C# 10 / .NET 6 have no ArgumentException.ThrowIfNullOrEmpty (.NET 7) and no
        // ArgumentOutOfRangeException.ThrowIfNegative (.NET 8): write the check explicitly.
        if (order.Quantity < 0)
            throw new ArgumentOutOfRangeException(nameof(order), order.Quantity, "Quantity must not be negative.");
        _repository.Save(order);
    }

    public static void Main()
    {
        try { new OrderService(null!); }
        catch (ArgumentNullException e) { Console.WriteLine(e.ParamName); }   // repository
    }
}

public interface IOrderRepository { void Save(Order order); }
public sealed class Order { public List<string> Items { get; } = new(); public int Quantity { get; set; } }
```

The house rule "`_x = x ?? throw new ArgumentNullException(nameof(x));`" is still correct. `ThrowIfNull` is shorter. Note that `ThrowIfNull` returns `void`, so it cannot be used in an expression.

##### New in C# 10: `[CallerArgumentExpression]`

```csharp
using System.Runtime.CompilerServices;

static class Ensure
{
    public static void That(bool condition, [CallerArgumentExpression("condition")] string? expression = null)
    {
        if (!condition) throw new ArgumentException($"Condition failed: {expression}");
    }
}

static class Program
{
    static void Main()
    {
        int quantity = 3;
        try { Ensure.That(quantity > 5 && quantity != 3); }
        catch (ArgumentException e) { Console.WriteLine(e.Message); }   // Condition failed: quantity > 5 && quantity != 3
    }
}
```

The attribute type ships with .NET 6 (`System.Runtime.CompilerServices.CallerArgumentExpressionAttribute`). On older targets, a polyfill with that exact name and namespace works. Callers must not pass the optional parameter explicitly, because an explicit argument overrides the captured text.

##### Nullable attributes

```csharp
using System.Diagnostics.CodeAnalysis;

var cache = new UserCache();
if (cache.TryGet(1, out var user))
    Console.WriteLine(user.Name.Length);          // no warning: [NotNullWhen(true)]
Console.WriteLine(UserCache.Normalize(null) is null);   // True

public sealed class UserCache
{
    private readonly Dictionary<int, User> _users = new() { [1] = new User("ann") };
    private string? _connection;

    public bool TryGet(int id, [NotNullWhen(true)] out User? user) => _users.TryGetValue(id, out user);

    [return: NotNullIfNotNull("value")]
    public static string? Normalize(string? value) => value?.Trim().ToLowerInvariant();

    [MemberNotNull(nameof(_connection))]
    private void EnsureConnection() => _connection ??= "Server=.";

    public int ConnectionLength() { EnsureConnection(); return _connection.Length; }   // no warning
}

public sealed record User(string Name);
```

- `[NotNullIfNotNull]` in .NET 6 takes a string (`"value"`). `nameof(value)` for a parameter inside the method's own attribute needs C# 11's extended `nameof` scope (CS0103 in C# 10).
- Use `[DoesNotReturn]` on throw helpers so flow analysis knows that code after them is unreachable.

##### New in C# 10: Improved definite assignment

The C# 10 compiler understands `?.` and `??` comparisons with constants:

```csharp
var c = new Parser();
if (c?.TryParse("42", out int a) == true) Console.WriteLine(a);    // a is definitely assigned here
if (c?.TryParse("43", out int b) ?? false) Console.WriteLine(b);

public sealed class Parser { public bool TryParse(string s, out int value) => int.TryParse(s, out value); }
```

Null-state analysis received the same improvement, so some `!` operators written for older compilers are now unnecessary. Remove them.

***

#### Pattern Matching

##### Type, property, relational and logical patterns (C# 7–9)

```csharp
Console.WriteLine(Discount(new PremiumCustomer(1000m)));          // 200.0
Console.WriteLine(Shipping(new Order(150m, IsPriority: true)));   // Express
Console.WriteLine(Classify(-5));                                  // negative

static decimal Discount(object customer) => customer switch
{
    PremiumCustomer p => p.PurchaseTotal * 0.2m,
    RegularCustomer { YearsActive: > 5 } r => r.PurchaseTotal * 0.1m,
    RegularCustomer r => r.PurchaseTotal * 0.05m,
    null => 0m,
    _ => throw new ArgumentException("Unknown customer type", nameof(customer)),
};

static string Shipping(Order order) => order switch
{
    { Total: > 100, IsPriority: true } => "Express",
    { Total: > 100 } => "Standard",
    { IsPriority: true } => "Priority",
    _ => "Economy",
};

static string Classify(int n) => n switch
{
    < 0 => "negative",
    0 => "zero",
    >= 1 and <= 9 => "digit",
    _ => "large",
};

public record PremiumCustomer(decimal PurchaseTotal);
public record RegularCustomer(decimal PurchaseTotal, int YearsActive);
public record Order(decimal Total, bool IsPriority);
```

##### New in C# 10: Extended property patterns

```csharp
var order = new Order(new Customer("ann", new Address("DE")), 120m);

// C# 10: dotted member access inside a property pattern
if (order is { Customer.Address.Country: "DE", Total: > 100 })
    Console.WriteLine("German order over 100");

// Equivalent C# 9 form
if (order is { Customer: { Address: { Country: "DE" } } })
    Console.WriteLine("same");

public record Address(string Country);
public record Customer(string Name, Address Address);
public record Order(Customer Customer, decimal Total);
```

Extended property patterns also test that each intermediate member is not null. `{ Customer.Address.Country: "DE" }` is false, without a `NullReferenceException`, when `Customer` or `Address` is null.

##### Review rules

- **Exhaustiveness**: a `switch` expression without `_` that misses a case produces warning CS8509 and throws `SwitchExpressionException` at runtime. Treat CS8509 as an error. For enums, include a `_ => throw new ArgumentOutOfRangeException(...)` arm, because any integer can be cast to an enum.
- **Order matters**: an arm that an earlier arm fully subsumes is error CS8510, but `when` guards hide subsumption from the compiler. Put specific arms first.
- **`is not null`** never calls a user-defined `==`. `!= null` might.
- **Constant patterns compare by value for strings**, ordinally and case-sensitively.
- **List patterns (`[1, .., 3]`) and `Span<char>` matched against string constants are C# 11.** Use `Length` and indexers instead.

```csharp
int[] numbers = { 1, 2, 3 };
// C# 11 would be: numbers is [1, .., 3]
bool startsAndEnds = numbers.Length >= 2 && numbers[0] == 1 && numbers[^1] == 3;
Console.WriteLine(startsAndEnds);   // True
```

***

#### Exceptions

```csharp
using System.Runtime.ExceptionServices;

try { Load("missing.json"); }
catch (ConfigurationLoadException e) when (e.InnerException is FileNotFoundException)
{
    Console.WriteLine($"{e.Message} <- {e.InnerException.GetType().Name}");
}

static void Load(string path)
{
    try
    {
        _ = File.ReadAllText(path);
    }
    catch (IOException ex)
    {
        // GOOD: wrap with context and keep the original as InnerException
        throw new ConfigurationLoadException($"Cannot load configuration from '{path}'.", ex);
    }
}

static void Rethrow(Exception captured)
{
    // Rethrow a captured exception (e.g. from another thread) without losing its stack trace
    ExceptionDispatchInfo.Capture(captured).Throw();
}

public sealed class ConfigurationLoadException : Exception
{
    public ConfigurationLoadException(string message, Exception inner) : base(message, inner) { }
}
```

Rules:

- **`throw;` not `throw ex;`.** `throw ex;` resets the stack trace (CA2200).
- **Catch specific exceptions.** `catch (Exception)` is acceptable only at process, request or message boundaries that log and translate. Never swallow silently (`catch { }`).
- **Exception filters (`when`)** run before the stack unwinds. Use them for conditional handling and for logging without catching (`when (Log(ex))` returning `false`).
- **Do not use exceptions for control flow.** Use `TryParse`/`TryGetValue` patterns.
- **`OperationCanceledException`** is the normal result of cancellation. Do not log it as an error when the caller's token is cancelled. Filter it: `catch (OperationCanceledException) when (ct.IsCancellationRequested)`.
- **Custom exceptions**: end the name with `Exception`, provide the `(string, Exception)` constructor, and do not add `[Serializable]` plus a serialization constructor for `BinaryFormatter` (obsolete, SYSLIB0011).
- **Do not throw `Exception`, `SystemException`, `NullReferenceException` or `IndexOutOfRangeException`** yourself (CA2201).
- **`Environment.FailFast`** only for corrupted state, never for validation.

***

#### Dates, Times and Numbers

##### `DateOnly` and `TimeOnly` (New in .NET 6)

```csharp
var birthday = new DateOnly(2024, 2, 29);
Console.WriteLine(birthday.AddYears(1).ToString("yyyy-MM-dd"));       // 2025-02-28
Console.WriteLine(birthday.ToDateTime(TimeOnly.MinValue).Kind);         // Unspecified

var late = new TimeOnly(23, 0);
var wrapped = late.Add(TimeSpan.FromHours(2), out int wrappedDays);
Console.WriteLine($"{wrapped:HH:mm} +{wrappedDays} day(s)");           // 01:00 +1 day(s)

try { TimeOnly.FromTimeSpan(TimeSpan.FromHours(25)); }
catch (ArgumentOutOfRangeException) { Console.WriteLine("TimeOnly is a time of day, not a duration"); }
```

.NET 6 gaps a reviewer must know:

- **System.Text.Json in .NET 6 cannot serialize `DateOnly`/`TimeOnly`.** It throws `NotSupportedException: Serialization and deserialization of 'System.DateOnly' instances are not supported`. Support arrived in .NET 7. Use a custom `JsonConverter<DateOnly>` (see System.Text.Json) or `DateTime`.
- **ASP.NET Core 6 minimal APIs bind `DateOnly`/`TimeOnly` from route and query values** because the types have `TryParse` (verified: `/d/2024-03-01?at=13:30` binds). A JSON body with a `DateOnly` member hits the serializer limitation above, and the request fails with **500** unless you register a converter.
- **EF Core 6 SQL Server does not map `DateOnly`/`TimeOnly`.** The SQL Server provider added them in EF Core 8. The SQLite provider in EF Core 6 handles them. Check the provider before accepting `DateOnly` entity properties.

##### `DateTime` vs `DateTimeOffset`

- Store and transmit instants as **UTC** (`DateTimeOffset.UtcNow` or `DateTime.UtcNow`). `DateTime.Now` in server code is a finding unless it is used for local display.
- `DateTime.Kind` is easy to lose: `Unspecified` values are treated as local by `ToUniversalTime()`. Prefer `DateTimeOffset` for timestamps.
- **There is no `TimeProvider` (.NET 8).** For testable time, inject an interface:

```csharp
var clock = new FixedClock(new DateTimeOffset(2024, 1, 1, 12, 0, 0, TimeSpan.Zero));
var token = new Token(clock.UtcNow.AddMinutes(5));
Console.WriteLine(token.IsExpired(clock));                    // False

public interface IClock { DateTimeOffset UtcNow { get; } }
public sealed class SystemClock : IClock { public DateTimeOffset UtcNow => DateTimeOffset.UtcNow; }
public sealed class FixedClock : IClock
{
    public FixedClock(DateTimeOffset now) => UtcNow = now;
    public DateTimeOffset UtcNow { get; }
}
public sealed record Token(DateTimeOffset ExpiresAt)
{
    public bool IsExpired(IClock clock) => clock.UtcNow >= ExpiresAt;
}
```

- **Time zones**: since .NET 6, `TimeZoneInfo.FindSystemTimeZoneById` accepts both IANA and Windows IDs on all platforms when ICU is available. `TimeZoneInfo.TryConvertIanaIdToWindowsId` and `TryConvertWindowsIdToIanaId` are new. Verified on Linux: `"Europe/Berlin"` → `"W. Europe Standard Time"`.

##### Numbers

```csharp
decimal price = 0.1m + 0.2m;
Console.WriteLine(price == 0.3m);              // True: decimal for money
Console.WriteLine(0.1 + 0.2 == 0.3);           // False: binary floating point

int big = int.MaxValue;
try { checked { big++; } }
catch (OverflowException) { Console.WriteLine("overflow detected"); }
unchecked { big++; }                           // silently wraps (default context)
Console.WriteLine(big);                        // -2147483648

Console.WriteLine(Math.Round(2.5));                                  // 2 (banker's rounding)
Console.WriteLine(Math.Round(2.5, MidpointRounding.AwayFromZero));   // 3
Console.WriteLine(int.TryParse("12a", out var n) ? n : -1);          // -1
```

- Money is `decimal`, never `double`/`float`.
- Arithmetic is unchecked by default. Use `checked` for counters, sizes and financial math where overflow is a bug, or enable `<CheckForOverflowUnderflow>` in tests.
- `Math.Round` uses banker's rounding by default. Specify `MidpointRounding` for business rules.
- Parse user input with `TryParse` and an explicit `CultureInfo`/`NumberStyles`. `int.Parse` throws.
- **Generic math (`INumber<T>`) is .NET 7.** In .NET 6, write overloads or use `dynamic`-free helpers per type.

##### Enums

```csharp
var status = (OrderStatus)42;                       // compiles: any int can be cast
Console.WriteLine(Enum.IsDefined(status));          // False — generic overload, .NET 5+
Console.WriteLine(Enum.TryParse<OrderStatus>("shipped", ignoreCase: true, out var s) ? s : OrderStatus.Unknown);   // Shipped
Console.WriteLine(Enum.TryParse<OrderStatus>("42", out var numeric) + " " + numeric);   // True 42 — numeric strings parse!

var perms = Permissions.Read | Permissions.Write;
Console.WriteLine(perms.HasFlag(Permissions.Write));   // True

public enum OrderStatus { Unknown = 0, Pending = 1, Shipped = 2 }

[Flags]
public enum Permissions { None = 0, Read = 1, Write = 2, Delete = 4 }
```

- `Enum.TryParse` accepts **any numeric string** and returns an undefined value. Follow it with `Enum.IsDefined` when parsing untrusted input.
- Define a `0` member (`None`/`Unknown`), because `default(TEnum)` is 0.
- Use `[Flags]` with powers of two for combinable values only.

***

#### Async/Await

##### Basic rules

```csharp
var repo = new UserRepository();
var user = await GetUserAsync(repo, 1, CancellationToken.None);
Console.WriteLine(user.Name);

// Always await I/O; propagate the CancellationToken; suffix with Async
static async Task<User> GetUserAsync(UserRepository repo, int id, CancellationToken ct)
{
    var user = await repo.FindAsync(id, ct);
    return user ?? throw new KeyNotFoundException($"User {id} not found");
}

public sealed record User(int Id, string Name);
public sealed class UserRepository
{
    public async Task<User?> FindAsync(int id, CancellationToken ct)
    {
        await Task.Delay(10, ct);
        return id == 1 ? new User(1, "ann") : null;
    }
}
```

House rules that remain correct for C# 10 / .NET 6:

- **Don't block on async code.** `task.Result`, `task.Wait()` and `GetAwaiter().GetResult()` cause thread-pool starvation under load and deadlocks under a synchronization context (WPF, WinForms, legacy ASP.NET). Use `await` all the way up. `static async Task Main` and top-level `await` make this possible in console apps.
- **Avoid `async void`** except for event handlers. Exceptions from `async void` go to the synchronization context or thread pool and **crash the process**. Event handlers must catch everything.
- **`ConfigureAwait(false)` in library code** that does not need the caller's context. In ASP.NET Core application code there is no synchronization context, so it is noise there. `ConfigureAwaitOptions` (.NET 8) does not exist.
- **Return the task directly** (`Task<T> M() => _inner.MAsync();`) only when there is no `using`, `try`, or other code after the call. Otherwise `await` it, or the `using` disposes before the operation finishes and exceptions skip the `try`.
- **Never `Task.Run` in ASP.NET Core request code** to "make it async". It only moves work to another pool thread. Use it to offload CPU-bound work from UI threads.
- **Don't use `Task.Factory.StartNew` with an async lambda.** It returns `Task<Task>`, and awaiting the outer task does not wait for the inner one. Use `Task.Run`.

```csharp
var form = new Form();
form.Raise();
await Task.Delay(100);
Console.WriteLine(form.Status);                      // handled: boom

public sealed class Form
{
    public event EventHandler? Click;
    public string Status { get; private set; } = "";
    public Form() => Click += Button_Click;
    public void Raise() => Click?.Invoke(this, EventArgs.Empty);

    private async void Button_Click(object? sender, EventArgs e)   // event handler: the one allowed async void
    {
        try
        {
            await ProcessOrderAsync();
        }
        catch (Exception ex)                         // must catch: an escaping exception crashes the process
        {
            Status = $"handled: {ex.Message}";
        }
    }

    private static async Task ProcessOrderAsync() { await Task.Yield(); throw new InvalidOperationException("boom"); }
}
```

##### Returning tasks: `using`/`try` pitfall

```csharp
// BAD: the stream is disposed when ReadBad returns — before ReadToEndAsync completes
static Task<string> ReadBad(string path)
{
    using var reader = new StreamReader(path);
    return reader.ReadToEndAsync();
}

// GOOD
static async Task<string> ReadGood(string path)
{
    using var reader = new StreamReader(path);
    return await reader.ReadToEndAsync();
}

var file = Path.GetTempFileName();
await File.WriteAllTextAsync(file, new string('x', 1_000_000));
Console.WriteLine((await ReadGood(file)).Length);    // 1000000
try { Console.WriteLine((await ReadBad(file)).Length); }
catch (ObjectDisposedException) { Console.WriteLine("ReadBad: ObjectDisposedException"); }
File.Delete(file);
```

The example prints `1000000` and then `ReadBad: ObjectDisposedException` on .NET 6.0.36. For small files the bad version can appear to work, which is why it survives code review.

##### Parallel async operations and `Task.WhenAll`

```csharp
var users = LoadAsync("users", 30);
var orders = LoadAsync("orders", 10);
await Task.WhenAll(users, orders);                   // run concurrently, then read results
Console.WriteLine($"{await users} {await orders}");

// await rethrows only the FIRST exception; inspect Task.Exception for all of them
var all = Task.WhenAll(Fail("one"), Fail("two"));
try { await all; }
catch (Exception ex)
{
    Console.WriteLine($"caught {ex.Message}; total {all.Exception!.InnerExceptions.Count}");   // caught one; total 2
}

static async Task<string> LoadAsync(string what, int ms) { await Task.Delay(ms); return what; }
static async Task Fail(string msg) { await Task.Yield(); throw new InvalidOperationException(msg); }
```

- Start independent operations first, then `await Task.WhenAll`. Awaiting them one by one serializes them.
- **Do not run independent EF Core queries in parallel on one `DbContext`** (see EF Core).
- `Task.WhenEach` (.NET 9) does not exist. To process results as they complete, loop over `Task.WhenAny` and remove each finished task, or use a `Channel<T>`.

##### Throttling: `Parallel.ForEachAsync` (New in .NET 6) and `SemaphoreSlim`

```csharp
using System.Collections.Concurrent;

var results = new ConcurrentBag<int>();
await Parallel.ForEachAsync(
    Enumerable.Range(1, 20),
    new ParallelOptions { MaxDegreeOfParallelism = 4, CancellationToken = CancellationToken.None },
    async (id, ct) =>
    {
        await Task.Delay(5, ct);                    // simulate I/O
        results.Add(id);
    });
Console.WriteLine(results.Count);                   // 20

// SemaphoreSlim: throttle inside code that already has a list of tasks
using var gate = new SemaphoreSlim(3);
var tasks = Enumerable.Range(1, 10).Select(async i =>
{
    await gate.WaitAsync();
    try { await Task.Delay(5); return i; }
    finally { gate.Release(); }
});
Console.WriteLine((await Task.WhenAll(tasks)).Sum());   // 55
```

- `Parallel.ForEachAsync` without `ParallelOptions` uses `MaxDegreeOfParallelism = Environment.ProcessorCount`. On a 4-CPU machine the measured maximum concurrency was 4. For I/O-bound work against a rate-limited API, set the limit explicitly.
- The body receives a `CancellationToken` that is cancelled when another iteration fails. Pass it on.
- `Parallel.ForEach` (synchronous) with an `async` lambda is a bug. The lambda becomes `async void`, and the loop returns immediately.
- `Parallel.ForAsync` (.NET 8) does not exist. Use `Parallel.ForEachAsync(Enumerable.Range(...), …)`.

##### Timeouts and cancellation

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

// Task.WaitAsync (.NET 6): stop WAITING; the underlying operation keeps running
try { await Task.Delay(1000).WaitAsync(TimeSpan.FromMilliseconds(20)); }
catch (TimeoutException) { Console.WriteLine("WaitAsync timeout"); }

// Linked token: caller cancellation OR our own timeout
static async Task CallWithTimeoutAsync(CancellationToken callerToken)
{
    using var linked = CancellationTokenSource.CreateLinkedTokenSource(callerToken);
    linked.CancelAfter(TimeSpan.FromSeconds(5));
    await Task.Delay(1, linked.Token);
}
await CallWithTimeoutAsync(CancellationToken.None);
Console.WriteLine("linked ok");

static async Task SlowAsync(CancellationToken ct) => await Task.Delay(TimeSpan.FromSeconds(10), ct);
```

Rules:

- **Every async API that does I/O takes a `CancellationToken`**, as the last parameter, defaulting to `default`. Analyzer CA2016 flags tokens that are not forwarded. In ASP.NET Core, accept `CancellationToken` in the endpoint or action (bound to `HttpContext.RequestAborted`).
- **`WaitAsync` does not cancel the operation.** It only stops waiting. Use it for operations that cannot accept a token, and be aware that they keep consuming resources.
- **Dispose `CancellationTokenSource`s that you create**, especially with `CancelAfter` or linked sources, which register callbacks and timers.
- `CancellationTokenSource.TryReset()` (.NET 6) lets a pooled source be reused only if it has not been cancelled. `CancelAsync` (.NET 8) does not exist.
- After cancellation, `OperationCanceledException` (or `TaskCanceledException`) is expected. `HttpClient` throws `TaskCanceledException` on its own `Timeout` too. In .NET 6 its `InnerException` is a `TimeoutException` when the client timeout fired.

##### `IAsyncEnumerable<T>` and cancellation

```csharp
using System.Runtime.CompilerServices;

using var cts = new CancellationTokenSource();
await foreach (var n in CountAsync().WithCancellation(cts.Token))
{
    Console.WriteLine(n);
    if (n == 2) cts.Cancel();      // the iterator observes the token through [EnumeratorCancellation]
}

static async IAsyncEnumerable<int> CountAsync([EnumeratorCancellation] CancellationToken ct = default)
{
    for (var i = 1; ; i++)
    {
        if (ct.IsCancellationRequested) yield break;
        await Task.Delay(10, CancellationToken.None);
        yield return i;
    }
}
```

- Without `[EnumeratorCancellation]`, a token passed through `WithCancellation` is **silently ignored** by the iterator.
- `await foreach` disposes the enumerator. Breaking out early runs the iterator's `finally` blocks.
- ASP.NET Core 6 and System.Text.Json 6 stream `IAsyncEnumerable<T>` responses, and `JsonSerializer.DeserializeAsyncEnumerable` reads JSON arrays incrementally.

##### `ValueTask` rules

```csharp
var cache = new PriceCache();
Console.WriteLine(await cache.GetAsync("a"));   // 1 (computed)
Console.WriteLine(await cache.GetAsync("a"));   // 1 (cached, no allocation)

public sealed class PriceCache
{
    private readonly Dictionary<string, int> _cache = new();

    public ValueTask<int> GetAsync(string key) =>
        _cache.TryGetValue(key, out var v) ? new ValueTask<int>(v) : new ValueTask<int>(LoadAsync(key));

    private async Task<int> LoadAsync(string key)
    {
        await Task.Delay(1);
        return _cache[key] = 1;
    }
}
```

- Use `ValueTask<T>` only when the synchronous path is hot and measured.
- **Await a `ValueTask` exactly once.** Never await it twice, never await it concurrently, and never call `.Result` before it completes. Call `.AsTask()` if you need any of those (CA2012).

##### New in C# 10: Per-method `[AsyncMethodBuilder]`

```csharp
using System.Runtime.CompilerServices;

for (var i = 0; i < 3; i++) Console.WriteLine(await ComputeAsync(i));

[AsyncMethodBuilder(typeof(PoolingAsyncValueTaskMethodBuilder<>))]   // .NET 6: pool the state machine boxes
static async ValueTask<int> ComputeAsync(int x)
{
    await Task.Yield();
    return x * 2;
}
```

`PoolingAsyncValueTaskMethodBuilder` and `PoolingAsyncValueTaskMethodBuilder<T>` are new in .NET 6. Pooling makes the "await exactly once" rule *critical*: awaiting a pooled `ValueTask` twice can observe another operation's result. Apply it only to hot paths that a profiler identified.

##### `TaskCompletionSource`

```csharp
var tcs = new TaskCompletionSource<int>(TaskCreationOptions.RunContinuationsAsynchronously);
_ = Task.Run(async () => { await Task.Delay(10); tcs.TrySetResult(7); });
Console.WriteLine(await tcs.Task);     // 7
```

Always pass `TaskCreationOptions.RunContinuationsAsynchronously`. Without it, the awaiting continuation runs synchronously inside `SetResult`, on the completing thread, while that thread may hold locks. Use `TrySet*` when several parties may complete the task. The non-generic `TaskCompletionSource` exists since .NET 5.

##### `PeriodicTimer` (New in .NET 6)

```csharp
using var timer = new PeriodicTimer(TimeSpan.FromMilliseconds(20));
using var cts = new CancellationTokenSource(TimeSpan.FromMilliseconds(110));
var ticks = 0;
try
{
    while (await timer.WaitForNextTickAsync(cts.Token))
    {
        ticks++;              // work runs on the awaiting flow; ticks never overlap
    }
}
catch (OperationCanceledException) { }
Console.WriteLine(ticks > 0);   // True
```

`PeriodicTimer` is the preferred loop for background polling in `BackgroundService`. Ticks never overlap, unlike `System.Threading.Timer` callbacks, which can run concurrently when a callback is slower than the period. Only one `WaitForNextTickAsync` may be outstanding at a time. Disposing the timer makes the next wait return `false`.

##### `lock` and `await`

```csharp
var account = new Account();
await Task.WhenAll(Enumerable.Range(0, 100).Select(_ => account.DepositAsync(1)));
Console.WriteLine(account.Balance);     // 100

public sealed class Account
{
    private readonly SemaphoreSlim _mutex = new(1, 1);    // async-compatible mutual exclusion
    public decimal Balance { get; private set; }

    public async Task DepositAsync(decimal amount)
    {
        await _mutex.WaitAsync();
        try
        {
            await Task.Yield();                           // await inside the critical section is fine here
            Balance += amount;
        }
        finally { _mutex.Release(); }
    }
}
```

- `await` inside a `lock` block is a compile error (CS1996). Use `SemaphoreSlim(1, 1)` for async critical sections.
- `System.Threading.Lock` (.NET 9 / C# 13) does not exist. Lock on a `private readonly object _gate = new();`, never on `this`, a `Type` or a string.

##### `AsyncLocal<T>` and `ExecutionContext`

`AsyncLocal<T>` values flow *into* awaited calls and child tasks, but changes made inside an awaited method do not flow back to the caller. Do not use `[ThreadStatic]` in async code: the continuation may resume on another thread.

***

#### Concurrency and Thread Safety

```csharp
using System.Collections.Concurrent;

// ConcurrentDictionary.GetOrAdd: the factory may run MORE THAN ONCE under contention
var cache = new ConcurrentDictionary<string, Lazy<ExpensiveClient>>();
ExpensiveClient Get(string key) =>
    cache.GetOrAdd(key, k => new Lazy<ExpensiveClient>(() => new ExpensiveClient(k))).Value;  // Lazy: created once
var clients = await Task.WhenAll(Enumerable.Range(0, 8).Select(_ => Task.Run(() => Get("a"))));
Console.WriteLine($"instances created: {ExpensiveClient.Created}");     // 1

// Random.Shared (.NET 6) is thread-safe; a shared `new Random()` is not
var n = Random.Shared.Next(1, 7);
Console.WriteLine(n is >= 1 and <= 6);

// Interlocked for counters
var counter = 0;
Parallel.For(0, 1000, _ => Interlocked.Increment(ref counter));
Console.WriteLine(counter);                                               // 1000

public sealed class ExpensiveClient
{
    public static int Created;
    public ExpensiveClient(string key) { Interlocked.Increment(ref Created); Thread.Sleep(20); }
}
```

- `GetOrAdd(key, factory)` runs the factory outside the lock. In one run with 8 contending threads, the plain factory ran **5 times** for one key. Wrap expensive or side-effecting creations in `Lazy<T>`.
- `ConcurrentDictionary` enumeration is not a snapshot, and `Count`/`IsEmpty` take all locks. Avoid them on hot paths.
- `Dictionary<,>`, `List<>` and `HashSet<>` are not thread-safe even for "mostly reads" with occasional writes. Concurrent writes can corrupt them, including infinite loops in `Dictionary`.
- **`new Random()` shared across threads** can return 0 forever after concurrent use corrupts it. Use `Random.Shared` (.NET 6) for non-security randomness and `RandomNumberGenerator` for security.
- `volatile` does not make compound operations atomic. Use `Interlocked` or a lock.
- **Immutable snapshots** (`ImmutableArray`, `ImmutableDictionary`, swapping a reference with `Interlocked.Exchange`) are often simpler than locking.

##### Channels for producer/consumer

```csharp
using System.Threading.Channels;

var channel = Channel.CreateBounded<int>(new BoundedChannelOptions(capacity: 10)
{
    FullMode = BoundedChannelFullMode.Wait,     // backpressure instead of unbounded memory growth
    SingleReader = true,
});

var producer = Task.Run(async () =>
{
    for (var i = 1; i <= 5; i++) await channel.Writer.WriteAsync(i);
    channel.Writer.Complete();                  // always complete, or the reader waits forever
});

var sum = 0;
await foreach (var item in channel.Reader.ReadAllAsync()) sum += item;
await producer;
Console.WriteLine(sum);                         // 15
```

Prefer `CreateBounded` for anything fed by external input. `CreateUnbounded` is a memory leak waiting for a slow consumer.

***

#### Collections and LINQ

##### Query syntax vs method syntax

Prefer method syntax (`users.Where(u => u.IsActive).OrderBy(u => u.Name)`). Query syntax (`from … join … group … into g select …`) is acceptable where it reads better, typically for joins and grouping. Both compile to the same calls.

##### LINQ best practices

```csharp
IEnumerable<int> Numbers() { Console.Write("[enumerated]"); yield return 1; yield return 2; }

// For a lazy IEnumerable<T>, Any() stops at the first element; Count() enumerates everything
Console.WriteLine(Numbers().Any());                       // [enumerated]True

// When the type exposes Count/Length (List<T>, arrays), use the property
var list = new List<int> { 1, 2, 3 };
Console.WriteLine(list.Count);

// Avoid multiple enumeration: materialize once
var items = Numbers();
var itemList = items.ToList();                            // enumerates once
if (itemList.Count > 0) foreach (var i in itemList) Console.Write(i);
Console.WriteLine();

// TryGetNonEnumeratedCount (.NET 6): count only if it is free
Console.WriteLine(Enumerable.Range(0, 5).TryGetNonEnumeratedCount(out var c1) + $" {c1}");               // True 5
Console.WriteLine(Enumerable.Range(0, 5).Where(x => x > 1).TryGetNonEnumeratedCount(out var c2) + $" {c2}");   // False 0

// FirstOrDefault when absence is expected; First/Single when absence is a bug
var firstEven = list.FirstOrDefault(x => x % 2 == 0);
var none = list.FirstOrDefault(x => x > 10, -1);         // .NET 6: explicit default value
Console.WriteLine($"{firstEven} {none}");                 // 2 -1
```

- **Deferred execution**: a query runs each time it is enumerated, and it sees the variables it captured *at enumeration time*. Materialize with `ToList()`/`ToArray()` before the source changes, before the `DbContext` is disposed, or before returning from a method that owns resources.
- **Multiple enumeration** of an `IEnumerable<T>` parameter repeats the work: database queries, file reads, iterator side effects. Accept `IReadOnlyCollection<T>`/`IReadOnlyList<T>` when you need the count or several passes. Analyzer CA1851 does not exist in the .NET 6 SDK, so reviewers must catch this by eye.
- **`Single` vs `First`**: `Single` asserts uniqueness and scans for a second match. Use it when duplicates mean corruption.
- **Don't use `Count() > 0`** on lazy sequences. Use `Any()`. On `List<T>` and arrays use `Count`/`Length` (CA1827/CA1829).
- **Project early** (`Select` only what you need), especially with EF Core, where it shrinks the SQL `SELECT` list.
- **`OrderBy` is a stable sort; `List<T>.Sort` and `Array.Sort` are unstable.** Equal keys may be reordered by `Sort`.
- `ToDictionary` throws `ArgumentException` on duplicate keys. Use `GroupBy` or `ToLookup` when keys may repeat.
- Modifying a `List<T>`/`Dictionary<,>` while enumerating it throws `InvalidOperationException`. Iterate over a copy (`ToList()`) or collect the changes first.

##### New in .NET 6: LINQ additions

```csharp
var people = new[] { ("ann", 31), ("bob", 25), ("cy", 31) };

Console.WriteLine(people.MaxBy(p => p.Item2));                              // (ann, 31) — first max wins
Console.WriteLine(people.MinBy(p => p.Item2));                              // (bob, 25)
Console.WriteLine(string.Join(" ", people.DistinctBy(p => p.Item2)));       // (ann, 31) (bob, 25)
Console.WriteLine(string.Join(" ", new[] { 1, 2 }.UnionBy(new[] { 2, 3 }, x => x)));   // 1 2 3
Console.WriteLine(string.Join(" ", Enumerable.Range(1, 7).Chunk(3).Select(c => c.Length)));   // 3 3 1
Console.WriteLine(new[] { 1, 2, 3, 4 }.Take(^2..).Sum());                   // 7 — Take(Range)
Console.WriteLine(new[] { 1, 2, 3 }.ElementAt(^1));                         // 3 — ElementAt(Index)
foreach (var (n, s, b) in new[] { 1, 2 }.Zip(new[] { "a", "b" }, new[] { true, false }))
    Console.Write($"{n}{s}{b} ");                                           // 1aTrue 2bFalse
Console.WriteLine();

// Empty-sequence behaviour of MaxBy/MinBy differs by element type
Console.WriteLine(Array.Empty<string>().MaxBy(s => s.Length) is null);      // True: reference type → null
try { Array.Empty<int>().MaxBy(i => i); }
catch (InvalidOperationException) { Console.WriteLine("value type → throws"); }
```

`Order()`/`OrderDescending()` (.NET 7), and `CountBy`/`AggregateBy`/`Index()` (.NET 9) are **not** available. Use `OrderBy(x => x)`, `GroupBy(k).Select(g => (g.Key, g.Count()))`, and `Select((x, i) => (i, x))`.

##### `PriorityQueue<TElement, TPriority>` (New in .NET 6)

```csharp
var queue = new PriorityQueue<string, int>();
queue.Enqueue("low", 5);
queue.Enqueue("high", 1);
queue.Enqueue("mid", 3);
while (queue.TryDequeue(out var job, out var priority)) Console.Write($"{job}({priority}) ");
Console.WriteLine();                                      // high(1) mid(3) low(5)

// Equal priorities are NOT dequeued in insertion order
var fifo = new PriorityQueue<string, int>();
foreach (var s in new[] { "a", "b", "c", "d", "e", "f" }) fifo.Enqueue(s, 1);
var order = new List<string>();
while (fifo.TryDequeue(out var s, out _)) order.Add(s);
Console.WriteLine(string.Join("", order));                // afedcb on .NET 6.0.36

// Stable alternative: add a sequence number to the priority
var stable = new PriorityQueue<string, (int Priority, long Seq)>();
long seq = 0;
foreach (var s in new[] { "a", "b", "c" }) stable.Enqueue(s, (1, seq++));
while (stable.TryDequeue(out var s, out _)) Console.Write(s);
Console.WriteLine();                                      // abc
```

`PriorityQueue` is a min-heap. It is not thread-safe, it does not support updating priorities or removing arbitrary elements, and it does **not** preserve FIFO order for equal priorities.

##### Dictionaries

```csharp
using System.Runtime.InteropServices;

var stock = new Dictionary<string, int>(StringComparer.OrdinalIgnoreCase) { ["apple"] = 3 };

// BAD: two lookups
if (stock.ContainsKey("APPLE")) Console.WriteLine(stock["APPLE"]);
// GOOD: one lookup
if (stock.TryGetValue("apple", out var qty)) Console.WriteLine(qty);

// Add vs indexer vs TryAdd
stock["pear"] = 1;                         // insert or overwrite
Console.WriteLine(stock.TryAdd("pear", 9)); // False: existing value kept
try { stock.Add("pear", 2); } catch (ArgumentException) { Console.WriteLine("Add throws on duplicates"); }

// .NET 6: update-or-insert with a single hash lookup (hot paths only)
Increment(stock, "kiwi");
Increment(stock, "kiwi");
Console.WriteLine(stock["kiwi"]);          // 2

static void Increment(Dictionary<string, int> d, string key)
{
    ref var slot = ref CollectionsMarshal.GetValueRefOrAddDefault(d, key, out _);
    slot++;                                // the ref is invalid after any other mutation of the dictionary
}
```

- Specify the comparer (`StringComparer.Ordinal`/`OrdinalIgnoreCase`) for string keys.
- Mutable objects used as keys must not change their hash code while they are in the dictionary.
- `CollectionsMarshal.GetValueRefOrAddDefault` returns a `ref` that must not be kept across other mutations. Since `ref` locals are not allowed in `async` methods before C# 13 (CS8177), put the logic in a synchronous helper, as shown.
- `FrozenDictionary` (.NET 8) does not exist. For read-mostly lookups, build a `Dictionary` once and expose it as `IReadOnlyDictionary`, or use `ImmutableDictionary`, which is slower for lookups.

##### Collection types in APIs

- Return `IReadOnlyList<T>`/`IReadOnlyCollection<T>`, or concrete immutable types, from public APIs. Never return internal mutable lists that callers could cast and mutate.
- Accept the least specific type that you need: `IEnumerable<T>` for one pass, and `IReadOnlyCollection<T>` when you need the count.
- Return empty collections (`Array.Empty<T>()`), not `null`.
- Collection expressions (`[]`, C# 12) are not available. Use `Array.Empty<T>()` and `new List<T>()`.

***

#### `Span<T>`, `Memory<T>` and Low-Allocation Code

```csharp
using System.Buffers;

// Parse without allocating substrings
ReadOnlySpan<char> line = "id=42;name=ann";
var idPart = line[..line.IndexOf(';')];               // "id=42"
var id = int.Parse(idPart[(idPart.IndexOf('=') + 1)..]);
Console.WriteLine(id);                                 // 42

// stackalloc for small, bounded buffers only
Span<byte> small = stackalloc byte[16];
small.Fill(0xFF);
Console.WriteLine(small[0]);                           // 255

// ArrayPool for larger temporary buffers — always return, never use after returning
var pool = ArrayPool<byte>.Shared;
var buffer = pool.Rent(4096);                          // may be LARGER than requested
try
{
    var used = buffer.AsSpan(0, 100);                  // use only the part you asked for
    used.Clear();
    Console.WriteLine(buffer.Length >= 4096);          // True
}
finally
{
    pool.Return(buffer);                               // add clearArray: true if it held secrets
}

// string.Create builds a string in place
var s = string.Create(5, 'x', (chars, c) => chars.Fill(c));
Console.WriteLine(s);                                  // xxxxx
```

Rules:

- **`Span<T>` and `ReadOnlySpan<T>` are `ref struct`s.** They cannot be fields of classes, cannot be boxed, cannot be captured by lambdas, and **cannot be used across `await` or `yield`** in C# 10. Use `Memory<T>`/`ReadOnlyMemory<T>` in async code, and call `.Span` inside synchronous helpers.
- **`stackalloc` with an input-dependent size** is a stack-overflow vulnerability. Bound it (`len <= 256 ? stackalloc byte[len] : pool.Rent(len)`).
- **`ArrayPool.Rent` returns a larger-than-requested array** that contains old data. Track the logical length, return the array exactly once, and never touch it after `Return`. Clear it if it held secrets.
- `MemoryMarshal`/`Unsafe` bypass safety. Each use needs a comment and a test.
- UTF-8 literals (`"abc"u8`, C# 11) are not available. Use `Encoding.UTF8.GetBytes` once into a `static readonly byte[]`, or a `ReadOnlySpan<byte>` property over a constant byte array (`new byte[] { … }`), which the compiler optimizes.
- `SearchValues<T>` (.NET 8) does not exist. Use `IndexOfAny` for a few values, or a precomputed `bool[]` or `HashSet<char>` lookup.

***

#### Resource Management: `IDisposable` and `IAsyncDisposable`

##### The dispose pattern

```csharp
using var manager = new ResourceManager(Path.GetTempFileName());
manager.DoWork();
Console.WriteLine("done");

public class ResourceManager : IDisposable
{
    private readonly FileStream _stream;
    private bool _disposed;

    public ResourceManager(string path) => _stream = File.OpenRead(path);

    public void DoWork()
    {
        // ObjectDisposedException.ThrowIf is .NET 7 — in .NET 6 throw explicitly
        if (_disposed) throw new ObjectDisposedException(GetType().FullName);
        _ = _stream.Length;
    }

    public void Dispose()
    {
        Dispose(true);
        GC.SuppressFinalize(this);
    }

    protected virtual void Dispose(bool disposing)
    {
        if (_disposed) return;
        if (disposing) _stream.Dispose();
        _disposed = true;
    }
}
```

- Use `using` declarations (`using var x = …;`, C# 8) or `using` blocks for every `IDisposable` you own. Analyzer CA2000 flags objects that are not disposed.
- **Do not dispose what you don't own**: injected services, `HttpClient`s from `IHttpClientFactory`, and streams passed to you unless documented otherwise.
- **Sealed classes without unmanaged resources** need only a simple `public void Dispose()`. Add a finalizer only when you directly own an unmanaged handle, and prefer `SafeHandle`.
- `Dispose` must be idempotent and must not throw.

##### `IAsyncDisposable` and `await using`

```csharp
await using (var writer = new AuditWriter())
{
    await writer.WriteAsync("order created");
}   // DisposeAsync awaited here

public sealed class AuditWriter : IAsyncDisposable
{
    private readonly MemoryStream _buffer = new();
    private readonly StreamWriter _writer;
    public AuditWriter() => _writer = new StreamWriter(_buffer);
    public Task WriteAsync(string line) => _writer.WriteLineAsync(line);
    public async ValueTask DisposeAsync()
    {
        await _writer.FlushAsync();                 // async flush instead of blocking in Dispose()
        Console.WriteLine($"flushed {_buffer.Length} bytes");
        await _writer.DisposeAsync();
    }
}
```

- A type that holds async resources (streams, DB connections, `DbContext`) should implement `IAsyncDisposable`. Callers must use `await using`. A plain `using` on such a type calls the synchronous `Dispose`, which may block or skip the async flush.
- Implement both `IDisposable` and `IAsyncDisposable` when callers may use either.

##### `Stream` reads can be partial (changed in .NET 6)

```csharp
using System.IO.Compression;

var data = new byte[200_000];
new Random(1).NextBytes(data.AsSpan(0, 100_000));
var compressed = new MemoryStream();
using (var gz = new GZipStream(compressed, CompressionLevel.Fastest, leaveOpen: true)) gz.Write(data);

compressed.Position = 0;
using var reader = new GZipStream(compressed, CompressionMode.Decompress);
var buffer = new byte[data.Length];

// BAD: assumes one Read fills the buffer
int once = reader.Read(buffer, 0, buffer.Length);
Console.WriteLine($"one Read: {once} of {buffer.Length}");     // one Read: 8177 of 200000

// GOOD: loop until the count is reached or the stream ends (Stream.ReadExactly is .NET 7)
int total = once;
while (total < buffer.Length)
{
    int n = reader.Read(buffer, total, buffer.Length - total);
    if (n == 0) throw new EndOfStreamException();
    total += n;
}
Console.WriteLine(total);                                       // 200000
```

.NET 6 changed `DeflateStream`, `GZipStream` and `CryptoStream` to return as soon as *some* data is available, instead of filling the buffer. Code that ignores the return value of `Read`/`ReadAsync` silently truncates data. `Stream.ReadExactly`/`ReadAtLeast` (.NET 7) do not exist, so write the loop.

***

#### Dependency Injection

##### Service registration and lifetimes

- `AddTransient`: a new instance per resolution. `AddScoped`: one per request or `CreateScope()`. `AddSingleton`: one for the application lifetime, so it must be thread-safe.
- Factory registrations (`services.AddScoped<IDbConnection>(sp => new SqliteConnection(...))`) are fine. The container owns and disposes `IDisposable` instances it creates, including factory results, when their scope ends. Code that resolves them should not dispose them.

##### Constructor injection

Inject dependencies through the constructor into `private readonly` fields, guard them with `?? throw new ArgumentNullException(nameof(x))` or `ArgumentNullException.ThrowIfNull`, and inject `ILogger<T>` rather than `ILoggerFactory`. Primary constructors (`class OrderService(IOrderRepository repository)`) are C# 12. In C# 10, write the constructor and the fields. A constructor with many parameters signals a class with too many responsibilities.

##### Lifetime bugs a reviewer must catch

```csharp
using Microsoft.Extensions.DependencyInjection;
using Microsoft.Extensions.Hosting;
using Microsoft.Extensions.Logging;

// A singleton that captures a scoped service ("captive dependency")
var host = Host.CreateDefaultBuilder()
    .ConfigureLogging(l => l.ClearProviders())
    .ConfigureServices(s => { s.AddScoped<AppDbContext>(); s.AddSingleton<PriceCache>(); })
    .Build();
var a = host.Services.GetRequiredService<PriceCache>();
var b = host.Services.GetRequiredService<PriceCache>();
Console.WriteLine(ReferenceEquals(a.Db, b.Db));   // True in Production: one "scoped" DbContext shared forever

public sealed class AppDbContext { }
public sealed class PriceCache
{
    public PriceCache(AppDbContext db) => Db = db;     // BAD: scoped dependency captured by a singleton
    public AppDbContext Db { get; }
}
```

- **Captive dependencies**: a singleton that depends on a scoped or transient service keeps one instance forever. With a `DbContext` that means cross-request data leaks and threading exceptions. `Host.CreateDefaultBuilder` and `WebApplication.CreateBuilder` enable `ValidateScopes` and `ValidateOnBuild` **only in the Development environment**. The example above builds fine with `DOTNET_ENVIRONMENT=Production` and throws `AggregateException: Some services are not able to be constructed` in Development (both verified). Enable validation in all environments in tests:

```csharp
using Microsoft.Extensions.DependencyInjection;

var services = new ServiceCollection();
services.AddScoped<Db>();
services.AddSingleton<Cache>();
try
{
    services.BuildServiceProvider(new ServiceProviderOptions { ValidateScopes = true, ValidateOnBuild = true });
}
catch (AggregateException e)
{
    Console.WriteLine(e.InnerExceptions[0].Message[..60]);
}

public sealed class Db { }
public sealed class Cache { public Cache(Db db) { } }
```

- **Consuming scoped services from singletons or `BackgroundService`**: inject `IServiceScopeFactory` and create a scope per unit of work (see Hosting).
- **Resolving from the root provider** (`app.Services.GetService<ScopedThing>()`) creates instances that live until shutdown. Always resolve scoped services inside `CreateScope()`.
- **Transient `IDisposable`s resolved from the root provider** are tracked and disposed only at shutdown, which is a memory leak in a loop.
- **Service locator** (`IServiceProvider` injected into business classes) hides dependencies. Allow it in factories and infrastructure only.
- `TryAdd*` in library `AddXyz()` extension methods lets the application override registrations. Plain `Add*` called twice registers twice, and the last one wins for single resolution.
- **Keyed services (`AddKeyedSingleton`, `[FromKeyedServices]`) are .NET 8.** In .NET 6, register a factory or a small resolver (`Func<string, IStorage>`) instead.

***

#### Options and Configuration

##### Options pattern with validation

```csharp
using System.ComponentModel.DataAnnotations;
using Microsoft.Extensions.Options;

var builder = WebApplication.CreateBuilder(new WebApplicationOptions { Args = args });
builder.Logging.ClearProviders();
builder.Configuration.AddInMemoryCollection(new Dictionary<string, string>
{
    ["Email:SmtpServer"] = "smtp.example.com",
    ["Email:Port"] = "70000",                                  // invalid
});

builder.Services.AddOptions<EmailSettings>()
    .Bind(builder.Configuration.GetSection(EmailSettings.SectionName))
    .ValidateDataAnnotations()
    .ValidateOnStart();                                         // .NET 6: fail at startup, not on first use

var app = builder.Build();
try
{
    await app.StartAsync();
}
catch (OptionsValidationException e)
{
    Console.WriteLine(e.Message);    // DataAnnotation validation failed for 'EmailSettings' members: 'Port' ...
}

public sealed class EmailSettings
{
    public const string SectionName = "Email";
    [Required] public string SmtpServer { get; set; } = "";
    [Range(1, 65535)] public int Port { get; set; } = 25;
    [Required, EmailAddress] public string FromAddress { get; set; } = "noreply@example.com";
}
```

- **`ValidateOnStart()` is new in .NET 6.** Without it, invalid configuration surfaces only when the options are first resolved, often in the middle of a request. `AddOptionsWithValidateOnStart<T>()` is .NET 8. Use `AddOptions<T>().Bind(...).ValidateDataAnnotations().ValidateOnStart()`.
- **`IOptions<T>`** is a singleton snapshot read once. **`IOptionsSnapshot<T>`** is scoped and re-read per request, and cannot be injected into singletons. **`IOptionsMonitor<T>`** is a singleton with change notifications. Choose based on whether reload-on-change is needed.
- `services.Configure<T>(section)` (the house-rule form) is fine when no validation is needed.
- Options classes bound from configuration need a public parameterless constructor and settable (`set` or `init`) properties. A positional record fails at bind time in .NET 6 with `InvalidOperationException: Cannot create instance of type '…' because it is missing a public parameterless constructor` (constructor binding arrived in .NET 7).

##### Configuration pitfalls

```csharp
using Microsoft.Extensions.Configuration;

var config = new ConfigurationBuilder()
    .AddInMemoryCollection(new Dictionary<string, string> { ["Cache:Size"] = "ten", ["Cache:Enabled"] = "true" })
    .Build();

// Missing section: Get<T>() returns null, not an empty object
Console.WriteLine(config.GetSection("Missing").Get<CacheSettings>() is null);    // True

// Key names are case-insensitive
Console.WriteLine(config["cache:enabled"]);                                       // true

// Invalid values throw at bind time
try { config.GetSection("Cache").Get<CacheSettings>(); }
catch (InvalidOperationException e) { Console.WriteLine(e.Message[..40]); }

public sealed class CacheSettings { public int Size { get; set; } public bool Enabled { get; set; } }
```

- **Environment variables** use `__` (double underscore) as the section separator: `Email__Port=587`. They override `appsettings.json` with the default builder order: appsettings, appsettings.{Environment}, user secrets (Development only), environment variables, command line.
- **Secrets never go in `appsettings*.json`** or source control. Use user secrets locally and environment variables, Key Vault or another secret store in production. Flag connection strings with passwords in committed files.
- Check `GetConnectionString` results for `null`, or validate them through options.
- `builder.Configuration` in .NET 6 minimal hosting is a `ConfigurationManager`: sources added later are visible immediately.

***

#### Logging

##### Message templates, not interpolation

```csharp
using Microsoft.Extensions.Logging;

using var factory = LoggerFactory.Create(b => b.AddSimpleConsole(o => o.SingleLine = true));
var logger = factory.CreateLogger("Orders");
var orderId = 42;

logger.LogInformation($"Processing order {orderId}");          // BAD: CA2254 — no structured property, always formats
logger.LogInformation("Processing order {OrderId}", orderId);   // GOOD: OrderId becomes a structured property

try { throw new InvalidOperationException("db down"); }
catch (Exception ex)
{
    logger.LogError(ex, "Failed to process order {OrderId}", orderId);   // exception is the FIRST argument
}

using (logger.BeginScope("Request {RequestId}", "r-1"))
{
    logger.LogWarning("Inside scope");
}
```

With `AnalysisMode` set to `All`, the .NET 6 SDK analyzers report CA2254 ("The logging message template should not vary between calls") for the interpolated call and CA1848 ("use the LoggerMessage delegates") for these extension-method calls.

##### High-performance logging: `[LoggerMessage]` source generator (New in .NET 6)

```csharp
using Microsoft.Extensions.Logging;

using var factory = LoggerFactory.Create(b => b.AddSimpleConsole(o => o.SingleLine = true));
var logger = factory.CreateLogger<Program>();
Log.OrderShipped(logger, 42, "DHL");

static partial class Log
{
    [LoggerMessage(EventId = 1001, Level = LogLevel.Information, Message = "Order {OrderId} shipped via {Carrier}")]
    public static partial void OrderShipped(ILogger logger, int orderId, string carrier);
}

public partial class Program { }
```

Use the generator on hot paths. It avoids boxing and template parsing, and it checks the level before formatting.

##### Logging rules

- **Never log secrets or personal data**: passwords, tokens, `Authorization` headers, full request bodies, connection strings. Records print all properties (see Records).
- **HTTP logging middleware** (`AddHttpLogging`/`UseHttpLogging`, new in ASP.NET Core 6) logs request and response headers and optionally bodies. Configure `RequestHeaders`/`ResponseHeaders` allow-lists and keep bodies off in production.
- Use `ILogger<T>` through DI. Do not create `LoggerFactory` instances per call: they own providers and threads.
- Choose levels deliberately: `Debug` for diagnostics, `Information` for business events, `Warning` for recoverable problems, `Error` for failed operations, `Critical` for process-level failures. Do not log-and-rethrow at every layer, because one exception becomes N log entries.
- Use stable template strings. Placeholder names are PascalCase and become property names in structured sinks.

***

#### HttpClient

##### Use `IHttpClientFactory` (typed or named clients)

```csharp
using System.Net.Http.Json;
using Microsoft.Extensions.DependencyInjection;

var services = new ServiceCollection();
services.AddHttpClient<GitHubClient>(c =>
{
    c.BaseAddress = new Uri("https://api.github.com/");
    c.Timeout = TimeSpan.FromSeconds(10);                     // default is 100 seconds
    c.DefaultRequestHeaders.UserAgent.ParseAdd("my-app/1.0");
});
using var provider = services.BuildServiceProvider();
var client = provider.GetRequiredService<GitHubClient>();
Console.WriteLine(client.BaseAddress);                        // https://api.github.com/

public sealed class GitHubClient
{
    private readonly HttpClient _http;
    public GitHubClient(HttpClient http) => _http = http;
    public Uri? BaseAddress => _http.BaseAddress;

    public async Task<Repo?> GetRepoAsync(string owner, string name, CancellationToken ct)
    {
        using var response = await _http.GetAsync($"repos/{Uri.EscapeDataString(owner)}/{Uri.EscapeDataString(name)}", ct);
        if (response.StatusCode == System.Net.HttpStatusCode.NotFound) return null;
        response.EnsureSuccessStatusCode();
        return await response.Content.ReadFromJsonAsync<Repo>(cancellationToken: ct);
    }
}

public sealed record Repo(string Name, int StargazersCount);
```

Rules:

- **Never `new HttpClient()` per request** (`using var client = new HttpClient()` in a method). Each instance owns a connection pool, so this leads to socket exhaustion (`TIME_WAIT`) under load.
- **A single static `HttpClient`** avoids exhaustion but never re-resolves DNS. Either use `IHttpClientFactory`, which rotates handlers every 2 minutes by default, or create the static client with `new SocketsHttpHandler { PooledConnectionLifetime = TimeSpan.FromMinutes(2) }`.
- **Typed clients are transient.** Injecting a typed client into a singleton captures one `HttpClient` and its handler forever, which loses DNS rotation. Inject typed clients only into transient or scoped services. In a singleton, inject `IHttpClientFactory` and call `CreateClient` per operation.
- **Dispose `HttpResponseMessage`** (`using var response = …`), especially with `HttpCompletionOption.ResponseHeadersRead`. Otherwise the connection is not returned to the pool.
- **Check the status**: `EnsureSuccessStatusCode()` or explicit handling. `GetFromJsonAsync` throws `HttpRequestException` on non-success codes.
- **Escape path segments** with `Uri.EscapeDataString`. Never build URLs from user input by concatenation (SSRF and path injection). `Uri.EscapeUriString` is obsolete in .NET 6 (SYSLIB0013).
- **Timeouts**: `HttpClient.Timeout` (default 100 s) applies per request including the body read. Combine it with the caller's `CancellationToken`. In .NET 6 a client timeout throws `TaskCanceledException` whose `InnerException` is a `TimeoutException` (verified).
- **Resilience**: `Microsoft.Extensions.Http.Polly` 6.0.x (`AddTransientHttpErrorPolicy`) works on .NET 6. `Microsoft.Extensions.Http.Resilience` 8.x (last 8.10.0) still targets `net6.0`, and 9.0+ dropped it. Retry only idempotent requests, and add jitter.
- `WebClient`, `WebRequest` and `HttpWebRequest` are **obsolete in .NET 6** (SYSLIB0014). Replace them with `HttpClient`.

```csharp
using System.Net.Http.Json;
using Microsoft.Extensions.DependencyInjection;
using Polly;
using Polly.Extensions.Http;

var services = new ServiceCollection();
services.AddHttpClient("payments", c => c.BaseAddress = new Uri("https://payments.example.com/"))
    .AddTransientHttpErrorPolicy(p => p.WaitAndRetryAsync(3, attempt =>
        TimeSpan.FromMilliseconds(200 * Math.Pow(2, attempt)) + TimeSpan.FromMilliseconds(Random.Shared.Next(0, 100))));
using var provider = services.BuildServiceProvider();
var factory = provider.GetRequiredService<IHttpClientFactory>();
Console.WriteLine(factory.CreateClient("payments").BaseAddress);   // https://payments.example.com/
```

***

#### System.Text.Json (.NET 6)

##### Defaults that surprise people

```csharp
using System.Text.Json;
using System.Text.Json.Serialization;

// 1. Property names are case-SENSITIVE by default
var p = JsonSerializer.Deserialize<Person>("{\"name\":\"ann\"}")!;
Console.WriteLine($"default: '{p.Name}'");                                         // default: ''
var web = new JsonSerializerOptions(JsonSerializerDefaults.Web);                     // camelCase + case-insensitive + numbers from strings
Console.WriteLine($"web: '{JsonSerializer.Deserialize<Person>("{\"name\":\"ann\"}", web)!.Name}'");   // web: 'ann'

// 2. Nullable annotations are NOT enforced: a missing non-nullable member becomes null
var r = JsonSerializer.Deserialize<UserDto>("{}")!;
Console.WriteLine(r.Email is null);                                                  // True — despite 'string Email'

// 3. Enums are numbers unless you add a converter
Console.WriteLine(JsonSerializer.Serialize(new { Status = Status.Shipped }));        // {"Status":1}
var enumOpts = new JsonSerializerOptions { Converters = { new JsonStringEnumConverter() } };
Console.WriteLine(JsonSerializer.Serialize(new { Status = Status.Shipped }, enumOpts));   // {"Status":"Shipped"}

// 4. Declared type wins: derived properties are dropped
Animal a = new Dog { Name = "rex", Barks = true };
Console.WriteLine(JsonSerializer.Serialize(a));                                      // {"Name":"rex"}
Console.WriteLine(JsonSerializer.Serialize<object>(a));                              // {"Barks":true,"Name":"rex"}

// 5. Fields are ignored; non-ASCII and HTML-sensitive characters are escaped
Console.WriteLine(JsonSerializer.Serialize(new WithField()));                        // {"Prop":2}
Console.WriteLine(JsonSerializer.Serialize("äö<>"));                                 // "\u00E4\u00F6\u003C\u003E" (escaped by default)

// 6. Cycles throw unless you opt in (.NET 6 adds IgnoreCycles)
var node = new Node(); node.Next = node;
try { JsonSerializer.Serialize(node); }
catch (JsonException) { Console.WriteLine("cycle → JsonException"); }
Console.WriteLine(JsonSerializer.Serialize(node, new JsonSerializerOptions { ReferenceHandler = ReferenceHandler.IgnoreCycles }));   // {"Next":null}

public sealed class Person { public string? Name { get; set; } }
public sealed record UserDto(string Name, string Email);
public enum Status { Pending, Shipped }
public class Animal { public string Name { get; set; } = ""; }
public sealed class Dog : Animal { public bool Barks { get; set; } }
public sealed class WithField { public int Field = 1; public int Prop { get; set; } = 2; }
public sealed class Node { public Node? Next { get; set; } }
```

Review rules:

- **Validate deserialized input.** Use data annotations, explicit checks or FluentValidation. Nullable annotations and constructor parameters do not guarantee presence. `[JsonRequired]` and C# 11 `required` are .NET 7 and are not available. `RespectNullableAnnotations` is .NET 9.
- **Reuse `JsonSerializerOptions` instances** (`static readonly`). Each new instance rebuilds its metadata cache, which is a large hidden cost when options are created per call.
- **Polymorphism**: `[JsonDerivedType]`/`[JsonPolymorphic]` are .NET 7. In .NET 6 write a custom `JsonConverter<TBase>` with an explicit discriminator. **Never** emulate Newtonsoft's `TypeNameHandling` by deserializing type names from input: that is a remote code execution vector.
- **ASP.NET Core uses `JsonSerializerDefaults.Web`**, but a bare `JsonSerializer.Deserialize` call uses the case-sensitive defaults. Mixed use explains "works in the controller, null in the background job" bugs.
- **`MaxDepth`** defaults to 64. Raise it only with a reason. Deeply nested untrusted JSON is a denial-of-service vector.
- Use `JsonSerializer.DeserializeAsync(stream)` and `DeserializeAsyncEnumerable` (.NET 6) for large payloads instead of reading the body into a string.

##### `DateOnly`/`TimeOnly` need a converter in .NET 6

```csharp
using System.Globalization;
using System.Text.Json;
using System.Text.Json.Serialization;

try { JsonSerializer.Serialize(new { Day = new DateOnly(2024, 1, 2) }); }
catch (NotSupportedException) { Console.WriteLine("DateOnly not supported by System.Text.Json 6"); }

var options = new JsonSerializerOptions { Converters = { new DateOnlyJsonConverter() } };
var json = JsonSerializer.Serialize(new Booking(new DateOnly(2024, 1, 2)), options);
Console.WriteLine(json);                                                   // {"Day":"2024-01-02"}
Console.WriteLine(JsonSerializer.Deserialize<Booking>(json, options));     // Booking { Day = 01/02/2024 }

public sealed record Booking(DateOnly Day);

public sealed class DateOnlyJsonConverter : JsonConverter<DateOnly>
{
    private const string Format = "yyyy-MM-dd";
    public override DateOnly Read(ref Utf8JsonReader reader, Type typeToConvert, JsonSerializerOptions options) =>
        DateOnly.ParseExact(reader.GetString()!, Format, CultureInfo.InvariantCulture);
    public override void Write(Utf8JsonWriter writer, DateOnly value, JsonSerializerOptions options) =>
        writer.WriteStringValue(value.ToString(Format, CultureInfo.InvariantCulture));
}
```

Register the converter for minimal APIs with `builder.Services.Configure<Microsoft.AspNetCore.Http.Json.JsonOptions>(o => o.SerializerOptions.Converters.Add(new DateOnlyJsonConverter()))`, and for controllers with `AddControllers().AddJsonOptions(...)`. `ConfigureHttpJsonOptions` is .NET 7. With the converter registered, a minimal API body `{"Day":"2024-03-01"}` binds and round-trips (verified).

##### Source generation (New in .NET 6)

```csharp
using System.Text.Json;
using System.Text.Json.Serialization;

var json = JsonSerializer.Serialize(new Product("Pen", 1.5m), AppJsonContext.Default.Product);
Console.WriteLine(json);                                             // {"name":"Pen","price":1.5}
var settable = JsonSerializer.Deserialize("{\"name\":\"Ink\"}", AppJsonContext.Default.Settings)!;
Console.WriteLine(settable.Name);                                    // Ink

try
{
    JsonSerializer.Deserialize("{\"name\":\"x\"}", AppJsonContext.Default.InitOnly);   // compiles (with a warning), fails at runtime
}
catch (InvalidOperationException e) { Console.WriteLine(e.Message); }
// Deserialization of init-only properties is currently not supported in source generation mode.

public sealed record Product(string Name, decimal Price);             // warning SYSLIB1037
public sealed class Settings { public string? Name { get; set; } }
public sealed class InitOnly { public string? Name { get; init; } }  // warning SYSLIB1037

[JsonSourceGenerationOptions(PropertyNamingPolicy = JsonKnownNamingPolicy.CamelCase)]
[JsonSerializable(typeof(Product))]
[JsonSerializable(typeof(Settings))]
[JsonSerializable(typeof(InitOnly))]
internal partial class AppJsonContext : JsonSerializerContext { }
```

- In .NET 6 the source generator **cannot deserialize `init`-only properties**. It emits warning **SYSLIB1037** at compile time and throws `InvalidOperationException` at runtime (verified on 6.0.36). Records' positional parameters deserialize through the constructor and worked in the test, but the warning is still reported for them. Do not ignore SYSLIB1037.
- A context's naming policy applies only when that context is used. Mixing the generated context and reflection-based calls with different options yields inconsistent JSON.
- The generator in .NET 6 does not support every feature of the reflection serializer. Run round-trip tests for each type you register.

##### `JsonNode` / `JsonDocument` (New in .NET 6: `System.Text.Json.Nodes`)

```csharp
using System.Text.Json;
using System.Text.Json.Nodes;

var node = JsonNode.Parse("{\"order\":{\"lines\":[{\"sku\":\"A\",\"qty\":2}]}}")!;
Console.WriteLine(node["order"]!["lines"]![0]!["qty"]!.GetValue<int>());   // 2
node["order"]!["status"] = "paid";                                          // mutable DOM
Console.WriteLine(node.ToJsonString());

using var doc = JsonDocument.Parse("{\"a\":1}");                             // read-only, pooled memory: dispose it
Console.WriteLine(doc.RootElement.GetProperty("a").GetInt32());              // 1
```

`JsonDocument` rents pooled buffers, so dispose it. A `JsonElement` kept after the document is disposed throws `ObjectDisposedException`. Use `JsonElement.Clone()` if you must keep one.

***

#### ASP.NET Core 6

##### Minimal hosting and minimal APIs (New in .NET 6)

```csharp
using System.ComponentModel.DataAnnotations;
using Microsoft.AspNetCore.Mvc;

var builder = WebApplication.CreateBuilder(args);
builder.Logging.ClearProviders();
builder.Services.AddSingleton<IUserStore, InMemoryUserStore>();

var app = builder.Build();
app.Urls.Add("http://127.0.0.1:0");

app.MapGet("/users/{id:int}", async (int id, IUserStore store, CancellationToken ct) =>
        await store.FindAsync(id, ct) is { } user ? Results.Ok(user) : Results.NotFound())
   .WithName("GetUser");

app.MapPost("/users", async ([FromBody] CreateUser request, IUserStore store, CancellationToken ct) =>
{
    // .NET 6 minimal APIs do NOT validate: there are no endpoint filters (7.0) and no automatic 400
    var errors = Validate(request);
    if (errors.Count > 0) return Results.ValidationProblem(errors);
    var user = await store.AddAsync(request.Name, ct);
    return Results.Created($"/users/{user.Id}", user);
});

await app.StartAsync();
using var http = new HttpClient { BaseAddress = new Uri(app.Urls.First()) };
Console.WriteLine((int)(await http.PostAsJsonAsync("/users", new { name = "" })).StatusCode);     // 400 (our check)
Console.WriteLine((int)(await http.PostAsJsonAsync("/users", new { name = "ann" })).StatusCode);  // 201
Console.WriteLine(await http.GetStringAsync("/users/1"));                                          // {"id":1,"name":"ann"}
Console.WriteLine((int)(await http.GetAsync("/users/abc")).StatusCode);                            // 404: route constraint
await app.StopAsync();

static Dictionary<string, string[]> Validate(CreateUser request)
{
    var results = new List<ValidationResult>();
    Validator.TryValidateObject(request, new ValidationContext(request), results, validateAllProperties: true);
    return results.ToDictionary(r => r.MemberNames.FirstOrDefault() ?? "", r => new[] { r.ErrorMessage ?? "invalid" });
}

public sealed record CreateUser([property: Required, StringLength(100, MinimumLength = 1)] string Name);
public sealed record User(int Id, string Name);
public interface IUserStore
{
    Task<User?> FindAsync(int id, CancellationToken ct);
    Task<User> AddAsync(string name, CancellationToken ct);
}
public sealed class InMemoryUserStore : IUserStore
{
    private readonly List<User> _users = new();
    private readonly object _gate = new();
    public Task<User?> FindAsync(int id, CancellationToken ct) { lock (_gate) return Task.FromResult(_users.FirstOrDefault(u => u.Id == id)); }
    public Task<User> AddAsync(string name, CancellationToken ct)
    {
        lock (_gate) { var u = new User(_users.Count + 1, name); _users.Add(u); return Task.FromResult(u); }
    }
}
```

What .NET 6 minimal APIs do and do not do (verified on ASP.NET Core 6.0.36):

- **No automatic model validation.** Posting `{}` to `app.MapPost("/users", (CreateUser u) => Results.Ok(u))` returns **200** with `{"name":null,"age":0}`, even with `[Required]`/`[Range]` on the record. Validate explicitly, as above. Endpoint filters (`AddEndpointFilter`) arrived in .NET 7. Controllers with `[ApiController]` do validate automatically.
- **Binding failures**: a non-parsable *query* value for an `int?` parameter returns **400**. A route value that violates a `{id:int}` constraint returns **404**.
- **`IFormFile`/`IFormFileCollection` parameters are not supported in .NET 6** (added in 7.0). The parameter is treated as a JSON body, and a multipart upload returns **415 Unsupported Media Type**. Read the form manually: `async (HttpRequest req) => { var form = await req.ReadFormAsync(); var file = form.Files["file"]; … }`, and enforce size limits.
- **No `MapGroup`, `TypedResults`, `Results<T1,T2>` or `[AsParameters]`** (7.0). Use `Results.*` returning `IResult`, and repeat common configuration with an extension method.
- **Complex types from the query string** need a static `TryParse` or `BindAsync`. Otherwise the parameter is inferred as the body.
- **Return DTOs, not EF entities.** Entities serialize navigation properties (cycles, over-exposure) and invite over-posting when bound from input.
- **Exception details**: with `WebApplication`, the Developer Exception Page is enabled automatically in the `Development` environment. In the test above, the response body of a throwing endpoint contained the exception message in Development and not in Production. Never deploy with `ASPNETCORE_ENVIRONMENT=Development`.

##### Middleware order

```csharp
var builder = WebApplication.CreateBuilder(args);
builder.Logging.ClearProviders();
builder.Services.AddAuthentication();
builder.Services.AddAuthorization();
builder.Services.AddCors(o => o.AddPolicy("spa", p => p.WithOrigins("https://app.example.com").AllowAnyHeader().AllowAnyMethod()));

var app = builder.Build();
if (!app.Environment.IsDevelopment())
{
    app.UseExceptionHandler("/error");    // generic error response in production
    app.UseHsts();
}
app.UseHttpsRedirection();
app.UseStaticFiles();
app.UseRouting();                          // optional with WebApplication; explicit when order matters
app.UseCors("spa");                        // after UseRouting, before auth
app.UseAuthentication();                   // who are you
app.UseAuthorization();                    // what may you do — after UseAuthentication
app.MapGet("/error", () => Results.Problem("An error occurred."));
app.MapGet("/health", () => "ok").AllowAnonymous();
Console.WriteLine("pipeline configured");
```

- `UseAuthentication` must come before `UseAuthorization`, and both after `UseRouting` and `UseCors` when those are explicit. A wrong order silently bypasses policies or breaks CORS preflight.
- **CORS**: `AllowAnyOrigin()` together with `AllowCredentials()` is rejected by the policy builder. Never reflect arbitrary origins with `SetIsOriginAllowed(_ => true)` plus credentials.
- **Authorization by default**: set a fallback policy (`options.FallbackPolicy = new AuthorizationPolicyBuilder().RequireAuthenticatedUser().Build()`) and mark public endpoints with `AllowAnonymous()`. Then a forgotten `[Authorize]` fails closed.
- **HTTPS**: `UseHsts` only outside Development. Behind a reverse proxy, configure `ForwardedHeadersOptions` (`KnownProxies`/`KnownNetworks`), or `Request.Scheme` and client IPs are wrong.
- **Rate limiting middleware is .NET 7.** On .NET 6, use the reverse proxy or gateway, or a library such as AspNetCoreRateLimit, for login and expensive endpoints.
- **`AddProblemDetails` and `IExceptionHandler` are 7.0/8.0.** In 6.0, use `UseExceptionHandler` with a handler endpoint that returns `Results.Problem(...)` without exception details.

##### Controllers

```csharp
using System.ComponentModel.DataAnnotations;
using Microsoft.AspNetCore.Mvc;

var builder = WebApplication.CreateBuilder(args);
builder.Logging.ClearProviders();
builder.Services.AddControllers();
var app = builder.Build();
app.Urls.Add("http://127.0.0.1:0");
app.MapControllers();
await app.StartAsync();
using var http = new HttpClient { BaseAddress = new Uri(app.Urls.First()) };
var response = await http.PostAsJsonAsync("/api/orders", new { quantity = 0 });
Console.WriteLine((int)response.StatusCode);       // 400: [ApiController] validates automatically
await app.StopAsync();

[ApiController]
[Route("api/orders")]
public sealed class OrdersController : ControllerBase
{
    [HttpPost]
    public ActionResult<OrderDto> Create(CreateOrder request) =>
        CreatedAtAction(nameof(Get), new { id = 1 }, new OrderDto(1, request.Quantity));

    [HttpGet("{id:int}")]
    public ActionResult<OrderDto> Get(int id) => id == 1 ? new OrderDto(1, 1) : NotFound();
}

public sealed class CreateOrder { [Range(1, 1000)] public int Quantity { get; set; } }
public sealed record OrderDto(int Id, int Quantity);
```

- **Over-posting**: bind to request DTOs that contain only the fields the client may set, never to entities with `IsAdmin` or `OwnerId` properties.
- `[ApiController]` returns `ValidationProblemDetails` (400) automatically. Without that attribute, check `ModelState.IsValid`.
- Mark actions `async` and accept a `CancellationToken` for I/O work.

##### Hosting and `BackgroundService`

```csharp
using Microsoft.Extensions.DependencyInjection;
using Microsoft.Extensions.Hosting;
using Microsoft.Extensions.Logging;

var host = Host.CreateDefaultBuilder()
    .ConfigureLogging(l => l.ClearProviders())
    .ConfigureServices(s =>
    {
        s.AddScoped<IJobStore, JobStore>();
        s.AddHostedService<CleanupWorker>();
    })
    .Build();
await host.StartAsync();
await Task.Delay(150);
await host.StopAsync();
Console.WriteLine($"runs: {JobStore.Runs > 0}");                 // runs: True

public sealed class CleanupWorker : BackgroundService
{
    private readonly IServiceScopeFactory _scopes;
    private readonly ILogger<CleanupWorker> _logger;
    public CleanupWorker(IServiceScopeFactory scopes, ILogger<CleanupWorker> logger) { _scopes = scopes; _logger = logger; }

    protected override async Task ExecuteAsync(CancellationToken stoppingToken)
    {
        using var timer = new PeriodicTimer(TimeSpan.FromMilliseconds(30));
        while (await timer.WaitForNextTickAsync(stoppingToken))   // throws OperationCanceledException on stop
        {
            try
            {
                using var scope = _scopes.CreateScope();                // new scope per unit of work
                var store = scope.ServiceProvider.GetRequiredService<IJobStore>();
                await store.CleanupAsync(stoppingToken);
            }
            catch (Exception ex) when (ex is not OperationCanceledException)
            {
                _logger.LogError(ex, "Cleanup failed");                 // keep the loop alive on transient errors
            }
        }
    }
}

public interface IJobStore { Task CleanupAsync(CancellationToken ct); }
public sealed class JobStore : IJobStore
{
    public static int Runs;
    public Task CleanupAsync(CancellationToken ct) { Interlocked.Increment(ref Runs); return Task.CompletedTask; }
}
```

- **An unhandled exception in `ExecuteAsync` stops the whole host in .NET 6.** `HostOptions.BackgroundServiceExceptionBehavior` defaults to `StopHost`. In the test, `host.RunAsync()` returned normally about 100 ms after the exception and the **process exit code was 0**, so orchestrators that restart only on failure do not restart it. Catch and log inside the loop, or set `Environment.ExitCode = 1` on fatal errors.
- `ExecuteAsync` runs synchronously until its first `await`. A long synchronous prologue blocks host startup. Start with `await Task.Yield()` or move the work into the loop.
- Scoped services (`DbContext`) need a scope per iteration, as shown. Injecting them into the hosted service's constructor is a captive dependency.
- When `stoppingToken` is cancelled, `WaitForNextTickAsync` throws `OperationCanceledException`. `BackgroundService` treats that as a normal stop.

##### `Program` visibility for integration tests

With top-level statements, the generated `Program` class is `internal` in .NET 6. Test projects cannot use `WebApplicationFactory<Program>` until you add the following line to the end of `Program.cs`, or add `InternalsVisibleTo`:

```csharp
var app = WebApplication.CreateBuilder(args).Build();
app.MapGet("/", () => "ok");
Console.WriteLine(typeof(Program).IsPublic);   // True only because of the declaration below

public partial class Program { }               // makes Program visible to WebApplicationFactory<Program> in test projects
```

***

#### EF Core 6

EF Core 6.0 shipped with .NET 6 and reached end of support on the same day (November 12, 2024, last version 6.0.36). EF Core 10 requires .NET 10.

##### `DbContext` lifetime and thread safety

```csharp
using System.Data.Common;
using Microsoft.Data.Sqlite;
using Microsoft.EntityFrameworkCore;
using Microsoft.EntityFrameworkCore.Diagnostics;

using var connection = new SqliteConnection("Data Source=:memory:");
connection.Open();
var options = new DbContextOptionsBuilder<ShopContext>()
    .UseSqlite(connection)
    .AddInterceptors(new SlowNetwork())          // simulates real network latency
    .Options;

using var db = new ShopContext(options);
db.Database.EnsureCreated();
try
{
    // BAD: two concurrent operations on one DbContext
    await Task.WhenAll(db.Products.ToListAsync(), db.Products.ToListAsync());
}
catch (InvalidOperationException e)
{
    Console.WriteLine(e.Message[..80]);   // A second operation was started on this context instance before a previous...
}

public sealed class ShopContext : DbContext
{
    public ShopContext(DbContextOptions<ShopContext> options) : base(options) { }
    public DbSet<Product> Products => Set<Product>();
}
public sealed class Product { public int Id { get; set; } public string Name { get; set; } = ""; }

public sealed class SlowNetwork : DbCommandInterceptor
{
    public override async ValueTask<InterceptionResult<DbDataReader>> ReaderExecutingAsync(
        DbCommand command, CommandEventData eventData, InterceptionResult<DbDataReader> result, CancellationToken ct = default)
    {
        await Task.Delay(100, ct);
        return result;
    }
}
```

- **A `DbContext` is not thread-safe.** One context serves one unit of work. Register it with `AddDbContext` (scoped). For parallel work, use `IDbContextFactory<T>` (`AddDbContextFactory`, EF Core 5+) and create one context per task.
- **Always `await` EF async calls** before starting the next one on the same context. A forgotten `await` creates exactly this concurrent use.
- `AddDbContextPool` reuses instances. Do not keep per-request state in fields of a pooled context.
- Keep contexts short-lived. A long-lived context accumulates tracked entities, and `SaveChanges` and `DetectChanges` slow down linearly.

##### Querying: SQL injection, tracking, includes, client evaluation

```csharp
using Microsoft.Data.Sqlite;
using Microsoft.EntityFrameworkCore;

using var connection = new SqliteConnection("Data Source=:memory:");
connection.Open();
var options = new DbContextOptionsBuilder<ShopContext>().UseSqlite(connection).Options;
using (var seed = new ShopContext(options))
{
    seed.Database.EnsureCreated();
    seed.Customers.Add(new Customer { Name = "ann", Born = new DateOnly(1990, 5, 1), Orders = { new Order { Total = 10m }, new Order { Total = 20m } } });
    seed.Customers.Add(new Customer { Name = "bob", Born = new DateOnly(1985, 1, 1) });
    seed.SaveChanges();
}

using var db = new ShopContext(options);
var evil = "x' OR '1'='1";

// BAD: interpolating into FromSqlRaw builds the SQL string — injection
var leaked = db.Customers.FromSqlRaw($"SELECT * FROM Customers WHERE Name = '{evil}'").ToList();
Console.WriteLine($"FromSqlRaw: {leaked.Count} rows");                 // 2 — every customer

// GOOD: FromSqlInterpolated turns holes into DbParameters (FromSql is EF Core 7)
var safe = db.Customers.FromSqlInterpolated($"SELECT * FROM Customers WHERE Name = {evil}").ToList();
Console.WriteLine($"FromSqlInterpolated: {safe.Count} rows");          // 0

// GOOD: FromSqlRaw with explicit parameters
var named = db.Customers.FromSqlRaw("SELECT * FROM Customers WHERE Name = {0}", "ann").ToList();
Console.WriteLine($"FromSqlRaw + parameter: {named.Count} row");       // 1

// Read-only queries: AsNoTracking; related data needs Include (no lazy loading by default)
var noInclude = db.Customers.AsNoTracking().Single(c => c.Name == "ann");
var withInclude = db.Customers.AsNoTracking().Include(c => c.Orders).Single(c => c.Name == "ann");
Console.WriteLine($"{noInclude.Orders.Count} vs {withInclude.Orders.Count}");   // 0 vs 2

// Projection: only the needed columns, no tracking
var counts = db.Customers.Select(c => new { c.Name, Orders = c.Orders.Count }).ToList();
Console.WriteLine(string.Join(", ", counts.Select(t => $"{t.Name}={t.Orders}")));   // ann=2, bob=0

// Provider limits: SQLite cannot SUM decimal columns server-side
try { db.Customers.Select(c => c.Orders.Sum(o => o.Total)).ToList(); }
catch (NotSupportedException) { Console.WriteLine("SQLite: no decimal Sum"); }

// Untranslatable predicates throw (no silent client evaluation since EF Core 3.0)
try { db.Customers.Where(c => Rules.IsVip(c.Name)).ToList(); }
catch (InvalidOperationException) { Console.WriteLine("could not be translated"); }

Console.WriteLine(db.Customers.Where(c => c.Name == evil).ToQueryString().Split('\n')[0]);   // .param set @__evil_0 ...

public static class Rules { public static bool IsVip(string name) => name.StartsWith("a"); }

public sealed class ShopContext : DbContext
{
    public ShopContext(DbContextOptions<ShopContext> options) : base(options) { }
    public DbSet<Customer> Customers => Set<Customer>();
    public DbSet<Order> Orders => Set<Order>();
}
public sealed class Customer
{
    public int Id { get; set; }
    public string Name { get; set; } = "";
    public DateOnly Born { get; set; }          // fine on SQLite; SQL Server provider maps DateOnly only from EF Core 8
    public List<Order> Orders { get; } = new();
}
public sealed class Order { public int Id { get; set; } public decimal Total { get; set; } public int CustomerId { get; set; } }
```

Review rules:

- **`FromSqlRaw`/`ExecuteSqlRaw` with `$"…"` or string concatenation is SQL injection.** The interpolation is evaluated *before* EF sees the string. Use `FromSqlInterpolated`/`ExecuteSqlInterpolated`, or `FromSqlRaw("… {0}", value)` with parameters. The shorter `FromSql`/`ExecuteSql` names are EF Core 7.
- **N+1 queries**: a loop that touches navigation properties (with lazy-loading proxies) or issues one query per item. Use `Include`, projection or a single `Where(x => ids.Contains(x.Id))`.
- **Cartesian explosion**: several collection `Include`s in one query multiply rows. Use `AsSplitQuery()` (EF Core 5+) or projection. EF Core logs a warning when multiple collections are included without a split configuration.
- **`AsNoTracking()`** for read-only queries. Tracking costs memory and CPU, and it returns stale tracked instances instead of fresh data.
- **`Find`** checks the change tracker first. `FirstOrDefault` always queries.
- **Bulk updates/deletes**: `ExecuteUpdate`/`ExecuteDelete` are EF Core 7. In EF Core 6, loading thousands of entities to modify them is slow. Use `ExecuteSqlInterpolated` for set-based changes, and remember that it bypasses the change tracker and concurrency tokens.
- **Raw scalar queries**: `Database.SqlQuery<T>` is EF Core 7. In 6, use a keyless entity type with `FromSqlInterpolated`, or ADO.NET via `db.Database.GetDbConnection()`.
- **Don't expose `IQueryable<T>`** from repositories to the API layer. Callers can then compose arbitrary queries, including unbounded ones.
- **Provider differences**: SQLite in EF Core 6 throws `NotSupportedException` for `Sum`/`Average` over `decimal` columns (verified), so tests on SQLite can fail where SQL Server works, and the reverse. Test against the production provider (for example, in containers).
- **Pagination**: always bound result sets (`Take`). Order before `Skip`/`Take`, or the page order is undefined.

##### Saving, concurrency and transactions

```csharp
using Microsoft.Data.Sqlite;
using Microsoft.EntityFrameworkCore;

using var connection = new SqliteConnection("Data Source=:memory:");
connection.Open();
var options = new DbContextOptionsBuilder<BankContext>().UseSqlite(connection).Options;
using (var setup = new BankContext(options)) { setup.Database.EnsureCreated(); setup.Accounts.Add(new Account { Balance = 100 }); setup.SaveChanges(); }

using var first = new BankContext(options);
using var second = new BankContext(options);
var a1 = first.Accounts.Single();
var a2 = second.Accounts.Single();
a1.Balance -= 30; a1.Version = Guid.NewGuid(); first.SaveChanges();
a2.Balance -= 50; a2.Version = Guid.NewGuid();
try { second.SaveChanges(); }
catch (DbUpdateConcurrencyException) { Console.WriteLine("lost update prevented"); }

public sealed class BankContext : DbContext
{
    public BankContext(DbContextOptions<BankContext> o) : base(o) { }
    public DbSet<Account> Accounts => Set<Account>();
}
public sealed class Account
{
    public int Id { get; set; }
    public decimal Balance { get; set; }
    [System.ComponentModel.DataAnnotations.ConcurrencyCheck] public Guid Version { get; set; }   // SQL Server: use [Timestamp] byte[] RowVersion
}
```

- **Read-modify-write without a concurrency token loses updates.** Use `[Timestamp]`/`IsRowVersion()` on SQL Server, or `[ConcurrencyCheck]`, and handle `DbUpdateConcurrencyException`.
- `SaveChanges` is already transactional. Use `BeginTransaction` only to span several `SaveChanges` calls or raw SQL. With retrying execution strategies (`EnableRetryOnFailure`), wrap such transactions in `db.Database.CreateExecutionStrategy().ExecuteAsync(...)`.
- **Migrations in production**: do not call `Database.Migrate()` from every app instance at startup, because concurrent instances race. Use migration bundles (`dotnet ef migrations bundle`, new in EF Core 6) or idempotent SQL scripts in the deployment pipeline.
- **`EnableSensitiveDataLogging()`** logs parameter values (PII, secrets). Allow it in Development only.
- **`decimal` precision**: configure it (`[Precision(18, 2)]`, new in EF Core 6, or `HasPrecision`). Otherwise SQL Server uses `decimal(18,2)` with a warning and may truncate.
- `EnsureCreated()` bypasses migrations. Use it in tests only.

##### New in EF Core 6 (usable)

- `[Precision]`, `[Unicode]`, `[EntityTypeConfiguration(typeof(...))]` attributes.
- `ConfigureConventions` for pre-convention configuration (for example, every `decimal` → precision 18,4, or every `string` → max length).
- SQL Server temporal tables (`ToTable(t => t.IsTemporal())`, `TemporalAsOf`).
- Compiled models (`dotnet ef dbcontext optimize`) for faster startup of large models.
- Migration bundles.
- `DbContextFactory` registration also registers the context type itself (EF Core 6).

***

#### Security

##### Input, injection and output

- **SQL**: parameterize, through EF Core (above) or Dapper/ADO.NET with `@param`. Never concatenate or interpolate untrusted input into SQL, shell commands, LDAP filters, XPath or regex patterns.
- **XSS**: Razor encodes `@value` by default. `Html.Raw`, `MarkupString` and manual `HtmlString` bypass it. Every use needs a sanitizer and a review. JSON embedded in `<script>` must use `JsonSerializer` (it escapes `<`, `>`, `&` by default). Do not switch to `JavaScriptEncoder.UnsafeRelaxedJsonEscaping` for HTML contexts.
- **Path traversal**:

```csharp
static string SafeCombine(string root, string userPath)
{
    var fullRoot = Path.GetFullPath(root).TrimEnd(Path.DirectorySeparatorChar) + Path.DirectorySeparatorChar;
    var full = Path.GetFullPath(Path.Combine(fullRoot, userPath));
    if (!full.StartsWith(fullRoot, StringComparison.Ordinal))      // Ordinal; OrdinalIgnoreCase on Windows
        throw new UnauthorizedAccessException("Path escapes the root directory.");
    return full;
}

var root = Path.Combine(Path.GetTempPath(), "uploads");
Console.WriteLine(Path.Combine(root, "/etc/passwd"));            // /etc/passwd — Path.Combine drops root for rooted input!
Console.WriteLine(SafeCombine(root, "a/b.txt").EndsWith("b.txt"));   // True
try { SafeCombine(root, "../../etc/passwd"); }
catch (UnauthorizedAccessException) { Console.WriteLine("blocked"); }
try { SafeCombine(root, "/etc/passwd"); }
catch (UnauthorizedAccessException) { Console.WriteLine("blocked rooted"); }
```

`Path.Combine` returns the second argument unchanged when that argument is rooted. Always canonicalize with `GetFullPath` and check the prefix, *including the trailing separator* (`/srv/app` must not match `/srv/app-secrets`). Also prefer generating server-side file names over using client file names (`IFormFile.FileName`).

##### Deserialization

- **`BinaryFormatter`** is obsolete (SYSLIB0011) and must never process untrusted data. ASP.NET Core disables it by default, and it always throws in .NET 9+. Replace it now, because migrating to .NET 10 requires that anyway.
- **Newtonsoft.Json `TypeNameHandling` other than `None`**, and `JavaScriptSerializer` with type resolvers, are remote code execution vectors with untrusted input.
- **XML**: on .NET Core, `XmlReader`/`XmlDocument` prohibit DTD processing by default (`DtdProcessing.Prohibit`). Do not enable `DtdProcessing.Parse` with a resolver for untrusted XML (XXE).

##### Cryptography (.NET 6)

```csharp
using System.Security.Cryptography;
using System.Text;

// Random tokens: RandomNumberGenerator (GetBytes(int) is new in .NET 6)
var token = Convert.ToBase64String(RandomNumberGenerator.GetBytes(32));
Console.WriteLine(token.Length);                                         // 44

// Password hashing: PBKDF2 with SHA-256+ and a high iteration count (static Pbkdf2 is new in .NET 6)
var salt = RandomNumberGenerator.GetBytes(16);
var hash = Rfc2898DeriveBytes.Pbkdf2("correct horse", salt, 600_000, HashAlgorithmName.SHA256, 32);
var again = Rfc2898DeriveBytes.Pbkdf2("correct horse", salt, 600_000, HashAlgorithmName.SHA256, 32);
Console.WriteLine(CryptographicOperations.FixedTimeEquals(hash, again));   // True — constant-time compare

// BAD: the old constructor defaults to SHA-1 and 1000 iterations, with NO warning on .NET 6
using var weak = new Rfc2898DeriveBytes("pw", 16);
Console.WriteLine($"{weak.HashAlgorithm.Name} {weak.IterationCount}");    // SHA1 1000

// Authenticated encryption: AesGcm (unique 12-byte nonce per message, 16-byte tag)
var key = RandomNumberGenerator.GetBytes(32);
using var aes = new AesGcm(key);
var nonce = RandomNumberGenerator.GetBytes(12);
var plaintext = Encoding.UTF8.GetBytes("secret");
var ciphertext = new byte[plaintext.Length];
var tag = new byte[16];
aes.Encrypt(nonce, plaintext, ciphertext, tag);
var decrypted = new byte[ciphertext.Length];
aes.Decrypt(nonce, ciphertext, tag, decrypted);
Console.WriteLine(Encoding.UTF8.GetString(decrypted));                  // secret

// Hashing (non-password): static HashData (.NET 5+)
Console.WriteLine(Convert.ToHexString(SHA256.HashData(Encoding.UTF8.GetBytes("abc")))[..16]);   // BA7816BF8F01CFEA
```

- **`System.Random` is not for security** (CA5394). Use `RandomNumberGenerator`.
- **Obsolete in .NET 6** (verified as new warnings compared with the `net5.0` reference assemblies): `RNGCryptoServiceProvider` (SYSLIB0023), `SHA256Managed`/`AesManaged` and the other `*Managed`/`*CryptoServiceProvider` derived types (SYSLIB0021), `WebRequest`/`WebClient` (SYSLIB0014), `Uri.EscapeUriString` (SYSLIB0013), `AppDomain.CreateDomain` (SYSLIB0024). Use `RandomNumberGenerator`, `SHA256.Create()`/`SHA256.HashData`, `Aes.Create()` and `HttpClient`.
- **`new Rfc2898DeriveBytes(password, saltSize)`** is the silent trap: it uses SHA-1 with 1000 iterations and gets no obsoletion warning until .NET 7 (SYSLIB0041). Require `Rfc2898DeriveBytes.Pbkdf2(..., HashAlgorithmName.SHA256 or SHA512, …)` with a high iteration count, or ASP.NET Core Identity's `PasswordHasher<T>`.
- **Compare secrets in constant time**: `CryptographicOperations.FixedTimeEquals`, not `SequenceEqual` or `==`.
- **Never reuse an AES-GCM nonce with the same key.** Do not use ECB or CBC without a MAC. Do not invent constructions.
- `X509CertificateLoader` (.NET 9) does not exist. Load certificates with `new X509Certificate2(...)` but restrict sources, and never trust a client-supplied PFX path.

##### Web security checklist (ASP.NET Core 6)

- Authentication cookies: `HttpOnly`, `Secure` and `SameSite=Lax`/`Strict`. Antiforgery tokens for cookie-authenticated form posts. Minimal APIs in .NET 6 have **no built-in antiforgery**, so do not accept cookie-authenticated, state-changing form posts there without it.
- JWT bearer validation: keep `ValidateIssuer`, `ValidateAudience`, `ValidateLifetime` and `ValidateIssuerSigningKey` enabled. Never set `RequireSignedTokens = false`.
- **Data Protection keys** must be persisted and shared across instances (`PersistKeysToFileSystem`, Redis, blob storage) and protected at rest. Otherwise cookies and antiforgery tokens break on every deploy or scale-out.
- **Open redirects**: use `LocalRedirect`/`Url.IsLocalUrl` for `returnUrl`.
- **SSRF**: allow-list hosts for server-side fetches of user-supplied URLs, and block link-local and metadata addresses (`169.254.169.254`).
- **Regex on untrusted input**: pass a timeout (`new Regex(pattern, options, TimeSpan.FromMilliseconds(100))`). `RegexOptions.NonBacktracking` and `[GeneratedRegex]` are .NET 7.
- **Process execution**: `ProcessStartInfo` with `UseShellExecute = false` and `ArgumentList` (no string concatenation into `Arguments`). Never pass input to `sh -c` or `cmd /c`.
- **The runtime itself**: .NET 6 receives no security patches. Treat "still on net6.0" as a security finding in internet-facing services.

***

#### Performance

- **Measure first**: use BenchmarkDotNet for micro-benchmarks, and `dotnet-counters`, `dotnet-trace` and `dotnet-dump` for production diagnostics.
- **Allocation hot spots**: string concatenation in loops, LINQ in tight loops (delegates, enumerators), `params object[]`, boxing value types (`object`, non-generic collections, `Enum.HasFlag` is fine since .NET Core 2.1), closures that capture locals (use `static` lambdas, C# 9, to prove that nothing is captured), `async` state machines in hot synchronous paths.
- **Strings**: `StringBuilder` for loops, `string.Join`/`Concat`, `string.Create`, span-based parsing, interpolated string handlers (C# 10 already avoids `string.Format` boxing).
- **Collections**: presize (`new List<T>(capacity)`, `new Dictionary<,>(capacity)`), use `TryGetValue` instead of `ContainsKey` plus the indexer, use `HashSet<T>` for membership instead of `List<T>.Contains` in loops.
- **Sealed classes**: seal classes that are not designed for inheritance. The JIT can devirtualize calls on them. CA1852 is .NET 7, so check by hand.
- **Regex**: cache `Regex` instances in `static readonly` fields, and use `RegexOptions.Compiled` for hot patterns. The source generator is .NET 7.
- **Dynamic PGO** is available in .NET 6 but **off by default**. Enable it with the environment variable `DOTNET_TieredPGO=1` (or `"System.Runtime.TieredPGO": true` in `runtimeconfig.json`), and measure. It is on by default from .NET 8.
- **ReadyToRun** (`<PublishReadyToRun>`) improves startup. Native AOT is .NET 7+ (console) and .NET 8+ (ASP.NET Core). Do not suggest `<PublishAot>` for a `net6.0` project.
- **Server GC** is the default for ASP.NET Core. In containers with small memory limits, consider `<ConcurrentGarbageCollection>` and `GCHeapHardLimit` tuning only after measuring.
- **EF Core**: projections, `AsNoTracking`, compiled queries (`EF.CompileAsyncQuery`) for hot queries, batching, and avoiding N+1.
- **`ValueTask`, pooling, `stackalloc`, `Unsafe`**: only with a benchmark in the pull request.

***

#### Testing

##### xUnit and Moq

Use `[Fact]` for single cases and `[Theory]` + `[InlineData]` for tables of inputs, Arrange/Act/Assert structure, and a fresh mock per test class instance:

```csharp
using Moq;
using Xunit;

public interface IOrderRepository { Task<Order?> FindAsync(int id, CancellationToken ct); }
public sealed record Order(int Id, decimal Total);
public sealed class NotFoundException : Exception { public NotFoundException(string m) : base(m) { } }

public sealed class OrderService
{
    private readonly IOrderRepository _repository;
    public OrderService(IOrderRepository repository) => _repository = repository;
    public async Task<Order> GetOrderAsync(int id, CancellationToken ct = default) =>
        await _repository.FindAsync(id, ct) ?? throw new NotFoundException($"Order {id} not found");
}

public class OrderServiceTests
{
    private readonly Mock<IOrderRepository> _repository = new();
    private readonly OrderService _service;

    public OrderServiceTests() => _service = new OrderService(_repository.Object);

    [Fact]
    public async Task GetOrderAsync_ExistingOrder_ReturnsOrder()
    {
        _repository.Setup(r => r.FindAsync(1, It.IsAny<CancellationToken>())).ReturnsAsync(new Order(1, 100m));

        var result = await _service.GetOrderAsync(1);

        Assert.Equal(1, result.Id);
        _repository.Verify(r => r.FindAsync(1, It.IsAny<CancellationToken>()), Times.Once);
    }

    [Fact]
    public async Task GetOrderAsync_MissingOrder_ThrowsNotFound()
    {
        _repository.Setup(r => r.FindAsync(999, It.IsAny<CancellationToken>())).ReturnsAsync((Order?)null);

        await Assert.ThrowsAsync<NotFoundException>(() => _service.GetOrderAsync(999));
    }
}
```

Rules:

- **Async tests return `Task`**, never `async void`, which xUnit cannot observe. Use `await Assert.ThrowsAsync<…>`. A non-awaited `Assert.ThrowsAsync` always passes.
- **No `Thread.Sleep`/`Task.Delay` for synchronization.** Use `TaskCompletionSource`, `SemaphoreSlim` or polling with a timeout. Inject time through an `IClock` interface, because `TimeProvider`/`FakeTimeProvider` are .NET 8.
- **Do not test against EF Core InMemory** to prove SQL behaviour. It ignores relational constraints, transactions and translation rules. Use SQLite in-memory for quick tests, or the real database engine in a container.
- Name tests `Method_Scenario_Expected`. Use one logical assertion per test, and table-driven `[Theory]` tests for rule-heavy code.
- Do not assert exception *messages* from the BCL or exact `ToString()` output of framework types. They change between runtime versions, which matters for the .NET 10 migration.
- **Test package versions must still support `net6.0`.** Check before upgrading test SDKs and runners. Many current major versions target `net8.0`+ only.

##### Integration tests with `WebApplicationFactory<Program>`

```csharp
// Requires `public partial class Program { }` in the web project (see ASP.NET Core 6) and
// Microsoft.AspNetCore.Mvc.Testing 6.0.x in the test project.
using Microsoft.AspNetCore.Mvc.Testing;
using Xunit;

public class ApiIntegrationTests : IClassFixture<WebApplicationFactory<Program>>
{
    private readonly HttpClient _client;

    public ApiIntegrationTests(WebApplicationFactory<Program> factory) => _client = factory.CreateClient();

    [Fact]
    public async Task GetUsers_ReturnsSuccessAndJson()
    {
        var response = await _client.GetAsync("/api/users");

        response.EnsureSuccessStatusCode();
        Assert.Equal("application/json; charset=utf-8", response.Content.Headers.ContentType?.ToString());
    }
}
```

- Replace external dependencies in `factory.WithWebHostBuilder(b => b.ConfigureServices(...))`. Remove the real `DbContextOptions<T>` registration before adding a test one, or both are registered.
- `WebApplicationFactory` runs the app in the `Development` environment by default. Use `builder.UseEnvironment("Testing")` to avoid enabling development-only behaviour, such as the developer exception page.

***

#### Analyzers and Warnings

- The .NET 6 SDK includes the .NET analyzers (`Microsoft.CodeAnalysis.NetAnalyzers`). Configure them with `<AnalysisLevel>` (`6.0`, `latest`, or `6.0-recommended`/`6.0-all` modes) or `<AnalysisMode>`, plus `.editorconfig` severities. `EnforceCodeStyleInBuild` turns IDE code-style rules into build diagnostics.
- Rules worth enabling as warnings (all present in the SDK 6.0.428 analyzers): CA2016 (forward `CancellationToken`), CA2254 (constant log templates), CA1848 (LoggerMessage), CA2007 (ConfigureAwait, libraries only), CA2012 (`ValueTask` misuse), CA2000 (dispose objects), CA1305/CA1307/CA1309/CA1310 (culture and comparison), CA1827/CA1829 (`Any`/`Count` property), CA1845/CA1846 (span-based string APIs), CA1847 (`string.Contains(char)`), CA5394 (insecure `Random`), CA2100 (SQL injection review), CA3075 (insecure XML), CA5350/CA5351 (weak crypto), CA2326/CA2327 (Newtonsoft `TypeNameHandling`), CA1416 (platform compatibility).
- **Not available in the .NET 6 SDK** (newer analyzers): CA1850 (static `HashData`), CA1851 (multiple enumeration), CA1852 (seal internal types), CA1854 (`TryGetValue`), CA1860, CA1869 (cache `JsonSerializerOptions`), CA2021. Reviewers must catch these patterns manually.
- **Warning wave 6** (C# 10) adds CS8826 (partial method declarations with signature differences, for example different parameter names). A default `net6.0` project build reported it. A bare `csc` call needs `-warn:6`. Warning wave 7 (CS8981, lowercase type names) is C# 11.
- **Nullable warnings** (CS8600–CS8634, CS8714 and others) are the most valuable warnings in the code base. Use `<WarningsAsErrors>nullable</WarningsAsErrors>`.
- Do not blanket-suppress with `#pragma warning disable` at file level. Suppress at the narrowest scope with a justification, or use `[SuppressMessage(..., Justification = "...")]`.

##### New in C# 10: `#line` span directive

```csharp
#line (1, 1) - (1, 30) 5 "Views/Index.razor"
var generated = 42;
#line default
Console.WriteLine(generated);
```

The span form `#line (startLine, startChar) - (endLine, endChar) charOffset "file"` lets generators (Razor, source generators) map generated code to precise source ranges. Hand-written code should not contain `#line` directives. If they appear in a pull request outside generated files, they hide the real location of errors and breakpoints.

***

#### Observability

##### Metrics with `System.Diagnostics.Metrics` (New in .NET 6)

```csharp
using System.Diagnostics;
using System.Diagnostics.Metrics;

using var meter = new Meter("Contoso.Orders", "1.0");
var ordersPlaced = meter.CreateCounter<long>("orders_placed", unit: "{order}", description: "Orders placed");
var duration = meter.CreateHistogram<double>("order_duration", unit: "ms");

var sw = Stopwatch.StartNew();
ordersPlaced.Add(1, new KeyValuePair<string, object?>("region", "eu"));    // low-cardinality tags only
duration.Record(sw.Elapsed.TotalMilliseconds);
Console.WriteLine($"{ordersPlaced.Name} {duration.Unit}");                  // orders_placed ms

// Tracing: ActivitySource + Activity (W3C trace context is the default since .NET 5)
using var source = new ActivitySource("Contoso.Orders");
using (var activity = source.StartActivity("PlaceOrder"))
{
    activity?.SetTag("order.id", 42);   // StartActivity returns null when nobody listens: use ?.
}
Console.WriteLine("traced");
```

- `Meter`, `Counter<T>`, `Histogram<T>`, `ObservableGauge<T>` are new in .NET 6. `UpDownCounter<T>` is .NET 7 and `Gauge<T>` (synchronous) is .NET 9.
- **Never use unbounded tag values** (user IDs, URLs with IDs, email addresses) as metric tags, because cardinality explodes.
- `ActivitySource.StartActivity` returns `null` without listeners. Always use `?.`.
- Use OpenTelemetry .NET exporters for metrics and traces. Check that the chosen OpenTelemetry package versions still target `net6.0` or `netstandard2.0`.
- Health checks: `AddHealthChecks()` + `MapHealthChecks("/health")`. Separate liveness from readiness, and do not expose detailed health output publicly.
- `dotnet-counters`, `dotnet-trace`, `dotnet-dump` and `dotnet-gcdump` work against .NET 6 processes. In containers, mount the diagnostics socket or install the tools in a debug sidecar.

***

#### Removed / Changed in .NET 6 — Migration Notes (from .NET 5 / .NET Core 3.1)

The following behaviour changes matter when you review code that moved to `net6.0`, or that was written with older assumptions.

##### Runtime and libraries

- **Partial reads**: `DeflateStream`, `GZipStream` and `CryptoStream` return as soon as some data is available (see Resource Management). Every `Read` call must use its return value.
- **Conditional evaluation in `Debug` methods**: `Debug.Assert(condition, $"…")` no longer evaluates the interpolation holes when the condition is true. This is the interpolated-string-handler behaviour shown under Strings.
- **`StringBuilder.Append($"…")`** now binds to a handler overload, which changes evaluation order relative to the older `Append(string)` call. Side effects in holes are a review smell.
- **`Task.FromResult` may return a cached singleton.** Never lock on, or compare by reference, tasks you did not create.
- **`FileStream` rewrite**: `Position` updates after `ReadAsync`/`WriteAsync` *complete*, the file offset is no longer synchronized with the OS, and files are locked with a shared lock on Unix. Code that mixes `FileStream` with raw handles or concurrent position-based access needs review. Use `RandomAccess` (.NET 6) for offset-based I/O.
- **`System.Drawing.Common` is Windows-only.** On Linux, `new Bitmap(1, 1)` threw `TypeInitializationException` → `PlatformNotSupportedException: System.Drawing.Common is not supported on non-Windows platforms` (verified with package 6.0.0). Use ImageSharp, SkiaSharp or similar for cross-platform imaging.
- **Unhandled exceptions in a `BackgroundService` stop the host** (`BackgroundServiceExceptionBehavior.StopHost`). In .NET 5 they were swallowed.
- **Resolving from a disposed `ServiceProvider`** throws `ObjectDisposedException` (verified).
- **`TimeSpan` JSON**: System.Text.Json 6 serializes `TimeSpan` as `"01:30:00"` (constant "c" format, verified). Earlier versions wrote it as an object of all its public properties (`{"Ticks":…,"Days":…}`), which does not round-trip. The source generator matched the reflection format only from 6.0.2, so services pinned to an early 6.0.x runtime can still emit the object form.
- **Nullable annotations** across the BCL were refined, so new CS86xx warnings after retargeting point to real assumptions. Fix them; do not suppress them.
- **Obsoletions with new SYSLIB diagnostic IDs**: the `net6.0` reference assemblies add SYSLIB0013–SYSLIB0033 (the `net5.0` ones stop at SYSLIB0012). Among them: SYSLIB0013 (`Uri.EscapeUriString`), SYSLIB0014 (`WebRequest`/`WebClient`/`ServicePoint`), SYSLIB0020 (`JsonSerializerOptions.IgnoreNullValues`, use `DefaultIgnoreCondition`), SYSLIB0021 (derived crypto types such as `SHA256Managed`), SYSLIB0022 (`Rijndael*`), SYSLIB0023 (`RNGCryptoServiceProvider`), SYSLIB0024 (`AppDomain.CreateDomain`/`Unload`), SYSLIB0026–SYSLIB0028 (mutable X509 APIs, `PublicKey.Key`, `X509Certificate2.PrivateKey`), and SYSLIB0032 (`HandleProcessCorruptedStateExceptions`). Older ones from .NET 5 include SYSLIB0011 (`BinaryFormatter`), SYSLIB0006 (`Thread.Abort`) and SYSLIB0012 (`Assembly.CodeBase`).

##### SDK and templates

- New templates use top-level statements, minimal hosting, `<Nullable>enable</Nullable>` and `<ImplicitUsings>enable</ImplicitUsings>`.
- `.editorconfig` files are passed to the compiler automatically, so analyzer severities in `.editorconfig` apply to builds.
- `dotnet run -p` is deprecated. Use `--project`.
- `-r <rid>` without `--self-contained`/`--no-self-contained` warns NETSDK1179 (verified on SDK 6.0.428), because the .NET 6 SDK still makes a RID-specific build self-contained by default. CI scripts should state the choice explicitly; .NET 8 changed the default to framework-dependent.

##### EF Core 6 breaking changes worth checking

- **SQLite connections are pooled** (Microsoft.Data.Sqlite 6.0). The database file stays open after `Close()`/`Dispose()` (verified: one handle remains until `SqliteConnection.ClearAllPools()`). Tests that delete or replace the file must clear the pool or use `Pooling=False`.
- **Unknown enum strings throw.** With `HasConversion<string>()`, a column value that matches no enum member throws `InvalidOperationException: Cannot convert string value 'Bogus' from the database to any value in the mapped 'St' enum` (verified). EF Core 5 silently returned the default member.
- **Nested optional owned dependents that share a table and have no required property** now fail model validation instead of losing data.
- **Pre-initialized reference navigations** (`public Bar Bar { get; set; } = new();`) are overwritten by query results in tracking queries too. Do not initialize reference navigations.
- **`DbSet<T>` no longer implements `IAsyncEnumerable<T>`.** Use `AsAsyncEnumerable()`.

##### C# 10 compiler breaking changes

- Lambdas and method groups with a natural type can change overload resolution (`Delegate`/`object`/`Expression` overloads, instance versus extension methods). See Lambdas.
- `!` is no longer allowed in patterns (`o is null!` is an error).
- A struct with field initializers must declare a constructor (CS8983), and that constructor must assign every field without an initializer (CS0171). In a `record struct` with a parameter list, every other constructor must chain to the primary constructor (or to an explicitly declared one): `: this()` is error CS8982.

***

#### Upgrading to .NET 10 (the recommended target)

Reviewers should support the migration and not block it. Key steps and the most common review points:

1. Change `<TargetFramework>net6.0</TargetFramework>` to `net10.0` and the `global.json` SDK to 10.0.x. C# 14 becomes the default. Remove explicit `<LangVersion>10</LangVersion>` unless there is a reason.
2. Upgrade `Microsoft.AspNetCore.*`, `Microsoft.Extensions.*`, `Microsoft.EntityFrameworkCore.*` and `System.Text.Json` packages to 10.0.x together. Remove `SuppressTfmSupportBuildWarnings`.
3. **Remove `BinaryFormatter`.** Since .NET 9 it always throws, unless the unsupported compatibility package is used.
4. Expect new obsoletion warnings, for example SYSLIB0041 (`Rfc2898DeriveBytes` constructors without a hash algorithm, .NET 7) and SYSLIB0057 (`X509Certificate2` loading constructors, .NET 9; use `X509CertificateLoader`).
5. Review the per-version breaking-change lists for 7, 8, 9 and 10, especially ASP.NET Core and EF Core. EF Core 7 changed SQL Server `SaveChanges` to use `OUTPUT` clauses, which matters for tables with triggers. EF Core 8+ changed `Contains` translation for parameter collections. EF Core 9 throws when the model has pending changes on `Migrate`.
6. Then modernize *after* the retarget compiles. This is the point where the newer features become available: primary constructors, collection expressions, `required` members, raw strings, `TimeProvider`, `FrozenDictionary`, keyed services, `ExecuteUpdate`/`ExecuteDelete`, rate limiting, `IExceptionHandler`, and built-in minimal-API validation (.NET 10).

***

#### Production / Review Checklist

##### Platform and version

- [ ] `net6.0` acknowledged as **out of support since November 12, 2024**, with a migration ticket to **`net10.0`** (LTS, supported until November 14, 2028). No new `net6.0` projects. .NET 8/9 are not upgrade targets (EOL November 10, 2026)
- [ ] Running the final patch 6.0.36 (runtime and ASP.NET Core) / SDK 6.0.428 or 6.0.136, with container images pinned to the 6.0.36 tags
- [ ] No `LangVersion=preview`/`EnablePreviewFeatures`. No C# 11+ syntax and no .NET 7+ APIs suggested or used (see "Do Not Use or Recommend")
- [ ] Package versions still support `net6.0`. No `SuppressTfmSupportBuildWarnings`
- [ ] `TreatWarningsAsErrors` (at least for nullable warnings), analyzers enabled, and `dotnet format --verify-no-changes` in CI

##### Language (C# 10)

- [ ] Value objects are `readonly record struct` or `record`. No mutable `record struct` where copies are modified. No secrets in record `ToString()`
- [ ] Struct constructors and field initializers do not establish invariants that `default`/arrays bypass. Every struct with field initializers declares the constructor it needs
- [ ] `global using` directives live in one file. Ambiguities are fixed with aliases
- [ ] File-scoped namespaces in new files
- [ ] No side effects in interpolation holes. Invariant culture for machine-readable interpolated strings (`string.Create(CultureInfo.InvariantCulture, …)`)
- [ ] Overload changes caused by lambda natural types reviewed when moving from C# 9
- [ ] `ArgumentNullException.ThrowIfNull` or explicit guards. No `ThrowIfNullOrEmpty`/`ThrowIfNegative`/`ObjectDisposedException.ThrowIf` (.NET 7/8)

##### Correctness

- [ ] Nullable warnings fixed, not suppressed. `!` only with justification. Deserialized input validated (STJ does not enforce non-nullable members)
- [ ] `switch` expressions exhaustive (CS8509 as error). Enums validated with `Enum.IsDefined` after parsing
- [ ] Ordinal string comparison for keys and identifiers. Explicit culture for formatting and parsing
- [ ] `decimal` for money. `checked` where overflow is a bug. UTC/`DateTimeOffset` for instants
- [ ] `DateOnly`/`TimeOnly` not used with System.Text.Json 6 without a converter, or with EF Core 6 on SQL Server
- [ ] Every `Stream.Read` return value used, with read loops (no `ReadExactly` in .NET 6)
- [ ] No multiple enumeration of `IEnumerable<T>`. Deferred queries materialized before their sources change or are disposed
- [ ] `PriorityQueue` not assumed to be FIFO for equal priorities. `MaxBy`/`MinBy` empty-sequence behaviour handled

##### Async and concurrency

- [ ] No `.Result`/`.Wait()`/`async void` (except handlers with try/catch). No `Parallel.ForEach` with async lambdas
- [ ] `CancellationToken` accepted and forwarded (CA2016). `CancellationTokenSource`s disposed. `WaitAsync` not mistaken for cancellation
- [ ] `[EnumeratorCancellation]` on async iterators. Channels bounded
- [ ] `ValueTask` awaited exactly once. Pooling builders only on measured hot paths
- [ ] `TaskCompletionSource` with `RunContinuationsAsynchronously`
- [ ] Shared state protected (`SemaphoreSlim` for async, `lock` on a private object, `Interlocked`). `Random.Shared`, not a shared `new Random()`. `GetOrAdd` factories idempotent or wrapped in `Lazy<T>`

##### ASP.NET Core 6 and hosting

- [ ] Minimal API input validated explicitly (no endpoint filters in 6.0). No `IFormFile` parameters in minimal APIs (use `ReadFormAsync`)
- [ ] DTOs, not entities, in requests and responses. No over-posting
- [ ] Middleware order: routing → CORS → authentication → authorization. Fallback authorization policy. No `AllowAnyOrigin` with credentials
- [ ] Production never runs with `ASPNETCORE_ENVIRONMENT=Development`. Exception handler returns generic problem details
- [ ] DI: no captive dependencies (scope validation enabled in tests). `IServiceScopeFactory` in background services. `ValidateOnStart()` for options
- [ ] `BackgroundService` loops catch and log. Fatal errors set a non-zero exit code
- [ ] `IHttpClientFactory`/typed clients, not `new HttpClient()` per call. Typed clients not captured by singletons. Responses disposed. Timeouts and retries configured
- [ ] Logging uses message templates or `[LoggerMessage]`. No secrets or PII in logs or HTTP logging
- [ ] Data Protection keys persisted and shared. Forwarded headers configured behind proxies

##### EF Core 6

- [ ] One `DbContext` per unit of work, never shared across threads or concurrent awaits. `IDbContextFactory` for parallel work
- [ ] No `FromSqlRaw`/`ExecuteSqlRaw` with interpolation or concatenation
- [ ] `AsNoTracking` for reads. Projections. No N+1. Split queries for multiple collection includes. Bounded, ordered pagination
- [ ] Concurrency tokens on contended entities. `DbUpdateConcurrencyException` handled
- [ ] Migrations applied by the pipeline (bundles or scripts), not by every instance at startup. `EnableSensitiveDataLogging` only in Development
- [ ] No EF Core 7+ APIs (`ExecuteUpdate`, `ExecuteDelete`, `FromSql`, `SqlQuery`, JSON columns)

##### Security

- [ ] Parameterized SQL. No `Html.Raw` on user data. Path traversal prevented with `GetFullPath` plus prefix-with-separator checks
- [ ] No `BinaryFormatter`, no Newtonsoft `TypeNameHandling`, no DTD processing for untrusted XML
- [ ] `RandomNumberGenerator` for tokens. `Rfc2898DeriveBytes.Pbkdf2` with SHA-256+ and high iterations (never the SHA-1/1000 constructor). `FixedTimeEquals`. AES-GCM with unique nonces
- [ ] No obsolete APIs (SYSLIB0013/0014/0021/0023). `HttpClient` instead of `WebClient`
- [ ] Regex timeouts for untrusted input. SSRF allow-lists. `ProcessStartInfo.ArgumentList` without a shell
- [ ] Secrets only in secret stores or environment variables, never in `appsettings*.json` or logs

##### Testing and observability

- [ ] Async tests return `Task` and await assertions. No sleeps for synchronization. Time injected via an interface
- [ ] Integration tests use `WebApplicationFactory<Program>` with `public partial class Program { }`, against the real database engine for query semantics
- [ ] Metrics via `Meter` with low-cardinality tags. `ActivitySource` with `?.`. Health checks split into liveness and readiness
