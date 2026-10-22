### C# 11 on .NET 7: The Complete Production Developer Guide

#### Overview

**C# 11** shipped with the **.NET 7 SDK** and Visual Studio 2022 17.4 on **November 8, 2022**.[^1][^4] .NET 7 is a **Standard Term Support (STS)** release and is **out of support**: its end of support was **May 14, 2024**. The final servicing release is **.NET 7.0.20** (runtime 7.0.20, ASP.NET Core 7.0.20, SDKs **7.0.410** / 7.0.317 / 7.0.120), published on **May 28, 2024**, two weeks after the end-of-support date, as a security release (CVE-2024-20672, a macOS-only X.509 denial-of-service issue).[^1][^2][^3] No .NET 7 patch has shipped since; every vulnerability disclosed after May 2024 stays unfixed in .NET 7. EF Core 7 and the `Microsoft.Extensions.*` 7.0.x packages reached end of support on the same date.[^2]

Status of the .NET line as of October 2026:[^2]

| Release | Type | Latest patch (date) | End of support |
| --- | --- | --- | --- |
| .NET 7 / C# 11 | STS | 7.0.20, SDK 7.0.410 (May 28, 2024) | **May 14, 2024 — out of support** |
| .NET 8 / C# 12 | LTS | 8.0.32 (October 13, 2026) | November 10, 2026 |
| .NET 9 / C# 13 | STS | 9.0.21 (October 13, 2026) | November 10, 2026 |
| .NET 10 / C# 14 | **LTS (current)** | 10.0.13, SDK 10.0.402 (October 13, 2026) | November 14, 2028 |
| .NET 11 / C# 15 | STS, **pre-release** | 11.0.0-rc.2 (October 13, 2026; go-live) | GA expected November 2026 (annual cadence) |

**Recommendation for every .NET 7 code base: upgrade to .NET 10 (LTS).** Do not upgrade to .NET 8 or .NET 9 as a destination — both leave support in November 2026. A reviewer flags a `net7.0` target as a finding in its own right (unpatched runtime, ASP.NET Core and EF Core), but still reviews the code *as C# 11 on .NET 7*: suggesting a C# 12+ feature or a .NET 8+ API in a fix for a `net7.0` project produces code that does not compile.

This guide targets **C# 11 on .NET 7** specifically. Everything from earlier versions is available and should be used where it fits: nullable reference types, switch expressions, `using` declarations, async streams, indices and ranges (C# 8); records, init-only setters, top-level statements, target-typed `new`, relational and logical patterns (C# 9); record structs, file-scoped namespaces, global usings, extended property patterns, lambda natural types, `CallerArgumentExpression`, constant interpolated strings (C# 10).[^4] Every section that covers a C# 11 or .NET 7 specific change is marked **New in C# 11** or **New in .NET 7**. **Do not use or recommend anything from C# 12 / .NET 8 or later** — see the explicit list below.

The guide is self-contained: it covers the language, the base class library, ASP.NET Core 7, EF Core 7, testing and tooling as they exist in .NET 7. Every code block was compiled with the C# compiler of SDK 7.0.410 (Roslyn 4.7.0) against the `net7.0` reference assemblies and, where it prints output, run on runtime 7.0.20. "New in C# 11" claims were checked by compiling with `LangVersion` 10, and the "not available" claims by compiling with `LangVersion` 11 (and the feature with `LangVersion` 12+ on a newer SDK).

Global truths about C# 11 / .NET 7:

- **Managed, garbage-collected, statically typed.** Value types (`struct`, `enum`, primitives) are copied; reference types (`class`, `record`, arrays, delegates, `string`) are shared by reference.
- **Nullable reference types are a compile-time analysis, not a runtime guarantee.** `string` and `string?` are the same runtime type; a non-nullable reference can still be `null` at run time (deserializers, reflection, `default`, `[SetsRequiredMembers]` constructors, `!`).
- **`async`/`await` is built on `Task`/`ValueTask`.** An `async` method runs synchronously until its first `await` of an incomplete task; blocking on a task (`.Result`, `.Wait()`) can deadlock or starve the thread pool.
- **The language version follows the target framework.** `net7.0` defaults to C# 11 regardless of which SDK builds it; `LangVersion` overrides that.
- **The BCL is part of the target framework.** An API exists only if the `net7.0` reference assemblies contain it — a newer SDK does not add APIs to `net7.0`.
- **Strings are immutable UTF-16; `u8` literals are UTF-8 `ReadOnlySpan<byte>`.** Culture-sensitive operations (`ToLower()`, `ToUpper()`, `string.Compare`, `StartsWith(string)`) depend on the current culture unless told otherwise.

***

#### Version-Aware Review

##### How to determine the language version

The compiler picks the default `LangVersion` from the target framework, not from the SDK:[^12]

| Target framework | Default C# version |
| --- | --- |
| .NET Framework, `netstandard2.0` | 7.3 |
| `netcoreapp3.x`, `netstandard2.1` | 8 |
| `net5.0` | 9 |
| `net6.0` | 10 |
| **`net7.0`** | **11** |
| `net8.0` | 12 |
| `net9.0` | 13 |
| `net10.0` | 14 |

Verified behaviour: building a `net7.0` project with SDK 10.0.402 still uses C# 11 (`LangVersion` evaluates to `11.0`, and a collection expression fails with `CS9058: Feature 'collection expressions' is not available in C# 11.0`). The same build prints `warning NETSDK1138: The target framework 'net7.0' is out of support`. SDK 7.0.410 itself does not print NETSDK1138 for `net7.0`.

Review the project file and `Directory.Build.props` for overrides:

```xml
<Project Sdk="Microsoft.NET.Sdk">
  <PropertyGroup>
    <TargetFramework>net7.0</TargetFramework>
    <Nullable>enable</Nullable>
    <ImplicitUsings>enable</ImplicitUsings>
    <!-- Absent: C# 11 (the net7.0 default). -->
    <!-- <LangVersion>latest</LangVersion>  = newest version the *installed compiler* supports;
         with SDK 10 this silently turns on C# 14 syntax for a net7.0 build. -->
    <!-- <LangVersion>preview</LangVersion> on SDK 7.0.4xx enables *unsupported, unfinished*
         C# 12 previews (primary constructors, collection literals, alias-any-type,
         lambda default parameters). Flag it in production code. -->
  </PropertyGroup>
</Project>
```

Findings to raise:

- `LangVersion` set to `latest`, `latestMajor` or a number above 11 in a `net7.0` project: the code base now depends on whichever SDK builds it, and Microsoft does not support language versions above the TFM default.[^12] Verified: SDK 10.0.402 with `<LangVersion>latest</LangVersion>` compiles C# 12 collection expressions for `net7.0` without error.
- `LangVersion` `preview` with SDK 7.0.4xx: the 7.0.410 compiler (Roslyn 4.7) accepts primary constructors on classes, collection literals (`int[] a = [1, 2, 3];`), `using Point = (int X, int Y);` and lambda default parameters under `preview` — preview implementations that changed before C# 12 shipped. Under `LangVersion` 11 each one fails with `CS8652: The feature '…' is currently in Preview and *unsupported*`.
- A `global.json` that pins the SDK (`"version": "7.0.410"`) keeps builds reproducible; `rollForward` policies such as `latestMajor` let a machine with only SDK 10 build the project (with NETSDK1138).
- Multi-targeting (`<TargetFrameworks>net7.0;net8.0</TargetFrameworks>`) compiles each target with its own default language version — code shared between them must stay C# 11.

##### Verify APIs against the pinned version

Before claiming an API "doesn't exist" or "should be used instead", check the `net7.0` reference assemblies (API browser at learn.microsoft.com with the ".NET 7" version selector, or compile against `net7.0`). Say "not available in .NET 7" only when the pinned target proves it. Two traps in both directions:

- Many APIs that look new are already in .NET 7: `ArgumentException.ThrowIfNullOrEmpty`, `ObjectDisposedException.ThrowIf`, `Enumerable.Order()`/`OrderDescending()`, `Stream.ReadExactly`, `Path.Exists`, `Directory.CreateTempSubdirectory`, `File.ReadLinesAsync`, `Stopwatch.GetElapsedTime`, `UnreachableException`, `Int128`, `[GeneratedRegex]`, `RegexOptions.NonBacktracking`, `TarFile`, `Enum.GetValuesAsUnderlyingType`, `IDictionary<,>.AsReadOnly()`, `WebApplication`'s `MapGroup`, `TypedResults`, `AddRateLimiter`, `AddOutputCache`, `AddProblemDetails`, `Host.CreateApplicationBuilder` — all **New in .NET 7** (verified present on 7.0.20, absent on 6.0.36).
- Many APIs that look old are **not** in .NET 7: `ArgumentOutOfRangeException.ThrowIfNegative`/`ThrowIfGreaterThan` and `ArgumentException.ThrowIfNullOrWhiteSpace` (.NET 8), `TimeProvider` (.NET 8), `FrozenDictionary` (.NET 8), `Random.Shared.GetItems` (.NET 8), keyed DI services (.NET 8), `JsonNamingPolicy.SnakeCaseLower` (.NET 8). Full list below.

Packages are a separate axis: a `net7.0` project can reference a NuGet package that also supports newer targets, and gets its `net7.0`/`net6.0`/`netstandard2.0` asset. Packages from .NET 9 onward that ship `buildTransitive` checks warn on `net7.0` (verified with System.Text.Json 9.0.10: `warning : System.Text.Json 9.0.10 doesn't support net7.0 and has not been tested with it`). Suppressing that warning with `SuppressTfmSupportBuildWarnings` hides a real incompatibility — flag it.

***

#### What's New in C# 11 and .NET 7

##### Language (C# 11) — every item verified on LangVersion 10 vs 11

| Feature | Under `LangVersion` 10 |
| --- | --- |
| Raw string literals `"""…"""`, `$$"""…"""` | `CS8936: Feature 'raw string literals' is not available in C# 10.0` |
| Generic math: `static abstract`/`static virtual` interface members | `CS8703: The modifier 'abstract' is not valid for this item in C# 10.0` (using `INumber<T>` fails with CS8936 'static abstract members in interfaces') |
| Generic attributes `[Validator<int>]` | CS8936 'generic attributes' |
| UTF-8 string literals `"abc"u8` | CS8936 'UTF-8 string literals' |
| Newlines inside interpolation holes of `$"…"` | `CS8967: Newlines inside a non-verbatim interpolated string are not supported in C# 10.0` |
| List patterns `[1, .., 3]` | CS8936 'list pattern' |
| File-local types `file class C` | CS8936 'file types' |
| `required` members + `[SetsRequiredMembers]` | CS8936 'required members' |
| Auto-default structs | `CS0171: Field 'S.B' must be fully assigned … Consider updating to language version '11.0'` |
| Pattern-match `Span<char>`/`ReadOnlySpan<char>` against a constant string | CS8936 'pattern matching ReadOnly/Span<char> on constant string' |
| `ref` fields, `scoped`, `[UnscopedRef]` | CS8936 'ref fields' |
| Unsigned right shift `>>>`, `>>>=` | CS8936 'unsigned right shift' |
| `checked` user-defined operators | CS8936 'checked user-defined operators' |
| Relaxed shift operator (right operand need not be `int`) | CS8936 'relaxed shift operator' |
| Method-group-to-delegate conversion caches the delegate for static methods | compiles, but a new delegate is allocated each time (verified with `ReferenceEquals`) |
| Extended `nameof` scope (`nameof(parameter)` in method attributes) | compiles with the 7.0.410 compiler even under LangVersion 10; the .NET 6 SDK compiler (6.0.428, Roslyn 4.3) rejects it with `CS0103` |
| `nint`/`nuint` are aliases of `IntPtr`/`UIntPtr` with full numeric operators ("numeric IntPtr") | gated by the **target framework** (`RuntimeFeature.NumericIntPtr`, `net7.0`+), not by LangVersion |
| Warning wave 7 (`CS8981`, lowercase type names) | reported only with `<AnalysisLevel>` 7+ (the `net7.0` default) |

Sources: the C# version history and the csharplang language-version history.[^4][^5]

##### Runtime and libraries (.NET 7) — highlights for reviewers

- **Generic math** interfaces (`INumber<T>`, `INumberBase<T>`, `IBinaryInteger<T>`, `IFloatingPoint<T>`, …) on all numeric types, plus `Int128`/`UInt128`.[^6][^23]
- **Regular expressions**: `[GeneratedRegex]` source generator, `RegexOptions.NonBacktracking` (linear-time engine), span APIs `Regex.EnumerateMatches`, `Regex.Count`, `IsMatch(ReadOnlySpan<char>)`, analyzer `SYSLIB1045` suggests the generator.[^6][^20]
- **System.Text.Json**: contract customization (`DefaultJsonTypeInfoResolver` + `Modifiers`), polymorphism (`[JsonPolymorphic]`, `[JsonDerivedType]`), `required` members and `[JsonRequired]`.[^19]
- **Rate limiting**: `System.Threading.RateLimiting` (`RateLimiter`, `TokenBucketRateLimiter`, `FixedWindowRateLimiter`, `SlidingWindowRateLimiter`, `ConcurrencyLimiter`, `PartitionedRateLimiter`) and the ASP.NET Core middleware.[^6][^21]
- **TAR archives**: `System.Formats.Tar` (`TarFile`, `TarReader`, `TarWriter`).[^6]
- **Streams**: `Stream.ReadExactly`/`ReadAtLeast` (+ async).[^6]
- **Unix file modes**: `UnixFileMode`, `File.GetUnixFileMode`/`SetUnixFileMode`, `Directory.CreateDirectory(path, mode)`, `FileStreamOptions.UnixCreateMode`.[^6]
- **Microsecond/nanosecond** members on `DateTime`, `DateTimeOffset`, `TimeOnly`, `TimeSpan`.[^6]
- **`[StringSyntax]`**, **`LibraryImport`** P/Invoke source generator, **`ObjectDisposedException.ThrowIf`**, **`ArgumentException.ThrowIfNullOrEmpty`**, **`UnreachableException`**, **`Enumerable.Order`/`OrderDescending`**, **`Stopwatch.GetElapsedTime`**, **`MemoryCache.GetCurrentStatistics()`**, **`Activity.CurrentChanged`**.[^6]
- **Native AOT** publishing for console apps (not ASP.NET Core; that arrived in .NET 8); **on-stack replacement** enabled; Dynamic PGO available but **off by default** (`<TieredPGO>true</TieredPGO>`; on by default only from .NET 8).[^6][^28]
- **SDK**: central package management (`Directory.Packages.props`), `dotnet new` tab completion and template constraints, built-in container publishing.[^6]
- **ASP.NET Core 7**: rate limiting middleware, output caching middleware, minimal API endpoint filters, route groups (`MapGroup`), `TypedResults` and `Results<T1,…>` unions, `[AsParameters]`, `IFormFile` binding in minimal APIs (no built-in antiforgery), problem details service (`AddProblemDetails`), request decompression middleware, HTTP/3 fully supported, `dotnet user-jwts`, single authentication scheme becomes the default scheme, API controllers infer `[FromServices]` parameters.[^8][^9]
- **EF Core 7**: `ExecuteUpdate`/`ExecuteDelete` bulk operations, JSON columns (SQL Server provider only), TPC inheritance mapping, faster `SaveChanges` (SQL Server `OUTPUT`, SQLite `RETURNING`), `FromSql`, `SqlQuery<T>` for scalar types, `ExecuteSql`, interceptors for materialization, stored-procedure mapping for inserts/updates/deletes.[^10]

***

#### Do Not Use or Recommend (not available in C# 11 / .NET 7)

The compiler of a `net7.0` project rejects these, or the API does not exist in the `net7.0` reference assemblies. Never suggest them as a fix. (Language items verified with SDK 7.0.410 at LangVersion 11; API items verified by reflection on runtimes 7.0.20 and 8.0.32/9.0.21/10.0.13.)

##### C# 12 (.NET 8, November 2023)[^13]

