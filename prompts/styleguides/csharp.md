### C# Style Guide

> **Version**: This is the **version-independent** C# guide. It applies when the project's C# language version or target framework cannot be determined. It covers C# 7.3 through C# 15 and .NET Framework 4.x, .NET Core 3.1 and .NET 5–11. The newest stable release is **.NET 10 (LTS) with C# 14**, released November 11, 2025; its latest patch is **10.0.13 (October 13, 2026)**, SDK 10.0.402, and it is supported until **November 14, 2028**.[^1][^2] **.NET 8 (LTS) and .NET 9 (STS) both reach end of support on November 10, 2026.** **.NET 11 / C# 15 is not released**: 11.0.0-rc.2 (SDK 11.0.100-rc.2.26504.105) shipped on October 13, 2026 with a go-live license; general availability is planned for November 2026.[^2][^3]
> **Scope**: Version-independent review rules for production C#: nullable reference types, types and records, pattern matching, exceptions, async/await and cancellation, `IDisposable`/`IAsyncDisposable`, collections, LINQ, strings and culture, dates and numbers, dependency injection and configuration, logging, `HttpClient`, JSON, ASP.NET Core security, EF Core, cryptography and injection risks, performance, testing and analyzers.
> **Version rule**: Every feature tied to a language or runtime version is labelled with the version that introduced it, for example "file-scoped namespaces (C# 10)", "`ArgumentNullException.ThrowIfNull` (.NET 6)", "primary constructors (C# 12)", "`field` keyword (C# 14)". Before suggesting such a feature, check the project's `LangVersion` and `TargetFramework` (see [Determine the Version First](#determine-the-version-first)). Never recommend a feature the project's version does not have.
> **Note**: This document is self-contained. Every example was compiled and run with SDK 10.0.402 / runtime 10.0.13 (C# 14, `net10.0`, Linux x64). Language-version boundaries were checked by compiling probes with `LangVersion` N−1 and N (SDK 10.0.402 for C# 7.3–14, SDK 11.0.100-rc.2 for C# 15); API boundaries were checked against the reference assemblies of .NET Framework 4.8, .NET Standard 2.0/2.1, .NET Core 3.1 and .NET 5–10. Windows-only behaviour (NLS globalization, WinForms/WPF, .NET Framework runtime behaviour) was not executed.

---

#### Determine the Version First

The C# language version and the .NET API surface are two separate axes. A reviewer needs both before suggesting a fix.

##### Where the version comes from

1. **`<LangVersion>`** in the `.csproj`, a `Directory.Build.props`/`Directory.Build.targets` higher up the tree, or a `-langversion` compiler switch. An explicit value wins.
2. **`<TargetFramework>`/`<TargetFrameworks>`** — without `LangVersion`, the SDK picks the default C# version from the target framework (table below). Multi-targeted projects compile each target with its own default.
3. **`global.json`** — pins the SDK and therefore the compiler (Roslyn) version. `rollForward` decides whether a newer SDK may be used.
4. **Container images and CI** (`mcr.microsoft.com/dotnet/sdk:8.0`, `actions/setup-dotnet` with `dotnet-version: 9.0.x`) decide the SDK when there is no `global.json`.
5. **Package versions**: `Microsoft.AspNetCore.*`, `Microsoft.EntityFrameworkCore.*` and `Microsoft.Extensions.*` major versions normally match the target framework (EF Core 8 ↔ `net8.0`).
6. **Old-style projects** (`<Project ToolsVersion=...>` with `<TargetFrameworkVersion>v4.7.2</TargetFrameworkVersion>`, `packages.config`) are .NET Framework projects: C# 7.3 by default, no `ImplicitUsings`, no nullable context unless enabled.

Default language version per target framework (verified with SDK 11.0.100-rc.2 and SDK 10.0.402, `dotnet msbuild -getProperty:LangVersion`):[^4]

| Target framework | Default C# |
| --- | --- |
| .NET Framework (all, including `net35`), `netstandard2.0`, `netstandard1.x`, `netcoreapp2.x` | 7.3 |
| `netstandard2.1`, `netcoreapp3.0`, `netcoreapp3.1` | 8.0 |
| `net5.0` | 9.0 |
| `net6.0` | 10.0 |
| `net7.0` | 11.0 |
| `net8.0` | 12.0 |
| `net9.0` | 13.0 |
| `net10.0` | 14.0 |
| `net11.0` (RC) | 15.0 (SDK 11 only; SDK 10.0.402 evaluates `net11.0` to 14.0) |

##### Rules for version-aware review

- **Label every version-specific suggestion.** Write "use file-scoped namespaces (C# 10)" or "`ArgumentNullException.ThrowIfNull` (.NET 6+)", never an unqualified "use X".
- **When the version is unknown, prefer the portable form** and mention the modern alternative with its version: `if (x is null) throw new ArgumentNullException(nameof(x));` works everywhere (`is null` needs C# 7.0); `ArgumentNullException.ThrowIfNull(x)` needs .NET 6.
- **Language features and APIs are independent.** `<LangVersion>12</LangVersion>` on `netstandard2.0` enables collection expressions, primary constructors, type aliases and default lambda parameters (pure syntax, verified) but not `required` members, `init` accessors or positional `record` types without polyfill types (CS0518/CS0656; a non-positional `record` with `set` accessors compiles) (`IsExternalInit`, `RequiredMemberAttribute`, `CompilerFeatureRequiredAttribute`, `SetsRequiredMembersAttribute`), and never features that need runtime support: default interface methods (C# 8), static abstract interface members (C# 11), `ref` fields (C# 11) and inline arrays (C# 12) require .NET Core 3.0+, .NET 7+, .NET 7+ and .NET 8+ respectively.
- **A newer C# version than the target framework's default is unsupported by Microsoft**, even though it often compiles. It can change overload resolution: C# 14's first-class span conversions bind `array.Reverse()` to the `void`-returning, in-place `MemoryExtensions.Reverse(Span<T>)` on `netcoreapp2.1`–`net9.0` (and on any target that references `System.Memory`); `net10.0` adds an `Enumerable.Reverse(T[])` overload that restores the old binding.[^5][^6]
- **`<LangVersion>latest</LangVersion>` or `preview` makes the language version depend on whichever SDK builds the code.** Microsoft's own documentation warns against `latest`; flag `preview` in production code.[^5]
- **Verify an API exists in the pinned target framework before recommending it, and before calling it "unavailable".** Say "not available in .NET X" only when the target framework's reference assemblies prove it (the tables below were verified that way). Many APIs reach older targets through NuGet packages (`System.Text.Json`, `Microsoft.Bcl.AsyncInterfaces`, `System.Threading.Channels`, `Microsoft.Bcl.TimeProvider`, `System.Collections.Immutable`).
- **`#if` guards in multi-targeted libraries**: `#if NET6_0_OR_GREATER` / `NET8_0_OR_GREATER` / `NETSTANDARD2_0` / `NETFRAMEWORK` are SDK-defined symbols; code using a new API in a multi-targeted project must be guarded or the older target fails to build.
- `#error version` in a source file makes the compiler print its version and the selected language version (CS8304) — a quick way to confirm in doubt.[^5]

---

#### Release and Support Status (October 2026)

Status as of October 22, 2026 (dates from Microsoft's release metadata and lifecycle pages):[^2][^7][^8]

| Platform | C# default | Released | Latest patch (date) | Support status |
| --- | --- | --- | --- | --- |
| .NET Framework 4.8.1 | 7.3 | Aug 9, 2022 | (Windows Update servicing) | **Supported** — component of Windows, follows the OS lifecycle |
| .NET Framework 4.7–4.8 | 7.3 | 2017–2019 | (Windows Update servicing) | Supported, follows the OS lifecycle |
| .NET Framework 4.6.2 | 7.3 | Aug 2, 2016 | — | Ends **January 12, 2027** |
| .NET Framework 4.5.2, 4.6, 4.6.1 | 7.3 | 2014–2015 | — | **Out of support** since April 26, 2022 |
| .NET Framework 3.5 SP1 | 7.3 | Nov 19, 2007 | — | Ends January 9, 2029 |
| .NET Core 3.1 (LTS) | 8.0 | Dec 3, 2019 | 3.1.32 (Dec 13, 2022) | **Out of support** since December 13, 2022 |
| .NET 5 (STS) | 9.0 | Nov 10, 2020 | 5.0.17 (May 10, 2022) | **Out of support** since May 10, 2022 |
| .NET 6 (LTS) | 10.0 | Nov 8, 2021 | 6.0.36 (Nov 12, 2024) | **Out of support** since November 12, 2024 |
| .NET 7 (STS) | 11.0 | Nov 8, 2022 | 7.0.20 (May 28, 2024) | **Out of support** since May 14, 2024 |
| .NET 8 (LTS) | 12.0 | Nov 14, 2023 | 8.0.32 (Oct 13, 2026) | Maintenance; **ends November 10, 2026** |
| .NET 9 (STS) | 13.0 | Nov 12, 2024 | 9.0.21 (Oct 13, 2026) | Maintenance; **ends November 10, 2026** |
| .NET 10 (LTS) | 14.0 | Nov 11, 2025 | 10.0.13 (Oct 13, 2026) | **Active**, supported until November 14, 2028 |
| .NET 11 (STS) | 15.0 | **not released** — RC2 on Oct 13, 2026 | 11.0.0-rc.2 | Go-live release candidate; GA planned for November 2026 |

Notes:

- **C# 7.3** shipped with Visual Studio 2017 version 15.7 (May 2018); **C# 8.0** with .NET Core 3.0 and Visual Studio 2019 16.3 (September 2019).[^9] C# versions after 8 ship with the .NET SDK of the same release.
- STS releases are supported for 24 months since .NET 9 (previously 18), which is why .NET 9 ends on the same day as .NET 8.[^7]
- The October 2026 Patch Tuesday releases (8.0.32, 9.0.21, 10.0.13) fix dozens of CVEs (10.0.13 alone fixes 44); a Dockerfile or `global.json` pinned to an older patch is a security finding.[^2]
- **Upgrade recommendation**: projects on .NET Core 3.1, .NET 5, 6 or 7 are unpatched and should move to **.NET 10 (LTS)**. Do not recommend .NET 8 or 9 as an upgrade target (they leave support in November 2026) and do not recommend .NET 11 for production before GA. .NET Framework applications are supported on 4.8/4.8.1 but receive no new features; recommend porting only when there is a reason to (new features, Linux, performance), and flag 4.6.2 or older targets.
- .NET Framework 4.8.1 is the last .NET Framework version; Microsoft states that new development should use modern .NET.[^8]

---

#### C# Language Features by Version

Every row was checked by compiling a probe with the listed `LangVersion` (succeeds) and the previous one (fails with "Feature '…' is not available in C# N"). Use this table to decide whether a suggestion compiles in the project.[^9][^10]

| Version | Features a reviewer is likely to suggest | Runtime/library requirement |
| --- | --- | --- |
| **C# 7.0–7.3** (2017–2018) | tuples and deconstruction, `out var`, pattern matching with `is T x` and `case T x when`, local functions, `ref` locals/returns, `in` parameters, `readonly struct`, `ref struct`, `Span<T>` support, `private protected`, digit separators, expression-bodied constructors/accessors, `throw` expressions, `default` literal, `stackalloc` initializers, tuple `==` | `ValueTuple` (in-box from .NET Framework 4.7 / .NET Core 2.0; NuGet package before) |
| **C# 8.0** (2019) | **nullable reference types**, switch expressions, property/tuple/positional patterns, `using` declarations, `await using`, async streams (`IAsyncEnumerable<T>`, `await foreach`), indices and ranges (`^1`, `..`), `??=`, static local functions, `readonly` members, default interface methods | async streams and `IAsyncDisposable` need .NET Core 3.0/`netstandard2.1` (or `Microsoft.Bcl.AsyncInterfaces`); ranges need `System.Index`/`System.Range`; **default interface methods need .NET Core 3.0+** (not .NET Framework) |
| **C# 9** (Nov 2020) | `record` (class) types, `init` accessors, top-level statements, target-typed `new()`, `not`/`and`/`or` and relational patterns (`is not null`, `is > 0 and < 10`), static lambdas, lambda discard parameters, covariant return types, `nint`/`nuint`, function pointers, module initializers, extended partial methods | `init`/records need `IsExternalInit` (.NET 5+; polyfill for older targets); covariant returns need .NET 5+ |
| **C# 10** (Nov 2021) | `record struct`, `global using`, **file-scoped namespaces**, extended property patterns (`{ A.B: 1 }`), constant interpolated strings, lambda natural types (`var f = () => 1;`), lambda attributes and explicit return types, `CallerArgumentExpression`, `with` on structs and anonymous types, parameterless struct constructors, interpolated string handlers | `ImplicitUsings` generates `global using` and therefore needs C# 10 |
| **C# 11** (Nov 2022) | raw string literals (`"""`), list patterns (`[1, .., 3]`), **`required` members**, generic math / static abstract interface members, UTF-8 literals (`"abc"u8`), `file`-local types, generic attributes, `>>>`, `ref` fields and `scoped`, newlines in interpolation holes, auto-default structs, `nameof` of parameters in attributes | `required` needs `RequiredMemberAttribute` (.NET 7+ or polyfill); static abstract members and `ref` fields need .NET 7+ |
| **C# 12** (Nov 2023) | **primary constructors** for classes and structs, **collection expressions** (`[1, 2, .. xs]`), inline arrays, default lambda parameters, alias any type (`using Point = (int X, int Y);`), `ref readonly` parameters, `[Experimental]` | inline arrays need .NET 8+ |
| **C# 13** (Nov 2024) | `params` collections (`params ReadOnlySpan<T>`), `System.Threading.Lock` support in `lock`, `\e` escape, implicit `^` index in object initializers, `ref` locals and `unsafe` in async methods/iterators, `ref struct` interfaces, `allows ref struct`, partial properties and indexers, `[OverloadResolutionPriority]` | `Lock` and `OverloadResolutionPriorityAttribute` are .NET 9 types |
| **C# 14** (Nov 2025) | **extension members** (`extension(T x) { ... }` blocks with properties, static members, operators), **`field` keyword**, null-conditional assignment (`a?.B = x`), `nameof(List<>)`, first-class span conversions, modifiers on simple lambda parameters (`(s, out r) => ...`), partial constructors and events, user-defined compound assignment operators (`operator +=`) | first-class spans change overload resolution on older targets |
| **C# 15** (RC, not released) | union types (`union Pet(Cat, Dog);`), `closed` classes, collection expression arguments (`[with(capacity: n), ..xs]`), extension indexers, labeled `break`/`continue`, non-virtual static members and constants in interfaces on targets without default-interface-method support (`netstandard2.0`) | unions and `closed` need .NET 11 runtime types |

Version-boundary details verified on the compilers:

- `ImplicitUsings=enable` with `LangVersion` 9 fails with CS8773 "Feature 'global using directive' is not available in C# 9.0" in the generated `GlobalUsings.g.cs`. This happens in multi-targeted libraries where the `netstandard2.0` leg defaults to C# 7.3.
- Using an inline array (`b[0]`) at C# 11 fails with CS9058; declaring the `[InlineArray]` struct alone compiles.
- `[CallerArgumentExpression]` is honoured by the C# 14 compiler even at `LangVersion` 7.3 and 9 (verified: the parameter receives `"1 + 2"`), and `nameof(parameter)` inside a method attribute (extended `nameof` scope, C# 11) compiles with the SDK 10 compiler at `LangVersion` 7.3–10, while the .NET 6 SDK compiler rejects it (CS0103). Neither proves a language version; they are the two exceptions to the N−1 rule above.
- C# 15 features fail under `LangVersion` 14 with CS9327 on SDK 11.0.100-rc.2; union declarations produce parse errors instead.
- Not part of any released C# version: dictionary expressions, null-conditional `await`, extension constants, chained comparisons. Never suggest them.

---

#### .NET APIs by Version

Verified by compiling against the reference assemblies of each target framework. "Package" means a NuGet package brings the API to older targets.

| API | First in | Notes |
| --- | --- | --- |
| `HashCode.Combine`, `string.Create`, `ProcessStartInfo.ArgumentList`, `CryptographicOperations.FixedTimeEquals`, `Dictionary.TryAdd`, `IReadOnlyDictionary.GetValueOrDefault`, `string.Contains(string, StringComparison)`, `SocketsHttpHandler` | .NET Core 2.x (not .NET Framework) | all available in .NET Core 3.1 |
| `System.Text.Json`, `IAsyncEnumerable<T>`, `IAsyncDisposable`, `System.Index`/`Range`, `Path.Join(string, string)` (span overloads: .NET Core 2.1), `RandomNumberGenerator.GetInt32`, `System.Threading.Channels`, `[NotNullWhen]`, `CallerArgumentExpressionAttribute` | .NET Core 3.0 | packages for .NET Framework/`netstandard2.0` |
| `[MemberNotNull]`, `IsExternalInit`, `[ModuleInitializer]`, `SHA256.HashData`, `Convert.ToHexString`, `Enum.GetValues<T>()`, `Environment.ProcessId`, `OperatingSystem.IsWindows()`, `Interlocked.Or/And`, `CollectionsMarshal.AsSpan`, `System.Net.Http.Json` in-box | .NET 5 | |
| `ArgumentNullException.ThrowIfNull`, `DateOnly`/`TimeOnly`, `PeriodicTimer`, `Parallel.ForEachAsync`, `Task.WaitAsync`, `MaxBy`/`MinBy`/`DistinctBy`/`Chunk`, `TryGetNonEnumeratedCount`, `Random.Shared`, `RandomNumberGenerator.GetBytes(int)`, `Rfc2898DeriveBytes.Pbkdf2`, `CollectionsMarshal.GetValueRefOrAddDefault`, `string.ReplaceLineEndings`, `System.Diagnostics.Metrics.Meter`, `[LoggerMessage]` generator, minimal APIs | .NET 6 | |
| `ArgumentException.ThrowIfNullOrEmpty`, `ObjectDisposedException.ThrowIf`, `Enumerable.Order`, `Stream.ReadExactly`/`ReadAtLeast`, `UnreachableException`, `[GeneratedRegex]`, `RegexOptions.NonBacktracking`, `required` attributes, `TypedResults`, rate-limiting middleware, EF Core `ExecuteUpdate`/`ExecuteDelete`/`FromSql` | .NET 7 / EF Core 7 | |
| `ArgumentException.ThrowIfNullOrWhiteSpace`, `ArgumentOutOfRangeException.ThrowIfNegative`/`ThrowIfZero`/`ThrowIfGreaterThan`…, `TimeProvider`, `FrozenDictionary`/`FrozenSet`, `SearchValues<T>`, `CompositeFormat`, `CancellationTokenSource.CancelAsync`, `ConfigureAwaitOptions`, `AesGcm(key, tagSizeInBytes)`, `RandomNumberGenerator.Shuffle`/`GetItems`, `IMeterFactory`, `[Experimental]`, `[InlineArray]`, keyed DI services, `IExceptionHandler`, `Microsoft.Extensions.Http.Resilience` | .NET 8 | `TimeProvider` also via `Microsoft.Bcl.TimeProvider` |
| `System.Threading.Lock`, `Task.WhenEach`, `CountBy`/`AggregateBy`/`Index`, `Guid.CreateVersion7`, `OrderedDictionary<TKey,TValue>`, `Base64Url`, `Convert.ToHexStringLower`, `X509CertificateLoader`, `MemoryExtensions.Split(ReadOnlySpan<char>, char)`, `JsonSerializerOptions.Web`, `RespectNullableAnnotations`, `[OverloadResolutionPriority]`; `BinaryFormatter` removed | .NET 9 | |
| `Enumerable.LeftJoin`/`RightJoin`, `Shuffle`, `Sequence`, `InfiniteSequence`, `System.Linq.AsyncEnumerable` in-box, `JsonSerializerOptions.Strict`, `AllowDuplicateProperties`, `CompareOptions.NumericOrdering`, `MLKem`/`MLDsa`, minimal-API `AddValidation()` | .NET 10 | |
| `Enumerable.FullJoin`, `Process.RunAndCaptureTextAsync`, `ZstandardStream`, `JsonNamingPolicy.PascalCase`, `Decimal32/64/128` | .NET 11 (RC) | not available in any released version |

---

#### Do Not Use or Recommend Without a Version Check

These are the suggestions reviewers most often get wrong. Each is correct **only** from the stated version on.

- **C# 15 / .NET 11 (pre-release, October 2026)**: union types, `closed` classes, `[with(...), ..xs]` collection expression arguments, extension indexers, labeled `break`/`continue`, `Enumerable.FullJoin`, `Process.Run*`, `JsonNamingPolicy.PascalCase`. Never suggest these for production code; .NET 11 is a release candidate.[^3]
- **C# 14 / .NET 10**: `field` in accessors, `extension(...)` blocks, `a?.B = x`, `nameof(List<>)`, `(s, out r) =>` lambdas, `operator +=` declarations, `LeftJoin`/`RightJoin`, `Shuffle`, in-box `AsyncEnumerable`, `JsonSerializerOptions.Strict`, `AddValidation()`.
- **C# 13 / .NET 9**: `params ReadOnlySpan<T>`, `System.Threading.Lock`, `\e`, partial properties, `Task.WhenEach`, `CountBy`, `Index()`, `Guid.CreateVersion7()`, `X509CertificateLoader`, `JsonSerializerOptions.Web`.
- **C# 12 / .NET 8**: primary constructors on classes, collection expressions (`int[] a = [1, 2];`), `TimeProvider`, frozen collections, `SearchValues`, `ThrowIfNullOrWhiteSpace`, `ThrowIfNegative`, keyed services, `IExceptionHandler`, `AddStandardResilienceHandler`.
- **C# 11 / .NET 7**: raw strings, list patterns, `required`, `"..."u8`, `file` types, `[GeneratedRegex]`, `ObjectDisposedException.ThrowIf`, `ThrowIfNullOrEmpty`, `ExecuteUpdate`/`ExecuteDelete`.
- **C# 10 / .NET 6**: file-scoped namespaces, `global using`, `record struct`, `ArgumentNullException.ThrowIfNull`, `DateOnly`, `Parallel.ForEachAsync`, `PeriodicTimer`, `MaxBy`/`DistinctBy`/`Chunk`, minimal APIs, `WebApplication.CreateBuilder`.
- **C# 9 / .NET 5**: records, `init`, `is not null`, target-typed `new()`, top-level statements.
- **C# 8 / .NET Core 3.0**: nullable reference types, switch expressions, `using var`, `await using`, `IAsyncEnumerable<T>`, `^1`/`..`. These are **not available by default on .NET Framework or `netstandard2.0` (C# 7.3)**.
- **`LangVersion` raised on .NET Framework/`netstandard2.0`**: syntax-only features work; runtime-backed ones (default interface methods, static abstract members, `ref` fields, inline arrays, covariant returns) do not, and `init`/`required`/records need polyfill types. Do not suggest them to a .NET Framework project without checking for polyfills (for example the `PolySharp` package or hand-written `IsExternalInit`).

---

#### Deprecated, Obsolete and Removed APIs

These apply to every modern .NET version; flag them regardless of the detected version (with the version where they became obsolete):[^11][^12]

| API | Status | Replacement |
| --- | --- | --- |
| `BinaryFormatter`, `SoapFormatter`, `NetDataContractSerializer`, `LosFormatter`, `ObjectStateFormatter` | `Serialize`/`Deserialize` obsolete (SYSLIB0011) since .NET 5, the whole type since .NET 8; **removed** in .NET 9 (always throws `PlatformNotSupportedException`); insecure on .NET Framework | System.Text.Json, protobuf, `DataContractSerializer` with known types; `System.Formats.Nrbf` to read legacy payloads |
| `WebRequest`, `HttpWebRequest`, `WebClient`, `ServicePoint` | Obsolete (SYSLIB0014) since .NET 6; `ServicePointManager` only since .NET 9 (same ID) | `HttpClient` (`SocketsHttpHandler` settings) |
| `Thread.Abort`, `Thread.Suspend`, CAS, `AppDomain.CreateDomain` | `Thread.Abort` throws `PlatformNotSupportedException` on .NET Core/.NET 5+ (SYSLIB0006) | cooperative cancellation (`CancellationToken`) |
| `RNGCryptoServiceProvider`, `SHA1Managed`/`SHA256Managed`, `*CryptoServiceProvider` constructors, `HashAlgorithm.Create(string)` | Obsolete: SYSLIB0023 (`RNGCryptoServiceProvider`) and SYSLIB0021 (derived `*Managed`/`*CryptoServiceProvider` types) since .NET 6; SYSLIB0045 (name-based `Create(string)` factories) since .NET 7 | `RandomNumberGenerator`, `SHA256.Create()`/`SHA256.HashData` |
| `Rfc2898DeriveBytes` constructors | Obsolete (SYSLIB0060) in .NET 10; the constructors without a hash algorithm (SHA-1, 1,000 iterations) were obsolete earlier (SYSLIB0041, .NET 7) | `Rfc2898DeriveBytes.Pbkdf2(...)` (.NET 6+) |
| `X509Certificate2(byte[])`/`(string)` constructors | Obsolete (SYSLIB0057) in .NET 9 | `X509CertificateLoader` (.NET 9+; `Microsoft.Bcl.Cryptography` package for .NET Framework 4.6.2+, `netstandard2.0` and .NET 8) |
| `AesGcm(byte[] key)` without tag size | Obsolete (SYSLIB0053) in .NET 8 | `new AesGcm(key, tagSizeInBytes)` |
| `XsltSettings.EnableScript` | Obsolete (SYSLIB0062) in .NET 10; XSLT scripts are not supported on .NET Core at all | no script blocks; extension objects |
| `Exception(SerializationInfo, StreamingContext)` serialization constructors | Obsolete (SYSLIB0051) in .NET 8 | omit them in new exception types |
| `JavaScriptSerializer`, `DataContractJsonSerializer` | .NET Framework era | System.Text.Json |
| `WebHostBuilder`, `IWebHost`, `WebHost` | Obsolete in ASP.NET Core 10 (ASPDEPR004, ASPDEPR008) | `WebApplication.CreateBuilder` / `Host.CreateApplicationBuilder` |
| `ForwardedHeadersOptions.KnownNetworks`, `Microsoft.AspNetCore.HttpOverrides.IPNetwork` | Obsolete in ASP.NET Core 10 (ASPDEPR005) | `KnownIPNetworks` with `System.Net.IPNetwork` |
| `Microsoft.Extensions.Http.Polly` (`AddPolicyHandler`) | Superseded in .NET 8 | `Microsoft.Extensions.Http.Resilience` |
| `System.Data.SqlClient` | Deprecated NuGet package | `Microsoft.Data.SqlClient` |

SYSLIB (and ASP.NET Core's ASPDEPR) IDs are separate diagnostic IDs: suppressing CS0618 does not suppress them (verified with `-nowarn:CS0618`), and `TreatWarningsAsErrors` turns them into build errors after an SDK upgrade.

---

#### Project Setup, Build and Analyzers

##### Baseline SDK-style project

```xml
<Project Sdk="Microsoft.NET.Sdk">
  <PropertyGroup>
    <TargetFramework>net10.0</TargetFramework>
    <!-- Omit LangVersion: the target framework picks the default. Never "preview" in production. -->
    <Nullable>enable</Nullable>                       <!-- C# 8+ -->
    <ImplicitUsings>enable</ImplicitUsings>           <!-- .NET 6 SDK, needs C# 10 -->
    <TreatWarningsAsErrors>true</TreatWarningsAsErrors>
    <AnalysisLevel>latest-recommended</AnalysisLevel> <!-- .NET 5 SDK+ -->
    <EnforceCodeStyleInBuild>true</EnforceCodeStyleInBuild>
    <Deterministic>true</Deterministic>
    <ContinuousIntegrationBuild Condition="'$(CI)' == 'true'">true</ContinuousIntegrationBuild>
  </PropertyGroup>
</Project>
```

Verified defaults: the .NET analyzers (`EnableNETAnalyzers`) are on by default only for `net5.0` and later; for `net48`, `netstandard2.0` and `netcoreapp3.1` the property is `false` and must be set explicitly (or the `Microsoft.CodeAnalysis.NetAnalyzers` package referenced). `Nullable` and `ImplicitUsings` have no default; templates add them.

Review rules:

- **`Nullable` enabled for all new code.** `#nullable disable` regions and blanket `<NoWarn>CS8600;CS8602;...</NoWarn>` need a justification. For a legacy code base, enabling per file (`#nullable enable`) or `<Nullable>warnings</Nullable>` are valid migration steps.
- **Pin the SDK** with `global.json` (`"rollForward": "latestPatch"` or `"latestFeature"`); without it, the build uses whatever SDK the machine has — including a preview SDK with a newer compiler and new warnings.
- **Central package management** (`Directory.Packages.props` with `ManagePackageVersionsCentrally`, NuGet 6.2+), lock files (`RestorePackagesWithLockFile` + `--locked-mode` in CI), and `packageSourceMapping` in `nuget.config` against dependency confusion.
- **NuGet Audit** (NuGet 6.8 / .NET 8 SDK) reports vulnerable packages at restore; the .NET 10 SDK audits transitive packages by default. Do not suppress NU1901–NU1904 globally.[^13]
- `NoWarn` on security analyzers (CA2100 SQL injection, CA3075 insecure DTD processing, CA5350/CA5351 weak crypto, CA2300–CA2330 insecure deserialization) needs a written reason.
- `AnalysisLevel` values: `latest`, `preview`, a version (`8.0`), optionally with a mode suffix: `-none`, `-default`, `-minimum`, `-recommended`, `-all`.[^14]
- `IDE*` style rules fail the build only with `EnforceCodeStyleInBuild=true`.
- Package major versions follow the framework: mixing `Microsoft.Extensions.*` 8.x and 10.x in one app produces NU1605 downgrade errors or `MissingMethodException` at run time.

##### Analyzer rules worth enforcing

```ini
root = true

[*.cs]
indent_style = space
indent_size = 4
dotnet_diagnostic.CA2016.severity = warning   # forward the CancellationToken
dotnet_diagnostic.CA1849.severity = warning   # call async methods in async methods
dotnet_diagnostic.CA2007.severity = none      # ConfigureAwait: enable in libraries only
dotnet_diagnostic.CA1848.severity = suggestion # LoggerMessage delegates (hot paths)
dotnet_diagnostic.CA2254.severity = warning   # logging template must be constant
dotnet_diagnostic.CA1851.severity = warning   # multiple enumeration of IEnumerable
dotnet_diagnostic.CA1305.severity = warning   # specify IFormatProvider
dotnet_diagnostic.CA1309.severity = warning   # use ordinal string comparison
dotnet_diagnostic.CA1869.severity = warning   # cache JsonSerializerOptions
dotnet_diagnostic.CA2000.severity = warning   # dispose objects before losing scope
```

Rule availability depends on the analyzer version: CA1851 and CA1869 arrived with the .NET 7 and .NET 8 analyzers, so an `.editorconfig` entry for an unknown rule ID is silently ignored on older SDKs.

---

#### Naming and Code Style

These house rules apply to every C# version:

```csharp
public interface IUserRepository
{
    Task<User?> FindAsync(int id, CancellationToken cancellationToken = default);
}

public sealed class UserService
{
    public const int MaxRetryCount = 3;              // constants: PascalCase, not SCREAMING_CASE
    private static readonly TimeSpan DefaultTimeout = TimeSpan.FromSeconds(30);
    private readonly IUserRepository _repository;    // private fields: _camelCase

    public UserService(IUserRepository repository) =>
        _repository = repository ?? throw new ArgumentNullException(nameof(repository));

    public bool IsActive { get; private set; }        // booleans: Is/Has/Can prefix

    // Async methods end with Async and take a CancellationToken last
    public async Task<User> GetUserAsync(int id, CancellationToken cancellationToken = default)
    {
        var user = await _repository.FindAsync(id, cancellationToken);
        return user ?? throw new KeyNotFoundException($"User {id} not found");
    }
}

public sealed class User
{
    public User(int id, string name) { Id = id; Name = name; }
    public int Id { get; }
    public string Name { get; }
}
```

- PascalCase for types, namespaces, methods, properties, events, constants and enum members; camelCase for parameters and locals; `_camelCase` for private fields; interfaces start with `I`; type parameters start with `T` (`TKey`, `TResult`).
- Methods returning `Task`/`ValueTask`/`IAsyncEnumerable<T>` end with `Async` (except event handlers and framework-defined names like `ExecuteAsync`, `Main`); synchronous methods do not.
- One public type per file; the file name matches the type name. Use file-scoped namespaces (C# 10) when the project's language version allows it; block-scoped namespaces otherwise.
- Member order: constants, static fields, instance fields, constructors, properties, events, public methods, private methods.
- `sealed` by default for classes not designed for inheritance (also lets the JIT devirtualize calls).
- `var` when the type is obvious from the right-hand side; an explicit type otherwise.
- No `this.` qualification except to disambiguate; in C# 14, `this.field` is such a case (see [Types](#types-classes-records-structs)).
- Prefer expression-bodied members for one-liners; avoid nested ternaries.
- Enum types are singular (`OrderStatus`) except `[Flags]` enums, which are plural (`FilePermissions`) and have power-of-two values plus `None = 0`.
- Solution layout: `src/` (Api, Core/Domain, Infrastructure), `tests/` (UnitTests, IntegrationTests); `.slnx` solution files are the default from the .NET 10 SDK (`.sln` remains valid).

---

#### Nullable Reference Types

Available from **C# 8** (and annotated BCL from .NET Core 3.0; .NET 5+ for most of the BCL). Nullable reference types are a **compile-time analysis only**: `string` and `string?` are the same runtime type, and nothing is checked at run time.

```csharp
using System.Diagnostics.CodeAnalysis;

string? maybe = Environment.GetEnvironmentVariable("NOT_SET_ANYWHERE");
// int bad = maybe.Length;              // warning CS8602: dereference of a possibly null reference
int len = maybe?.Length ?? 0;
Console.WriteLine(len);                  // 0

if (TryFind("a", out var value))
    Console.WriteLine(value.Length);     // 5 — no warning thanks to [NotNullWhen(true)]

var names = new string[2];               // no warning, but both elements are null
Console.WriteLine(names[0] is null);     // True
var pair = default(Pair);
Console.WriteLine(pair.Name is null);    // True, despite the non-nullable 'string Name'

static bool TryFind(string key, [NotNullWhen(true)] out string? value)
{
    value = key == "a" ? "alpha" : null;
    return value is not null;
}

struct Pair { public string Name; }
```

Rules:

- **`!` (null-forgiving) is a code smell.** Each use needs a reason. `= null!` on properties is acceptable only for members populated by a framework (EF Core navigation properties, DI-populated properties); `required` (C# 11) is usually better.
- Annotate APIs with `[NotNullWhen]`, `[MaybeNullWhen]`, `[NotNull]`, `[NotNullIfNotNull]`, `[DoesNotReturn]` (.NET Core 3.0) and `[MemberNotNull]` (.NET 5) instead of suppressing warnings. For `netstandard2.0`/.NET Framework these attributes must be polyfilled (internal copies in `System.Diagnostics.CodeAnalysis` work).
- **Public APIs still validate arguments**: callers with nullable disabled, reflection, deserializers and `default` can pass `null`. Portable form: `if (arg is null) throw new ArgumentNullException(nameof(arg));`. Modern helpers: `ArgumentNullException.ThrowIfNull` (.NET 6), `ArgumentException.ThrowIfNullOrEmpty` (.NET 7), `ThrowIfNullOrWhiteSpace` (.NET 8).
- **Deserializers ignore annotations by default.** System.Text.Json writes `null` into a non-nullable `string` property unless `RespectNullableAnnotations` (.NET 9) or `JsonSerializerOptions.Strict` (.NET 10) is used.
- Arrays of non-nullable references (`new string[3]`) and `default` structs contain `null` without any warning (verified above).
- `<Nullable>enable</Nullable>` on a `netstandard2.0` library compiles (C# 8+ needed via `LangVersion`), but the referenced BCL is not annotated — expect fewer warnings, not more safety.
- `is null`/`is not null` (`is not` needs C# 9) are preferred over `== null` because user-defined `==` operators can lie.
- Fix warnings instead of disabling them; a pull request that adds `#nullable disable` to silence new warnings hides real bugs.

---

#### Types: Classes, Records, Structs

##### Records (C# 9; `record struct` C# 10)

```csharp
var a = new Money(10m, "EUR");
var b = a with { Amount = 20m };                     // non-destructive mutation
Console.WriteLine(a == new Money(10m, "EUR"));       // True: value equality
Console.WriteLine(b);                                // Money { Amount = 20, Currency = EUR }

var t1 = new Tagged("x", new[] { "a" });
var t2 = new Tagged("x", new[] { "a" });
Console.WriteLine(t1 == t2);                         // False: arrays compare by reference

var login = new Login("admin", "hunter2");
Console.WriteLine(login);                            // Login { User = admin }

public sealed record Money(decimal Amount, string Currency);
public sealed record Tagged(string Name, string[] Tags);

public sealed record Login(string User, string Password)
{
    // Records print every public property in ToString() — including secrets.
    private bool PrintMembers(System.Text.StringBuilder builder)
    {
        builder.Append("User = ").Append(User);
        return true;
    }
}
```

- Record equality is member-wise with `EqualityComparer<T>.Default`; collection members compare **by reference**. Records with arrays/lists are rarely correct as dictionary keys or in `Distinct()`.
- Positional records generate `init` properties; a record is not deeply immutable if a property holds a mutable collection.
- `record struct` (C# 10) is **mutable** by default; use `readonly record struct` for value objects.
- `with` copies every field, including fields that must be unique (IDs, locks, `CancellationTokenSource`).
- Records print all public members in `ToString()` — a `Password` or token property ends up in logs. Override `PrintMembers` or keep secrets out of records.
- Do not use records as EF Core entities that are tracked and mutated: value equality conflicts with identity tracking.

##### `required` and `init`

```csharp
var settings = new SmtpSettings { Host = "smtp.example.test" };
Console.WriteLine($"{settings.Host}:{settings.Port}");   // smtp.example.test:25
// var broken = new SmtpSettings();   // error CS9035: required member 'Host' must be set

public sealed class SmtpSettings
{
    public required string Host { get; init; }   // required: C# 11; init: C# 9
    public int Port { get; init; } = 25;
}
```

`required` (C# 11, .NET 7 attribute types) is enforced at object creation and honoured by System.Text.Json (.NET 7+) during deserialization. A constructor marked `[SetsRequiredMembers]` disables the check — review such constructors. `init`-only properties cannot be set in object initializers by C# 8 or older consumers (for example a .NET Framework project consuming a `netstandard2.0` library).

##### Primary constructors (C# 12)

Primary-constructor parameters of a **class** are captured mutable state, not `readonly` fields:

```csharp
var counter = new Counter(5);
counter.Increment();
Console.WriteLine(counter.Value);       // 6: the parameter itself was mutated

public sealed class Counter(int start)
{
    public int Value => start;
    public void Increment() => start++; // legal: there is no 'readonly' for captured parameters
}
```

- Capturing a parameter **and** assigning it to a field/property stores it twice (warning CS9124); the copies can diverge.
- For DI services, assign to a `private readonly` field when immutability matters: `private readonly IFoo _foo = foo ?? throw new ArgumentNullException(nameof(foo));`.
- Records' primary constructors (C# 9) generate properties; classes' and structs' (C# 12) do not.

##### The `field` keyword (C# 14)

```csharp
var p = new Person { Name = "  Ada " };
Console.WriteLine($"[{p.Name}]");    // [Ada]

public sealed class Person
{
    public string Name
    {
        get;
        set => field = value?.Trim() ?? throw new ArgumentNullException(nameof(value));
    } = "";
}
```

Upgrading to C# 14 changes the meaning of the identifier `field` inside accessors: in a type that has a member named `field`, `get { return field; }` now reads the synthesized backing field. The compiler only warns (CS9258). Verified: with `LangVersion` 13 such a property returns the member's value (1); with 14 it returns 0. Use `this.field` or rename the member; treat CS9258 as an error when moving to C# 14.[^6]

##### Structs

- Make structs `readonly` when possible (`readonly struct`, C# 7.2), or mark non-mutating methods `readonly` (C# 8) to avoid defensive copies.
- Keep structs small (16–24 bytes is a common rule of thumb); large structs are copied on every assignment and call.
- Mutating a struct obtained from a property, list indexer or `readonly` field mutates a **copy** (`list[0].X = 1` does not compile; `GetPoint().Move()` silently changes a temporary).
- Implement `IEquatable<T>` and override `GetHashCode` on structs used as dictionary keys; the default `ValueType.Equals` can use reflection and boxes.

##### Classes and inheritance

- `sealed` by default; `virtual` only when designed for it. Do not call virtual members from constructors.
- Override `Equals` and `GetHashCode` together (`HashCode.Combine`, .NET Core 2.1+), and `==`/`!=` with them.
- `ToString()` must not throw and must not leak secrets.
- Mutable static state is shared by all threads and requests; prefer instance state managed by DI.

---

#### Pattern Matching

Type patterns with designation (`is T x`) are C# 7.0; switch expressions, property and positional patterns C# 8; `not`/`and`/`or` and relational patterns C# 9; extended property patterns C# 10; list patterns C# 11.

```csharp
Console.WriteLine(Discount(new Premium(1000m)));          // 200.0
Console.WriteLine(Shipping(new Order(150m, true)));       // Express
Console.WriteLine(Classify(new[] { 1, 2, 3 }));           // starts with 1, ends with 3

static decimal Discount(Customer customer) => customer switch
{
    Premium p => p.Total * 0.2m,
    Regular { Years: > 5 } r => r.Total * 0.1m,
    Regular r => r.Total * 0.05m,
    _ => throw new ArgumentOutOfRangeException(nameof(customer)),
};

static string Shipping(Order order) => order switch
{
    { Total: > 100, IsPriority: true } => "Express",
    { Total: > 100 } => "Standard",
    { IsPriority: true } => "Priority",
    _ => "Economy",
};

static string Classify(int[] xs) => xs switch   // list patterns: C# 11
{
    [] => "empty",
    [var only] => $"one: {only}",
    [1, .., 3] => "starts with 1, ends with 3",
    _ => "other",
};

abstract record Customer(decimal Total);
sealed record Premium(decimal Total) : Customer(Total);
sealed record Regular(decimal Total, int Years) : Customer(Total);
record Order(decimal Total, bool IsPriority);
```

Review points:

- **Precedence of `not`/`and`/`or`**: `x is not null or ""` means `(not null) or ""`, which is true for every non-null `x`. Write `x is not (null or "")`. The compiler warns (CS9336 "The pattern is redundant") for simple cases only under `LangVersion` 14 or later; at 13 and below, even the SDK 10 compiler says nothing (verified).

```csharp
object o = 42;
Console.WriteLine(o is not null or 42);    // True  — probably not what the author meant (CS9336 from C# 14)
Console.WriteLine(o is not (null or 42));  // False
```

- A `switch` expression without a matching arm throws `SwitchExpressionException` (.NET Core 3.0+; `InvalidOperationException` on older frameworks) at run time; never suppress CS8509 ("not exhaustive"). Class hierarchies are never considered exhaustive before C# 15's `closed` classes, so a discard arm that throws is required.
- Order matters: a broader arm before a narrower one makes the narrower one unreachable (CS8510 error for switch expressions).
- `is` patterns on `double` use `Equals` semantics: `double.NaN is double.NaN` is `true` while `double.NaN == double.NaN` is `false`.
- On C# 7.3, use `switch` statements with `case T x when ...:`; do not suggest switch expressions.

---

#### Exceptions

```csharp
try
{
    Parse("x");
}
catch (FormatException ex) when (ex.Data.Contains("input"))   // filter runs before the stack unwinds
{
    Console.WriteLine($"bad input: {ex.Data["input"]}");      // bad input: x
}

static int Parse(string s)
{
    try
    {
        return int.Parse(s, System.Globalization.CultureInfo.InvariantCulture);
    }
    catch (FormatException ex)
    {
        ex.Data["input"] = s;
        throw;          // rethrow preserves the stack trace; 'throw ex;' resets it (CA2200)
    }
}
```

Rules:

- `throw;` not `throw ex;`. To add context, wrap: `throw new OrderImportException("...", ex);` (inner exception preserved). To rethrow a captured exception elsewhere, `ExceptionDispatchInfo.Capture(ex).Throw()` keeps the original trace.
- Catch specific exceptions. `catch (Exception)` belongs only at boundaries (request pipeline, message loop, background-service loop, `Main`) and must log.
- **Never treat `OperationCanceledException` as an error** when cancellation was requested; filter with `when (ct.IsCancellationRequested)` and do not log it at `Error` level.
- Empty `catch { }` blocks are findings unless commented and narrow.
- Do not use exceptions for expected control flow in hot paths; provide `TryXxx` methods (`int.TryParse`, `Dictionary.TryGetValue`).
- Custom exceptions end with `Exception`, derive from `Exception` (not `ApplicationException`), and provide the standard constructors. The serialization constructor `(SerializationInfo, StreamingContext)` is obsolete from .NET 8 (SYSLIB0051); do not add it to new types on modern .NET.
- `finally` and `using` run on exceptions, but not after `Environment.FailFast`, a process kill, or a stack overflow (which cannot be caught on .NET Core/.NET 5+).
- Exception messages must not contain secrets, connection strings or raw user input that ends up in logs; never return `ex.ToString()` to HTTP clients.
- `Task.WhenAll` rethrows only the **first** exception when awaited; inspect each task (or `whenAllTask.Exception!.InnerExceptions`) to see all failures:

```csharp
var all = Task.WhenAll(FailAsync("a"), FailAsync("b"));
try
{
    await all;
}
catch (Exception ex)
{
    Console.WriteLine(ex.Message);                                  // a
    Console.WriteLine(all.Exception!.InnerExceptions.Count);        // 2
}

static async Task FailAsync(string message)
{
    await Task.Yield();
    throw new InvalidOperationException(message);
}
```

---

#### Async/Await

##### Core rules

```csharp
using System.Diagnostics;

using var cts = new CancellationTokenSource(TimeSpan.FromSeconds(5));
var sw = Stopwatch.StartNew();

// Start independent operations first, then await them together.
Task<int> a = LoadAsync(1, cts.Token);
Task<int> b = LoadAsync(2, cts.Token);
int[] results = await Task.WhenAll(a, b);
Console.WriteLine($"{results.Sum()} parallel={sw.ElapsedMilliseconds < 350}");   // 3 parallel=True

static async Task<int> LoadAsync(int id, CancellationToken ct)
{
    await Task.Delay(200, ct);     // always pass the token down (CA2016)
    return id;
}
```

- **Never block on async code** — `.Result`, `.Wait()`, `.GetAwaiter().GetResult()`. In ASP.NET Core it starves the thread pool; under a `SynchronizationContext` (WinForms, WPF, classic ASP.NET on .NET Framework) it deadlocks. Go async all the way up; `static async Task Main` is available from C# 7.1.
- **No `async void`** except event handlers. An exception escaping an `async void` method is rethrown on the synchronization context or the thread pool and **terminates the process**; the caller cannot catch it. Event handlers wrap their body in `try`/`catch`.

```csharp
// BAD: an exception here would crash the process:
// button.Click += async (_, _) => await SaveAsync();
// GOOD:
EventHandler handler = async (_, _) =>
{
    try { await SaveAsync(); }
    catch (Exception ex) { Console.WriteLine($"save failed: {ex.Message}"); }
};
handler(null, EventArgs.Empty);
await Task.Delay(100);                  // let the handler finish (demo only)

static async Task SaveAsync() { await Task.Yield(); throw new IOException("disk full"); }
```

- **No fire-and-forget** (`_ = DoAsync();`) in services: exceptions go unobserved, the work outlives the request scope (disposed `DbContext`), and shutdown does not wait for it. Use a `Channel<T>` with a `BackgroundService` or a queue.
- `Parallel.ForEach` with an `async` lambda creates `async void` delegates — a bug. Use `Parallel.ForEachAsync` (.NET 6) or `Task.WhenAll` with throttling.
- **Do not elide `async`/`await` inside `using` or `try`**: returning the task directly disposes the resource before the operation finishes.

```csharp
Console.WriteLine(await ReadBad());      // ObjectDisposedException caught: disposed too early
Console.WriteLine(await ReadGood());     // ok

static Task<string> ReadBad()
{
    try
    {
        using var reader = new SlowReader();
        return reader.ReadAsync();       // BAD: reader is disposed when this method returns
    }
    finally { }
}

static async Task<string> ReadGood()
{
    using var reader = new SlowReader();
    return await reader.ReadAsync();     // GOOD: disposed after the read completes
}

sealed class SlowReader : IDisposable
{
    private bool _disposed;
    public async Task<string> ReadAsync()
    {
        await Task.Delay(10);
        return _disposed ? "ObjectDisposedException caught: disposed too early" : "ok";
    }
    public void Dispose() => _disposed = true;
}
```

- **`ConfigureAwait(false)`** belongs in general-purpose libraries that may run under a UI or classic ASP.NET `SynchronizationContext`. ASP.NET Core and console apps have no `SynchronizationContext`, so application code there does not need it. Once a library uses it, use it on every `await` (including `await using` and `await foreach` via `.ConfigureAwait(false)`). `ConfigureAwaitOptions` (`SuppressThrowing`, `ForceYielding`) is .NET 8+.
- **`ValueTask`/`ValueTask<T>`** (`ValueTask<T>` .NET Core 2.0, non-generic `ValueTask` .NET Core 2.1; `System.Threading.Tasks.Extensions` package for .NET Framework/`netstandard2.0`): await exactly once; never await twice, never call `.Result` before completion, never pass to `Task.WhenAll` without `.AsTask()`. Return `Task` unless profiling shows allocation pressure on a mostly-synchronous path.
- `Task.Run` in ASP.NET Core request code only moves work to another pool thread. Use it for CPU-bound work off a UI thread, not to "make code async".
- `Thread.Sleep` in async code blocks a pool thread; use `await Task.Delay(delay, ct)`.
- `TaskCompletionSource` should be created with `TaskCreationOptions.RunContinuationsAsynchronously`; otherwise continuations run inline on the thread that calls `SetResult`, which can deadlock or reorder code.
- Async methods that complete synchronously still allocate a state machine; don't optimize this without measurements.
- `async` lambdas passed to APIs that expect `Action` (e.g. `List<T>.ForEach`, `Timer` callbacks) become `async void`. Check the delegate type.

##### Async streams (C# 8, .NET Core 3.0)

```csharp
using System.Runtime.CompilerServices;

using var cts = new CancellationTokenSource();
try
{
    await foreach (var n in CountAsync().WithCancellation(cts.Token))
    {
        Console.WriteLine(n);             // 1, 2
        if (n == 2) cts.Cancel();
    }
}
catch (OperationCanceledException) when (cts.IsCancellationRequested)
{
    Console.WriteLine("stopped");
}

static async IAsyncEnumerable<int> CountAsync([EnumeratorCancellation] CancellationToken ct = default)
{
    for (int i = 1; ; i++)
    {
        await Task.Delay(10, ct);
        yield return i;
    }
}
```

Without `[EnumeratorCancellation]`, `WithCancellation(token)` never reaches the iterator body (warning CS8425). LINQ over `IAsyncEnumerable<T>` is built in from .NET 10 (`System.Linq.AsyncEnumerable` class); earlier targets get the same API from Microsoft's `System.Linq.AsyncEnumerable` NuGet package or the older community `System.Linq.Async` package. `System.Linq.Async` 6.x conflicts with the in-box class on .NET 10: remove it or upgrade to 7.0.0, and use `<ExcludeAssets>compile</ExcludeAssets>` when it arrives transitively.[^15]

---

#### Cancellation and Concurrency

##### Cancellation

```csharp
using var cts = new CancellationTokenSource();
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

- Every public async API takes `CancellationToken cancellationToken = default` as the **last** parameter and forwards it to every async call (CA2016).
- ASP.NET Core: bind a `CancellationToken` parameter in the action/endpoint (it is `HttpContext.RequestAborted`); hosted services use the `stoppingToken`.
- `CancellationTokenSource.CreateLinkedTokenSource(a, b)` registers callbacks on the parents; **dispose** the linked source or long-lived parent tokens accumulate registrations (a memory leak).
- A `CancellationTokenSource` with a timeout holds a timer; dispose it.
- Check `ct.ThrowIfCancellationRequested()` in CPU-bound loops.
- Catch `OperationCanceledException` (the base type); `TaskCanceledException` derives from it.
- `HttpClient` throws `TaskCanceledException` on timeout too; distinguish with `ct.IsCancellationRequested` (on .NET 5+ the timeout case has an inner `TimeoutException`).

##### Synchronization

```csharp
var counter = new SafeCounter();
await Parallel.ForEachAsync(Enumerable.Range(0, 1000),          // .NET 6
    new ParallelOptions { MaxDegreeOfParallelism = 8 },
    (_, _) => { counter.Increment(); return ValueTask.CompletedTask; });
Console.WriteLine(counter.Value);    // 1000

var gate = new SemaphoreSlim(3);     // async-compatible throttle
await gate.WaitAsync();
try { /* at most 3 concurrent callers */ }
finally { gate.Release(); }

sealed class SafeCounter
{
    private readonly object _sync = new();   // System.Threading.Lock on .NET 9+ / C# 13
    private int _value;
    public void Increment() { lock (_sync) { _value++; } }
    public int Value { get { lock (_sync) { return _value; } } }
}
```

- **`await` inside `lock` does not compile** (CS1996). Use `SemaphoreSlim.WaitAsync` for async critical sections and release in `finally`. A `SemaphoreSlim` is not reentrant.
- Never `lock (this)`, `lock (typeof(T))`, or lock on strings or other publicly reachable objects.
- `System.Threading.Lock` (.NET 9, C# 13) is faster than `Monitor`; do not cast it to `object` or store it in an `object`-typed variable (that falls back to `Monitor`, warning CS9216).
- `Interlocked` for counters and flags; `ConcurrentDictionary`, `Channel<T>` for producer/consumer; immutable collections or copy-on-write for read-mostly state.
- `ConcurrentDictionary.GetOrAdd(key, factory)` may invoke the factory more than once under contention; wrap expensive or side-effecting factories in `Lazy<T>`.
- `Dictionary<TKey,TValue>`, `List<T>` and `HashSet<T>` are not thread-safe even for "mostly reads with occasional writes" — concurrent writes can corrupt them and cause infinite loops.
- Static mutable state in web apps (static caches, static `HttpContext` references) is shared by all requests.
- **Unbounded `Task.WhenAll(items.Select(...))`** over thousands of items floods downstream services; throttle with `Parallel.ForEachAsync`, `SemaphoreSlim` or a bounded `Channel`.
- Bounded channels (`Channel.CreateBounded`, .NET Core 3.0 / package) apply back-pressure; an unbounded channel fed by untrusted producers is a memory DoS.

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

---

#### IDisposable and IAsyncDisposable

`using` declarations (`using var x = ...;`) and `await using` are C# 8; `IAsyncDisposable` is .NET Core 3.0 (`Microsoft.Bcl.AsyncInterfaces` for older targets).

```csharp
await using (var resource = new AsyncResource())
{
    await resource.WorkAsync();
}                                           // DisposeAsync awaited here

using var stream = new MemoryStream();      // disposed at the end of the enclosing scope

sealed class AsyncResource : IAsyncDisposable
{
    private readonly MemoryStream _stream = new();
    private bool _disposed;

    public async Task WorkAsync()
    {
        if (_disposed) throw new ObjectDisposedException(nameof(AsyncResource));   // ObjectDisposedException.ThrowIf: .NET 7
        await _stream.WriteAsync(new byte[] { 1, 2, 3 });
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

- Every `IDisposable` you create and own must be disposed: `Stream`, `HttpResponseMessage`, `CancellationTokenSource`, `Timer`, `SqlConnection`/`DbCommand`/`DbDataReader`, `RSA`/`Aes`/`HMAC`, `JsonDocument`, `Process`, `X509Certificate2` (holds native key handles).
- Do **not** dispose what you do not own: DI-resolved services (the container disposes them), streams passed in by the caller (unless documented), `HttpClient` instances from `IHttpClientFactory` (harmless but pointless).
- Prefer `await using` for `IAsyncDisposable` types (`DbContext`, `ServiceProvider` with async-disposable services, `Utf8JsonWriter` over async streams). Disposing a `ServiceProvider` synchronously while it holds `IAsyncDisposable`-only services throws `InvalidOperationException`.
- The full `Dispose(bool disposing)` pattern with a finalizer is only for types that **directly** own unmanaged handles — and those should wrap them in a `SafeHandle`. A `sealed` class owning managed disposables needs only `Dispose()`:

```csharp
public sealed class ReportWriter : IDisposable
{
    private readonly StreamWriter _writer;
    private bool _disposed;

    public ReportWriter(string path) => _writer = new StreamWriter(path);

    public void Write(string line)
    {
        if (_disposed) throw new ObjectDisposedException(nameof(ReportWriter));
        _writer.WriteLine(line);
    }

    public void Dispose()
    {
        if (_disposed) return;
        _disposed = true;
        _writer.Dispose();
    }
}
```

  A non-sealed base class that may have derived types with unmanaged state uses `protected virtual void Dispose(bool disposing)` and `GC.SuppressFinalize(this)` in `Dispose()` (CA1063/CA1816).
- `Dispose` must be idempotent and must not throw.
- A constructor that creates several disposables must dispose the earlier ones if a later step throws.
- `JsonDocument` rents pooled memory: dispose it and do not let `JsonElement`s escape (use `element.Clone()`).
- `using` on a `null` value is safe (no call is made).

---

#### Collections

- Return the narrowest useful abstraction: `IReadOnlyList<T>`/`IReadOnlyCollection<T>` for materialized results, `IEnumerable<T>` only for lazily streamed sequences. Accept the broadest abstraction the method needs.
- Never return `null` for a collection; return an empty one (`Array.Empty<T>()`, `[]` in C# 12).
- Modifying a `List<T>` or `Dictionary<TKey,TValue>` while enumerating it throws `InvalidOperationException`. (`Dictionary.Remove` and `Clear` during enumeration are allowed on .NET Core 3.0+, but not on .NET Framework.)

```csharp
var list = new List<int> { 1, 2, 3 };
try
{
    foreach (var x in list)
        if (x == 2) list.Remove(x);
}
catch (InvalidOperationException ex)
{
    Console.WriteLine(ex.Message);   // Collection was modified; enumeration operation may not execute.
}
list.RemoveAll(x => x == 2);         // GOOD
Console.WriteLine(string.Join(",", list));   // 1,3
```

- `Dictionary<string, T>` with identifiers from users, protocols or files: pass `StringComparer.Ordinal` or `StringComparer.OrdinalIgnoreCase` explicitly; never a culture-sensitive comparer.
- Use `TryGetValue` instead of `ContainsKey` + indexer (two lookups); `TryAdd` (.NET Core 2.0) instead of `ContainsKey` + `Add`.
- Presize collections when the size is known (`new List<T>(n)`, `new Dictionary<K,V>(n)`).
- `HashSet<T>` for membership tests instead of `List<T>.Contains` in loops (O(1) vs O(n)).
- Immutable collections (`System.Collections.Immutable`, in-box in .NET Core 2.x and later, a package for .NET Framework/`netstandard2.0`) for shared, rarely changing state; frozen collections (`FrozenDictionary`, `FrozenSet`, .NET 8) for lookup tables built once at startup.
- `ReadOnlyCollection<T>`/`AsReadOnly()` is a **view**: the owner can still change the underlying list.
- Collection expressions (C# 12): `int[] a = [1, 2, 3]; List<int> l = [.. a, 4];` The target type decides the concrete type; `IEnumerable<T> x = [1, 2]` creates a compiler-synthesized read-only type — do not cast it to `List<T>`.
- `params ReadOnlySpan<T>` (C# 13) avoids the array allocation of `params T[]` for new APIs.
- `Span<T>`/`ReadOnlySpan<T>` (C# 7.2; in-box in .NET Core 2.1+, `System.Memory` package for .NET Framework) are `ref struct`s: they cannot be stored in class fields, captured by lambdas, or used across `await` (C# 13 allows them in async methods only when not live across an `await`).
- `stackalloc` only with a small constant upper bound; `stackalloc byte[userControlledLength]` is a stack-overflow DoS. Pattern: `Span<byte> buf = len <= 256 ? stackalloc byte[256] : new byte[len];` (C# 8 allows `stackalloc` in nested expressions).
- `ArrayPool<T>.Shared.Rent(n)` may return a larger array; use only the first `n` elements, return it exactly once, never use it after `Return`, and clear it (`clearArray: true`) if it held secrets.

---

#### LINQ

```csharp
int calls = 0;
IEnumerable<int> Numbers() { for (int i = 0; i < 3; i++) { calls++; yield return i; } }

var q = Numbers().Where(n => n > 0);   // deferred: nothing has run yet
Console.WriteLine(calls);              // 0
_ = q.Count();
_ = q.Any();                           // enumerates again
Console.WriteLine(calls);              // 5 (3 + 2): multiple enumeration

var threshold = 0;
var lazy = new[] { 1, 2, 3 }.Where(x => x > threshold);
threshold = 2;                         // the captured variable changes the result
Console.WriteLine(string.Join(",", lazy));    // 3

var materialized = Numbers().Where(n => n > 0).ToList();   // enumerate once
Console.WriteLine(materialized.Count); // 2 — the Count property, not Count()
```

Review rules:

- **Deferred execution**: a query runs each time it is enumerated. Multiple enumeration (CA1851) re-runs the producer — a database query, an HTTP call, an iterator with side effects. Materialize once with `ToList()`/`ToArray()` when a sequence is used more than once.
- Exceptions from a deferred query surface where it is enumerated, not where it is built — often outside the `try` block or after the `using` that owned the data source was disposed.
- `Any()` instead of `Count() > 0` for lazy sequences; the `Count`/`Length` property for materialized collections. `TryGetNonEnumeratedCount` (.NET 6) avoids enumeration when possible.
- `First`/`Single` when absence is a bug (fail fast); `FirstOrDefault`/`SingleOrDefault` only when absence is expected and handled — they return `default` (`0` for `int`, `null` for references). `Single` scans the whole sequence to prove uniqueness. The `FirstOrDefault(defaultValue)` overloads are .NET 6+.
- `OrderBy(...).OrderBy(...)` discards the first ordering; use `ThenBy`.
- `Distinct`, `GroupBy`, `ToDictionary`, `ToHashSet`, `Contains` use the default equality comparer; pass `StringComparer.OrdinalIgnoreCase` where needed. `ToDictionary` throws on duplicate keys.
- `Select` with side effects or `ToList().ForEach(...)` hides intent — use `foreach`.
- Prefer method syntax for simple queries; query syntax is fine for joins and `let`.
- LINQ in hot paths allocates enumerators and delegates; measure before rewriting. Newer runtimes (.NET 7–10) reduce this overhead considerably.
- Version-specific operators: `MaxBy`/`MinBy`/`DistinctBy`/`Chunk` (.NET 6), `Order`/`OrderDescending` (.NET 7), `CountBy`/`AggregateBy`/`Index` (.NET 9), `LeftJoin`/`RightJoin`/`Shuffle` (.NET 10). On older targets use `GroupBy`/`OrderBy` equivalents or a package (e.g. MoreLINQ).
- `Enumerable.Shuffle` and `Random` are not cryptographically secure — use `RandomNumberGenerator.Shuffle` (.NET 8) for security-relevant sampling.
- **`IQueryable<T>` is different**: the same LINQ code is translated to SQL by EF Core and fails at run time if a method cannot be translated (see [EF Core](#ef-core)).

---

#### Strings, Culture and Encoding

Strings are immutable UTF-16. Many string APIs are **culture-sensitive by default**, and the culture comes from the machine or thread. On Linux and macOS .NET always uses ICU; on Windows, .NET 5+ uses ICU on Windows 10 May 2019 Update and later, while .NET Framework and .NET Core 3.1 use NLS. Results can even differ between ICU library versions: Microsoft's .NET 5 documentation shows `"\r\n".IndexOf("\n")` returning -1 under ICU, while the same call returns 1 on .NET 6, 8 and 10 with the ICU 74 of Ubuntu 24.04 (verified). The same code can therefore behave differently after an upgrade, between developer machines and containers, or after a base-image update.[^16]

```csharp
using System.Globalization;

CultureInfo.CurrentCulture = CultureInfo.GetCultureInfo("en-US");
string softHyphen = "co\u00ADop";                                       // "co-op" with an invisible soft hyphen
Console.WriteLine(softHyphen.IndexOf("coop"));                          // 0 — culture-sensitive: ignorable characters are skipped
Console.WriteLine(softHyphen.IndexOf("coop", StringComparison.Ordinal)); // -1
Console.WriteLine(softHyphen.Contains("coop"));                         // False — Contains(string) is ordinal
Console.WriteLine(string.Compare("\u00C5", "A\u030A", StringComparison.InvariantCulture)); // 0: canonically equivalent
Console.WriteLine("\u00C5" == "A\u030A");                               // False: == is ordinal

var tr = CultureInfo.GetCultureInfo("tr-TR");
Console.WriteLine("title".ToUpper(tr));                               // TİTLE (dotted capital I)
Console.WriteLine("title".ToUpperInvariant());                        // TITLE

var de = CultureInfo.GetCultureInfo("de-DE");
Console.WriteLine(double.Parse("1,5", de));                           // 1.5 (printed with en-US)
Console.WriteLine(double.Parse("1,5", CultureInfo.InvariantCulture)); // 15 — ',' is a group separator
Console.WriteLine(1234.5.ToString(de));                               // 1234,5
Console.WriteLine(string.Compare("a", "B", StringComparison.Ordinal) > 0);           // True ('a' = 97 > 'B' = 66)
Console.WriteLine(string.Compare("a", "B", StringComparison.InvariantCulture) < 0);  // True (linguistic order)
```

Rules:

- **Identifiers, protocol data, file names, HTTP headers, keys, enum names, SQL/JSON tokens**: always `StringComparison.Ordinal` or `OrdinalIgnoreCase` (CA1309/CA1310). Use culture-sensitive comparison only for text shown to users (sorting names in a UI).
- `string.IndexOf(string)`, `StartsWith(string)`, `EndsWith(string)`, `Compare`, `CompareTo` and `ToUpper()`/`ToLower()` without arguments use the **current culture**. `string.Equals`, `==`, `Contains(string)`, `Replace(string, string)` and the `char` overloads are ordinal.
- Case-insensitive equality: `string.Equals(a, b, StringComparison.OrdinalIgnoreCase)`, not `a.ToLower() == b.ToLower()` (allocates and is culture-dependent — the Turkish-I bug).
- **Formatting and parsing for machines** (logs, files, APIs, invariant storage): `CultureInfo.InvariantCulture` (CA1305). `double.Parse("1,5")` returns `1.5` on a German server and `15` on an English one. `DateTime.Parse` without a culture and format is equally fragile — use `ParseExact`/`TryParseExact` with `InvariantCulture` or ISO 8601 (`"O"`).
- String interpolation (`$"{price}"`) uses the current culture. Use `FormattableString.Invariant($"...")`, `string.Create(CultureInfo.InvariantCulture, $"...")` (.NET 6) or explicit `ToString(CultureInfo.InvariantCulture)` for machine-readable output.
- `<InvariantGlobalization>true</InvariantGlobalization>` (common in containers and Native AOT) makes every culture behave like the invariant culture; culture-specific formatting then silently disappears. Creating a specific culture (`new CultureInfo("de-DE")`) throws `CultureNotFoundException` in that mode on .NET 6+ (verified on 6.0.36 and 8.0.32) unless `PredefinedCulturesOnly` is set to `false`.
- Build strings in loops with `StringBuilder` (or `string.Join`/`string.Concat`), not `+=`.
- `string.IsNullOrWhiteSpace` for user input checks; `Trim()` does not remove every Unicode separator you might expect from user input — normalize (`Normalize(NormalizationForm.FormC)`) before comparing user-visible identifiers.
- Encoding: `Encoding.UTF8.GetBytes` (no BOM); `new UTF8Encoding(false)` when writing files that must not start with a BOM (`Encoding.UTF8` passed to a `StreamWriter` writes one). `Encoding.Default` is the ANSI code page on .NET Framework but UTF-8 on .NET Core/.NET 5+ — code that relies on it changes behaviour after a port.
- Regex: specify `RegexOptions.CultureInvariant` with `IgnoreCase`; set a timeout (`new Regex(pattern, options, TimeSpan.FromSeconds(1))`) or use `RegexOptions.NonBacktracking` (.NET 7) for patterns or input from users; use `[GeneratedRegex]` (.NET 7) or a static cached `Regex` instead of constructing one per call.
- Raw string literals (C# 11) for JSON/SQL/regex in code; verbatim strings (`@"..."`) on older versions.

---

#### Numbers, Dates and Times

```csharp
using System.Globalization;

Console.WriteLine(0.1 + 0.2 == 0.3);              // False (binary floating point)
Console.WriteLine(0.1m + 0.2m == 0.3m);           // True  (decimal)
Console.WriteLine(Math.Round(2.5));               // 2 — banker's rounding (MidpointRounding.ToEven)
Console.WriteLine(Math.Round(2.5, MidpointRounding.AwayFromZero));   // 3

int big = int.MaxValue;
Console.WriteLine(unchecked(big + 1));            // -2147483648: silent overflow by default
try { Console.WriteLine(checked(big + 1)); }
catch (OverflowException) { Console.WriteLine("overflow"); }

var utc = new DateTimeOffset(2026, 3, 29, 0, 30, 0, TimeSpan.Zero);
var berlin = TimeZoneInfo.FindSystemTimeZoneById("Europe/Berlin");
const string Fmt = "yyyy-MM-dd HH:mm zzz";
Console.WriteLine(TimeZoneInfo.ConvertTime(utc, berlin).ToString(Fmt, CultureInfo.InvariantCulture));            // 2026-03-29 01:30 +01:00
Console.WriteLine(TimeZoneInfo.ConvertTime(utc.AddHours(1), berlin).ToString(Fmt, CultureInfo.InvariantCulture)); // 2026-03-29 03:30 +02:00 (DST starts)
```

- **Money**: `decimal`, never `double`/`float`. Specify the rounding mode explicitly; `Math.Round` defaults to banker's rounding.
- Integer arithmetic overflows silently in the default `unchecked` context. Use `checked` (or `<CheckForOverflowUnderflow>`) for values derived from user input, sizes and money in minor units.
- Never compare floating-point values with `==`; use a tolerance.
- `int.Parse` throws on bad input; use `TryParse` with `NumberStyles` and `CultureInfo.InvariantCulture` for external data.
- **Timestamps**: store and transmit UTC (`DateTimeOffset.UtcNow` or `DateTime.UtcNow`); `DateTime.Now` depends on the server's time zone and is ambiguous during DST transitions. `DateTimeOffset` keeps the offset and is preferred for points in time.
- `DateTime.Kind` (`Unspecified` is the default for parsed values) is a common source of off-by-hours bugs; `ToUniversalTime()` on an `Unspecified` value assumes local time.
- Time zone IDs: IANA IDs (`Europe/Berlin`) work on Linux and, with ICU, on Windows from .NET 6; Windows IDs (`W. Europe Standard Time`) work on Linux from .NET 6 as well (conversion via ICU). On .NET Framework only Windows IDs exist.
- `DateOnly`/`TimeOnly` (.NET 6) for dates without time (birthdays, business days).
- **Testable time**: inject `TimeProvider` (.NET 8; `Microsoft.Bcl.TimeProvider` package for older targets) or an own `IClock` interface instead of calling `DateTime.UtcNow` directly; `FakeTimeProvider` (`Microsoft.Extensions.TimeProvider.Testing`) in tests.
- `Stopwatch` (or `Stopwatch.GetTimestamp`/`GetElapsedTime`, .NET 7) for measuring durations, never differences of `DateTime.Now`.
- `Random` is not thread-safe before `Random.Shared` (.NET 6); never use `Random` for security (see [Security](#security)). `new Random()` in a tight loop on .NET Framework produces identical sequences (time-based seed).
- `Guid.NewGuid()` is random (v4); `Guid.CreateVersion7()` (.NET 9) produces time-ordered GUIDs that index better as database keys. Neither is a secret token generator by contract — use `RandomNumberGenerator` for tokens.

---

#### Dependency Injection, Options and Hosting

`Microsoft.Extensions.DependencyInjection`, `Options`, `Configuration`, `Logging` and the Generic Host exist since .NET Core 2.x/3.0 and are available as packages for .NET Framework and `netstandard2.0`. `WebApplication.CreateBuilder` is .NET 6; `Host.CreateApplicationBuilder` is .NET 7.

##### Lifetimes and captive dependencies

```csharp
using Microsoft.Extensions.DependencyInjection;

var services = new ServiceCollection();
services.AddScoped<IUnitOfWork, UnitOfWork>();
services.AddSingleton<ReportCache>();            // depends on a scoped service: captive dependency

try
{
    using var root = services.BuildServiceProvider(new ServiceProviderOptions { ValidateScopes = true, ValidateOnBuild = true });
}
catch (AggregateException ex)
{
    Console.WriteLine(ex.InnerException?.Message);
    // Error while validating the service descriptor 'ServiceType: ReportCache ...': Cannot consume scoped service 'IUnitOfWork' from singleton 'ReportCache'.
}

interface IUnitOfWork { }
sealed class UnitOfWork : IUnitOfWork { }
sealed class ReportCache
{
    public ReportCache(IUnitOfWork uow) { }
}
```

- **Singleton → scoped** (a singleton holding a `DbContext`, a scoped repository or `IOptionsSnapshot<T>`) is a captive dependency: the scoped instance lives forever and is shared across threads. ASP.NET Core enables `ValidateScopes`/`ValidateOnBuild` only in the Development environment.
- Singletons and background services create scopes explicitly: `using var scope = scopeFactory.CreateScope();` (`CreateAsyncScope()`, .NET 6, when services are `IAsyncDisposable`).
- `services.BuildServiceProvider()` inside registration code creates a second container with duplicate singletons (ASP0000).
- Transient `IDisposable` services resolved from the root provider are kept until the root is disposed — a memory leak in long-running apps. Resolve them inside a scope.
- Constructor injection; no service locator (`IServiceProvider` injected into domain classes). Constructors stay cheap and side-effect free.
- Keyed services (`AddKeyedSingleton`, `[FromKeyedServices]`) are .NET 8+; on older versions use a factory or named options.
- Constructor null checks for injected services are optional in application code with nullable enabled (the container never passes `null`); keep them in reusable libraries.

##### Options pattern

```csharp
using System.ComponentModel.DataAnnotations;
using Microsoft.Extensions.Configuration;
using Microsoft.Extensions.DependencyInjection;
using Microsoft.Extensions.Hosting;
using Microsoft.Extensions.Options;   // ValidateDataAnnotations: Microsoft.Extensions.Options.DataAnnotations package

var builder = Host.CreateApplicationBuilder();
builder.Configuration.AddInMemoryCollection(new Dictionary<string, string?> { ["Email:Port"] = "70000" });
builder.Services.AddOptions<EmailOptions>()
    .Bind(builder.Configuration.GetSection(EmailOptions.Section))
    .ValidateDataAnnotations()
    .ValidateOnStart();                 // .NET 6: fail at startup instead of on first use

using var host = builder.Build();
try
{
    await host.StartAsync();
}
catch (OptionsValidationException ex)
{
    Console.WriteLine(ex.Failures.Count());   // 2 (SmtpServer is required, Port out of range)
}

public sealed class EmailOptions
{
    public const string Section = "Email";
    [Required] public string? SmtpServer { get; set; }
    [Range(1, 65535)] public int Port { get; set; } = 587;
}
```

- `IOptions<T>` is a singleton snapshot read once; `IOptionsSnapshot<T>` is scoped (re-read per request, not injectable into singletons); `IOptionsMonitor<T>` supports change notifications in singletons.
- Without validation, a misconfigured value surfaces on the first request that reads `.Value`. Use `ValidateDataAnnotations()` + `ValidateOnStart()` (.NET 6) or `IValidateOptions<T>`.
- Options classes declare nullable or `required` properties honestly; a non-nullable `string` property that configuration never sets stays `null` (and .NET 10's binder writes explicit JSON `null` values into properties).[^17]
- **Secrets never go into `appsettings.json` in source control.** Use environment variables (`ConnectionStrings__Default`), user secrets (Development only), or a secret store (Azure Key Vault, AWS Secrets Manager, Kubernetes secrets). Do not log bound options objects that contain secrets.

##### Hosted and background services

```csharp
using Microsoft.Extensions.DependencyInjection;
using Microsoft.Extensions.Hosting;
using Microsoft.Extensions.Logging;

var builder = Host.CreateApplicationBuilder();
builder.Services.AddHostedService<QueueWorker>();
using var host = builder.Build();
await host.StartAsync();
await Task.Delay(150);
await host.StopAsync();

sealed class QueueWorker : BackgroundService
{
    private readonly ILogger<QueueWorker> _logger;
    public QueueWorker(ILogger<QueueWorker> logger) => _logger = logger;

    protected override async Task ExecuteAsync(CancellationToken stoppingToken)
    {
        using var timer = new PeriodicTimer(TimeSpan.FromMilliseconds(50));   // .NET 6
        try
        {
            while (await timer.WaitForNextTickAsync(stoppingToken))
            {
                try
                {
                    Console.WriteLine("tick");
                }
                catch (Exception ex)            // one failed iteration must not kill the worker
                {
                    _logger.LogError(ex, "Tick failed");
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

- Since .NET 6, an unhandled exception in `ExecuteAsync` **stops the host** (`BackgroundServiceExceptionBehavior.StopHost`); on .NET Core 3.1/.NET 5 it was silently ignored. Catch and log per iteration; do not switch to `Ignore` to hide failures.
- Before .NET 10, the synchronous part of `ExecuteAsync` (before the first `await`) runs on the startup path and blocks other services from starting; from .NET 10 the whole method runs on a background thread.[^17] Long synchronous work at the top of `ExecuteAsync` is a finding on every version.
- Use `PeriodicTimer` (.NET 6) or `Task.Delay` loops with the stopping token; `System.Threading.Timer` callbacks can overlap and swallow exceptions.
- Container orchestrators send SIGTERM and wait for a grace period (Kubernetes: 30 s by default); keep `HostOptions.ShutdownTimeout` below it. The default shutdown timeout is 30 s from .NET 6 (5 s before).
- Console apps **without** the Generic Host on .NET 10 no longer get a default SIGTERM handler; `AppDomain.ProcessExit` cleanup does not run on `docker stop`. Use the Generic Host or `PosixSignalRegistration` (.NET 6).[^17]

---

#### Logging and Observability

```csharp
using Microsoft.Extensions.Logging;

using var factory = LoggerFactory.Create(b => b.AddSimpleConsole(o => o.SingleLine = true));
var logger = factory.CreateLogger("Orders");

int orderId = 42;
logger.LogInformation("Order {OrderId} shipped", orderId);    // GOOD: constant template, structured
// logger.LogInformation($"Order {orderId} shipped");         // BAD: CA2254 — no structure, always allocates
Log.OrderCreated(logger, orderId, "EUR");

static partial class Log
{
    [LoggerMessage(EventId = 1001, Level = LogLevel.Information, Message = "Order {OrderId} created in {Currency}")]
    public static partial void OrderCreated(ILogger logger, int orderId, string currency);   // .NET 6 source generator
}
```

- **Message templates are constants** with named placeholders (`{OrderId}`); never string interpolation or concatenation in log calls (CA2254). Interpolation defeats structured logging, allocates even when the level is disabled, and can let user input inject template syntax.
- Pass the exception object as the first argument: `logger.LogError(ex, "Import of {File} failed", file)`; logging only `ex.Message` loses the stack trace.
- Hot paths: `[LoggerMessage]` source generation (.NET 6) or `LoggerMessage.Define` (all versions), or guard expensive arguments with `logger.IsEnabled(LogLevel.Debug)`.
- **Never log secrets, tokens, passwords, connection strings, full request/response bodies or unredacted PII.** `Microsoft.Extensions.Compliance.Redaction` (.NET 8) supports redaction of classified data.
- Log levels: `Error` for failures that need attention, `Warning` for degraded behaviour, `Information` for business events, `Debug`/`Trace` for diagnostics. Do not log the same exception at every layer.
- User-controlled values in log messages can forge log lines when written to plain-text sinks; structured logging with a JSON formatter neutralizes this.
- Metrics and tracing: `System.Diagnostics.Metrics.Meter` (.NET 6; `IMeterFactory` in DI from .NET 8) and `ActivitySource` (.NET 5), exported with OpenTelemetry. Keep tag cardinality bounded — no user IDs, raw URLs or GUIDs as tag values.
- Health checks (`AddHealthChecks`, ASP.NET Core 2.2+) separate liveness from readiness; readiness checks may hit dependencies, liveness checks must not.

---

#### HttpClient

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
Console.WriteLine(gh.BaseAddress);       // https://api.github.com/

sealed class GitHubClient
{
    private readonly HttpClient _http;
    public GitHubClient(HttpClient http) => _http = http;
    public Uri? BaseAddress => _http.BaseAddress;

    public async Task<Repo?> GetRepoAsync(string owner, string name, CancellationToken ct)
    {
        using var response = await _http.GetAsync(
            $"repos/{Uri.EscapeDataString(owner)}/{Uri.EscapeDataString(name)}", ct);
        if (response.StatusCode == System.Net.HttpStatusCode.NotFound) return null;
        response.EnsureSuccessStatusCode();
        return await response.Content.ReadFromJsonAsync<Repo>(cancellationToken: ct);
    }
}

sealed record Repo(string Name, int StargazersCount);
```

- **Never `new HttpClient()` per request** (socket exhaustion, `TIME_WAIT`), and never a single static `HttpClient` with the default handler in long-running processes (stale DNS). Use `IHttpClientFactory` (.NET Core 2.1; package for .NET Framework) or one long-lived `HttpClient` over `SocketsHttpHandler { PooledConnectionLifetime = TimeSpan.FromMinutes(2) }` (.NET Core 2.1+).
- **Typed clients must not be captured in singletons**: the factory rotates handlers, and a typed client stored in a singleton keeps one handler forever. Inject `IHttpClientFactory` into singletons.
- Set a `Timeout` (default 100 s) and pass a `CancellationToken`.
- Dispose `HttpResponseMessage` (`using var response`), or use `GetFromJsonAsync` (System.Net.Http.Json, .NET 5 in-box / package). Use `HttpCompletionOption.ResponseHeadersRead` and stream large bodies.
- `EnsureSuccessStatusCode()` or explicit status handling — never deserialize an error body as a success DTO.
- Escape path segments (`Uri.EscapeDataString`) and build query strings with `QueryHelpers.AddQueryString` or encoding helpers, never by concatenating user input.
- Do not disable certificate validation (`ServerCertificateCustomValidationCallback = (_, _, _, _) => true`, `HttpClientHandler.DangerousAcceptAnyServerCertificateValidator`) outside tests.
- Resilience: `Microsoft.Extensions.Http.Resilience` (`AddStandardResilienceHandler()`, .NET 8) or Polly-based handlers on older versions. Retry only idempotent requests; do not stack several retry layers.
- **SSRF**: forwarding user-supplied URLs to `HttpClient` lets attackers reach internal services and cloud metadata endpoints. Validate scheme and host against an allow-list and block private, loopback and link-local addresses after DNS resolution.

---

#### JSON Serialization

System.Text.Json is in-box from .NET Core 3.0 (NuGet package for .NET Framework/`netstandard2.0`); Newtonsoft.Json remains common. Defaults differ and are a frequent source of bugs.

```csharp
using System.Text.Json;

string dup = """{ "Value": 1, "Value": -1 }""";
Console.WriteLine(JsonSerializer.Deserialize<Rec>(dup)!.Value);                          // -1: last duplicate wins
Console.WriteLine(JsonSerializer.Deserialize<Rec>("""{"value":3}""")!.Value);            // 0: case-sensitive by default
var web = new JsonSerializerOptions(JsonSerializerDefaults.Web);                         // .NET 5
Console.WriteLine(JsonSerializer.Deserialize<Rec>("""{"value":3}""", web)!.Value);       // 3
Console.WriteLine(JsonSerializer.Deserialize<Rec>("""{"Value":3,"Extra":2}""")!.Value);  // 3: unknown members ignored
Console.WriteLine(JsonSerializer.Deserialize<Person>("""{"Name":null}""")!.Name is null); // True: null in a non-nullable property

record Rec(int Value);
record Person(string Name);
```

- **Cache `JsonSerializerOptions`** in a `static readonly` field or DI. Creating options per call rebuilds the metadata cache every time — a severe performance problem (CA1869).
- ASP.NET Core and `JsonSerializerDefaults.Web` use camelCase and case-insensitive matching and accept numbers in strings; plain `JsonSerializer` calls do not. Mixing them causes "all fields are default" bugs.
- Strict input: `UnmappedMemberHandling.Disallow` (.NET 8), `RespectNullableAnnotations` and `RespectRequiredConstructorParameters` (.NET 9), `AllowDuplicateProperties = false` and `JsonSerializerOptions.Strict` (.NET 10). Duplicate keys are a parser-differential attack vector (one component checks the first `"role"`, another uses the last).[^18]
- Polymorphism: System.Text.Json only via `[JsonPolymorphic]`/`[JsonDerivedType]` allow-lists (.NET 7). **Newtonsoft.Json `TypeNameHandling` other than `None` on untrusted input is remote code execution**; so is a custom `SerializationBinder` that allows arbitrary types.
- Use `SerializeAsync`/`DeserializeAsync` with streams for large payloads instead of `ReadAsStringAsync` + `Deserialize`.
- Keep `MaxDepth` at its default (64) for untrusted input.
- Source generation (`JsonSerializerContext`, .NET 6) is required for trimming/Native AOT and speeds up startup.
- Dates: System.Text.Json reads and writes ISO 8601 only; Newtonsoft's `DateParseHandling` silently converts date-like strings to `DateTime` (and can shift them by the local offset).
- `JsonDocument` is `IDisposable` (pooled memory); `JsonNode` (.NET 6) is a mutable DOM without pooling.

---

#### ASP.NET Core

Minimal APIs are .NET 6; `TypedResults` and route groups (`MapGroup`) .NET 7; `IExceptionHandler` .NET 8; built-in minimal API validation (`AddValidation()`) .NET 10.[^19]

```csharp
using System.ComponentModel.DataAnnotations;
using Microsoft.AspNetCore.Http.HttpResults;

var builder = WebApplication.CreateBuilder(args);
builder.Services.AddProblemDetails();                  // .NET 7
builder.Services.AddAuthentication().AddCookie();
builder.Services.AddAuthorization();
builder.Services.AddSingleton<IProductStore, InMemoryProductStore>();

var app = builder.Build();
app.UseExceptionHandler();
app.UseAuthentication();
app.UseAuthorization();

var products = app.MapGroup("/products").RequireAuthorization();
products.MapGet("/{sku}", async Task<Results<Ok<Product>, NotFound>> (string sku, IProductStore store, CancellationToken ct) =>
    await store.FindAsync(sku, ct) is { } p ? TypedResults.Ok(p) : TypedResults.NotFound());

await app.StartAsync();
Console.WriteLine("started");
await app.StopAsync();

public sealed record Product([property: Required, StringLength(32)] string Sku, [property: Range(1, 10_000)] int Quantity);

public interface IProductStore { Task<Product?> FindAsync(string sku, CancellationToken ct); }

sealed class InMemoryProductStore : IProductStore
{
    public Task<Product?> FindAsync(string sku, CancellationToken ct) =>
        Task.FromResult<Product?>(sku == "A1" ? new Product("A1", 3) : null);
}
```

Security review points (all versions):

- **Authorization on every endpoint**: `[Authorize]`/`RequireAuthorization()` on controllers or route groups, or a fallback policy (`AuthorizationOptions.FallbackPolicy`) that requires authenticated users. `[AllowAnonymous]` needs a reason.
- **Object-level authorization**: check that the caller may access *this* resource (`orders/{id}` belonging to another tenant/user) — the most common API vulnerability (OWASP API1, BOLA). Authentication alone is not authorization.
- **Middleware order**: `UseRouting` → `UseCors` → `UseAuthentication` → `UseAuthorization` → endpoints. Authorization middleware missing or placed after the endpoints is a security bug. With `WebApplication` (.NET 6) routing is added automatically, but the auth middleware order still matters.
- **Validation**: controllers with `[ApiController]` return 400 automatically for invalid models; minimal APIs validate nothing before .NET 10, and on .NET 10 only after `builder.Services.AddValidation()`. Validate input explicitly otherwise.
- **Mass assignment / over-posting**: never bind request bodies directly to EF entities; use request DTOs.
- **CSRF**: cookie-authenticated endpoints that accept forms need antiforgery (`[ValidateAntiForgeryToken]`/`AutoValidateAntiforgeryToken` in MVC; `AddAntiforgery()`+`UseAntiforgery()` for minimal APIs in .NET 8). `.DisableAntiforgery()` or `[IgnoreAntiforgeryToken]` needs a reason.
- **CORS**: `AllowAnyOrigin()` together with `AllowCredentials()` is rejected by the framework; `SetIsOriginAllowed(_ => true)` + `AllowCredentials()` is the same hole. Allow-list origins.
- **Errors**: RFC 7807/9457 problem details (`AddProblemDetails`, .NET 7); `UseDeveloperExceptionPage` only in Development; never return exception messages or stack traces to clients.
- **HTTPS and headers**: `UseHttpsRedirection`, `UseHsts` (non-Development), secure cookie flags (`Secure`, `HttpOnly`, `SameSite`).
- **Forwarded headers** behind a proxy: `UseForwardedHeaders` with `KnownProxies`/`KnownIPNetworks` (`KnownNetworks` is obsolete in .NET 10); trusting `X-Forwarded-For` from anyone lets clients spoof their IP.
- **Open redirects**: validate `returnUrl` with `Url.IsLocalUrl` or use `LocalRedirect`.
- **Rate limiting** (`AddRateLimiter`, .NET 7; a package or custom middleware before) on login, sign-up, password reset and expensive endpoints; request body size limits for uploads.
- **File uploads**: never trust `IFormFile.FileName` or `ContentType`; generate server-side names, check size and content, store outside the web root.
- **Kestrel patch level**: CVE-2025-55315 (HTTP request smuggling, October 2025) is fixed in 8.0.21, 9.0.10 and 10.0.0-rc.2 (October 14, 2025) and later; unsupported versions (.NET 6/7, .NET Core 3.1) received no fix.[^20]
- Cookie authentication returns 401/403 instead of login redirects for API endpoints from .NET 10; SPAs that relied on following redirects need updating.[^19]
- `WebHostBuilder`/`IWebHost` (obsolete in .NET 10) and `Startup` classes still work; do not demand a rewrite to minimal hosting in a review unless the project is migrating anyway.

---

#### EF Core

EF Core versions follow .NET: EF Core 8 targets `net8.0`, EF Core 9 `net8.0`, EF Core 10 `net10.0`, EF Core 3.1 `netstandard2.0`. Version-specific APIs: split queries and filtered `Include` (EF Core 5), `ExecuteUpdate`/`ExecuteDelete`, `FromSql` and scalar `SqlQuery<T>` (EF Core 7), complex types and `SqlQuery<T>` for unmapped types (EF Core 8), `LeftJoin`/`RightJoin` translation and `ExecuteUpdate` with statement lambdas (EF Core 10).[^21]

```csharp
using Microsoft.EntityFrameworkCore;

await using var db = new ShopContext();
await db.Database.OpenConnectionAsync();
await db.Database.EnsureCreatedAsync();
db.Blogs.Add(new Blog { Name = "a", Posts = { new Post { Title = "p1" }, new Post { Title = "p2" } } });
db.Blogs.Add(new Blog { Name = "b" });
await db.SaveChangesAsync();
db.ChangeTracker.Clear();

// GOOD: one query, read-only, projected
var summaries = await db.Blogs
    .AsNoTracking()
    .Select(b => new { b.Name, PostCount = b.Posts.Count })
    .ToListAsync();
Console.WriteLine(string.Join(",", summaries.Select(s => $"{s.Name}:{s.PostCount}")));   // a:2,b:0

// GOOD: interpolated raw SQL is parameterized (EF Core 7 FromSql; FromSqlInterpolated since 3.0)
var name = "a' OR 1=1 --";
Console.WriteLine(await db.Blogs.FromSql($"SELECT * FROM Blogs WHERE Name = {name}").CountAsync());   // 0
// BAD: FromSqlRaw("... WHERE Name = '" + name + "'") — SQL injection (EF1003 / CA2100)

// Bulk update without loading entities (EF Core 7+): bypasses the change tracker
int affected = await db.Blogs.ExecuteUpdateAsync(s => s.SetProperty(b => b.Views, b => b.Views + 1));
Console.WriteLine(affected);                                                                  // 2

sealed class ShopContext : DbContext
{
    public DbSet<Blog> Blogs => Set<Blog>();
    public DbSet<Post> Posts => Set<Post>();
    protected override void OnConfiguring(DbContextOptionsBuilder o) => o.UseSqlite("Data Source=:memory:");
}

sealed class Blog
{
    public int Id { get; set; }
    public string Name { get; set; } = "";
    public int Views { get; set; }
    public List<Post> Posts { get; } = new();
}

sealed class Post
{
    public int Id { get; set; }
    public string Title { get; set; } = "";
    public int BlogId { get; set; }
}
```

Query pitfalls:

- **N+1 queries**: loading a list and touching a navigation per item (lazy-loading proxies, `Entry(x).Collection(...).Load()` in a loop) issues one query per row. Use `Include`/`ThenInclude`, projection (`Select`), or `AsSplitQuery()` (EF Core 5) for several collection includes (cartesian explosion).
- **Client evaluation**: EF Core 3.0+ throws `InvalidOperationException` ("could not be translated") for untranslatable expressions in `Where`/`OrderBy`; EF Core 2.x silently evaluated them in memory. Calling `AsEnumerable()`/`ToList()` early to "fix" the error pulls the whole table into memory.
- **Tracking**: queries track entities by default. Use `AsNoTracking()` for read-only queries. Identity resolution returns the **same, possibly stale** instance for a second query in the same context; `ExecuteUpdate`/`ExecuteDelete` do not update tracked entities.
- `ExecuteUpdate`/`ExecuteDelete` bypass `SaveChanges` interceptors, change tracking, and concurrency tokens unless filtered on them.
- **Async all the way** with a `CancellationToken`: `ToListAsync`, `FirstOrDefaultAsync`, `SaveChangesAsync`. `IQueryable` methods without `Async` block the thread.
- **`DbContext` is not thread-safe** and is registered as scoped. Running two queries concurrently on one context (`Task.WhenAll`) throws "A second operation was started on this context instance". Use `IDbContextFactory<T>` (EF Core 5) for parallel or background work.
- Pagination: `Skip`/`Take` need a deterministic `OrderBy`; prefer keyset pagination for deep pages.
- `.Count()` on `IQueryable` runs `SELECT COUNT(*)`; `.ToList().Count` loads everything.
- Long-lived contexts accumulate tracked entities (memory, slow `DetectChanges`); keep units of work short.
- `Contains` over a local collection: EF Core 8+ sends the list as one JSON parameter (SQL Server/SQLite), EF Core 10 as multiple scalar parameters by default; very large lists can hit parameter limits (2,100 on SQL Server).[^21]

Raw SQL:

- `FromSql($"...")`, `FromSqlInterpolated`, `ExecuteSql($"...")`, `SqlQuery<T>($"...")` turn interpolation holes into `DbParameter`s. `FromSqlRaw`/`ExecuteSqlRaw` with concatenated or `string.Format`-ed input are SQL injection; EF Core's analyzer reports EF1003 for concatenation and EF1002 for an interpolated string passed to a `*Raw` method (verified with EF Core 10.0.13).
- Building the interpolated string in a variable first (`var sql = $"...{x}"; db.Blogs.FromSqlRaw(sql)`) loses parameterization — pass the `FormattableString` directly. The analyzer does **not** flag this variable form (verified), so reviewers must.
- Identifiers (table/column names, `ORDER BY` columns) cannot be parameters; map user input through an allow-list.
- ADO.NET/Dapper: `command.Parameters.Add(...)` or Dapper parameter objects; CA2100 flags concatenated `CommandText`.

Model and migrations:

- Review generated migrations: renames generated as drop + add lose data; type changes can truncate.
- Do not run `Database.Migrate()` from several app instances at startup; run migrations as a deployment step (SQL scripts or migration bundles, EF Core 6).
- Configure `decimal` precision (`HasPrecision`, EF Core 5; `HasColumnType` before), string lengths, indexes and concurrency tokens (`IsRowVersion`).
- `EnableSensitiveDataLogging()` logs parameter values (PII, secrets) — Development only.
- The EF Core `InMemory` provider is not a relational database (no transactions, no SQL translation errors); do not use it to test query behaviour.

---

#### Security

##### Deserialization

- `BinaryFormatter`, `SoapFormatter`, `NetDataContractSerializer`, `LosFormatter`, `ObjectStateFormatter` on untrusted data are remote code execution on every platform, including .NET Framework where they still work. On .NET 9+ `BinaryFormatter` always throws.[^11]
- Newtonsoft.Json `TypeNameHandling.All/Auto/Objects/Arrays`, `JavaScriptSerializer` with a type resolver, and `XmlSerializer`/`DataContractSerializer` constructed from a type name read from input are equally dangerous.
- System.Text.Json polymorphism only through `[JsonDerivedType]` allow-lists.

##### Path traversal

```csharp
string root = Path.GetFullPath(Path.Combine(Path.GetTempPath(), "uploads"));
Directory.CreateDirectory(root);

Console.WriteLine(Path.Combine(root, "/etc/passwd"));              // /etc/passwd — an absolute segment discards root
Console.WriteLine(TryResolve(root, "../../etc/passwd", out _));    // False
Console.WriteLine(TryResolve(root, "a/b.txt", out var ok));        // True

static bool TryResolve(string root, string userPath, out string fullPath)
{
    // A trailing separator prevents "/uploads-evil" from passing a "/uploads" prefix check
    string rootWithSep = root.EndsWith(Path.DirectorySeparatorChar) ? root : root + Path.DirectorySeparatorChar;
    fullPath = Path.GetFullPath(Path.Combine(rootWithSep, userPath));
    return fullPath.StartsWith(rootWithSep,
        OperatingSystem.IsWindows() ? StringComparison.OrdinalIgnoreCase : StringComparison.Ordinal);
}
```

- `Path.Combine`/`Path.Join` do not sanitize; canonicalize with `Path.GetFullPath` and prefix-check against the root **with a trailing separator**.
- Use `Path.GetFileName(upload.FileName)` and preferably a server-generated file name.
- ZIP extraction: code that iterates entries and calls `Path.Combine(dest, entry.FullName)` itself is "Zip Slip"; `ZipFile.ExtractToDirectory` validates entry paths. Also cap the total uncompressed size (zip bombs).

##### Cryptography

```csharp
using System.Security.Cryptography;
using System.Text;

// Password hashing: PBKDF2 (Rfc2898DeriveBytes.Pbkdf2, .NET 6) with a random salt
byte[] salt = RandomNumberGenerator.GetBytes(16);
byte[] hash = Rfc2898DeriveBytes.Pbkdf2(Encoding.UTF8.GetBytes("correct horse"), salt, 600_000, HashAlgorithmName.SHA256, 32);
byte[] attempt = Rfc2898DeriveBytes.Pbkdf2(Encoding.UTF8.GetBytes("correct horse"), salt, 600_000, HashAlgorithmName.SHA256, 32);
Console.WriteLine(CryptographicOperations.FixedTimeEquals(hash, attempt));   // True — constant-time comparison

// Authenticated encryption (AesGcm: .NET Core 3.0; tag-size constructor: .NET 8)
byte[] key = RandomNumberGenerator.GetBytes(32);
byte[] nonce = RandomNumberGenerator.GetBytes(12);   // unique per message; never reuse with the same key
byte[] plaintext = Encoding.UTF8.GetBytes("secret");
byte[] ciphertext = new byte[plaintext.Length];
byte[] tag = new byte[16];
using (var aes = new AesGcm(key, tag.Length))
    aes.Encrypt(nonce, plaintext, ciphertext, tag);
byte[] decrypted = new byte[ciphertext.Length];
using (var aes = new AesGcm(key, tag.Length))
    aes.Decrypt(nonce, ciphertext, tag, decrypted);
Console.WriteLine(Encoding.UTF8.GetString(decrypted));                       // secret

// Tokens
string token = Convert.ToBase64String(RandomNumberGenerator.GetBytes(32));
Console.WriteLine(token.Length);                                             // 44
```

- **Never `System.Random`** for tokens, salts, passwords, nonces or anything that must be unguessable; use `RandomNumberGenerator` (`GetBytes(int)` .NET 6; `RandomNumberGenerator.Create().GetBytes(buffer)` on older targets).
- Compare MACs, hashes and tokens with `CryptographicOperations.FixedTimeEquals` (.NET Core 2.1), never `SequenceEqual` or `==`.
- **Passwords**: ASP.NET Core Identity's `PasswordHasher<T>`, PBKDF2 with at least 600,000 iterations of HMAC-SHA256 (OWASP), or Argon2id/bcrypt from vetted libraries. Never MD5, SHA-1 or a single unsalted SHA-256. On .NET Framework, `new Rfc2898DeriveBytes(password, salt, iterations, HashAlgorithmName.SHA256)` (4.7.2+) is the available API; its single-parameter-hash constructors default to SHA-1.
- **Symmetric encryption**: AES-GCM or ChaCha20-Poly1305 (authenticated). AES-CBC without a MAC, ECB mode, static or reused IVs/nonces and hard-coded keys are findings.
- **Asymmetric**: RSA ≥ 2048 bits with OAEP (SHA-256) or PSS; ECDSA P-256 or stronger. Dispose key objects.
- **TLS**: do not set `ServicePointManager.SecurityProtocol`/`SslProtocols` to a fixed old version; let the OS choose (TLS 1.2+). On .NET Framework, target 4.7+ so the OS default applies.
- **Certificates**: on .NET 9+ load with `X509CertificateLoader` (the byte/path constructors are obsolete, SYSLIB0057); never disable chain validation.
- Keys and secrets come from a key store or environment, never from source code; ASP.NET Core Data Protection keys must be persisted to shared, protected storage when running more than one instance.

##### Injection and parsing

- **SQL**: parameterized queries only (see [EF Core](#ef-core)).
- **XML**: `XmlReader.Create` prohibits DTDs by default; `XmlDocument`/`XmlTextReader` on .NET Framework before 4.5.2 resolve external entities (XXE). Set `DtdProcessing.Prohibit` and `XmlResolver = null` explicitly in code that may run on .NET Framework. Never enable XSLT scripts (`XsltSettings.EnableScript`, obsolete in .NET 10).
- **Regex DoS (ReDoS)**: patterns or input from users need a match timeout or `RegexOptions.NonBacktracking` (.NET 7). The default timeout is infinite.
- **Process execution**: `ProcessStartInfo.ArgumentList` (.NET Core 2.1) instead of a concatenated `Arguments` string; `UseShellExecute = false`; never pass user input to `cmd /c` or `sh -c`.
- **LDAP, XPath, OData filters**: escape or parameterize like SQL.
- **Output encoding**: Razor encodes by default; `Html.Raw`, `MarkupString` (Blazor) and hand-built HTML strings with user data are XSS.
- **Logging**: no secrets; user input only as structured parameters.

---

#### Performance

- Measure first: BenchmarkDotNet for micro-benchmarks, `dotnet-counters`, `dotnet-trace` and `dotnet-gcdump` for production-like loads.
- Avoid allocations in hot paths: `Span<T>`/`ReadOnlySpan<T>`, bounded `stackalloc`, `ArrayPool<T>`, `StringBuilder` in loops, cached `Regex` or `[GeneratedRegex]` (.NET 7), `SearchValues<T>` (.NET 8), `CompositeFormat` (.NET 8), `FrozenDictionary` (.NET 8).
- Cache expensive objects: `HttpClient` (via the factory), `JsonSerializerOptions`, compiled `Regex`, `XmlSerializer` instances created with constructors other than `XmlSerializer(Type)`/`(Type, string)` (they generate a new dynamic assembly on every call — a memory leak).
- Collections: presize; `TryGetValue` instead of `ContainsKey` + indexer; `HashSet<T>` for membership; `CollectionsMarshal.GetValueRefOrAddDefault` (.NET 6) for update-in-place counters.
- Async: avoid sync-over-async and needless `Task.Run`; `ValueTask` only when measured.
- Boxing: avoid passing structs as `object` or non-generic interfaces in hot loops; `Enum.HasFlag` is a JIT intrinsic on .NET Core 2.1+ but boxes on .NET Framework.
- LINQ is fine outside hot loops; do not rewrite readable LINQ into loops without a benchmark — the JIT on .NET 8–10 removes much of the overhead.
- GC: Server GC is the default for ASP.NET Core; in memory-limited containers consider `DOTNET_GCHeapHardLimit` or DATAS (default for Server GC from .NET 9).
- Startup: ReadyToRun or Native AOT (.NET 7+ for console apps, .NET 8+ for ASP.NET Core minimal APIs).
- Exceptions are expensive; do not use them for validation of every request.
- `string.Concat`/interpolation in loops, `+=` on strings, repeated `ToList()` and `Count()` on lazy sequences are the most common accidental O(n²) patterns.

---

#### Testing

```csharp
using Xunit;

public class PriceCalculatorTests
{
    [Fact]
    public void Apply_DiscountIsApplied() => Assert.Equal(90m, PriceCalculator.Apply(100m, 0.1m));

    [Theory]
    [InlineData(100, 0.0, 100)]
    [InlineData(100, 0.25, 75)]
    public void Apply_VariousRates(decimal price, decimal rate, decimal expected) =>
        Assert.Equal(expected, PriceCalculator.Apply(price, rate));

    [Fact]
    public void Apply_RejectsNegativeRate() =>
        Assert.Throws<ArgumentOutOfRangeException>(() => PriceCalculator.Apply(100m, -1m));

    [Fact]
    public async Task Cancellation_IsHonored()
    {
        using var cts = new CancellationTokenSource();
        cts.Cancel();
        await Assert.ThrowsAnyAsync<OperationCanceledException>(() => Task.Delay(1000, cts.Token));
    }
}

public static class PriceCalculator
{
    public static decimal Apply(decimal price, decimal rate)
    {
        if (rate < 0) throw new ArgumentOutOfRangeException(nameof(rate));
        return price * (1 - rate);
    }
}
```

- Name tests `Method_Scenario_Expected` or similar; Arrange/Act/Assert; table-driven `[Theory]`/`[TestCase]`/`[DataRow]` for rule-heavy code.
- **Always `await` async assertions** (`await Assert.ThrowsAsync<T>(...)`); an un-awaited assertion task never fails the test. xUnit analyzers report xUnit2021. NUnit 5 (September 2026) changed `Assert.ThrowsAsync` to return a `Task` that must be awaited.[^22]
- Framework versions as of October 2026: xUnit v2 (2.9.x) and xUnit v3 (xunit.v3 4.0.x), MSTest 4.x, NUnit 4.x/5.0. Do not mix xUnit v2 and v3 APIs; v3 test projects are executables and offer `TestContext.Current.CancellationToken`.[^22]
- Tests must not depend on order, shared static state, the current culture, the local time zone or the wall clock. Use `TimeProvider`/`FakeTimeProvider` (.NET 8) or an injected clock; set `CultureInfo` explicitly in culture-sensitive tests.
- No `Thread.Sleep`/`Task.Delay` to wait for background work; await the work or poll with a timeout.
- Mock interfaces you own (Moq, NSubstitute, FakeItEasy). Do not mock `HttpClient` — mock `HttpMessageHandler` or run a fake server; do not mock `DbContext` — use SQLite in-memory or a real database via Testcontainers.
- ASP.NET Core integration tests: `WebApplicationFactory<Program>` (`Microsoft.AspNetCore.Mvc.Testing`); for top-level-statement apps before .NET 10, add `public partial class Program { }` to expose `Program`.
- Coverage numbers are not a quality gate on their own; tests must assert behaviour.
- Assertions on exception messages are brittle (localized framework messages); assert the type and parameter name.

---

#### Trimming and Native AOT

Trimming is .NET 6+ (production-ready), Native AOT .NET 7+ (console) and .NET 8+ (ASP.NET Core minimal APIs, gRPC, worker services).

- Treat IL2xxx (trim) and IL3xxx (AOT) warnings as errors; a suppressed `IL2026` usually means a runtime failure after publish.
- Reflection-based JSON serialization, configuration binding, `Activator.CreateInstance(Type)` on arbitrary types, `Assembly.Load`, `Reflection.Emit`, `dynamic` and `Expression.Compile()` (interpreted) are not AOT-friendly. Use source generators: `JsonSerializerContext`, `[LoggerMessage]`, `[GeneratedRegex]`, the configuration binding generator (.NET 8), `[LibraryImport]` (.NET 7) instead of `[DllImport]`.
- Annotate reflection with `[DynamicallyAccessedMembers]`, `[RequiresUnreferencedCode]`, `[RequiresDynamicCode]` instead of suppressing warnings.
- MVC controllers, Razor Pages and Blazor Server do not support Native AOT; EF Core AOT support (precompiled queries) is experimental.
- `<InvariantGlobalization>true</InvariantGlobalization>` changes culture behaviour (see [Strings](#strings-culture-and-encoding)).

---

#### Upgrade Pitfalls Across Versions — Migration Notes

Behaviour that changes silently when a project moves between versions. Check these when a diff changes `TargetFramework`, `LangVersion`, the SDK or the base image:

1. **.NET Framework → .NET Core/.NET 5+**: no `AppDomain`s, remoting, WCF server, Web Forms or CAS; `BinaryFormatter` disabled/removed; `Encoding.Default` becomes UTF-8; code pages need `CodePagesEncodingProvider`; `HttpWebRequest` is a wrapper over `HttpClient`; `System.Drawing.Common` is Windows-only from .NET 6; configuration moves from `web.config`/`ConfigurationManager` to `Microsoft.Extensions.Configuration`; `Dictionary` allows `Remove` during enumeration.
2. **.NET Core 3.1 → .NET 5+ on Windows**: ICU replaces NLS — culture-sensitive `IndexOf`/`StartsWith`/`Compare` results change (Microsoft's documented example: `"\r\n".IndexOf("\n")` returned -1). Base-image updates that bring a new ICU version can change such results again; ordinal comparison is immune.[^16]
3. **.NET 5 → 6**: unhandled `BackgroundService` exceptions stop the host; `ImplicitUsings` and the minimal hosting model in templates; default shutdown timeout 30 s; `WebRequest` obsolete (SYSLIB0014); with `InvariantGlobalization`, creating a non-invariant culture throws (predefined cultures only).
4. **.NET 6 → 7/8**: `Rfc2898DeriveBytes` constructors without a hash algorithm obsolete (SYSLIB0041) and `HashAlgorithm.Create(string)` obsolete (SYSLIB0045) in .NET 7; from .NET 8, container images listen on port 8080 instead of 80 and ship a non-root `app` user, the `AesGcm` constructor without tag size is obsolete (SYSLIB0053) and the whole `BinaryFormatter` type is obsolete.
5. **.NET 8 → 9**: `BinaryFormatter` removed; `X509Certificate2` constructors obsolete (SYSLIB0057); `ServicePointManager` obsolete (SYSLIB0014); `System.Threading.Lock` available; STS support extended to 24 months.
6. **.NET 9 → 10 / C# 13 → 14**: `field` inside accessors rebinds (CS9258); first-class span conversions change overload resolution (covariant arrays can throw `ArrayTypeMismatchException`; expression trees contain `MemoryExtensions.Contains`); `extension` becomes a contextual keyword; configuration binding preserves `null`; no default SIGTERM handler for non-host apps; `BackgroundService.ExecuteAsync` fully on a background thread; `System.Linq.Async` conflicts with in-box `AsyncEnumerable`; cookie auth returns 401/403 for API endpoints; `Rfc2898DeriveBytes` constructors obsolete (SYSLIB0060).[^6][^17]
7. **Raising `LangVersion` without changing the target framework** is unsupported; C# 14 with `net8.0`/`net9.0` binds `array.Reverse()` to the in-place `MemoryExtensions.Reverse`. `var y = x.Reverse();` and `foreach (var i in x.Reverse())` stop compiling (CS0815/CS1579), but a bare statement `x.Reverse();` — a no-op with `Enumerable.Reverse` — now silently reverses the array in place (verified on `net8.0`: C# 12 prints `1,2,3`, C# 14 prints `3,2,1`).[^6]
8. **EF Core major upgrades**: 3.0 stopped client evaluation; 5 introduced split queries (opt-in); 7 changed SQL Server `Encrypt` default to `true` (Microsoft.Data.SqlClient 4); 8 changed `Contains` translation to JSON parameters; 10 changed it to multiple parameters and simplified parameter names. Read the breaking-changes page of every major version.[^21]
9. **Test framework majors** (xUnit v2 → v3, NUnit 4 → 5, MSTest 3 → 4) have their own breaking changes; un-awaited async assertions are the most dangerous.

Example of a C# 14 upgrade break (C# 13 prints `IEnumerable`, C# 14 throws):

```csharp
string[] strings = { "a" };
object[] objects = strings;                    // array covariance
try
{
    C.R(objects);                              // C# 14 binds R<T>(Span<T>): new Span<object>(string[]) throws
}
catch (ArrayTypeMismatchException)
{
    Console.WriteLine("ArrayTypeMismatchException");
}

static class C
{
    public static void R<T>(IEnumerable<T> e) => Console.WriteLine("IEnumerable");
    public static void R<T>(Span<T> s) => Console.WriteLine("Span");
}
```

---

#### Production / Review Checklist

##### Version

- [ ] `LangVersion`/`TargetFramework` determined; every suggested feature labelled with and available in that version
- [ ] No pre-release features (C# 15, .NET 11 RC, `LangVersion` `preview`) in production code
- [ ] Supported runtime: .NET 10 (or .NET Framework 4.8/4.8.1); .NET 8/9 migration planned before November 10, 2026; no .NET Core 3.1/5/6/7
- [ ] Latest patch (10.0.13 / 9.0.21 / 8.0.32 as of October 2026) in `global.json`, Dockerfiles and CI
- [ ] Package major versions aligned with the target framework

##### Language and types

- [ ] Nullable enabled; `!` justified; public APIs validate arguments
- [ ] Records: no mutable collections in value-equality types, no secrets in `ToString()`; `readonly record struct` for value objects
- [ ] Primary-constructor parameters not double-captured (CS9124) or mutated unintentionally
- [ ] Pattern combinators parenthesized (`is not (A or B)`); no suppressed CS8509
- [ ] C# 14 upgrade: CS9258 (`field`) treated as an error; span overload changes reviewed

##### Errors, async, concurrency

- [ ] `throw;` not `throw ex;`; no empty catches; `catch (Exception)` only at boundaries with logging
- [ ] No `.Result`/`.Wait()`, no `async void` (except guarded event handlers), no fire-and-forget
- [ ] `CancellationToken` accepted and forwarded (CA2016); linked/timeout `CancellationTokenSource` disposed
- [ ] `ConfigureAwait(false)` consistently in libraries
- [ ] No `await` elision inside `using`/`try`
- [ ] Locks: private lock objects; `SemaphoreSlim` released in `finally`; shared collections thread-safe
- [ ] Bounded concurrency and bounded queues

##### Resources and data

- [ ] Every owned `IDisposable` disposed; `await using` for async-disposable types
- [ ] No multiple enumeration of lazy sequences; `Any()`/`Count` used correctly; `ThenBy` after `OrderBy`
- [ ] Ordinal string comparison for identifiers; `InvariantCulture` for machine-readable formatting/parsing
- [ ] `decimal` for money; UTC timestamps; `TimeProvider`/clock abstraction for testable time
- [ ] EF Core: no N+1, `AsNoTracking` for reads, async with tokens, no shared `DbContext`, no raw SQL concatenation
- [ ] `IHttpClientFactory` or long-lived `SocketsHttpHandler`; timeouts; responses disposed; typed clients not in singletons
- [ ] `JsonSerializerOptions` cached; no `TypeNameHandling`/`BinaryFormatter`

##### Security

- [ ] Authorization on every endpoint, object-level checks, antiforgery for cookie-authenticated forms, CORS allow-list, rate limiting
- [ ] Input validated (DTOs, no over-posting); problem details without stack traces
- [ ] Paths canonicalized and prefix-checked; uploads renamed and size-limited
- [ ] `RandomNumberGenerator` for secrets; `FixedTimeEquals` for comparisons; PBKDF2 (≥ 600,000 iterations)/Identity hasher/Argon2id for passwords; AES-GCM for encryption
- [ ] XML without DTD processing; regex timeouts or `NonBacktracking`; `ArgumentList` for processes; SSRF allow-lists
- [ ] Secrets only from environment/secret stores; never logged

##### Tooling, tests, observability

- [ ] `TreatWarningsAsErrors`, analyzers enabled (explicitly on .NET Framework/`netstandard2.0`), no blanket `NoWarn`
- [ ] SDK pinned in `global.json`; central package management; lock files; NuGet Audit clean
- [ ] Tests await async assertions, avoid sleeps and shared state, use real providers for data access
- [ ] Constant log templates, exceptions passed as the first argument, no PII; bounded metric tag cardinality
- [ ] Trim/AOT warnings fixed, not suppressed