- **Primary constructors** on `class`/`struct` (`class Service(ILogger log) { … }`) — LangVersion 11: `CS8652 … 'primary constructors' is currently in Preview`. Records already have positional parameters in C# 11; ordinary classes do not.
- **Collection expressions** `[1, 2, 3]`, `[..a, ..b]`, `List<int> l = [];` — `CS8652 'collection literals'` (SDK 7) / `CS9058` (newer SDK). Use `new[] { 1, 2, 3 }`, `new List<int> { … }`, `Array.Empty<T>()`, `a.Concat(b).ToArray()`.
- **Inline arrays** `[InlineArray(n)]` — the attribute type does not exist in .NET 7.
- **Default lambda parameters** `(int x = 3) => x`, `params` in lambdas.
- **`ref readonly` parameters** — `CS9070: 'readonly' is not supported as a parameter modifier`.
- **Alias any type** `using Point = (int X, int Y);`, aliases to arrays/pointers/tuples.
- **`[Experimental]`** attribute (type missing in .NET 7).
- **Interceptors** (preview in C# 12).

##### C# 13 (.NET 9, November 2024)[^14]

- `params ReadOnlySpan<T>` / `params IEnumerable<T>` collections — `CS0225: The params parameter must be a single dimensional array`.
- `System.Threading.Lock` and the new `lock` semantics (type missing in .NET 7; keep `private readonly object _gate = new();`).
- `\e` escape — `CS1009: Unrecognized escape sequence`; write `\u001b`.
- Partial properties/indexers, `ref struct` implementing interfaces, `allows ref struct` anti-constraint, `ref`/`unsafe` in iterators and async methods, `[OverloadResolutionPriority]`, implicit `^` index in object initializers.

##### C# 14 (.NET 10, November 2025)[^15]

- `extension` blocks / extension members (properties, static extension members) — use classic `static` extension methods.
- The `field` keyword in property accessors — `CS8050` on LangVersion 11; declare a backing field.
- Null-conditional assignment `a?.B = c` — `CS0131`.
- `nameof(List<>)` with unbound generic types — `CS0305`.
- Modifiers on simple lambda parameters `(s, out r) => …`, first-class span conversions, partial events/constructors, user-defined compound assignment operators.

##### C# 15 (.NET 11, release candidate in October 2026)

Pre-release only; nothing from it applies to a `net7.0` code base.

##### .NET 8+ library and framework APIs absent from .NET 7

| Do not suggest | Introduced | .NET 7 alternative |
| --- | --- | --- |
| `ArgumentOutOfRangeException.ThrowIfNegative`, `ThrowIfZero`, `ThrowIfGreaterThan`, `ThrowIfLessThan`, … | .NET 8 | `if (x < 0) throw new ArgumentOutOfRangeException(nameof(x), x, "…");` |
| `ArgumentException.ThrowIfNullOrWhiteSpace` | .NET 8 | `string.IsNullOrWhiteSpace` + `throw` (`ThrowIfNullOrEmpty` **is** available) |
| `TimeProvider`, `FakeTimeProvider` | .NET 8 | an injected clock interface; `Microsoft.Bcl.TimeProvider` package (netstandard2.0 build) |
| `FrozenDictionary`, `FrozenSet`, `ToFrozenDictionary()` | .NET 8 | `ImmutableDictionary`, `ReadOnlyDictionary`, `IDictionary<,>.AsReadOnly()` (.NET 7) |
| `SearchValues<T>`, `CompositeFormat`, `MemoryExtensions.ContainsAny`, `MemoryExtensions.Split(span, …)` | .NET 8 | `IndexOfAny`, `IndexOfAnyExcept` (.NET 7), cached `string.Format` |
| `Random.GetItems`, `Random.Shuffle`, `RandomNumberGenerator.GetString`/`GetHexString` | .NET 8 | loop with `Random.Shared.Next` / `RandomNumberGenerator.GetInt32`; `Convert.ToHexString(RandomNumberGenerator.GetBytes(n))` |
| `SHA3_256`, `SHA3_384`, `SHA3_512` | .NET 8 | SHA-2 (`SHA256.HashData`) |
| `ConfigureAwaitOptions`, `Task.ConfigureAwait(ConfigureAwaitOptions.SuppressThrowing)` | .NET 8 | `ConfigureAwait(false)` + `try/catch` |
| `CancellationTokenSource.CancelAsync()` | .NET 8 | `Cancel()` |
| `IPNetwork` (System.Net) | .NET 8 | manual CIDR parsing, or ASP.NET Core's `Microsoft.AspNetCore.HttpOverrides.IPNetwork` |
| `JsonNamingPolicy.SnakeCaseLower`/`KebabCaseLower` etc. | .NET 8 | custom `JsonNamingPolicy` subclass |
| `JsonUnmappedMemberHandling.Disallow`, `[JsonObjectCreationHandling]`, `JsonSerializerOptions.MakeReadOnly()`, `TypeInfoResolverChain`, `JsonNode.DeepEquals`, `JsonNode.GetValueKind` | .NET 8 | custom converters / `JsonTypeInfoResolver.Combine` (.NET 7) |
| `JsonSerializerOptions.Web`, `AllowOutOfOrderMetadataProperties`, `RespectNullableAnnotations`, `JsonSchemaExporter` | .NET 9 | `new JsonSerializerOptions(JsonSerializerDefaults.Web)` (cache it) |
| `HttpClientJsonExtensions.GetFromJsonAsAsyncEnumerable` | .NET 8 | `ReadAsStreamAsync` + `JsonSerializer.DeserializeAsyncEnumerable` |
| `CollectionsMarshal.SetCount`, `PeriodicTimer.Period` setter, `IMeterFactory`, `Instrument.Tags` | .NET 8 | — |
| `Guid.CreateVersion7()`, `Base64Url`, `Convert.ToHexStringLower`, `CryptographicOperations.HashData`, `Dictionary.GetAlternateLookup`, `OrderedDictionary<TKey,TValue>`, `TaskCompletionSource.SetFromTask` | .NET 9 | custom code / `Convert.ToHexString(...).ToLowerInvariant()` |
| `Enumerable.CountBy`, `AggregateBy`, `Index`, `Task.WhenEach` | .NET 9 | `GroupBy(...).Select(g => (g.Key, g.Count()))`, `Select((x, i) => …)`, `Task.WhenAny` loop |
| `Enumerable.LeftJoin`/`RightJoin`, `Enumerable.Shuffle`, `System.Linq.AsyncEnumerable` (in-box) | .NET 10 | `GroupJoin` + `SelectMany(... DefaultIfEmpty())`; `System.Linq.Async` package |
| Keyed services: `AddKeyedSingleton`, `[FromKeyedServices]`, `GetRequiredKeyedService` | .NET 8 | factory delegates or a named-instance registry |
| `IHostedLifecycleService` | .NET 8 | `IHostedService` + `IHostApplicationLifetime` |
| `ValidateOptionsResultBuilder`, `[OptionsValidator]` source generator, configuration binder source generator | .NET 8 | `IValidateOptions<T>`, `ValidateDataAnnotations()` |
| `IExceptionHandler`, `WebApplication.CreateSlimBuilder`/`CreateEmptyBuilder`, `ShortCircuit()`, `UseAntiforgery()`/`RequireAntiforgeryToken` for minimal APIs, `MapIdentityApi`, `AddStackExchangeRedisOutputCache`, Native AOT for ASP.NET Core, `ConfigureHttpClientDefaults` | .NET 8 | `UseExceptionHandler(...)`, `CreateBuilder`, `IAntiforgery` manually |
| `HybridCache`, `MapStaticAssets`, built-in OpenAPI document generation (`AddOpenApi`) | .NET 9 | `IMemoryCache`/`IDistributedCache`, `UseStaticFiles`, Swashbuckle + `Microsoft.AspNetCore.OpenApi` 7.0 (`WithOpenApi`) |
| `Results<T1,…,T7+>` (7+ type arguments) | later | .NET 7 provides `Results<T1,T2>` up to `Results<T1,…,T6>` |
| EF Core 8+: complex types (`ComplexProperty`), primitive collections, `SqlQuery<T>` for unmapped types, JSON columns on SQLite, `UseSqlReturningClause`/`UseSqlOutputClause`, `HierarchyId`; EF Core 9+/10+ features (`ExecuteUpdate` with non-expression setters, `LeftJoin`) | EF 8–10 | see the EF Core section |

##### Deprecated or removed in .NET 7 (do not keep using)

- `BinaryFormatter.Serialize`/`Deserialize`, `Formatter`/`IFormatter` serialize methods: `SYSLIB0011` is a **compile error** in SDK builds targeting `net7.0` (verified: `error SYSLIB0011`), not just a warning.[^38]
- New obsoletions with custom diagnostic IDs (verified as warnings on 7.0.410):[^25] `SYSLIB0039` (`SslProtocols.Tls`/`Tls11`), `SYSLIB0040` (`EncryptionPolicy.NoEncryption`/`AllowNoEncryption`), `SYSLIB0041` (`Rfc2898DeriveBytes` constructors with default SHA-1/1000 iterations), `SYSLIB0042` (`ToXmlString`/`FromXmlString` on the Windows CNG EC types), `SYSLIB0043` (`ECDiffieHellmanPublicKey.ToByteArray`), `SYSLIB0044` (`AssemblyName.CodeBase`), `SYSLIB0045` (`HashAlgorithm.Create(string)` and other name-based crypto factories), `SYSLIB0046` (`ControlledExecution.Run`), `SYSLIB0047` (`XmlSecureResolver`), `SYSLIB0036` (`Regex.CompileToAssembly`), `SYSLIB0037` (`AssemblyName.HashAlgorithm` etc.), `SYSLIB0038` (`SerializationFormat.Binary`).
- Still obsolete from earlier releases: `WebRequest`/`HttpWebRequest`/`WebClient`/`ServicePoint` (`SYSLIB0014`), `Thread.Abort` (`SYSLIB0006`), Code Access Security (`SYSLIB0003`).

***
#### Naming and Code Style

Use the .NET naming conventions; analyzers (`IDE1006`) and `.editorconfig` enforce them.

```csharp
// PascalCase: namespaces, types, methods, properties, events, constants, public fields
public sealed class OrderService
{
    public const int MaxRetryCount = 3;            // constants: PascalCase, not SCREAMING_CASE
    private static readonly TimeSpan DefaultTimeout = TimeSpan.FromSeconds(30);

    // camelCase with underscore prefix for private instance fields
    private readonly IOrderRepository _repository;
    private int _processedCount;

    public OrderService(IOrderRepository repository) => _repository = repository;

    // Async methods end in "Async"; parameters and locals are camelCase
    public async Task<Order?> FindOrderAsync(int orderId, CancellationToken cancellationToken)
    {
        var order = await _repository.FindAsync(orderId, cancellationToken);
        _processedCount++;
        return order;
    }

    // Booleans read as predicates
    public bool HasProcessedOrders => _processedCount > 0;
}

public interface IOrderRepository                     // interfaces: I prefix
{
    Task<Order?> FindAsync(int id, CancellationToken cancellationToken);
}

public sealed record Order(int Id, decimal Total);    // records: PascalCase positional parameters
```

**New in C# 11 — warning wave 7:** a type whose name contains only lowercase ASCII letters triggers `CS8981: The type name '…' only contains lower-cased ascii characters. Such names may become reserved for the language.` It is reported when the warning level is 7 or higher, which is the default for SDK-style projects targeting `net7.0` (verified: SDK build shows the warning; `csc -warn:6` does not).[^24] C# 11 itself reserved `required`, `scoped` and `file` as type names (`CS9029` for a type named `required`).[^5]

```csharp
// warning CS8981 under AnalysisLevel 7 (net7.0 default)
Console.WriteLine(new point().GetType().Name);
class point { }
```

Other house rules:

- One top-level type per file, file name = type name; file-scoped namespaces (`namespace MyApp.Orders;`).
- Seal classes that are not designed for inheritance (`sealed`); it documents intent and enables devirtualization.
- Prefer `var` when the type is obvious from the right-hand side; spell the type when it is not.
- Keep member order: constants, static fields, instance fields, constructors, properties, public methods, private methods.
- Enable `<Nullable>enable</Nullable>`, `<ImplicitUsings>enable</ImplicitUsings>` and `<TreatWarningsAsErrors>true</TreatWarningsAsErrors>` (at least `<WarningsAsErrors>nullable</WarningsAsErrors>`) in new projects.

***

#### New in C# 11: Raw String Literals

A raw string literal starts and ends with at least three `"` characters. Content needs no escaping; a multi-line literal has its common indentation removed, measured from the closing delimiter.[^4]

```csharp
var name = "widget";
var count = 3;

// Multi-line: the closing """ sets the left margin; the first and last newlines are dropped
var sql = """
    SELECT id, name
    FROM products
    WHERE name = @name
    """;
Console.WriteLine(sql);

// Single-line: quotes and braces need no escaping
var pattern = """He said "hi" and {braces} stay literal""";
Console.WriteLine(pattern);

// Interpolated raw string: the number of $ = number of braces that open a hole.
// With $$, single braces are literal JSON braces and {{expr}} interpolates.
var json = $$"""
    {"name": "{{name}}", "count": {{count}}, "literal": "{x}"}
    """;
Console.WriteLine(json);
```

Rules a reviewer must know (all verified):

- Every content line must start with the same whitespace as the closing line: `CS8999: Line does not start with the same whitespace as the closing line of the raw string literal`.
- A multi-line raw literal must start content on the line after the opening quotes: `var a = """abc` followed by a newline is `CS8997: Unterminated raw string literal`.
- In `$"""…"""`, `{{` is **not** an escape for a literal brace (unlike `$"…"`); it is `CS9006: The interpolated raw string literal does not start with enough '$' characters…`. Use `$$"""…"""` when the content contains braces.
- To include `"""` in the content, use four quotes as delimiters (`""""…""""`).
- Raw strings are ordinary `string` constants (when not interpolated) — they can be `const` and used in attributes.

**Review:** raw literals do not make SQL safe. `$"""SELECT … WHERE name = '{name}'"""` is string concatenation; use parameters (see Security).

***

#### New in C# 11: Generic Math and Static Abstract Interface Members

Interfaces can declare `static abstract` (and `static virtual`) members, including operators. .NET 7 implements the generic math interfaces (`INumber<T>`, `IAdditionOperators<,,>`, `IBinaryInteger<T>`, `IFloatingPointIeee754<T>`, …) on every primitive numeric type.[^23]

```csharp
using System.Numerics;

Console.WriteLine(Sum(new[] { 1, 2, 3 }));          // 6
Console.WriteLine(Sum(new[] { 1.5, 2.5 }));         // 4
Console.WriteLine(Midpoint(1.0, 2.0));              // 1.5

// Overflow is silent in unchecked context, exactly like int arithmetic
Console.WriteLine(Midpoint(int.MaxValue, int.MaxValue));   // -1

try { SumChecked(new[] { int.MaxValue, 1 }); }
catch (OverflowException) { Console.WriteLine("overflow detected"); }

// Explicit conversion policy instead of casts
Console.WriteLine(int.CreateChecked(3.9));          // 3
Console.WriteLine(int.CreateSaturating(1e20));      // 2147483647
Console.WriteLine(int.CreateTruncating(300L << 32 | 5)); // 5
try { int.CreateChecked(1e20); }
catch (OverflowException) { Console.WriteLine("CreateChecked throws"); }

static T Sum<T>(IEnumerable<T> values) where T : INumber<T>
{
    T total = T.Zero;
    foreach (var v in values) total += v;
    return total;
}

static T SumChecked<T>(IEnumerable<T> values) where T : INumber<T>
{
    T total = T.Zero;
    foreach (var v in values) total = checked(total + v);   // uses the checked operator
    return total;
}

static T Midpoint<T>(T a, T b) where T : INumber<T> => (a + b) / (T.One + T.One);
```

Own static abstract members — a factory contract without reflection:

```csharp
Console.WriteLine(Parse<Celsius>("21.5"));

static T Parse<T>(string s) where T : IParsableValue<T> => T.Parse(s);

interface IParsableValue<TSelf> where TSelf : IParsableValue<TSelf>
{
    static abstract TSelf Parse(string s);
}

readonly record struct Celsius(double Value) : IParsableValue<Celsius>
{
    public static Celsius Parse(string s) =>
        new(double.Parse(s, System.Globalization.CultureInfo.InvariantCulture));
}
```

Pitfalls (verified):

- An interface with static abstract members cannot be used as a type argument: `G<IShape>()` fails with `CS8920: The interface 'IShape' cannot be used as type argument. Static member … does not have a most specific implementation`. Such interfaces are constraints, not variable types.
- `Midpoint(int.MaxValue, int.MaxValue)` returns `-1`: generic math does not add overflow checking. Use `checked(...)`, `CreateChecked`, or widen the type.
- `.NET 7` also ships `IParsable<T>`/`ISpanParsable<T>` (implemented by the primitives), which ASP.NET Core 7 MVC/API controllers use for model binding (`static bool TryParse(string?, IFormatProvider?, out T)`).[^8]
- Generic math on hot paths is JIT-specialized for value types; do not box values through `INumber<T>`-typed locals of an interface type (that is not allowed anyway — see `CS8920`).

`checked` user-defined operators and the unsigned right shift are part of the same feature set:

```csharp
var a = new Money(int.MaxValue);
var b = new Money(1);
Console.WriteLine((a + b).Cents);                     // -2147483648 (unchecked operator)
try { _ = checked(a + b); }
catch (OverflowException) { Console.WriteLine("checked operator threw"); }

Console.WriteLine(-16 >> 28);                         // -1: arithmetic shift keeps the sign
Console.WriteLine(-16 >>> 28);                        // 15: logical shift (new >>> operator)

readonly record struct Money(int Cents)
{
    public static Money operator +(Money x, Money y) => new(unchecked(x.Cents + y.Cents));
    public static Money operator checked +(Money x, Money y) => new(checked(x.Cents + y.Cents));
}
```

***

#### New in C# 11 / .NET 7: `nint`, `nuint` and Numeric `IntPtr`

`nint`/`nuint` are now plain aliases of `System.IntPtr`/`System.UIntPtr`, and on `net7.0` both get the full set of numeric operators ("numeric IntPtr"). This is controlled by the **target framework** (`RuntimeFeature.NumericIntPtr`), not by `LangVersion`.[^5]

Verified consequences when retargeting from `net6.0` to `net7.0`:

- `IntPtr p = 5; IntPtr q = p * 2;` compiles on `net7.0`; on `net6.0` it fails (`CS0266`, `CS0019`).
- `checked(IntPtr.MaxValue + 1)` **throws `OverflowException`** on `net7.0`; on `net6.0` it silently wrapped. Code that relied on wrap-around pointer arithmetic inside `checked` blocks changes behaviour.
- `UIntPtr x = 1; int y = 2; x + y` is an error on `net7.0`: `CS0034: Operator '+' is ambiguous on operands of type 'nuint' and 'int'`. Fix with `UIntPtr.Add(x, y)` or `x + unchecked((nuint)y)`.

***

#### New in C# 11: Generic Attributes

```csharp
using System.Reflection;

var attr = typeof(CreateOrderRequest).GetCustomAttribute<ValidatorAttribute<CreateOrderValidator>>();
Console.WriteLine(attr?.ValidatorType.Name);   // CreateOrderValidator

[AttributeUsage(AttributeTargets.Class)]
sealed class ValidatorAttribute<TValidator> : Attribute where TValidator : class
{
    public Type ValidatorType => typeof(TValidator);
}

sealed class CreateOrderValidator { }

[Validator<CreateOrderValidator>]
sealed class CreateOrderRequest { }
```

Restrictions (verified): the type argument must be representable in metadata — `[Validator<string?>]`, `[Validator<dynamic>]` and `[Validator<(int A, int B)>]` all fail with `CS8970`; a type parameter of the enclosing type (`[Validator<U>]` inside `class C<U>`) fails with `CS8968: an attribute type argument cannot use type parameters`. Use `string`, `object` or `ValueTuple<int, int>` instead. Reading the attribute through reflection needs the closed type (`GetCustomAttribute<ValidatorAttribute<CreateOrderValidator>>()`) or an enumeration over `GetCustomAttributes()`.

***

#### New in C# 11: UTF-8 String Literals

`"text"u8` is a `ReadOnlySpan<byte>` with the UTF-8 encoding of the literal (plus a hidden null terminator that is not part of the span's `Length`).

```csharp
using System.Text;

ReadOnlySpan<byte> greeting = "héllo"u8;
Console.WriteLine(greeting.Length);                       // 6: 'é' is two bytes
Console.WriteLine(Encoding.UTF8.GetByteCount("héllo"));   // 6

byte[] copy = "héllo"u8.ToArray();                        // allocates a new array each call
Console.WriteLine(copy.Length);

Console.WriteLine(Http.StatusLinePrefix.Length);          // 8

static class Http
{
    // Idiom: expose u8 data through a static property; no allocation per access
    public static ReadOnlySpan<byte> StatusLinePrefix => "HTTP/1.1"u8;
}
```

Restrictions (verified):

- A u8 literal is not a constant: `const ReadOnlySpan<byte> X = "a"u8;` fails (`CS0133`), and `ReadOnlySpan<byte>` cannot be a static field (`CS8345`). Use a `static ReadOnlySpan<byte> Name => "…"u8;` property.
- `ReadOnlySpan<byte>` locals are not allowed in `async` methods in C# 11 (`CS4012: Parameters or locals of type 'ReadOnlySpan<byte>' cannot be declared in async methods`). Move the span code into a synchronous helper.
- **Review:** `"…"u8.ToArray()` in a loop allocates every time; a `static readonly byte[]` is better when an array is required.

***

#### New in C# 11: Newlines in Interpolation Holes

The expression inside `{ }` of a normal (non-verbatim) interpolated string may span lines, which makes `switch` expressions inside holes readable:

```csharp
var status = 503;
Console.WriteLine($"Request failed: {status switch
{
    >= 500 => "server error",
    >= 400 => "client error",
    _ => "unexpected"
}}");
```

Under C# 10 this is `CS8967`. Do not put long logic into holes; extract a method when the hole grows past a few lines.

***

#### New in C# 11: List Patterns

List patterns match arrays, `List<T>`, spans, `string` and any type that is *countable* (`Length`/`Count`) and *indexable* (`this[int]` or `this[Index]`); slice patterns (`.. var rest`) also require a range indexer or `Slice(int, int)`.[^4]

```csharp
int[] numbers = { 1, 2, 3, 4, 5 };

Console.WriteLine(Describe(numbers));                 // first 1, 4 more
Console.WriteLine(Describe(Array.Empty<int>()));      // empty
Console.WriteLine(numbers is [1, .., 5]);             // True
Console.WriteLine("abc" is ['a', .., 'c']);           // True: string supports slices

var command = "deploy prod --force".Split(' ');
Console.WriteLine(command switch
{
    ["deploy", var env] => $"deploy to {env}",
    ["deploy", var env, "--force"] => $"force deploy to {env}",
    ["rollback", ..] => "rollback",
    _ => "unknown command"
});

var list = new List<int> { 1, 2, 3 };
Console.WriteLine(list is [1, _, _]);                 // List<T> supports element patterns
Console.WriteLine(list is [1, ..]);                   // and discard slices

static string Describe(int[] values) => values switch
{
    [] => "empty",
    [var only] => $"one: {only}",
    [var first, .. var rest] => $"first {first}, {rest.Length} more",
};
```

Pitfalls (verified):

- `List<T>` has no range indexer, so `list is [_, .. var middle, _]` fails with `CS1503: Argument 1: cannot convert from 'System.Range' to 'int'`. Discard slices (`..`) work; capturing slices need arrays, spans or strings.
- `IEnumerable<T>` cannot be list-matched (`CS8985 … No suitable 'Length' or 'Count' property was found`). Materialize first, or use LINQ.
- A slice of an **array** (`.. var rest`) allocates a new array (it calls `RuntimeHelpers.GetSubArray`); a slice of a `Span<T>`/`ReadOnlySpan<T>` does not. In hot paths match on `numbers.AsSpan()`.
- `Length`/`Count` are assumed non-negative for exhaustiveness; arms like `{ Count: < 0 }` are reported as errors (subsumed patterns).[^5]

***

#### New in C# 11: Pattern Matching `Span<char>` on Constant Strings

```csharp
Console.WriteLine(Classify("put".AsSpan()));          // write
Console.WriteLine(Classify("x y".AsSpan(0, 1)));      // unknown
ReadOnlySpan<char> empty = default;
Console.WriteLine(empty is "");                       // True: default span equals ""

static string Classify(ReadOnlySpan<char> verb) => verb switch
{
    "get" => "read",
    "put" or "post" => "write",
    _ => "unknown",
};
```

Comparison is ordinal (`SequenceEqual`). `span is null` does not compile (`CS9135: A constant value of type 'ReadOnlySpan<char>' is expected`) — spans are never null; a `default` span is empty.

***

#### New in C# 11: Required Members

`required` on a field or property forces every object creation to set it in an object initializer; a constructor marked `[SetsRequiredMembers]` opts out.[^34]

```csharp
using System.Diagnostics.CodeAnalysis;

var a = new Customer { Email = "a@example.com", Name = "Ada" };
var b = new Customer("b@example.com", "Bob");
Console.WriteLine($"{a.Name} {b.Name}");

sealed class Customer
{
    public required string Email { get; init; }
    public required string Name { get; init; }
    public string? Phone { get; init; }

    public Customer() { }

    [SetsRequiredMembers]
    public Customer(string email, string name)
    {
        Email = email;
        Name = name;
    }
}
```

Compile-time rules (verified):

- `new Customer()` without `Email`/`Name` → `CS9035: Required member 'Customer.Email' must be set in the object initializer or attribute constructor`.
- A required member must be at least as visible as its type (`protected required` in a public class → `CS9032`).
- A type with required members cannot satisfy a `new()` constraint (`CS9040`) — this breaks generic factories such as `Activator`-style helpers and some test fixtures.
- Required members of a base class are required for derived classes too: `new Derived()` fails unless `Derived`'s constructor is `[SetsRequiredMembers]` or the initializer sets them.
- `required` removes the nullable warning `CS8618` for that member, because the caller must assign it.

Runtime gaps (verified):

```csharp
using System.Diagnostics.CodeAnalysis;

// BAD: [SetsRequiredMembers] is a promise the compiler does not check
var p = new Person("Ada");
Console.WriteLine(p.Last is null ? "Last is null" : p.Last);   // "Last is null"

sealed class Person
{
    public required string First { get; init; }
    public required string Last { get; init; }

    [SetsRequiredMembers]
    public Person(string first) { First = first; }   // forgets Last: only warning CS8618
}
```

- System.Text.Json (.NET 7) honours `required`: deserializing `{"Name":"a"}` into a type with `required int Age` throws `JsonException: JSON deserialization for type '…' was missing required properties, including the following: Age`. But `{"Name":null}` is accepted for a non-nullable `required string Name` — `required` checks presence, not non-null.
- Reflection, `Activator.CreateInstance`, older serializers and ORMs bypass `required` entirely.

***

#### New in C# 11: Auto-Default Structs

A struct constructor no longer has to assign every field; unassigned fields are zero-initialized.

```csharp
var c = new Counter(5);
Console.WriteLine($"{c.Start} {c.Count} {c.Label ?? "null"}");   // 5 0 null

struct Counter
{
    public int Start;
    public int Count;
    public string? Label;

    public Counter(int start) { Start = start; }   // C# 10: CS0171; C# 11: Count = 0, Label = null
}
```

**Review:** the old error caught forgotten assignments. With C# 11 the compiler only reports `CS0649` (field never assigned) or `CS8618` (non-nullable reference field left null). Check new struct constructors for fields that silently stay `default`. Remember that `default(Counter)` and array elements never run any constructor or field initializer.

***

#### New in C# 11: File-Local Types

`file class Helper` is visible only inside its source file. Two files may each declare a `file` type with the same name. Primarily for source generators.

```csharp
Console.WriteLine(new PriceCalculator().Total(10m, 3));

public sealed class PriceCalculator
{
    public decimal Total(decimal unitPrice, int quantity) => TaxRules.Apply(unitPrice * quantity);
}

file static class TaxRules
{
    public static decimal Apply(decimal net) => decimal.Round(net * 1.19m, 2);
}
```

A file-local type cannot appear in the signature of a non-file-local member (`CS9051: File-local type 'Helper' cannot be used in a member signature in non-file-local type 'Api'`).

***

#### New in C# 11: `ref` Fields and `scoped`

`ref struct` types can contain `ref` fields; `scoped` restricts a reference so it cannot escape the method; `[UnscopedRef]` widens it.[^33] These features target low-level library code (`Span<T>` itself is now implemented with a `ref` field).

```csharp
int value = 1;
var holder = new RefHolder(ref value);
holder.Value = 42;
Console.WriteLine(value);                    // 42: writes through the ref field

Span<int> buffer = stackalloc int[4];
Fill(buffer, 7);
Console.WriteLine(buffer[3]);                // 7

static void Fill(scoped Span<int> target, int v)
{
    target.Fill(v);                          // 'scoped': target cannot be stored or returned
}

ref struct RefHolder
{
    public ref int Value;
    public RefHolder(ref int value) { Value = ref value; }
}
```

Compiler breaking changes that come with the new ref-safety rules (C# 11 **or** `net7.0`): an `out` parameter can no longer be returned by `ref` (`CS8166`, fix with `[UnscopedRef]` or `ref`), and a `ref struct` instance method is assumed to capture unscoped `ref`/`in` arguments, which turns some formerly accepted span code into errors.[^5] Treat a new `[UnscopedRef]` or `Unsafe.AsRef` in application code as a red flag that needs justification.

***

#### New in C# 11: Method Group Conversions Are Cached

Converting a **static** method group to a delegate now reuses a cached delegate instance, like a non-capturing lambda already did.

```csharp
Func<int, int>? previous = null;
var reused = 0;
for (var i = 0; i < 3; i++)
{
    Func<int, int> f = Square;           // C# 11: same instance every iteration
    if (ReferenceEquals(f, previous)) reused++;
    previous = f;
}
Console.WriteLine($"reused {reused}/2");  // C# 11: 2/2; LangVersion 10: 0/2

static int Square(int x) => x * x;
```

Instance method groups (`list.ForEach(this.Handle)`) still allocate a delegate per conversion. Code that compares delegate identity (event unsubscription with a fresh method group still works because delegates compare by target and method, not by reference) is unaffected.

***

#### New in C# 11: Extended `nameof` Scope

`nameof(parameter)` is allowed in attributes on the method and on its parameters, which fixes the common nullable-attribute pattern:

```csharp
using System.Diagnostics.CodeAnalysis;

Console.WriteLine(Normalize("  Hi ") ?? "(null)");

[return: NotNullIfNotNull(nameof(input))]
static string? Normalize(string? input) => input?.Trim();
```

The SDK 7.0.410 compiler accepts this even with `LangVersion` 10; the .NET 6 SDK compiler reports `CS0103: The name 'input' does not exist in the current context`. Prefer `nameof` over string literals in `[NotNullIfNotNull]`, `[MemberNotNull]`, `[CallerArgumentExpression]`.

***
#### Nullable Reference Types

Enable nullable analysis in every project (`<Nullable>enable</Nullable>`) and treat nullable warnings as errors. The analysis is compile-time only.

```csharp
using System.Diagnostics.CodeAnalysis;

var users = new Dictionary<int, string> { [1] = "ada" };

if (TryFind(users, 1, out var name))
{
    Console.WriteLine(name.ToUpperInvariant());   // no warning: [NotNullWhen(true)]
}

string? maybe = users.GetValueOrDefault(2);
Console.WriteLine(maybe?.Length ?? 0);            // null-safe access

ArgumentException.ThrowIfNullOrEmpty(name);       // New in .NET 7
Console.WriteLine(Describe(null));

static bool TryFind(Dictionary<int, string> map, int id, [NotNullWhen(true)] out string? value)
    => map.TryGetValue(id, out value);

static string Describe(object? value) => value switch
{
    null => "nothing",
    string s => $"string of {s.Length}",
    _ => value.ToString() ?? "",
};
```

Review rules:

- **`!` (null-forgiving) is a claim, not a check.** Every `!` needs a reason that holds at run time; `= null!` on properties is acceptable only for framework-initialized members (EF Core navigation properties, options classes bound by configuration) — prefer `required` (C# 11) for everything constructed by your own code.
- Validate public inputs at run time even with nullable annotations: callers may have nullable disabled, and deserializers/reflection ignore annotations. Use `ArgumentNullException.ThrowIfNull(arg)` (.NET 6) and `ArgumentException.ThrowIfNullOrEmpty(str)` (**New in .NET 7**). `ThrowIfNullOrWhiteSpace` and the `ArgumentOutOfRangeException.ThrowIf*` helpers are .NET 8 — write the check by hand on .NET 7.
- System.Text.Json in .NET 7 does **not** enforce nullable annotations: a `null` JSON value deserializes into a non-nullable `string` property (verified above with `required`). Validate after deserialization.
- Use the attributes `[NotNullWhen]`, `[MaybeNullWhen]`, `[NotNullIfNotNull]`, `[MemberNotNull]`, `[DoesNotReturn]` to describe contracts instead of sprinkling `!` at call sites.
- `default` of a generic `T` with an unconstrained `T` may be null: return `T?` and annotate.

***

#### Records, Classes and Structs

```csharp
var original = new Order(1, new List<string> { "gift" });
var copy = original with { Id = 2 };
copy.Tags.Add("express");                                  // BAD: shallow copy shares the list
Console.WriteLine(original.Tags.Count);                    // 2

var a = new Order(1, new List<string> { "x" });
var b = new Order(1, new List<string> { "x" });
Console.WriteLine(a == b);                                 // False: List<T> compares by reference

var p1 = new Point(1, 2);
var p2 = p1 with { Y = 5 };
Console.WriteLine($"{p1} {p2} {p1 == new Point(1, 2)}");   // value equality for value members

record Order(int Id, List<string> Tags);                   // BAD for value semantics
readonly record struct Point(int X, int Y);                // GOOD: small immutable value
sealed record Address(string Street, string City);         // GOOD: immutable members only
```

Review rules:

- Records give value equality only if every member does. Collections inside records (`List<T>`, arrays) compare by reference and are shared by `with`. Use `ImmutableArray<T>`/`IReadOnlyList<T>` plus custom `Equals` when value semantics over collections matter.
- `record` (class) for immutable reference-type data; `readonly record struct` for small values (≲ 16–24 bytes) — non-readonly `record struct` has mutable positional properties.
- Seal records unless they are designed as a hierarchy; record equality includes `EqualityContract`, so a base and a derived record with the same values are not equal.
- Do not use records as EF Core entities that are tracked and mutated; value equality and `with` copies confuse identity-based change tracking.
- Mutable structs are a bug magnet (copies on every read from a `readonly` field, `foreach` variable, or property). Mark structs `readonly` when possible.
- C# 11 has no primary constructors for ordinary classes — constructor injection uses explicit fields (see Dependency Injection).

***

#### Pattern Matching

C# 11 pattern matching includes type, constant, relational (`> 5`), logical (`and`, `or`, `not`), property (`{ Total: > 100 }`), extended property (`{ Customer.Address.City: "Berlin" }`, C# 10), positional, list (C# 11) and `Span<char>`-constant (C# 11) patterns.

```csharp
Console.WriteLine(Shipping(new Order(150m, true, new Customer("Berlin"))));
Console.WriteLine(Shipping(new Order(20m, false, new Customer("Hamburg"))));
Console.WriteLine(Discount(new PremiumCustomer(2_000m)));
Console.WriteLine(Classify(42));

static string Shipping(Order order) => order switch
{
    { Total: > 100m, IsPriority: true } => "express",
    { Customer.City: "Berlin" } => "same-day courier",       // extended property pattern
    { Total: > 100m } => "standard",
    _ => "economy",
};

static decimal Discount(object customer) => customer switch
{
    PremiumCustomer { Purchases: > 1_000m } p => p.Purchases * 0.2m,
    PremiumCustomer p => p.Purchases * 0.1m,
    null => 0m,
    _ => throw new ArgumentException("Unknown customer type", nameof(customer)),
};

static string Classify(int n) => n switch
{
    < 0 => "negative",
    0 => "zero",
    > 0 and <= 9 => "digit",
    _ => "large",
};

sealed record Customer(string City);
sealed record Order(decimal Total, bool IsPriority, Customer Customer);
sealed record PremiumCustomer(decimal Purchases);
```

Review rules:

- Order matters: put specific arms before general ones; the compiler reports unreachable arms (`CS8510`) only when it can prove subsumption.
- A non-exhaustive `switch` expression throws `SwitchExpressionException` at run time and warns `CS8509`; do not silence the warning — add the missing arm or a `_ =>` arm that throws a meaningful exception.
- `is not null` is the idiomatic null check; `== null` can be hijacked by a user-defined `operator ==`.
- `x is { }` means "not null" and is fine; `x is var y` always matches (including null).

***

#### Exceptions

```csharp
using System.Diagnostics;

try
{
    Process(-1);
}
catch (ArgumentOutOfRangeException ex) when (ex.ParamName == "quantity")   // exception filter
{
    Console.WriteLine($"invalid input: {ex.Message}");
}

try { Rethrow(); }
catch (InvalidOperationException ex)
{
    // 'throw;' preserved the original frame (Fail) in the stack trace
    Console.WriteLine(ex.StackTrace!.Contains("Fail"));     // True
}

Console.WriteLine(Area(new Square(3)));

static void Process(int quantity)
{
    if (quantity < 0)   // ThrowIfNegative is .NET 8; write the check explicitly on .NET 7
        throw new ArgumentOutOfRangeException(nameof(quantity), quantity, "Must not be negative.");
}

static void Rethrow()
{
    try { Fail(); }
    catch (InvalidOperationException)
    {
        // log here...
        throw;                         // GOOD: keeps the stack trace. 'throw ex;' resets it (CA2200)
    }
}

static void Fail() => throw new InvalidOperationException("failed");

static double Area(Shape shape) => shape switch
{
    Square s => s.Side * s.Side,
    Circle c => Math.PI * c.Radius * c.Radius,
    _ => throw new UnreachableException($"Unknown shape {shape.GetType()}"),   // New in .NET 7
};

abstract record Shape;
sealed record Square(double Side) : Shape;
sealed record Circle(double Radius) : Shape;
```

Review rules:

- Catch specific exceptions. `catch (Exception)` belongs at process/request boundaries (middleware, background loops) where the exception is logged with context and translated.
- Never swallow exceptions silently (`catch { }`); never use exceptions for expected control flow (use `TryParse`, `TryGetValue`).
- `throw ex;` resets the stack trace (analyzer `CA2200`, which is on by default). Use `throw;` or wrap with an inner exception: `throw new OrderException("…", ex);`.
- `UnreachableException` (**New in .NET 7**) documents "cannot happen" branches; do not use it for input validation.
- `OperationCanceledException` is not an error when the caller cancelled: catch it separately (`catch (OperationCanceledException) when (ct.IsCancellationRequested)`) and do not log it as a failure.
- Custom exceptions: derive from `Exception` (not `ApplicationException`), provide the three standard constructors, do not add `[Serializable]`/serialization constructors for `BinaryFormatter` (obsolete, error in .NET 7).
- Exception messages and `ProblemDetails` returned to clients must not contain stack traces, SQL, file paths or secrets.

***

#### Async/Await

##### Fundamentals

```csharp
var sw = System.Diagnostics.Stopwatch.StartNew();

// GOOD: start independent operations, then await them together
var userTask = LoadAsync("user", 100);
var ordersTask = LoadAsync("orders", 100);
await Task.WhenAll(userTask, ordersTask);
Console.WriteLine($"{await userTask} {await ordersTask} in ~{sw.ElapsedMilliseconds / 100 * 100} ms");

// Task.WhenAll: 'await' rethrows only the FIRST exception; inspect the task for all of them
var all = Task.WhenAll(FailAsync("a"), FailAsync("b"));
try { await all; }
catch (InvalidOperationException ex)
{
    Console.WriteLine($"first: {ex.Message}; total: {all.Exception!.InnerExceptions.Count}");
}

static async Task<string> LoadAsync(string what, int ms)
{
    await Task.Delay(ms);
    return what;
}

static async Task FailAsync(string message)
{
    await Task.Yield();
    throw new InvalidOperationException(message);
}
```

Review rules:

- **No sync-over-async.** `.Result`, `.Wait()`, `.GetAwaiter().GetResult()` on an incomplete task block a thread; in UI or legacy ASP.NET synchronization contexts they deadlock, in ASP.NET Core they cause thread-pool starvation under load. Make the call chain async.
- **No `async void`** except event handlers. An exception escaping an `async void` method is rethrown on the thread pool and **terminates the process** (verified: the `AppDomain.UnhandledException` handler runs, then the process aborts). Event handlers must catch everything.
- **Do not fire and forget** `Task`s in request code; unobserved exceptions disappear and the work races with request teardown. Use a background queue (`Channel<T>` + `BackgroundService`).
- `ConfigureAwait(false)` in library code that does not need the caller's context. ASP.NET Core has no synchronization context, so application code there does not need it; analyzer `CA2007` enforces it when enabled.
- Return the task directly (`Task<T> M() => _inner.MAsync();`) only when there is no `using`/`try` around it — otherwise the resource is disposed before the task completes. When in doubt, `async`/`await`.
- `ValueTask`/`ValueTask<T>`: await exactly once; never await twice, never `.Result` before completion, never `Task.WhenAll` without `.AsTask()`. Use it for hot paths that usually complete synchronously; default to `Task`.
- `Lazy<Task<T>>` (or `AsyncLazy`) for single initialization; `Task.Run` only to move CPU-bound work off a thread that must stay responsive — never wrap I/O in `Task.Run`.

##### Cancellation and timeouts

```csharp
using var cts = new CancellationTokenSource(TimeSpan.FromMilliseconds(50));
try
{
    await SlowOperationAsync(cts.Token);
}
catch (OperationCanceledException) when (cts.IsCancellationRequested)
{
    Console.WriteLine("cancelled by timeout");
}

// Task.WaitAsync (.NET 6): stop WAITING after a timeout; the operation itself keeps running
var pending = new TaskCompletionSource<int>(TaskCreationOptions.RunContinuationsAsynchronously);
try
{
    await pending.Task.WaitAsync(TimeSpan.FromMilliseconds(20));
}
catch (TimeoutException)
{
    Console.WriteLine($"timed out; underlying task: {pending.Task.Status}");
}

// Linked tokens: caller cancellation OR our own timeout
using var caller = new CancellationTokenSource();
using var linked = CancellationTokenSource.CreateLinkedTokenSource(caller.Token);
linked.CancelAfter(TimeSpan.FromSeconds(5));
await SlowOperationAsync(linked.Token, iterations: 1);
Console.WriteLine("completed within budget");

static async Task SlowOperationAsync(CancellationToken cancellationToken, int iterations = 100)
{
    for (var i = 0; i < iterations; i++)
    {
        await Task.Delay(10, cancellationToken);   // pass the token to every async call
    }
}
```

Review rules:

- Every public async API takes a `CancellationToken` (last parameter, `cancellationToken = default` only on public library surfaces) and passes it to every awaited call. ASP.NET Core injects `HttpContext.RequestAborted` as a `CancellationToken` parameter in minimal APIs and controller actions.
- `TaskCanceledException` derives from `OperationCanceledException`; catch the base type. `HttpClient.Timeout` surfaces as `TaskCanceledException` whose `InnerException` is a `TimeoutException` (verified on .NET 7).
- Dispose `CancellationTokenSource` instances that use timers (`CancelAfter`, timeout constructor) or links.
- `CancellationTokenSource.CancelAsync()` is .NET 8; use `Cancel()`.
- `TaskCompletionSource` without `TaskCreationOptions.RunContinuationsAsynchronously` runs awaiting continuations synchronously inside `SetResult`, which can cause re-entrancy and deadlocks.

##### Async streams (`IAsyncEnumerable<T>`)

```csharp
using System.Runtime.CompilerServices;

using var cts = new CancellationTokenSource(TimeSpan.FromMilliseconds(60));
var received = 0;
try
{
    await foreach (var item in ReadEventsAsync().WithCancellation(cts.Token))
    {
        received++;
    }
}
catch (OperationCanceledException)
{
    Console.WriteLine($"stopped after {received} items");
}

static async IAsyncEnumerable<int> ReadEventsAsync(
    [EnumeratorCancellation] CancellationToken cancellationToken = default)
{
    for (var i = 0; ; i++)
    {
        await Task.Delay(10, cancellationToken);
        yield return i;
    }
}
```

Without `[EnumeratorCancellation]` the token passed through `WithCancellation` never reaches the iterator; the compiler warns `CS8425` (verified). LINQ over async streams needs the `System.Linq.Async` package — the in-box `System.Linq.AsyncEnumerable` is .NET 10.

##### Concurrency primitives

```csharp
using System.Threading.Channels;

// Bounded parallelism for async work (.NET 6+)
var maxConcurrent = 0;
var current = 0;
await Parallel.ForEachAsync(Enumerable.Range(0, 20),
    new ParallelOptions { MaxDegreeOfParallelism = 3 },
    async (i, ct) =>
    {
        var now = Interlocked.Increment(ref current);
        InterlockedMax(ref maxConcurrent, now);
        await Task.Delay(5, ct);
        Interlocked.Decrement(ref current);
    });
Console.WriteLine($"max concurrency: {maxConcurrent}");   // 3

// Producer/consumer with back-pressure
var channel = Channel.CreateBounded<int>(new BoundedChannelOptions(capacity: 2)
{
    FullMode = BoundedChannelFullMode.Wait,
    SingleReader = true,
});
var producer = Task.Run(async () =>
{
    for (var i = 1; i <= 5; i++) await channel.Writer.WriteAsync(i);
    channel.Writer.Complete();
});
var sum = 0;
await foreach (var item in channel.Reader.ReadAllAsync()) sum += item;
await producer;
Console.WriteLine($"sum: {sum}");                          // 15

// Async-compatible mutual exclusion: SemaphoreSlim, never 'lock' around 'await'
var gate = new SemaphoreSlim(1, 1);
await gate.WaitAsync();
try { /* critical section that awaits */ await Task.Yield(); }
finally { gate.Release(); }

// Periodic work (.NET 6+): no overlapping ticks, cancellable
using var timer = new PeriodicTimer(TimeSpan.FromMilliseconds(10));
var ticks = 0;
while (ticks < 3 && await timer.WaitForNextTickAsync()) ticks++;
Console.WriteLine($"ticks: {ticks}");

static void InterlockedMax(ref int target, int value)
{
    int observed;
    while ((observed = Volatile.Read(ref target)) < value &&
           Interlocked.CompareExchange(ref target, value, observed) != observed) { }
}
```

Review rules:

- `lock` cannot contain `await` (`CS1996`); use `SemaphoreSlim.WaitAsync` with `try/finally`. `System.Threading.Lock` is .NET 9 — on .NET 7 lock on a `private readonly object`.
- Never `lock (this)`, `lock (typeof(X))` or lock on strings.
- `Parallel.ForEach` with an `async` lambda is a bug (the lambda becomes `async void`); use `Parallel.ForEachAsync` or `Task.WhenAll` with a `SemaphoreSlim`.
- Unbounded `Task.WhenAll(items.Select(CallApiAsync))` over user-controlled sizes overloads downstream services; bound it.
- Shared mutable state needs `Interlocked`, `lock`, or concurrent collections; `static` fields in ASP.NET Core are shared by all requests.

***

#### Spans, Memory and Allocation

```csharp
using System.Buffers;

// Parse without substring allocations
var line = "id=42;name=widget;price=9.99";
foreach (var part in line.Split(';'))      // fine for cold paths
{
    _ = part;
}
Console.WriteLine(GetValue(line.AsSpan(), "name").ToString());   // widget
Console.WriteLine(RandomToken());

// ArrayPool for larger temporary buffers (this top-level code awaits, so it cannot
// declare Span<T> locals: CS4012 — span work lives in the synchronous helpers below)

var buffer = ArrayPool<byte>.Shared.Rent(64 * 1024);
try
{
    using var source = new MemoryStream(new byte[100_000]);
    int read, total = 0;
    while ((read = await source.ReadAsync(buffer.AsMemory())) > 0) total += read;
    Console.WriteLine(total);                              // 100000
}
finally
{
    ArrayPool<byte>.Shared.Return(buffer);                 // never use buffer after Return
}

// string.Create: build a string in place
var hex = string.Create(8, 0xCAFEBABE, static (span, value) =>
{
    value.TryFormat(span, out _, "X8");
});
Console.WriteLine(hex);                                    // CAFEBABE

static string RandomToken()
{
    Span<byte> bytes = stackalloc byte[16];   // stackalloc only for small, bounded sizes
    Random.Shared.NextBytes(bytes);           // use RandomNumberGenerator for secrets
    return Convert.ToHexString(bytes);
}

static ReadOnlySpan<char> GetValue(ReadOnlySpan<char> input, ReadOnlySpan<char> key)
{
    while (!input.IsEmpty)
    {
        var end = input.IndexOf(';');
        var pair = end < 0 ? input : input[..end];
        var eq = pair.IndexOf('=');
        if (eq > 0 && pair[..eq].SequenceEqual(key)) return pair[(eq + 1)..];
        input = end < 0 ? ReadOnlySpan<char>.Empty : input[(end + 1)..];
    }
    return ReadOnlySpan<char>.Empty;
}
```

Review rules:

- `stackalloc` with a size derived from input is a stack-overflow vector; cap it (`len <= 256 ? stackalloc byte[len] : ArrayPool…`).
- `Span<T>` cannot be stored in fields of classes, captured by lambdas, or used across `await` (`CS4012`); use `Memory<T>` for those cases.
- After `ArrayPool.Return` the array may be handed to someone else; any later use is a data race. Rented arrays can be **larger** than requested — use the requested length, not `buffer.Length`, when slicing results.
- `Stream.Read` may return fewer bytes than requested; use `ReadExactly`/`ReadAtLeast` (**New in .NET 7**) or loop.
- `MemoryExtensions.IndexOfAnyExcept` and `CommonPrefixLength` are **New in .NET 7**; `SearchValues`, `ContainsAny` and `Split` on spans are .NET 8.

***

#### Collections

```csharp
using System.Collections.Concurrent;
using System.Collections.Immutable;

// Mutation during enumeration
var stock = new Dictionary<string, int> { ["a"] = 0, ["b"] = 2 };
foreach (var key in stock.Keys.ToList())          // GOOD: snapshot before mutating
{
    if (stock[key] == 0) stock.Remove(key);
}
try
{
    foreach (var key in stock.Keys) stock[key + "-copy"] = 0;   // BAD: adding throws
}
catch (InvalidOperationException) { Console.WriteLine("collection was modified"); }

// ConcurrentDictionary.GetOrAdd may run the factory more than once under contention
var cache = new ConcurrentDictionary<int, Lazy<string>>();
var calls = 0;
Parallel.For(0, 100, _ =>
{
    var value = cache.GetOrAdd(1, key => new Lazy<string>(() =>
    {
        Interlocked.Increment(ref calls);
        return $"value {key}";
    })).Value;                                          // Lazy<T> makes creation run once
});
Console.WriteLine($"expensive factory ran {calls} time(s)");     // 1

// Read-only exposure
IDictionary<string, int> settings = new Dictionary<string, int> { ["timeout"] = 30 };
var readOnly = settings.AsReadOnly();               // New in .NET 7 (ReadOnlyDictionary)
var frozenish = ImmutableDictionary.CreateRange(settings);   // FrozenDictionary is .NET 8
Console.WriteLine($"{readOnly["timeout"]} {frozenish.Count}");

// PriorityQueue (.NET 6)
var queue = new PriorityQueue<string, int>();
queue.Enqueue("low", 10);
queue.Enqueue("high", 1);
Console.WriteLine(queue.Dequeue());                 // high
```

Review rules:

- Removing from a `Dictionary<,>` during enumeration happens to work on .NET Core 3.0+ (verified), but adding throws `InvalidOperationException`; do not rely on either — snapshot first.
- `ConcurrentDictionary.GetOrAdd(key, factory)` runs the factory without a lock and can run it several times (verified: multiple calls with a plain factory); wrap expensive or side-effecting factories in `Lazy<T>`.
- Return `IReadOnlyList<T>`/`IReadOnlyCollection<T>` from APIs; a `List<T>` cast to `IEnumerable<T>` can be cast back and mutated by callers.
- `ReadOnlyDictionary` and `AsReadOnly()` are views — the owner can still mutate the underlying dictionary. Use `ImmutableDictionary`/`ImmutableArray` for true immutability.
- `HashSet<string>`/`Dictionary<string,…>` with user-facing keys: specify the comparer (`StringComparer.Ordinal` or `OrdinalIgnoreCase`).
- `List<T>` in hot paths: pre-size with capacity when the count is known; `CollectionsMarshal.AsSpan(list)` (.NET 5) gives a span over the backing array (do not add/remove while using it).

***

#### LINQ

```csharp
int[] numbers = { 3, 1, 2 };
Console.WriteLine(string.Join(",", numbers.Order()));              // New in .NET 7: 1,2,3
Console.WriteLine(string.Join(",", numbers.OrderDescending()));    // 3,2,1
Console.WriteLine(string.Join(",", new[] { "b", "B", "a" }.Order(StringComparer.Ordinal)));  // B,a,b

// Deferred execution: the query runs on every enumeration
var calls = 0;
var doubled = numbers.Select(x => { calls++; return x * 2; });
_ = doubled.Count();
_ = doubled.Sum();
Console.WriteLine($"selector ran {calls} times");                  // 6, not 3
var materialized = numbers.Select(x => x * 2).ToList();            // GOOD: enumerate once

// Deferred execution sees later changes to the source
var list = new List<int> { 1, 2, 3 };
var greaterThanOne = list.Where(x => x > 1);
list.Add(4);
Console.WriteLine(greaterThanOne.Count());                         // 3

// First/Single semantics
try { _ = new[] { 1, 1 }.Single(x => x == 1); }
catch (InvalidOperationException ex) { Console.WriteLine(ex.Message); }   // Sequence contains more than one matching element
Console.WriteLine(Array.Empty<int>().FirstOrDefault(-1));          // -1 (.NET 6 overload)

// .NET 6 additions available in .NET 7
Console.WriteLine(new[] { 5, 9, 2 }.MaxBy(x => -x));               // 2
Console.WriteLine(string.Join("|", Enumerable.Range(1, 5).Chunk(2).Select(c => string.Join(",", c))));  // 1,2|3,4|5
Console.WriteLine(new[] { "a", "bb", "cc" }.DistinctBy(s => s.Length).Count());   // 2
```

Review rules:

- Multiple enumeration of an `IEnumerable<T>` re-runs the query (database round-trips, HTTP calls, iterator side effects). Materialize with `ToList()`/`ToArray()` once, or accept `IReadOnlyCollection<T>`.
- `Any()` instead of `Count() > 0` on lazy sequences; use the `Count`/`Length` property on materialized collections (analyzer `CA1829`).
- `First()` when absence is a bug, `FirstOrDefault()` when absence is expected and handled, `Single()` only when duplicates are a bug — `Single` must scan for a second match.
- `OrderBy(x => x)` → `Order()` (**New in .NET 7**). Do not suggest `CountBy`, `AggregateBy`, `Index` (.NET 9) or `LeftJoin` (.NET 10).
- `string` ordering with `Order()` uses the current culture by default; pass `StringComparer.Ordinal` for machine data.
- LINQ in hot loops allocates enumerators and delegates; profile before rewriting, but flag LINQ inside per-element loops (`O(n²)` `list.Where(...).First()` inside a `foreach` over the same list).
- `IQueryable<T>` (EF Core): a method call the provider cannot translate throws at run time in EF Core 3.0+ (no silent client evaluation), except in the final projection.

***
#### Resource Management: `IDisposable` and `IAsyncDisposable`

```csharp
var path = Path.Combine(Path.GetTempPath(), $"demo-{Guid.NewGuid():N}.txt");

// using declarations dispose at the end of the scope, in reverse order
await using (var writer = new StreamWriter(path))
{
    await writer.WriteLineAsync("hello");
}                                                   // flushed and closed here

using (var reader = new StreamReader(path))
{
    Console.WriteLine(await reader.ReadLineAsync());
}
File.Delete(path);

await using var resource = new AsyncResource();
resource.Use();
Console.WriteLine("end of scope");                  // DisposeAsync runs after this line

sealed class AsyncResource : IAsyncDisposable, IDisposable
{
    private readonly MemoryStream _stream = new();
    private bool _disposed;

    public void Use()
    {
        ObjectDisposedException.ThrowIf(_disposed, this);   // New in .NET 7
        _stream.WriteByte(1);
    }

    public async ValueTask DisposeAsync()
    {
        if (_disposed) return;
        _disposed = true;
        await _stream.DisposeAsync();
        Console.WriteLine("disposed asynchronously");
    }

    public void Dispose()
    {
        if (_disposed) return;
        _disposed = true;
        _stream.Dispose();
    }
}
```

Review rules:

- Every `IDisposable` created by the code is disposed on all paths (`using`); analyzer `CA2000` finds leaks when enabled. Objects obtained from DI are disposed by the container — never dispose an injected service.
- Prefer `await using` for types implementing `IAsyncDisposable` (`DbContext`, `Stream`, `Utf8JsonWriter`, `ServiceProvider`) in async code; synchronous `Dispose` may block on I/O.
- Implement the full `Dispose(bool)` pattern with a finalizer only when the class owns an unmanaged handle directly — and then prefer wrapping the handle in a `SafeHandle`. Sealed classes that hold only managed disposables need just `Dispose()`.
- Disposal must be idempotent; members called after disposal throw `ObjectDisposedException` (`ObjectDisposedException.ThrowIf`, **New in .NET 7**).
- A `using var` inside a method that returns the resource (or a task over it) disposes too early.

***

#### Dependency Injection

```csharp
using Microsoft.Extensions.DependencyInjection;

var services = new ServiceCollection();
services.AddSingleton<IClock, SystemClock>();            // stateless, thread-safe
services.AddScoped<IOrderRepository, InMemoryOrderRepository>();   // per request/scope
services.AddTransient<OrderService>();                   // lightweight, no shared state

await using var provider = services.BuildServiceProvider(new ServiceProviderOptions
{
    ValidateScopes = true,      // the ASP.NET Core host enables both in Development only
    ValidateOnBuild = true,
});

await using (var scope = provider.CreateAsyncScope())
{
    var service = scope.ServiceProvider.GetRequiredService<OrderService>();
    Console.WriteLine(await service.PlaceAsync(42m));
}

public interface IClock { DateTimeOffset UtcNow { get; } }
public sealed class SystemClock : IClock { public DateTimeOffset UtcNow => DateTimeOffset.UtcNow; }

public interface IOrderRepository { Task<int> AddAsync(decimal total, CancellationToken ct); }
public sealed class InMemoryOrderRepository : IOrderRepository
{
    private int _nextId;
    public Task<int> AddAsync(decimal total, CancellationToken ct) => Task.FromResult(++_nextId);
}

// C# 11 has no primary constructors for classes: explicit readonly fields
public sealed class OrderService
{
    private readonly IOrderRepository _repository;
    private readonly IClock _clock;

    public OrderService(IOrderRepository repository, IClock clock)
    {
        _repository = repository;
        _clock = clock;
    }

    public async Task<string> PlaceAsync(decimal total, CancellationToken ct = default)
    {
        var id = await _repository.AddAsync(total, ct);
        return $"order {id} at {_clock.UtcNow:O}";
    }
}
```

Captive dependencies — a singleton holding a scoped service — are the most common DI bug. Verified on .NET 7:

```csharp
using Microsoft.Extensions.DependencyInjection;

var services = new ServiceCollection();
services.AddScoped<RequestContext>();
services.AddSingleton<ReportCache>();       // BAD: singleton depends on a scoped service

try
{
    services.BuildServiceProvider(new ServiceProviderOptions { ValidateScopes = true, ValidateOnBuild = true });
}
catch (AggregateException ex)
{
    // Cannot consume scoped service 'RequestContext' from singleton 'ReportCache'.
    Console.WriteLine(ex.InnerExceptions[0].InnerException!.Message);
}

// Without validation (Production default) the scoped instance is silently captured forever
using var provider = services.BuildServiceProvider();
Console.WriteLine(provider.GetRequiredService<ReportCache>().Context.Id != Guid.Empty);

sealed class RequestContext { public Guid Id { get; } = Guid.NewGuid(); }
sealed class ReportCache
{
    public RequestContext Context { get; }
    public ReportCache(RequestContext context) => Context = context;
}
```

Review rules:

- Lifetimes: a service may depend only on services with the same or longer lifetime. `DbContext` is scoped; never inject it into a singleton or `BackgroundService` — inject `IServiceScopeFactory` and create a scope per unit of work, or use `IDbContextFactory<T>`.
- No service locator (`IServiceProvider.GetService` inside business code) except in factories and hosted services that create scopes.
- Constructor injection only; avoid `[FromServices]` property injection patterns. In ASP.NET Core 7 API controllers, action parameters whose type is registered in DI are bound from services automatically (breaking change in 7.0) — a complex parameter type accidentally registered in DI is no longer read from the body.[^9]
- Keyed services (`AddKeyedSingleton`, `[FromKeyedServices]`) are .NET 8; on .NET 7 register a factory or a dictionary of named instances.
- Disposable transients resolved from the root provider are kept alive until the root is disposed — a memory leak in long-running processes.

***

#### Configuration and Options

```csharp
using System.ComponentModel.DataAnnotations;
using Microsoft.Extensions.Configuration;
using Microsoft.Extensions.DependencyInjection;
using Microsoft.Extensions.Hosting;
using Microsoft.Extensions.Logging;
using Microsoft.Extensions.Options;

var builder = Host.CreateApplicationBuilder(args);       // New in .NET 7
builder.Logging.ClearProviders();
builder.Configuration.AddInMemoryCollection(new Dictionary<string, string?>
{
    ["Smtp:Host"] = "",
    ["Smtp:Port"] = "70000",
});

builder.Services.AddOptions<SmtpOptions>()
    .Bind(builder.Configuration.GetSection(SmtpOptions.SectionName))
    .ValidateDataAnnotations()
    .ValidateOnStart();                                  // fail at startup, not on first use

using var host = builder.Build();
try
{
    await host.StartAsync();
}
catch (OptionsValidationException ex)
{
    foreach (var failure in ex.Failures) Console.WriteLine(failure);
}

public sealed class SmtpOptions
{
    public const string SectionName = "Smtp";

    [Required] public string Host { get; set; } = "";
    [Range(1, 65535)] public int Port { get; set; } = 587;
}
```

Output (verified): both the empty `Host` (`[Required]` rejects empty strings) and the out-of-range `Port` are reported, and the host does not start.

Review rules:

- `IOptions<T>` is a singleton snapshot read once; `IOptionsSnapshot<T>` is scoped and re-reads per scope; `IOptionsMonitor<T>` is a singleton with change notifications. Injecting `IOptionsSnapshot<T>` into a singleton is a captive dependency.
- Without `ValidateOnStart()`, validation runs on the first `.Value` access — the app starts with broken configuration.
- Options classes bound from configuration use settable properties; `required`/`init` is not useful there (the binder sets properties after construction).
- Configuration binding to dictionaries **extends** values in .NET 7 rather than replacing them (breaking change).[^7]
- Secrets come from environment variables, user secrets (development) or a vault provider — never from `appsettings.json` committed to the repository. Environment variables use `__` as the section separator (`Smtp__Password`); `DOTNET_`/`ASPNETCORE_` prefixes configure the host.
- The source-generated configuration binder and `[OptionsValidator]` are .NET 8 — do not suggest them.

***

#### Logging

```csharp
using Microsoft.Extensions.Logging;

using var factory = LoggerFactory.Create(b => b
    .AddSimpleConsole(o => o.SingleLine = true)
    .SetMinimumLevel(LogLevel.Information));
var logger = factory.CreateLogger("Orders");

var orderId = 42;
var total = 99.5m;

// BAD: interpolation formats the string even when the level is disabled,
// and destroys the structured properties (CA2254)
logger.LogDebug($"Order {orderId} placed with total {total}");

// GOOD: message template with named placeholders (structured logging)
logger.LogInformation("Order {OrderId} placed with total {Total}", orderId, total);

// BEST for hot paths: source-generated logging (.NET 6+), no boxing, level check built in
Log.OrderPlaced(logger, orderId, total);

try { throw new InvalidOperationException("payment declined"); }
catch (InvalidOperationException ex)
{
    logger.LogError(ex, "Payment failed for order {OrderId}", orderId);   // exception first
}

static partial class Log
{
    [LoggerMessage(EventId = 1001, Level = LogLevel.Information,
        Message = "Order {OrderId} placed with total {Total}")]
    public static partial void OrderPlaced(ILogger logger, int orderId, decimal total);
}
```

Review rules:

- Message templates must be constant; placeholders are PascalCase names, not positional `{0}`. Analyzers: `CA2254` (template not constant), `CA1848` (use `LoggerMessage`) — both appear only with `AnalysisLevel` `latest-recommended` or higher (verified: default analysis shows neither).
- Pass the exception as the first argument (`LogError(ex, …)`), not `ex.Message` in the template.
- Never log secrets, tokens, passwords, connection strings, full request bodies, or personal data without masking.
- Use `ILogger<T>` via DI in services; `BeginScope` for correlation data. ASP.NET Core adds the trace/span IDs to the scope automatically when `IncludeScopes` is enabled.
- High-volume `LogDebug`/`LogTrace` with expensive arguments: guard with `logger.IsEnabled(LogLevel.Debug)` or use the source generator.

***

#### HttpClient

```csharp
using System.Net;
using System.Net.Http.Json;
using Microsoft.Extensions.DependencyInjection;

var services = new ServiceCollection();
services.AddHttpClient<GitHubClient>(client =>
{
    client.BaseAddress = new Uri("https://api.github.com/");
    client.Timeout = TimeSpan.FromSeconds(10);
    client.DefaultRequestHeaders.UserAgent.ParseAdd("demo/1.0");
})
.ConfigurePrimaryHttpMessageHandler(() => new FakeHandler());   // test double for this sample

await using var provider = services.BuildServiceProvider();
var gitHub = provider.GetRequiredService<GitHubClient>();
var repo = await gitHub.GetRepoAsync("dotnet", "runtime", CancellationToken.None);
Console.WriteLine(repo?.FullName);

// Without IHttpClientFactory: ONE long-lived client with a pooled-connection lifetime
var shared = new HttpClient(new SocketsHttpHandler
{
    PooledConnectionLifetime = TimeSpan.FromMinutes(2),   // picks up DNS changes
});
shared.Dispose();

public sealed record Repo(string FullName, int StargazersCount);

public sealed class GitHubClient
{
    private readonly HttpClient _http;
    public GitHubClient(HttpClient http) => _http = http;

    public async Task<Repo?> GetRepoAsync(string owner, string name, CancellationToken ct)
    {
        using var response = await _http.GetAsync(
            $"repos/{Uri.EscapeDataString(owner)}/{Uri.EscapeDataString(name)}", ct);
        if (response.StatusCode == HttpStatusCode.NotFound) return null;
        response.EnsureSuccessStatusCode();
        return await response.Content.ReadFromJsonAsync<Repo>(cancellationToken: ct);
    }
}

sealed class FakeHandler : HttpMessageHandler
{
    protected override Task<HttpResponseMessage> SendAsync(HttpRequestMessage request, CancellationToken ct) =>
        Task.FromResult(new HttpResponseMessage(HttpStatusCode.OK)
        {
            Content = JsonContent.Create(new { fullName = "dotnet/runtime", stargazersCount = 1 }),
        });
}
```

Review rules:

- `new HttpClient()` per request exhausts sockets (`TIME_WAIT`); a static client without `PooledConnectionLifetime` ignores DNS changes. Use `IHttpClientFactory` (typed or named clients) or one shared client with `SocketsHttpHandler.PooledConnectionLifetime`.[^27]
- Typed clients are transient; do not capture a typed client in a singleton (it pins one handler forever and defeats handler rotation).
- Always pass a `CancellationToken`; set `Timeout` deliberately (default 100 s). Dispose `HttpResponseMessage` (`using var response`), especially with `HttpCompletionOption.ResponseHeadersRead`.
- Check status codes before deserializing; `GetFromJsonAsync` throws `HttpRequestException` on non-success codes, which callers often do not expect.
- Escape URL segments (`Uri.EscapeDataString`) built from input — path injection (`../admin`) otherwise.
- Resilience on .NET 7: `Microsoft.Extensions.Http.Polly` 7.0.x (`AddTransientHttpErrorPolicy`, `AddPolicyHandler`). `Microsoft.Extensions.Http.Resilience` (`AddStandardResilienceHandler`) is part of the .NET 8 wave; its 8.x packages ship a `net6.0` asset, but `ConfigureHttpClientDefaults` (to apply it to all clients) is .NET 8 only. Retry only idempotent requests.
- `GetFromJsonAsAsyncEnumerable` is .NET 8; stream with `ReadAsStreamAsync` + `JsonSerializer.DeserializeAsyncEnumerable<T>`.

***

#### System.Text.Json

##### Defaults that surprise reviewers (verified on .NET 7)

```csharp
using System.Text.Json;

// Cache options: creating JsonSerializerOptions per call rebuilds metadata caches
var web = new JsonSerializerOptions(JsonSerializerDefaults.Web);

Console.WriteLine(JsonSerializer.Serialize(new { FirstName = "a", Count = 1 }, web));
// {"firstName":"a","count":1}

Console.WriteLine(JsonSerializer.Deserialize<Person>("""{"firstName":"x"}""")!.FirstName ?? "null");
// null: default options are case-SENSITIVE (ASP.NET Core uses Web defaults: case-insensitive, camelCase)

Console.WriteLine(JsonSerializer.Deserialize<Person>("""{"FirstName":"x","Unknown":1}""")!.FirstName);
// x: unknown members are silently ignored (Disallow handling is .NET 8)

try { JsonSerializer.Deserialize<Counter>("""{"Count":"5"}"""); }
catch (JsonException) { Console.WriteLine("quoted number rejected by default options"); }
Console.WriteLine(JsonSerializer.Deserialize<Counter>("""{"Count":"5"}""", web)!.Count);   // 5 with Web defaults

Console.WriteLine(JsonSerializer.Serialize(new { Html = "<script>&'" }));
// {"Html":"<script>&'"}: HTML-sensitive characters escaped by default

sealed class Person { public string? FirstName { get; set; } }
sealed class Counter { public int Count { get; set; } }
```

##### New in .NET 7: polymorphism

```csharp
using System.Text.Json;
using System.Text.Json.Serialization;

Shape shape = new Circle { Radius = 2 };
var json = JsonSerializer.Serialize(shape);               // declared type Shape
Console.WriteLine(json);                                  // {"$kind":"circle","Radius":2}
Console.WriteLine(JsonSerializer.Deserialize<Shape>(json)!.GetType().Name);   // Circle

// The discriminator must be the FIRST property in .NET 7
try { JsonSerializer.Deserialize<Shape>("""{"Radius":2,"$kind":"circle"}"""); }
catch (NotSupportedException) { Console.WriteLine("out-of-order discriminator: NotSupportedException"); }

try { JsonSerializer.Deserialize<Shape>("""{"$kind":"hexagon"}"""); }
catch (JsonException) { Console.WriteLine("unknown discriminator: JsonException"); }

// Polymorphism applies only when the DECLARED type is the annotated base type
Console.WriteLine(JsonSerializer.Serialize(new Circle { Radius = 1 }));   // {"Radius":1}
Console.WriteLine(JsonSerializer.Serialize<object>(new Circle { Radius = 1 }));   // {"Radius":1} on .NET 7

[JsonPolymorphic(TypeDiscriminatorPropertyName = "$kind")]
[JsonDerivedType(typeof(Circle), "circle")]
[JsonDerivedType(typeof(Square), "square")]
abstract class Shape { }
sealed class Circle : Shape { public double Radius { get; set; } }
sealed class Square : Shape { public double Side { get; set; } }
```

Verified pitfalls: a discriminator that is not the first JSON property makes .NET 7 fall back to the abstract base type and throw **`NotSupportedException`** (not `JsonException`) — `catch (JsonException)` around deserialization of client input misses it and produces a 500. `AllowOutOfOrderMetadataProperties` is .NET 9. Serializing as `object` omits the discriminator on .NET 7 (the same call on .NET 8.0.32 emits `"$kind"`). Never deserialize polymorphic types from untrusted input with `TypeNameHandling`-style type names (Newtonsoft.Json `TypeNameHandling.All/Auto` is a remote-code-execution risk); STJ only allows the derived types you register.[^30]

##### New in .NET 7: contract customization

```csharp
using System.Text.Json;
using System.Text.Json.Serialization.Metadata;

var options = new JsonSerializerOptions
{
    TypeInfoResolver = new DefaultJsonTypeInfoResolver
    {
        Modifiers = { HideSensitiveProperties },
    },
};

Console.WriteLine(JsonSerializer.Serialize(new User { Name = "ada", PasswordHash = "secret" }, options));
// {"Name":"ada"}

static void HideSensitiveProperties(JsonTypeInfo typeInfo)
{
    if (typeInfo.Kind != JsonTypeInfoKind.Object) return;
    foreach (var property in typeInfo.Properties)
    {
        if (property.AttributeProvider?.IsDefined(typeof(SensitiveAttribute), inherit: true) == true)
        {
            property.ShouldSerialize = static (_, _) => false;
        }
    }
}

[AttributeUsage(AttributeTargets.Property)]
sealed class SensitiveAttribute : Attribute { }

sealed class User
{
    public string Name { get; set; } = "";
    [Sensitive] public string PasswordHash { get; set; } = "";
}
```

Combine resolvers with `JsonTypeInfoResolver.Combine(...)` (.NET 7); `TypeInfoResolverChain` and `WithAddedModifier` are .NET 8.[^31]

##### Source generation and required properties

```csharp
using System.Text.Json;
using System.Text.Json.Serialization;

var product = new Product(1, "widget");
var json = JsonSerializer.Serialize(product, AppJsonContext.Default.Product);
Console.WriteLine(json);                                                 // {"id":1,"name":"widget"}
Console.WriteLine(JsonSerializer.Deserialize(json, AppJsonContext.Default.Product)!.Name);

try { JsonSerializer.Deserialize<Signup>("""{"Email":"a@example.com"}"""); }
catch (JsonException ex) { Console.WriteLine(ex.Message); }
// JSON deserialization for type 'Signup' was missing required properties, including the following: Age

public sealed record Product(int Id, string Name);

public sealed class Signup
{
    public required string Email { get; init; }
    [JsonRequired] public int Age { get; set; }     // New in .NET 7: required in JSON only
}

[JsonSerializable(typeof(Product))]
[JsonSourceGenerationOptions(PropertyNamingPolicy = JsonKnownNamingPolicy.CamelCase)]
internal sealed partial class AppJsonContext : JsonSerializerContext { }
```

Review rules:

- Reuse one `JsonSerializerOptions` instance (static or DI); per-call `new JsonSerializerOptions()` is a known throughput and memory problem.
- Validate deserialized input (nullability, ranges, lengths); STJ does not enforce nullable annotations in .NET 7.
- Set `MaxDepth` (default 64) and request body size limits for untrusted payloads; avoid `JsonDocument` without `using` (it rents pooled memory).
- Snake-case or kebab-case naming policies are .NET 8; write a `JsonNamingPolicy` subclass on .NET 7.
- Source-generated contexts are required for trimming/Native AOT; they also avoid reflection at startup.

***

#### New in .NET 7: Regular Expressions

```csharp
using System.Diagnostics;
using System.Text.RegularExpressions;

var input = new string('a', 30) + "!";

// BAD: nested quantifiers + backtracking engine + no timeout = ReDoS
var sw = Stopwatch.StartNew();
try
{
    Regex.IsMatch(input, "^(a+)+$", RegexOptions.None, TimeSpan.FromMilliseconds(100));
}
catch (RegexMatchTimeoutException)
{
    Console.WriteLine("backtracking engine hit the timeout");
}

// GOOD for untrusted patterns/inputs: linear-time engine
Console.WriteLine(Regex.IsMatch(input, "^(a+)+$", RegexOptions.NonBacktracking));   // False, fast

try { _ = new Regex(@"(a)\1", RegexOptions.NonBacktracking); }
catch (NotSupportedException) { Console.WriteLine("NonBacktracking rejects backreferences"); }

// GOOD for fixed patterns: source-generated regex, compiled at build time
Console.WriteLine(Patterns.Digits().IsMatch("abc123"));          // True
Console.WriteLine(Patterns.Digits().Count("1 22 333"));          // 3 (New in .NET 7)
foreach (var match in Patterns.Digits().EnumerateMatches("a1b22"))   // allocation-free
{
    Console.WriteLine($"{match.Index}:{match.Length}");
}

static partial class Patterns
{
    [GeneratedRegex(@"\d+", RegexOptions.CultureInvariant, matchTimeoutMilliseconds: 1000)]
    public static partial Regex Digits();
}
```

Review rules:

- Patterns or inputs from users: `RegexOptions.NonBacktracking` (no lookarounds, backreferences, atomic groups or `RightToLeft`) or a match timeout; never an unbounded backtracking regex.[^20]
- Constant patterns: `[GeneratedRegex]` on a `static partial` **method** (analyzer `SYSLIB1045` suggests it). Partial **properties** for `[GeneratedRegex]` are .NET 9.
- Cache `Regex` instances; `new Regex(...)` per call re-parses. `RegexOptions.Compiled` costs startup time; the source generator is preferable in .NET 7.
- .NET 7 corrected some character-range behaviours under `IgnoreCase` (breaking change "Regex patterns with ranges corrected").[^7]

***

#### Files, Streams and Archives

```csharp
using System.Formats.Tar;

var work = Directory.CreateTempSubdirectory("review-");          // New in .NET 7
try
{
    var path = Path.Combine(work.FullName, "data.txt");
    await File.WriteAllTextAsync(path, "line 1\nline 2\n");
    Console.WriteLine(Path.Exists(path));                         // New in .NET 7

    await foreach (var line in File.ReadLinesAsync(path))          // New in .NET 7
    {
        Console.WriteLine(line);
    }

    // Stream.Read may return fewer bytes than asked; ReadExactly loops for you
    await using var stream = File.OpenRead(path);
    var header = new byte[4];
    await stream.ReadExactlyAsync(header);                         // New in .NET 7
    Console.WriteLine(System.Text.Encoding.ASCII.GetString(header));

    // Unix permissions (New in .NET 7; no-op/unsupported on Windows)
    if (!OperatingSystem.IsWindows())
    {
        File.SetUnixFileMode(path, UnixFileMode.UserRead | UnixFileMode.UserWrite);
        Console.WriteLine(File.GetUnixFileMode(path));
    }

    // TAR (New in .NET 7): ExtractToDirectory rejects entries escaping the destination
    var tarPath = Path.Combine(work.FullName, "evil.tar");
    await using (var tarStream = File.Create(tarPath))
    await using (var writer = new TarWriter(tarStream))
    {
        var entry = new PaxTarEntry(TarEntryType.RegularFile, "../escape.txt")
        {
            DataStream = new MemoryStream("x"u8.ToArray()),
        };
        await writer.WriteEntryAsync(entry);
    }
    var destination = Directory.CreateDirectory(Path.Combine(work.FullName, "out")).FullName;
    try
    {
        await TarFile.ExtractToDirectoryAsync(tarPath, destination, overwriteFiles: false);
    }
    catch (IOException ex)
    {
        Console.WriteLine($"blocked: {ex.Message[..40]}…");
    }
}
finally
{
    work.Delete(recursive: true);
}
```

Review rules:

- `TarFile.ExtractToDirectory` rejects `../` entries (verified), but manual extraction with `TarReader` + `entry.ExtractToFile(Path.Combine(dest, entry.Name))` does **not** — validate every entry path (see Path traversal). The same applies to `ZipArchive` entries ("zip slip"); `ZipFile.ExtractToDirectory` validates, manual loops do not.
- Limit decompressed sizes (zip/tar bombs): check entry lengths and count bytes while copying.
- Prefer async file APIs in server code; `File.ReadAllText` on large files loads everything into memory — stream instead.
- Specify encodings explicitly when interoperating (`File.WriteAllText` defaults to UTF-8 without BOM).
- `Path.Combine(root, userInput)` returns `userInput` when it is rooted (`/etc/passwd`) — verified; never combine untrusted input without validation.

***
#### ASP.NET Core 7 Essentials

##### Minimal APIs: route groups, typed results, filters (New in .NET 7)

```csharp
using Microsoft.AspNetCore.Builder;
using Microsoft.AspNetCore.Http;
using Microsoft.AspNetCore.Http.HttpResults;
using Microsoft.Extensions.DependencyInjection;

var builder = WebApplication.CreateBuilder(args);
builder.Services.AddProblemDetails();                       // New in .NET 7
builder.Services.AddSingleton<TodoStore>();

var app = builder.Build();
app.UseExceptionHandler();                                  // problem details for unhandled exceptions
app.UseStatusCodePages();                                   // problem details for empty 4xx/5xx

var todos = app.MapGroup("/api/todos");                     // New in .NET 7

// Results<...> documents every possible response for OpenAPI and unit tests
todos.MapGet("/{id:int}", Results<Ok<Todo>, NotFound> (int id, TodoStore store) =>
    store.Find(id) is { } todo ? TypedResults.Ok(todo) : TypedResults.NotFound());

todos.MapPost("/", (Todo todo, TodoStore store) =>
    {
        store.Add(todo);
        return TypedResults.Created($"/api/todos/{todo.Id}", todo);
    })
    .AddEndpointFilter(async (context, next) =>             // New in .NET 7
    {
        var todo = context.GetArgument<Todo>(0);
        if (string.IsNullOrWhiteSpace(todo.Title))
        {
            return TypedResults.ValidationProblem(new Dictionary<string, string[]>
            {
                ["title"] = new[] { "Title is required." },
            });
        }
        return await next(context);
    });

app.Run();

public sealed record Todo(int Id, string Title);

public sealed class TodoStore
{
    private readonly System.Collections.Concurrent.ConcurrentDictionary<int, Todo> _items = new();
    public Todo? Find(int id) => _items.TryGetValue(id, out var t) ? t : null;
    public void Add(Todo todo) => _items[todo.Id] = todo;
}
```

Verified behaviour of this pipeline on ASP.NET Core 7.0.20 (Kestrel, real HTTP requests):

- `GET /api/todos/2` → `404`; `POST` with a blank title → `400 application/problem+json` with an `errors` dictionary.
- An unhandled exception → `500` whose body does **not** contain the exception message (Production environment).
- **The default problem-details writer in .NET 7 only writes JSON when the request's `Accept` header allows `application/json`.** A request without `Accept` (e.g. a bare `HttpClient`) gets an **empty** 500 body from `UseExceptionHandler()` and a `text/plain` body (`Status Code: 404; Not Found`) from `UseStatusCodePages()`. Clients and tests must send `Accept: application/json`.
- `Results<T1,…>` exists for 2 to 6 type arguments in .NET 7.

Other minimal-API facts for .NET 7:[^8]

- `[AsParameters]` binds a parameter object (route, query, header, services) — **New in .NET 7**.
- Arrays and `StringValues` bind from query strings and headers; the body can be bound as `Stream`/`PipeReader`.
- `IFormFile`/`IFormFileCollection` binding is **new in .NET 7** and has **no built-in antiforgery** protection; cookie-authenticated upload endpoints need `IAntiforgery.ValidateRequestAsync` called manually. Automatic antiforgery for minimal APIs (`UseAntiforgery()`) is .NET 8.
- OpenAPI: `Microsoft.AspNetCore.OpenApi` 7.0.x package (`WithOpenApi()`) plus Swashbuckle; built-in document generation (`AddOpenApi`) is .NET 9.
- Minimal APIs bind route/query values through a static `TryParse` method (since .NET 6); MVC and API controllers can bind types implementing `IParsable<T>` (**New in .NET 7**). A value that fails to parse produces a `400`.

##### Middleware order

```csharp
using Microsoft.AspNetCore.Authentication.JwtBearer;
using Microsoft.AspNetCore.Builder;
using Microsoft.AspNetCore.Http;
using Microsoft.Extensions.DependencyInjection;
using Microsoft.Extensions.Hosting;

var builder = WebApplication.CreateBuilder(args);
builder.Services.AddProblemDetails();
builder.Services.AddCors(o => o.AddPolicy("spa", p => p.WithOrigins("https://app.example.com")));
builder.Services.AddAuthentication().AddJwtBearer();      // single scheme = default scheme (New in .NET 7)
builder.Services.AddAuthorization();
builder.Services.AddRateLimiter(_ => { });
builder.Services.AddOutputCache();

var app = builder.Build();

if (!app.Environment.IsDevelopment())
{
    app.UseExceptionHandler();
    app.UseHsts();
}
app.UseHttpsRedirection();
app.UseCors("spa");                  // before auth and output cache
app.UseAuthentication();
app.UseAuthorization();
app.UseRateLimiter();                // after routing data is available (WebApplication adds UseRouting implicitly)
app.UseOutputCache();                // after CORS, authentication and authorization

app.MapGet("/me", (System.Security.Claims.ClaimsPrincipal user) => user.Identity?.Name)
   .RequireAuthorization();

app.Run();
```

Review rules:

- `UseOutputCache` must come after `UseCors`, `UseAuthentication` and `UseAuthorization`; otherwise cached content can be served to unauthorized callers.[^22]
- `UseHttpsRedirection`/`UseHsts` in production; Kestrel no longer binds HTTPS by default in .NET 7 (breaking change) — configure endpoints explicitly.[^9]
- With a single authentication scheme registered, .NET 7 uses it as the default scheme automatically;[^8] with several, set `DefaultScheme`/`DefaultChallengeScheme` explicitly.
- `JwtBearer`: never disable `ValidateIssuer`, `ValidateAudience`, `ValidateLifetime` or `ValidateIssuerSigningKey`; never set `RequireHttpsMetadata = false` outside development. `dotnet user-jwts` (**New in .NET 7**) issues local development tokens.
- CORS: `AllowAnyOrigin()` with credentials is rejected by the framework; `SetIsOriginAllowed(_ => true)` + `AllowCredentials()` is the same vulnerability in disguise.
- Forwarded headers behind a proxy: `UseForwardedHeaders` with `KnownProxies`/`KnownNetworks` restricted, or client IPs (used by rate limiting and logging) are spoofable.

##### New in .NET 7: rate limiting middleware

```csharp
using System.Threading.RateLimiting;
using Microsoft.AspNetCore.Builder;
using Microsoft.AspNetCore.Http;
using Microsoft.AspNetCore.RateLimiting;
using Microsoft.Extensions.DependencyInjection;

var builder = WebApplication.CreateBuilder(args);
builder.Services.AddRateLimiter(options =>
{
    options.RejectionStatusCode = StatusCodes.Status429TooManyRequests;   // default is 503!

    options.AddFixedWindowLimiter("login", o =>
    {
        o.PermitLimit = 5;
        o.Window = TimeSpan.FromMinutes(1);
        o.QueueLimit = 0;
    });

    // Per-client partitioning; the key must not be attacker-controlled without bounds
    options.GlobalLimiter = PartitionedRateLimiter.Create<HttpContext, string>(context =>
        RateLimitPartition.GetTokenBucketLimiter(
            context.Connection.RemoteIpAddress?.ToString() ?? "unknown",
            _ => new TokenBucketRateLimiterOptions
            {
                TokenLimit = 100,
                TokensPerPeriod = 50,
                ReplenishmentPeriod = TimeSpan.FromSeconds(10),
                QueueLimit = 0,
                AutoReplenishment = true,
            }));
});

var app = builder.Build();
app.UseRateLimiter();
app.MapPost("/login", () => Results.Ok()).RequireRateLimiting("login");
app.Run();
```

Verified: without setting `RejectionStatusCode`, a request over the limit gets **`503 Service Unavailable`**, which load balancers and clients treat as a server fault and may retry. Set `429` and consider `OnRejected` to add `Retry-After`.[^21] The limiter state is in-process — with several instances each one enforces its own limit.

##### New in .NET 7: output caching

```csharp
using Microsoft.AspNetCore.Builder;
using Microsoft.AspNetCore.Http;
using Microsoft.AspNetCore.OutputCaching;
using Microsoft.Extensions.DependencyInjection;

var builder = WebApplication.CreateBuilder(args);
builder.Services.AddOutputCache(options =>
{
    options.AddBasePolicy(b => b.Expire(TimeSpan.FromSeconds(30)));
    options.AddPolicy("ByCulture", b => b.SetVaryByQuery("culture").Tag("catalog"));
});

var app = builder.Build();
app.UseOutputCache();

app.MapGet("/catalog", () => DateTime.UtcNow.ToString("O")).CacheOutput("ByCulture");
app.MapPost("/catalog/purge", async (IOutputCacheStore store, CancellationToken ct) =>
{
    await store.EvictByTagAsync("catalog", ct);
    return Results.NoContent();
});
app.Run();
```

Default policy (verified for the authenticated case): only `200` responses to `GET`/`HEAD` are cached; responses that set cookies and responses to requests with an `Authorization` header or an authenticated user are not cached; default expiration 60 s; storage is in-process memory (`SizeLimit` 100 MB, `MaximumBodySize` 64 MB). A custom policy that enables caching for authenticated requests must vary by user, or one user's data leaks to another. The Redis output-cache store is .NET 8.[^22]

##### Controllers

- `[ApiController]` gives automatic `400` validation responses and binding-source inference. **.NET 7:** parameters whose type is registered in DI are now inferred as `[FromServices]`.[^9]
- Return `ActionResult<T>`; validate with data annotations or FluentValidation; never bind EF entities directly from requests (over-posting) — use request DTOs.
- MVC's empty-body detection changed in .NET 7: it uses `IHttpRequestBodyDetectionFeature.CanHaveBody`, so a request without `Content-Length`/chunked encoding is treated as having no body ("A non-empty request body is required."); optional `[FromBody]` parameters now bind correctly. Clients must send proper headers.[^9]

***

#### EF Core 7

EF Core 7.0.20 is the last EF Core 7 release and is out of support together with .NET 7.[^2] EF Core 7 targets `net6.0`, so it also runs on .NET 7.[^11]

##### New in EF Core 7: `ExecuteUpdate` / `ExecuteDelete`

```csharp
using Microsoft.EntityFrameworkCore;

await using var db = new ShopContext();
await db.Database.OpenConnectionAsync();                 // keep the in-memory SQLite DB alive
await db.Database.EnsureCreatedAsync();
db.Products.AddRange(
    new Product { Name = "a", Price = 10m, Stock = 0 },
    new Product { Name = "b", Price = 20m, Stock = 5 });
await db.SaveChangesAsync();

var tracked = await db.Products.SingleAsync(p => p.Name == "b");

// One UPDATE statement; no entities loaded; no SaveChanges needed
var updated = await db.Products
    .Where(p => p.Stock > 0)
    .ExecuteUpdateAsync(s => s.SetProperty(p => p.Price, p => p.Price * 2));

Console.WriteLine($"updated {updated}; tracked entity still says {tracked.Price}");   // 20: stale
var fresh = await db.Products.AsNoTracking().Where(p => p.Name == "b").Select(p => p.Price).SingleAsync();
Console.WriteLine($"database says {fresh}");                                          // 40

var deleted = await db.Products.Where(p => p.Stock == 0).ExecuteDeleteAsync();
Console.WriteLine($"deleted {deleted}");

public sealed class Product
{
    public int Id { get; set; }
    public required string Name { get; set; }
    public decimal Price { get; set; }
    public int Stock { get; set; }
}

public sealed class ShopContext : DbContext
{
    public DbSet<Product> Products => Set<Product>();
    protected override void OnConfiguring(DbContextOptionsBuilder options) =>
        options.UseSqlite("Data Source=:memory:");
}
```

Review rules for bulk operations:[^10]

- They bypass the change tracker: already-tracked entities keep stale values (verified above), `SaveChanges` interceptors, `SavingChanges` events, auditing/soft-delete logic implemented in `SaveChanges`, and concurrency tokens do **not** run.
- Several `ExecuteUpdate`/`ExecuteDelete` calls are not one transaction; wrap them in `BeginTransactionAsync` when they must be atomic.
- `ExecuteDelete` on principals with dependents relies on database cascade rules, not EF's client-side cascade.
- EF Core 7 `SetProperty` requires expression lambdas; the non-expression setter overloads arrived in EF Core 10.

##### Raw SQL and injection

```csharp
using Microsoft.EntityFrameworkCore;

await using var db = new UserContext();
await db.Database.OpenConnectionAsync();
await db.Database.EnsureCreatedAsync();
db.Users.Add(new User { Name = "alice" });
await db.SaveChangesAsync();

var input = "alice' OR '1'='1";

// GOOD: FromSql (New in EF Core 7) turns interpolation holes into DbParameters
var safe = await db.Users.FromSql($"SELECT * FROM Users WHERE Name = {input}").ToListAsync();
Console.WriteLine($"FromSql: {safe.Count} rows");                     // 0

// BAD: string concatenation into FromSqlRaw
var injected = await db.Users.FromSqlRaw("SELECT * FROM Users WHERE Name = '" + input + "'").ToListAsync();
Console.WriteLine($"FromSqlRaw + concatenation: {injected.Count} rows");   // 1: injection

// Scalar queries (New in EF Core 7) and parameterized commands
var ids = await db.Database.SqlQuery<int>($"SELECT Id AS Value FROM Users").ToListAsync();
var affected = await db.Database.ExecuteSqlAsync($"UPDATE Users SET Name = {"bob"} WHERE Id = {ids[0]}");
Console.WriteLine($"{ids.Count} id(s), {affected} row(s) updated");

public sealed class User { public int Id { get; set; } public string Name { get; set; } = ""; }

public sealed class UserContext : DbContext
{
    public DbSet<User> Users => Set<User>();
    protected override void OnConfiguring(DbContextOptionsBuilder options) =>
        options.UseSqlite("Data Source=:memory:");
}
```

- `FromSql`, `ExecuteSql` and `SqlQuery` (scalar types) are **New in EF Core 7**; `SqlQuery<T>` for unmapped entity-like types is EF Core 8.[^37]
- `FromSqlRaw`/`ExecuteSqlRaw` with a `string` built by concatenation or by an interpolated `string` variable is SQL injection; with `FormattableString` / `{0}` placeholders plus arguments it is parameterized. Identifiers (column names) cannot be parameters — whitelist them.

##### EF Core 7 breaking changes to check during review[^11]

- **SQL Server `Encrypt=True` by default** (Microsoft.Data.SqlClient 4+): connections fail against servers without a trusted certificate. The fix is a valid certificate — `TrustServerCertificate=True` or `Encrypt=False` in production connection strings are security findings.
- **SQL Server tables with triggers** (and some computed columns) fail on `SaveChanges` because EF 7 uses `OUTPUT` without `INTO`: configure `entity.ToTable(tb => tb.HasTrigger("TriggerName"))` (`UseSqlOutputClause(false)` is EF 8).
- **SQLite**: `RETURNING` is used for saves — tables with `AFTER` triggers or virtual tables need `ReplaceService<IUpdateSqlGenerator, SqliteLegacyUpdateSqlGenerator>()`; without WAL mode, busy/locked errors become more likely.
- Orphaned dependents of optional relationships are no longer deleted automatically.
- JSON columns (`OwnsOne(..., b => b.ToJson())`) are implemented only by the **SQL Server** provider in EF 7 (SQLite support is EF 8); verified: the SQLite provider 7.0.20 fails at `EnsureCreated` with a `NullReferenceException`.

##### General EF Core rules

- `DbContext` is not thread-safe: one context per request/unit of work; never share it across concurrent tasks (`Task.WhenAll` over queries on the same context throws `InvalidOperationException`).
- Read-only queries: `AsNoTracking()`; project with `Select` to DTOs instead of loading full graphs.
- N+1: lazy loading in loops issues one query per item; use `Include`/projection. Cartesian explosion with several collection `Include`s: `AsSplitQuery()`.
- Always `await` the async APIs (`ToListAsync`, `SaveChangesAsync`) with a `CancellationToken`; `IQueryable` enumerated synchronously in ASP.NET Core blocks threads.
- Migrations: review generated migrations for data loss (`DropColumn`, type changes); apply them in a controlled deployment step, not with `Database.Migrate()` racing on several instances at startup.
- Connection strings and credentials come from configuration/secret stores; scaffolded `OnConfiguring` with an inline connection string must not be committed.

***

#### Security

##### Injection

- SQL: parameters only (`SqlCommand.Parameters`, Dapper parameters, EF `FromSql`/LINQ). Raw string literals and interpolation do not parameterize.
- Command execution: `ProcessStartInfo` with `ArgumentList` (each argument separate), `UseShellExecute = false`; never `/bin/sh -c` with input.
- LDAP, XPath, regex patterns, log messages (CRLF injection in plain-text logs) and HTTP headers built from input need encoding/validation.
- HTML: Razor encodes by default; `Html.Raw`/`MarkupString` with user data is XSS.

##### Path traversal

```csharp
var root = Path.GetFullPath(Path.Combine(Path.GetTempPath(), "uploads"));

Console.WriteLine(Path.Combine(root, "/etc/passwd"));          // /etc/passwd: rooted input wins
Console.WriteLine(IsInside(root, "../../etc/passwd"));         // False
Console.WriteLine(IsInside(root, "reports/2026.txt"));         // True
Console.WriteLine(IsInside(root, "/etc/passwd"));              // False

static bool IsInside(string root, string userPath)
{
    var full = Path.GetFullPath(Path.Combine(root, userPath));
    var prefix = root.EndsWith(Path.DirectorySeparatorChar) ? root : root + Path.DirectorySeparatorChar;
    return full.StartsWith(prefix, OperatingSystem.IsWindows()
        ? StringComparison.OrdinalIgnoreCase
        : StringComparison.Ordinal);
}
```

Check the prefix *with* a trailing separator (`/data/uploads2` starts with `/data/uploads`). For uploads, generate the file name on the server (`Path.GetRandomFileName()`), never trust `IFormFile.FileName`, and enforce size limits (`[RequestSizeLimit]`, `FormOptions.MultipartBodyLengthLimit`).

##### Cryptography and secrets

```csharp
using System.Security.Cryptography;
using System.Text;

// Password hashing: PBKDF2-SHA256 with a high iteration count and a random salt
byte[] salt = RandomNumberGenerator.GetBytes(16);
byte[] hash = Rfc2898DeriveBytes.Pbkdf2("correct horse", salt, 600_000, HashAlgorithmName.SHA256, 32);
byte[] check = Rfc2898DeriveBytes.Pbkdf2("correct horse", salt, 600_000, HashAlgorithmName.SHA256, 32);
Console.WriteLine(CryptographicOperations.FixedTimeEquals(hash, check));   // constant-time compare

// Tokens: CSPRNG, not System.Random
string token = Convert.ToBase64String(RandomNumberGenerator.GetBytes(32));
Console.WriteLine(token.Length);                                           // 44

// One-shot hashing (.NET 5+)
Console.WriteLine(Convert.ToHexString(SHA256.HashData(Encoding.UTF8.GetBytes("abc"))));

// Authenticated encryption
var key = RandomNumberGenerator.GetBytes(32);
using var aes = new AesGcm(key);
var nonce = RandomNumberGenerator.GetBytes(AesGcm.NonceByteSizes.MaxSize);   // unique per message
var plaintext = Encoding.UTF8.GetBytes("secret");
var ciphertext = new byte[plaintext.Length];
var tag = new byte[AesGcm.TagByteSizes.MaxSize];
aes.Encrypt(nonce, plaintext, ciphertext, tag);
var roundTrip = new byte[ciphertext.Length];
aes.Decrypt(nonce, ciphertext, tag, roundTrip);
Console.WriteLine(Encoding.UTF8.GetString(roundTrip));
```

Review rules:

- `new Rfc2898DeriveBytes(password, saltSize)` (SHA-1, 1000 iterations) is obsolete in .NET 7 (`SYSLIB0041`); use the static `Pbkdf2` with SHA-256 and ≥ 600,000 iterations (OWASP guidance), or ASP.NET Core Identity's `PasswordHasher<T>`.
- `System.Random`/`Random.Shared` for tokens, IDs or salts is a vulnerability; use `RandomNumberGenerator`. `RandomNumberGenerator.GetString`/`GetHexString` are .NET 8.
- Compare MACs/tokens with `CryptographicOperations.FixedTimeEquals`, not `SequenceEqual` or `==`.
- No MD5/SHA-1 for security, no ECB, no AES-CBC without a MAC; reuse of an AES-GCM nonce with the same key breaks confidentiality and integrity.
- `HashAlgorithm.Create("SHA256")` and other name-based factories are obsolete (`SYSLIB0045`); call `SHA256.Create()` or the static `HashData`.
- TLS: do not pin `SslProtocols.Tls`/`Tls11` (`SYSLIB0039`); leave `SslProtocols.None` (OS default). `ServerCertificateCustomValidationCallback = (_, _, _, _) => true` or `HttpClientHandler.DangerousAcceptAnyServerCertificateValidator` in production code is a critical finding.
- Data protection keys in multi-instance deployments must be persisted and shared (`PersistKeysTo…`), or cookies/antiforgery tokens break and keys regenerate on restart.

##### Deserialization and XML

- `BinaryFormatter`, `SoapFormatter`, `NetDataContractSerializer`, `LosFormatter`, `ObjectStateFormatter` and Newtonsoft.Json with `TypeNameHandling` other than `None` on untrusted data are remote-code-execution risks. `BinaryFormatter.Serialize`/`Deserialize` is a compile **error** (`SYSLIB0011`) in .NET 7 SDK builds — `<NoWarn>SYSLIB0011</NoWarn>` or `EnableUnsafeBinaryFormatterSerialization` in a diff is a red flag.[^38]
- `XmlReader`/`XmlDocument` in .NET Core do not resolve external entities by default; do not set an `XmlUrlResolver`. `XmlSecureResolver` is obsolete (`SYSLIB0047`) — use `XmlResolver.ThrowingResolver` (**New in .NET 7**).

##### Unsupported runtime

Running on .NET 7 means known, unpatched vulnerabilities in the runtime, ASP.NET Core (Kestrel) and EF Core (all CVEs published after May 2024). Container images based on `mcr.microsoft.com/dotnet/aspnet:7.0` no longer receive OS-package updates either. The fix is retargeting to `net10.0`, not patching around it.

***

#### Performance

- Measure first: BenchmarkDotNet for micro-benchmarks, `dotnet-counters`/`dotnet-trace` for services. Do not accept "optimizations" without numbers.
- **Dynamic PGO is off by default in .NET 7** (`<TieredPGO>true</TieredPGO>` enables it; it is on by default from .NET 8). On-stack replacement is enabled, so long-running loops in Tier-0 methods get optimized.[^6][^28]
- **Native AOT** in .NET 7 supports console apps only (no ASP.NET Core), requires trimming-compatible code and reflection-free serialization (STJ source generation).[^29]
- Server GC is the ASP.NET Core default; containers with tight memory limits may need `<ConcurrentGarbageCollection>`/`GCHeapHardLimit` tuning (`DOTNET_GCHeapHardLimit`).
- Avoid allocations in hot paths: `StringBuilder` or `string.Create` for string building, spans for parsing, `ArrayPool<T>`, `struct` enumerators (`foreach` over `List<T>` directly, not via `IEnumerable<T>`), cached delegates and `JsonSerializerOptions`.
- `async` everywhere for I/O; `ValueTask` only where profiling shows allocation pressure.
- `[GeneratedRegex]`, STJ source generation, `[LoggerMessage]` and `LibraryImport` move work from run time to build time.
- Do not hand-roll loops instead of LINQ/BCL methods for "performance" without a benchmark; many BCL span and LINQ paths are vectorized.
- Database: most production performance problems are N+1 queries, missing indexes, tracking queries for read paths and over-fetching — check those before micro-optimizing C#.

***

#### Testing

```csharp
using Xunit;

public sealed class PriceCalculatorTests
{
    [Theory]
    [InlineData(10, 1, 11.90)]
    [InlineData(0, 5, 0)]
    public void Total_IncludesVat(decimal unitPrice, int quantity, decimal expected)
    {
        Assert.Equal(expected, PriceCalculator.Total(unitPrice, quantity));
    }

    [Fact]
    public void Total_RejectsNegativeQuantity()
    {
        var ex = Assert.Throws<ArgumentOutOfRangeException>(() => PriceCalculator.Total(1m, -1));
        Assert.Equal("quantity", ex.ParamName);
    }

    [Fact]
    public async Task LoadAsync_HonoursCancellation()
    {
        using var cts = new CancellationTokenSource();
        cts.Cancel();
        await Assert.ThrowsAnyAsync<OperationCanceledException>(() => PriceCalculator.LoadAsync(cts.Token));
    }
}

public static class PriceCalculator
{
    public static decimal Total(decimal unitPrice, int quantity)
    {
        if (quantity < 0)
            throw new ArgumentOutOfRangeException(nameof(quantity), quantity, "Must not be negative.");
        return decimal.Round(unitPrice * quantity * 1.19m, 2);
    }

    public static Task LoadAsync(CancellationToken ct) => Task.Delay(1000, ct);
}
```

Integration tests for ASP.NET Core 7 use `WebApplicationFactory<Program>` from `Microsoft.AspNetCore.Mvc.Testing` 7.0.x. With top-level statements the generated `Program` class is internal; in .NET 7 add `public partial class Program { }` at the end of `Program.cs` (or `InternalsVisibleTo` the test assembly).[^35]

```csharp
using Microsoft.AspNetCore.Mvc.Testing;
using Xunit;

public sealed class ApiTests : IClassFixture<WebApplicationFactory<Program>>
{
    private readonly WebApplicationFactory<Program> _factory;
    public ApiTests(WebApplicationFactory<Program> factory) => _factory = factory;

    [Fact]
    public async Task UnknownTodo_ReturnsProblemDetails()
    {
        using var client = _factory.CreateClient();
        client.DefaultRequestHeaders.Accept.ParseAdd("application/json");   // .NET 7 problem details need it
        using var response = await client.GetAsync("/api/todos/999");
        Assert.Equal(System.Net.HttpStatusCode.NotFound, response.StatusCode);
    }
}

public partial class Program { }   // stands in for the web app's Program.cs here
```

Review rules:

- Tests are deterministic: no `DateTime.Now`, `Random` without seed, `Task.Delay`-based synchronization or shared static state. `TimeProvider`/`FakeTimeProvider` are .NET 8 — inject an own clock abstraction on .NET 7.
- Async tests return `Task`, never `async void` (xUnit cannot observe failures in `async void` tests).
- Test behaviour, not implementation details; mock boundaries (HTTP, clock, message bus), not the class under test. Moq 4.20.0 and 4.20.1 contained the SponsorLink build-time data collection (hashed e-mail addresses from git config); 4.20.2 removed it — flag pins to 4.20.0/4.20.1.[^40]
- EF Core: the in-memory provider does not enforce relational constraints or translate SQL; prefer SQLite in-memory or Testcontainers for repository tests.
- New tests accompany bug fixes; table-driven `[Theory]` tests for rule-heavy logic.

***

#### Tooling, Analyzers and Build Hygiene

```xml
<Project>
  <!-- Directory.Build.props -->
  <PropertyGroup>
    <TargetFramework>net7.0</TargetFramework>
    <Nullable>enable</Nullable>
    <ImplicitUsings>enable</ImplicitUsings>
    <TreatWarningsAsErrors>true</TreatWarningsAsErrors>
    <AnalysisLevel>latest-recommended</AnalysisLevel>
    <EnforceCodeStyleInBuild>true</EnforceCodeStyleInBuild>
    <Deterministic>true</Deterministic>
    <ManagePackageVersionsCentrally>true</ManagePackageVersionsCentrally>
  </PropertyGroup>
</Project>
```

- Default analysis (`AnalysisLevel` = `latest`, which is 7.0 on SDK 7, mode `Default`) reports very little: in a test project only `CA2200` fired. With `latest-recommended` the same code additionally reported `CA1304`/`CA1311` (culture-sensitive `ToLower()`), `CA1829` (`Count()` on a list), `CA1848` and `CA2254` (logging); `latest-all` added `CA2000` and `CA2007` (verified on SDK 7.0.410).[^26]
- Sync-over-async (`.Result`, `.Wait()`) is not flagged by the built-in analyzers; `Microsoft.VisualStudio.Threading.Analyzers` (`VSTHRD002`) does.
- Central package management (`Directory.Packages.props`, **New in .NET 7 SDK**) keeps versions consistent across projects.[^6]
- Vulnerable packages: `dotnet list package --vulnerable --include-transitive` works on SDK 7; automatic NuGet audit during restore (`NuGetAudit`) needs SDK 8+.[^39]
- `dotnet format` for style; `.editorconfig` for naming and severity configuration; `<WarningsAsErrors>nullable</WarningsAsErrors>` as a minimum.
- `global.json` pins the SDK; CI images that only contain SDK 8+/10 build `net7.0` with `NETSDK1138` warnings — which `TreatWarningsAsErrors` turns into errors. Do not "fix" that by suppressing NETSDK1138; retarget.

***

#### Observability

- `ILogger` with message templates (structured), `[LoggerMessage]` source generation, scopes for correlation.
- `System.Diagnostics.Metrics` (`Meter`, `Counter<T>`, `Histogram<T>`, `UpDownCounter<T>` — all available in .NET 7) and `ActivitySource`/`Activity` for traces; export with OpenTelemetry. `IMeterFactory` and the built-in ASP.NET Core metrics instruments are .NET 8 — on .NET 7 create static `Meter` instances.
- `Activity.CurrentChanged` and `Activity.EnumerateTagObjects()` are **New in .NET 7**.[^6]
- Health checks (`AddHealthChecks`, `MapHealthChecks`) separate liveness from readiness; do not expose detailed health output publicly.
- `MemoryCache.GetCurrentStatistics()` (**New in .NET 7**) requires `TrackStatistics = true` in `MemoryCacheOptions`.
- `dotnet-counters`, `dotnet-trace`, `dotnet-dump` work against .NET 7 processes; `EventPipe`-based tooling needs no code changes.

***

#### Removed / Changed in .NET 7 and C# 11 — Migration Notes

From .NET 6 / C# 10 to .NET 7 / C# 11 (what a reviewer of a freshly migrated code base should look for):[^5][^7][^9][^11]

- **Language:** `required`, `scoped` and `file` cannot be type names; lowercase type names warn (`CS8981`); `out` parameters cannot be returned by `ref`; stricter `ref struct` escape rules; `nint`/`IntPtr` arithmetic is checked inside `checked` blocks and `UIntPtr + int` is ambiguous on `net7.0`; `struct` constructors no longer must assign all fields (lost safety net).
- **Libraries:** `BinaryFormatter` APIs are compile errors; new `SYSLIB0036`–`SYSLIB0047` obsoletions; `DateTime.Add*` methods round with higher precision (results can differ in the last ticks); `Equals` for `NaN` changed; `BrotliStream` rejects undefined `CompressionLevel` values; reflection-invoke exceptions changed; the legacy `FileStream` strategy switch is gone; numeric format strings have a maximum precision of 999,999,999.
- **Extensions:** configuration binding into dictionaries extends values; `AddEnvironmentVariables(prefix)` now works with prefixes containing `__`; with `WebApplicationBuilder`, command-line arguments and `DOTNET_`-prefixed variables now override `ASPNETCORE_`-prefixed variables for host settings (environment name, content root) — `ASPNETCORE_ENVIRONMENT` loses against `DOTNET_ENVIRONMENT`.
- **ASP.NET Core 7:** API controllers infer `[FromServices]`; Kestrel's default HTTPS binding removed; the libuv transport removed; single authentication scheme becomes the default scheme; output caching API renamed between previews and GA; SignalR hub methods resolve parameters from DI.
- **EF Core 7:** SQL Server `Encrypt=True` default; triggers need `HasTrigger`; SQLite `RETURNING`; orphaned optional dependents not deleted; `HasCheckConstraint`/`HasComment` move to the table builder (`ToTable(t => t.HasCheckConstraint(...))`).
- **SDK:** `dotnet test -a`/`-r` switches changed meaning; automatic RuntimeIdentifier only for publish; `--output` invalid for solution-level commands.

From .NET 7 to .NET 10 (the recommended upgrade): retarget to `net10.0`, update all `Microsoft.*` packages to 10.0.x and EF Core to 10.0.x, rebuild with `TreatWarningsAsErrors` to surface new obsoletions, and review the .NET 8, 9 and 10 breaking-change lists (notably: .NET 8 container images run as non-root on port 8080, `BinaryFormatter` is removed entirely in .NET 9, EF Core 8+ changes `Contains` translation with JSON/OPENJSON on SQL Server).[^16][^17][^18] Only after retargeting may the code adopt C# 12–14 features and the .NET 8–10 APIs listed in "Do Not Use or Recommend".

***

#### Production / Review Checklist

##### Platform and version

- [ ] Target framework `net7.0` flagged as out of support (since May 14, 2024); upgrade to .NET 10 LTS recommended
- [ ] No `LangVersion` above 11, `latest` or `preview` in `net7.0` projects; `global.json` pins an SDK
- [ ] No C# 12+ syntax (primary constructors on classes, collection expressions, `field`, `extension`, `params` spans, `\e`)
- [ ] No .NET 8+ APIs (`TimeProvider`, `FrozenDictionary`, `ThrowIfNegative`, keyed services, `SearchValues`, `IExceptionHandler`, `CreateSlimBuilder`, `Lock`, `CountBy`, `LeftJoin`)
- [ ] NuGet packages pinned to versions that support `net7.0`; no suppressed `SuppressTfmSupportBuildWarnings`; `dotnet list package --vulnerable` clean

##### C# 11 features

- [ ] `required` members used for mandatory data; `[SetsRequiredMembers]` constructors really set every required member
- [ ] Structs relying on auto-default fields checked for forgotten assignments
- [ ] Raw string literals not used to build SQL/HTML/shell commands from input
- [ ] List patterns on `List<T>` do not use capturing slices; array slices not used in hot paths
- [ ] `u8` literals exposed via `static ReadOnlySpan<byte>` properties, not `.ToArray()` in loops
- [ ] Generic math code handles overflow (`checked`, `CreateChecked`); static-abstract interfaces used only as constraints
- [ ] `IntPtr`/`nint` arithmetic reviewed for `checked` behaviour change and `UIntPtr + int` ambiguity after retargeting

##### Correctness

- [ ] Nullable warnings enabled and treated as errors; `!` justified; public inputs validated at run time
- [ ] No `async void` (except event handlers), no `.Result`/`.Wait()`, no fire-and-forget tasks in request code
- [ ] `CancellationToken` accepted and forwarded; `[EnumeratorCancellation]` on async iterators
- [ ] No mutation of collections during enumeration; `ConcurrentDictionary` factories idempotent or wrapped in `Lazy<T>`
- [ ] No multiple enumeration of lazy `IEnumerable<T>`/`IQueryable<T>`
- [ ] Records with collection members not relied upon for value equality; no shared mutable state via `with`
- [ ] `throw;` instead of `throw ex;`; specific exception types; cancellation not logged as error
- [ ] Every `IDisposable`/`IAsyncDisposable` disposed; injected services never disposed manually

##### ASP.NET Core / EF Core

- [ ] Middleware order: exception handler → HTTPS/HSTS → CORS → authentication → authorization → rate limiter → output cache
- [ ] Rate limiter `RejectionStatusCode = 429`; partition keys bounded and not spoofable
- [ ] Output cache never caches per-user data without varying by user
- [ ] Problem details reach clients that omit `Accept` (or clients send `Accept: application/json`)
- [ ] Minimal-API `IFormFile` endpoints with cookie auth validate antiforgery manually
- [ ] No captive dependencies; `DbContext` scoped; background services create scopes
- [ ] Options validated with `ValidateOnStart()`
- [ ] `ExecuteUpdate`/`ExecuteDelete` callers aware that tracking, interceptors and `SaveChanges` logic are bypassed
- [ ] SQL Server connection strings without `TrustServerCertificate=True`/`Encrypt=False` in production; triggers configured with `HasTrigger`
- [ ] Raw SQL only via `FromSql`/`ExecuteSql`/`SqlQuery` interpolation or explicit parameters

##### Security

- [ ] No `BinaryFormatter`, no `TypeNameHandling`, no polymorphic deserialization of unregistered types
- [ ] Untrusted regex patterns/inputs use `NonBacktracking` or timeouts
- [ ] Archive and upload paths validated against the destination root
- [ ] Passwords hashed with PBKDF2-SHA256 (≥ 600k iterations) or Identity's hasher; tokens from `RandomNumberGenerator`; constant-time comparisons
- [ ] TLS validation never disabled; no TLS 1.0/1.1 pinning
- [ ] Secrets only from environment/secret stores; no secrets or personal data in logs or problem details
- [ ] JWT validation parameters fully enabled; CORS origins explicit

##### Performance and operations

- [ ] One cached `JsonSerializerOptions`; `IHttpClientFactory` or a shared `HttpClient` with `PooledConnectionLifetime`
- [ ] `[GeneratedRegex]`, `[LoggerMessage]`, STJ source generation used for hot paths
- [ ] `<TieredPGO>true</TieredPGO>` considered (off by default in .NET 7)
- [ ] `AnalysisLevel` `latest-recommended` (or stricter) and `TreatWarningsAsErrors` enabled
- [ ] Structured logs, metrics and traces in place; health checks separate liveness and readiness

***

[^1]: .NET 7.0 release notes and release metadata (7.0.0 on 2022-11-08 … 7.0.20 / SDK 7.0.410 on 2024-05-28). https://github.com/dotnet/core/blob/main/release-notes/7.0/README.md , https://dotnetcli.blob.core.windows.net/dotnet/release-metadata/7.0/releases.json
[^2]: .NET and .NET Core official support policy; release index with support phases and end-of-support dates (7.0 EOL 2024-05-14; 8.0 and 9.0 2026-11-10; 10.0 2028-11-14; 11.0 RC2 go-live). https://dotnet.microsoft.com/platform/support/policy/dotnet-core , https://dotnetcli.blob.core.windows.net/dotnet/release-metadata/releases-index.json
[^3]: .NET 7.0.20 – May 28, 2024 (CVE-2024-20672). https://github.com/dotnet/core/blob/main/release-notes/7.0/7.0.20/7.0.20.md
[^4]: The history of C# — C# version 11. https://learn.microsoft.com/dotnet/csharp/whats-new/csharp-version-history#c-version-11 ; csharplang Language Version History. https://github.com/dotnet/csharplang/blob/main/Language-Version-History.md
[^5]: Roslyn: Breaking changes in the compiler after .NET 6 (C# 11 / .NET 7). https://github.com/dotnet/roslyn/blob/main/docs/compilers/CSharp/Compiler%20Breaking%20Changes%20-%20DotNet%207.md
[^6]: What's new in .NET 7. https://learn.microsoft.com/dotnet/core/whats-new/dotnet-7
[^7]: Breaking changes in .NET 7. https://learn.microsoft.com/dotnet/core/compatibility/7.0
[^8]: What's new in ASP.NET Core in .NET 7. https://learn.microsoft.com/aspnet/core/release-notes/aspnetcore-7.0
[^9]: Breaking changes in ASP.NET Core 7. https://learn.microsoft.com/aspnet/core/breaking-changes/7/overview
[^10]: What's new in EF Core 7.0. https://learn.microsoft.com/ef/core/what-is-new/ef-core-7.0/whatsnew
[^11]: Breaking changes in EF Core 7.0. https://learn.microsoft.com/ef/core/what-is-new/ef-core-7.0/breaking-changes
[^12]: C# language versioning (defaults per target framework). https://learn.microsoft.com/dotnet/csharp/language-reference/configure-language-version
[^13]: What's new in C# 12. https://learn.microsoft.com/dotnet/csharp/whats-new/csharp-12
[^14]: What's new in C# 13. https://learn.microsoft.com/dotnet/csharp/whats-new/csharp-13
[^15]: What's new in C# 14. https://learn.microsoft.com/dotnet/csharp/whats-new/csharp-14
[^16]: What's new in .NET 8 and breaking changes in .NET 8. https://learn.microsoft.com/dotnet/core/whats-new/dotnet-8/overview , https://learn.microsoft.com/dotnet/core/compatibility/8.0
[^17]: What's new in .NET 9 and breaking changes in .NET 9. https://learn.microsoft.com/dotnet/core/whats-new/dotnet-9/overview , https://learn.microsoft.com/dotnet/core/compatibility/9.0
[^18]: What's new in .NET 10 and breaking changes in .NET 10. https://learn.microsoft.com/dotnet/core/whats-new/dotnet-10/overview , https://learn.microsoft.com/dotnet/core/compatibility/10.0
[^19]: What's new in System.Text.Json in .NET 7. https://devblogs.microsoft.com/dotnet/system-text-json-in-dotnet-7/
[^20]: Regular expression improvements in .NET 7; .NET regular expression source generators. https://devblogs.microsoft.com/dotnet/regular-expression-improvements-in-dotnet-7/ , https://learn.microsoft.com/dotnet/standard/base-types/regular-expression-source-generators
[^21]: Rate limiting middleware in ASP.NET Core. https://learn.microsoft.com/aspnet/core/performance/rate-limit?view=aspnetcore-7.0
[^22]: Output caching middleware in ASP.NET Core (7.0 version: default policy, limits, ordering). https://learn.microsoft.com/aspnet/core/performance/caching/output?view=aspnetcore-7.0
[^23]: Generic math. https://learn.microsoft.com/dotnet/standard/generics/math
[^24]: C# compiler warning waves (CS8981, wave 7). https://learn.microsoft.com/dotnet/csharp/language-reference/compiler-messages/warning-waves
[^25]: Obsolete features in .NET 5+ (SYSLIB0001–SYSLIB0047). https://learn.microsoft.com/dotnet/fundamentals/syslib-diagnostics/obsoletions-overview
[^26]: MSBuild properties for code analysis (AnalysisLevel, AnalysisMode). https://learn.microsoft.com/dotnet/core/project-sdk/msbuild-props#code-analysis-properties
[^27]: Guidelines for using HttpClient. https://learn.microsoft.com/dotnet/fundamentals/networking/http/httpclient-guidelines
[^28]: Performance improvements in .NET 7; Performance improvements in .NET 8 (Dynamic PGO on by default). https://devblogs.microsoft.com/dotnet/performance_improvements_in_net_7/ , https://devblogs.microsoft.com/dotnet/performance-improvements-in-net-8/
[^29]: Native AOT deployment overview. https://learn.microsoft.com/dotnet/core/deploying/native-aot/
[^30]: How to serialize properties of derived classes with System.Text.Json (polymorphism). https://learn.microsoft.com/dotnet/standard/serialization/system-text-json/polymorphism
[^31]: Customize a JSON contract. https://learn.microsoft.com/dotnet/standard/serialization/system-text-json/custom-contracts
[^33]: Low-level struct improvements (ref fields, scoped, UnscopedRef) proposal. https://github.com/dotnet/csharplang/blob/main/proposals/csharp-11.0/low-level-struct-improvements.md
[^34]: Required members proposal. https://github.com/dotnet/csharplang/blob/main/proposals/csharp-11.0/required-members.md
[^35]: Integration tests in ASP.NET Core (WebApplicationFactory, `public partial class Program`). https://learn.microsoft.com/aspnet/core/test/integration-tests?view=aspnetcore-7.0
[^37]: SQL Queries — EF Core (FromSql in EF 7, SqlQuery unmapped types in EF 8). https://learn.microsoft.com/ef/core/querying/sql-queries
[^38]: Breaking change: BinaryFormatter serialization APIs produce compiler errors (.NET 7). https://learn.microsoft.com/dotnet/core/compatibility/serialization/7.0/binaryformatter-apis-produce-errors
[^39]: Auditing package dependencies for security vulnerabilities (NuGetAudit, .NET 8 SDK). https://learn.microsoft.com/nuget/concepts/auditing-packages
[^40]: GitLab advisory database — Moq 4.20.0/4.20.1 SponsorLink data collection, removed in 4.20.2. https://advisories.gitlab.com/pkg/nuget/moq/
