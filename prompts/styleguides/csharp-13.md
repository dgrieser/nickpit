### C# 13 on .NET 9: The Complete Production Developer Guide

> **Version**: C# 13 ships with the .NET 9 SDK. .NET 9.0.0 was released on **November 12, 2024**; the latest patch is **.NET 9.0.21 / SDK 9.0.319 (October 13, 2026)** — 9.0.122 is the matching SDK in the older 9.0.1xx feature band.
> **Support status (October 2026)**: .NET 9 is a **Standard Term Support (STS)** release in its final weeks. STS support was extended from 18 to 24 months, so .NET 9 reaches **end of support on November 10, 2026** — the same day as .NET 8. A final update on that date ships only if a critical issue is known; after that there are no more security fixes. The newest stable release is **.NET 10 (LTS, C# 14)**, released November 11, 2025, latest patch 10.0.13 (October 13, 2026), supported until November 2028. **.NET 11 / C# 15 is at Release Candidate 2** (11.0.0-rc.2, October 13, 2026) and is not a stable release. Recommend migrating C# 13/.NET 9 code to .NET 10 LTS; do not recommend .NET 11 for production yet.
> **Scope**: The C# 13 language, the .NET 9 base class library and runtime, ASP.NET Core 9, EF Core 9, and the surrounding tooling — idioms, pitfalls, security, performance, testing and review rules. Sections that cover C# 13 or .NET 9 specifics are marked **New in C# 13** / **New in .NET 9**.
> **Note**: This document is self-contained. Everything from C# 8–12 and .NET 5–8 that is still correct is carried forward; anything from C# 14 / .NET 10 or later is listed under "Do not use or recommend".

---

#### Overview

C# 13 is the language version that the .NET 9 SDK uses by default for projects that target `net9.0`. Its headline changes are **`params` collections** (`params ReadOnlySpan<T>`, `params IEnumerable<T>`, `params List<T>` …), the **`System.Threading.Lock` type and its `lock` statement integration**, the **`\e` escape sequence**, the **implicit "from the end" index (`^1`) in object initializers**, **`ref` locals and `unsafe` blocks in `async` methods and iterators**, **`ref struct` types implementing interfaces**, the **`allows ref struct` anti-constraint** on generic type parameters, **partial properties and indexers**, **`[OverloadResolutionPriority]`**, and **method-group natural type improvements**. The `field` contextual keyword shipped only as a *preview* in C# 13; it became a stable feature in C# 14.

.NET 9 itself adds, among others: `CountBy`/`AggregateBy`/`Index` in LINQ, `Task.WhenEach`, `Guid.CreateVersion7`, `Base64Url`, `OrderedDictionary<TKey,TValue>`, `ReadOnlySet<T>`, span-based alternate lookups for `Dictionary`/`HashSet`, `params ReadOnlySpan<T>` overloads across the BCL, `X509CertificateLoader`, `CryptographicOperations.HashData(HashAlgorithmName, …)`, KMAC, `JsonSerializerOptions.Web`, `RespectNullableAnnotations`, `JsonSchemaExporter`, the prioritized unbounded channel, `Gauge<T>`, `Activity.AddLink`, and the removal of `BinaryFormatter` (it now always throws). ASP.NET Core 9 adds built-in OpenAPI document generation, `MapStaticAssets`, `HybridCache` (GA in `Microsoft.Extensions.Caching.Hybrid` 9.3.0, March 2025), `TypedResults.InternalServerError`, and the exception-handler `StatusCodeSelector`. EF Core 9 adds `UseSeeding`/`UseAsyncSeeding`, a migration lock, `ToHashSetAsync`, and turns forgotten migrations into a hard error.

Global truths for a C# 13 / .NET 9 code base:

- **The language version follows the target framework.** `net9.0` ⇒ C# 13 by default. `LangVersion` can lower it; raising it above 13 on `net9.0` is unsupported (and requires the .NET 10 SDK).
- **Nullable reference types are a static analysis, not a runtime guarantee.** Annotations do not stop `null` from arriving through deserialization, reflection, or `!`.
- **`async`/`await` is the concurrency model.** Blocking on tasks (`.Result`, `.Wait()`) and `async void` are bugs outside of narrow cases.
- **The GC is generational and, for Server GC, adaptive.** .NET 9 enables DATAS (dynamic adaptation to application sizes) by default with Server GC, so heap sizes now track the live data size more closely.
- **`string` is immutable UTF-16; spans are the zero-copy view.** `Span<T>`/`ReadOnlySpan<T>` are `ref struct`s; `Memory<T>` is their heap-storable counterpart.
- **Integer arithmetic is unchecked by default.** Overflow wraps silently unless the code uses `checked` or `<CheckForOverflowUnderflow>`.
- **Equality of `record`s is shallow.** Members that are arrays or collections compare by reference.

***

#### Version-Aware Review

Before claiming that an API or syntax is unavailable, determine what the project actually compiles against:

1. **`TargetFramework(s)`** in the `.csproj` (or `Directory.Build.props`). `net9.0` means the .NET 9 BCL and C# 13 by default. Multi-targeted projects (`net8.0;net9.0`) must compile on *every* target: a .NET 9-only API needs `#if NET9_0_OR_GREATER` or a fallback.
2. **`LangVersion`**. Absent ⇒ the default for the target framework (13 for `net9.0`). `latest`/`latestMajor` mean "the newest the *installed SDK* supports" — with the .NET 10 SDK that is C# 14 even on `net9.0`, which is an unsupported combination. `preview` is never acceptable in production code.
3. **`global.json`** pins the SDK. SDK 9.0.3xx contains Roslyn 4.14, which accepts `LangVersion` values up to `13.0` plus `preview`; `<LangVersion>14</LangVersion>` fails with `CS1617: Invalid option '14' for /langversion`.
4. **Package versions** — ASP.NET Core, EF Core and `Microsoft.Extensions.*` APIs depend on the package/shared-framework version, not on C#. EF Core 9 targets `net8.0`, so EF Core 9 code may run on .NET 8; EF Core 10 requires `net10.0`.

Verified behaviour of the SDKs around the boundary:

| Setup | Result |
|---|---|
| SDK 9.0.319, `net9.0`, no `LangVersion` | C# 13; `x?.P = v` → `CS8652 … 'null conditional assignment' is currently in Preview` |
| SDK 9.0.319, `LangVersion=12` | C# 13 features → `CS9202: Feature '…' is not available in C# 12.0. Please use language version 13.0 or greater.` (exceptions: partial properties → `CS8703`, the method-group example below → `CS8917`) |
| SDK 9.0.319, `LangVersion=preview` | early previews of C# 14 compile: `field`, null-conditional assignment, `nameof(List<>)`, simple-lambda parameter modifiers, first-class span conversions, `extension` blocks, partial constructors and partial events (user-defined `+=` does not) — unsupported, flag it |
| SDK 10.0.402, `net9.0`, no `LangVersion` | still C# 13: C# 14 syntax → `CS9260: Feature '…' is not available in C# 13.0` |
| SDK 10.0.402, `net9.0`, `LangVersion=14` | C# 14 syntax compiles against the .NET 9 BCL — unsupported; flag it |

Rules for the reviewer:

- Say "not available in C# 13 / .NET 9" only for items in the lists below or after checking the pinned version. Many APIs that look new (`TimeProvider`, `FrozenDictionary`, keyed DI, `[Experimental]`, collection expressions, primary constructors) are already available from .NET 8 / C# 12.
- Flag `LangVersion` `preview` (anywhere) and `latest`/`14` on `net9.0`.
- Flag code that relies on .NET 9-only APIs in a library that also targets `net8.0` or `netstandard2.0` without conditional compilation.
- Because .NET 9 leaves support on November 10, 2026, a review of a long-lived service should note the upgrade to `net10.0` as a follow-up, not as a blocker for unrelated changes.

***

#### What's New in C# 13 and .NET 9

##### Language (C# 13)

| Feature | Summary | Section |
|---|---|---|
| `params` collections | `params` on `Span<T>`, `ReadOnlySpan<T>`, `IEnumerable<T>`, `IReadOnlyCollection<T>`, `IReadOnlyList<T>`, `ICollection<T>`, `IList<T>`, and collection types with `Add` | [params collections](#new-in-c-13-params-collections) |
| `System.Threading.Lock` | `lock (lockObj)` on a `Lock` uses `Lock.EnterScope()` instead of `Monitor` | [Lock](#new-in-c-13--net-9-the-lock-type) |
| `\e` | Escape character U+001B in char/string literals | [Small features](#new-in-c-13-small-syntax-features) |
| Implicit index in initializers | `new T { Buffer = { [^1] = 0 } }` | [Small features](#new-in-c-13-small-syntax-features) |
| `ref`/`unsafe` in async & iterators | `ref` locals, `ref struct` locals and `unsafe` blocks allowed, but not across `await`/`yield` | [ref in async](#new-in-c-13-ref-locals-ref-struct-locals-and-unsafe-in-async-methods-and-iterators) |
| `ref struct` interfaces | `ref struct` types may implement interfaces (no boxing conversion) | [ref struct generics](#new-in-c-13-ref-struct-interfaces-and-allows-ref-struct) |
| `allows ref struct` | Anti-constraint so a type parameter accepts `Span<T>` etc. | [ref struct generics](#new-in-c-13-ref-struct-interfaces-and-allows-ref-struct) |
| Partial properties/indexers | One declaring and one implementing declaration | [Partial properties](#new-in-c-13-partial-properties-and-indexers) |
| `[OverloadResolutionPriority]` | Library authors rank overloads | [Overload resolution priority](#new-in-c-13-overloadresolutionpriority) |
| Method group natural type | Candidates are pruned per scope; more `var d = obj.M;` cases infer a delegate type | [Method groups](#new-in-c-13-method-group-natural-type) |
| `field` keyword | **Preview only** in C# 13 — do not use | [field](#the-field-keyword-is-preview-in-c-13--do-not-use) |

##### Libraries and runtime (.NET 9)

- **LINQ**: `CountBy`, `AggregateBy`, `Index`.
- **Collections**: `OrderedDictionary<TKey,TValue>`, `ReadOnlySet<T>`, `PriorityQueue.Remove`, `GetAlternateLookup<ReadOnlySpan<char>>()` on `Dictionary`/`HashSet`/`ConcurrentDictionary`.
- **Threading**: `Task.WhenEach`, `Channel.CreateUnboundedPrioritized`, `Interlocked.Exchange`/`CompareExchange` for `byte`/`sbyte`/`short`/`ushort` and generic overloads that accept enums and primitives, `System.Threading.Lock`.
- **Primitives**: `Guid.CreateVersion7()`, `Base64Url`, `Convert.ToHexStringLower`, `TimeSpan.FromSeconds(long, long …)` integer overloads, `BigMul` on `int`/`long`/`uint`/`ulong`.
- **Spans**: `ReadOnlySpan<char>.Split(…)` enumeration, `SearchValues<string>`, `Regex.EnumerateSplits`, `File.WriteAllText(string, ReadOnlySpan<char>)`, 60+ `params ReadOnlySpan<T>` overloads (`string.Join`, `Path.Combine`, `Task.WhenAll`, …).
- **JSON**: `JsonSerializerOptions.Web`, `RespectNullableAnnotations`, `RespectRequiredConstructorParameters`, `IndentCharacter`/`IndentSize`, `JsonSchemaExporter`, `JsonStringEnumMemberNameAttribute`, ordered `JsonObject` APIs, multi-document `Utf8JsonReader`.
- **Crypto**: `X509CertificateLoader` (the old `X509Certificate2` loading constructors are obsolete, `SYSLIB0057`), `CryptographicOperations.HashData(HashAlgorithmName, …)`, `Kmac128/256`, `SafeEvpPKeyHandle.OpenKeyFromProvider`.
- **Diagnostics**: `Debug.Assert` messages include the condition text, `Gauge<T>`, `Activity.AddLink`, wildcard meter listening.
- **Networking**: `SocketsHttpHandler` is the default primary handler for `IHttpClientFactory`; header values are no longer logged by default; `System.Net.ServerSentEvents` (NuGet); WebSocket `KeepAliveTimeout`.
- **Serialization**: `BinaryFormatter` always throws `PlatformNotSupportedException`.
- **Runtime**: Server GC uses DATAS by default; faster managed exception handling (2–4× in micro-benchmarks); stack allocation of non-escaping boxes; loop optimizations; floating-point → integer conversions saturate on x86/x64; CET shadow stacks on Windows.
- **Trimming/AOT**: `[FeatureSwitchDefinition]` and `[FeatureGuard]`; `UnsafeAccessor` supports generics.
- **Reflection/IO/numerics**: `PersistedAssemblyBuilder` (save `Reflection.Emit` assemblies to disk), `System.Reflection.Metadata.TypeName` parsing, `ZLibCompressionOptions`/`BrotliCompressionOptions` (zlib-ng replaces zlib), `TarEntry.DataOffset`, `BigInteger` length limit, `Tensor<T>` in the `System.Numerics.Tensors` 9.x package (marked `[Experimental]`, `SYSLIB5001`).
- **Behaviour changes worth knowing**: `BinaryReader.ReadString()` returns `"�"` for malformed UTF-8 instead of throwing; passing a `StringValues` (e.g. a header value) to `string.Join`/`string.Concat`/`Path.Combine`/`Path.Join`/`StringBuilder.AppendJoin` is now `CS0121` (ambiguous between the `params string[]` and `params ReadOnlySpan<string>` overloads; cast to `string[]`); `TimeSpan.FromMilliseconds(int/long)` in expression trees is `CS0854` (see Numbers).

##### ASP.NET Core 9, EF Core 9, SDK

- ASP.NET Core 9: `Microsoft.AspNetCore.OpenApi` `AddOpenApi()`/`MapOpenApi()` (OpenAPI 3.0 documents), `MapStaticAssets()`, `HybridCache`, `TypedResults.InternalServerError`, `ProducesProblem` on route groups, `ExceptionHandlerOptions.StatusCodeSelector`, keyed services in middleware, `[DisableHttpMetrics]`, analyzer `ASP0026`, OIDC Pushed Authorization Requests, deletable Data Protection keys (`IDeletableKeyManager`; deleting a key makes its payloads unreadable), SignalR polymorphic hub parameters and Native AOT support. Swashbuckle is no longer in the Web API template.
- EF Core 9: pending model changes and user transactions around `Migrate()` now throw; `UseSeeding`/`UseAsyncSeeding`; migration locking; `ToHashSetAsync`; Native AOT/precompiled queries (experimental).
- SDK 9: Terminal Logger on by default, `dotnet test` runs target frameworks in parallel, `dotnet tool install --allow-roll-forward`, BuildChecks (`/check`), workload sets, new CA rules (CA1514, CA1515, CA1871, CA1872, CA2022, CA2262–CA2265), NuGet audit sources.

***

#### Do Not Use or Recommend (Newer Than C# 13 / .NET 9)

The project compiles with C# 13 against the .NET 9 BCL. None of the following exists there; never suggest them. The language features, the BCL and ASP.NET Core APIs, and the EF Core APIs named below were compiled on SDK 9.0.319 / `net9.0` (EF Core 9.0.21) and fail there; the C# 14 features and BCL APIs were also compiled successfully on SDK 10.0.402 / `net10.0`.

**C# 14 (.NET 10) language features**

- **Extension members / extension blocks**: `extension<T>(IEnumerable<T> source) { public bool IsEmpty => …; }`, extension properties, static extension members, extension operators. Use classic `static` extension *methods*.
- **`field` keyword as a stable feature** (`public string Name { get; set => field = value.Trim(); }`). In C# 13 it is preview-only (`CS8652`).
- **Null-conditional assignment**: `customer?.Order = x;`, `a?[i] = x;`, `x?.Count += 1;`. Use `if (customer is not null) customer.Order = x;`.
- **`nameof` with unbound generics**: `nameof(List<>)`. Use `nameof(List<int>)`.
- **First-class span conversions**: implicit `T[]` → `ReadOnlySpan<T>` as an extension-method receiver (`array.MySpanExtension()`), and the related inference improvements. In C# 13 call `array.AsSpan().MySpanExtension()`.
- **Modifiers on simple lambda parameters**: `(text, out result) => …`. Use explicitly typed parameters: `(string text, out int result) => …`.
- **Partial constructors and partial events**.
- **User-defined compound assignment operators** (`public void operator +=(int x)`), and user-defined instance `++`/`--`.
- File-based app directives (`#:package`, `#:sdk`, `#:property`) and `dotnet run app.cs`.

**.NET 10 APIs (not in the .NET 9 BCL)**

- LINQ: `Enumerable.LeftJoin`, `Enumerable.RightJoin`, `Enumerable.Shuffle` (and the `Queryable` counterparts). `Random.Shuffle` (since .NET 8) exists.
- `System.Linq.AsyncEnumerable` in the BCL (LINQ over `IAsyncEnumerable<T>`). On .NET 9 use the `System.Linq.Async` NuGet package or hand-written `await foreach`.
- JSON: `JsonSerializerOptions.Strict`, `AllowDuplicateProperties`, `PipeReader` deserialization overloads.
- `OrderedDictionary<TKey,TValue>.TryAdd(key, value, out int index)` and `TryGetValue(key, out value, out int index)` overloads (the type itself is .NET 9).
- `CompareOptions.NumericOrdering`.
- `Rfc2898DeriveBytes` constructors being obsolete (`SYSLIB0060`) is .NET 10; on .NET 9 the static `Rfc2898DeriveBytes.Pbkdf2(…)` is preferred but the explicit-algorithm constructors produce no warning.
- `Mutex` constructors taking `NamedWaitHandleOptions` (.NET 10).
- ASP.NET Core 10: `builder.Services.AddValidation()` (minimal API validation), `TypedResults.ServerSentEvents`, OpenAPI 3.1 documents, `ForwardedHeadersOptions.KnownIPNetworks` (.NET 9 has `KnownNetworks` with `Microsoft.AspNetCore.HttpOverrides.IPNetwork`), passkey support in Identity, the automatic `public partial class Program` source generator for `WebApplicationFactory`.
- EF Core 10: `LeftJoin`/`RightJoin` translation, named query filters, `ExecuteUpdateAsync` with a non-expression lambda (regular statement body), vector search, JSON mapping of complex types.
- SDK 10: `dotnet tool exec`/`dnx`, `dotnet run app.cs` file-based apps, `NuGetAuditMode=all` as the default (on `net9.0` the default is `direct`). The noun-first `dotnet package add`/`dotnet package list` commands already exist in SDK 9.0.3xx.

**C# 15 / .NET 11** is at RC2 — none of its features are usable on .NET 9 either.

##### Deprecated or removed as of .NET 9 — flag when seen

- `BinaryFormatter` — throws `PlatformNotSupportedException` in every project type; the APIs only remain so that code compiles (with `SYSLIB0011` as an error). In code ported from .NET Framework, the related `SoapFormatter`/`NetDataContractSerializer`/`LosFormatter` patterns have no .NET 9 equivalent and must not be re-created with custom type-name resolution.
- `new X509Certificate2(byte[]/string …)` and `X509Certificate2Collection.Import(…)` loading overloads — `SYSLIB0057`; use `X509CertificateLoader.LoadCertificate*`/`LoadPkcs12*`.
- `Thread.VolatileRead/VolatileWrite` (`SYSLIB0054`; use `Volatile.Read/Write`), `ServicePointManager` (`SYSLIB0014`, has no effect on `HttpClient`), `AuthenticationManager` (`SYSLIB0009`), `Assembly.LoadFrom(…, AssemblyHashAlgorithm)` (`SYSLIB0056`).
- Older still-flagged obsoletions: `WebRequest`/`HttpWebRequest`/`WebClient` (`SYSLIB0014`), `Rfc2898DeriveBytes` constructors without a hash algorithm (`SYSLIB0041`), `RNGCryptoServiceProvider` and `*CryptoServiceProvider`/`*Managed` crypto types (`SYSLIB0021`/`SYSLIB0023`), `Thread.Abort`, Code Access Security.
- Targeting `net7.0` (or any older `netX.0`/`netcoreappX.Y`) produces the out-of-support warning `NETSDK1138` — but only in executable projects: the SDK sets `CheckEolTargetFramework=true` only when `OutputType` is not `Library`, so an EOL library target builds silently (verified on SDK 9.0.319). `netstandard1.x` targets produce `NETSDK1215` in every project type.
- `String.Trim/TrimStart/TrimEnd(ReadOnlySpan<char>)` overloads from the .NET 9 previews were removed from the reference assemblies at GA (they remain in `System.Private.CoreLib` for binary compatibility, but source cannot call them: `CS1503`). Do not "fix" code to call them; use `MemoryExtensions.Trim(span, trimChars)` on spans.

***

#### New in C# 13: `params` Collections

`params` now works on any type that a collection expression can create: `Span<T>`, `ReadOnlySpan<T>`, `IEnumerable<T>`, `IReadOnlyCollection<T>`, `IReadOnlyList<T>`, `ICollection<T>`, `IList<T>`, and concrete collection types with an accessible `Add` (for example `List<T>`).

```csharp
static int Sum(params ReadOnlySpan<int> values)   // no array allocation for Sum(1, 2, 3)
{
    int total = 0;
    foreach (int v in values) total += v;
    return total;
}

static int Count<T>(params IEnumerable<T> items) => items.Count();
static string Describe(params List<string> items) => $"{items.GetType().Name}:{items.Count}";

Console.WriteLine(Sum(1, 2, 3));        // 6
Console.WriteLine(Sum());               // 0 — empty span, never null
Console.WriteLine(Count("a", "b"));     // 2
Console.WriteLine(Describe("x", "y"));  // List`1:2
```

With `LangVersion=12` every such declaration fails with `CS9202: Feature 'params collections' is not available in C# 12.0`.

##### Overload resolution prefers the span overload

When both a `params T[]` and a `params ReadOnlySpan<T>` overload are applicable to an expanded-form call, C# 13 picks the span overload. .NET 9 adds `params ReadOnlySpan<T>` overloads to `string.Join`, `string.Concat`, `string.Format`, `Path.Combine`, `Path.Join`, `Task.WhenAll`, `Task.WhenAny`, `CancellationTokenSource.CreateLinkedTokenSource`, `Console.WriteLine(string, …)`, `StringBuilder.AppendJoin`, `ImmutableArray.Create`, `Counter<T>.Add` and others, so recompiling existing calls removes the hidden array allocation.

```csharp
Console.WriteLine(Overloads.Pick(1, 2));             // span
Console.WriteLine(Overloads.Pick([1, 2]));           // span (collection expression)
Console.WriteLine(Overloads.Pick(new[] { 1, 2 }));   // array (explicit array argument)

static class Overloads
{
    public static string Pick(params int[] values) => "array";
    public static string Pick(params ReadOnlySpan<int> values) => "span";
}
```

##### Breaking change: expression trees cannot bind to span overloads

Expression trees cannot contain `ref struct` values. A call that compiled under C# 12 inside an `Expression<…>` — including LINQ-to-Entities queries — now binds to the new span overload and fails to compile:

```csharp
// Does not compile with C# 13 on .NET 9 (compiles with LangVersion 12):
using System.Linq.Expressions;

Expression<Func<string, string, string>> join = (x, y) => string.Join("", x, y);
// error CS8640: Expression tree cannot contain value of ref struct or restricted type 'ReadOnlySpan'.
// error CS9226: An expression tree may not contain an expanded form of non-array params collection parameter.
```

```csharp
using System.Linq.Expressions;

// GOOD: pass an explicit array so the params-array overload is chosen
Expression<Func<string, string, string>> join = (x, y) => string.Join("", new[] { x, y });
Console.WriteLine(join.Compile()("a", "b")); // ab
```

##### Review notes for `params` collections

- Prefer `params ReadOnlySpan<T>` for new hot-path APIs that only read the arguments; keep a `params T[]` overload only for binary compatibility.
- A `params ReadOnlySpan<T>` parameter cannot be captured by a lambda or stored in a field, and `async` methods cannot declare span parameters at all (`CS4012: Parameters of type 'ReadOnlySpan<int>' cannot be declared in async methods or async lambda expressions.`). For `async` APIs take `params IEnumerable<T>`/`IReadOnlyList<T>`.
- With `params IEnumerable<T>`/`IReadOnlyList<T>` the compiler synthesizes the storage; do not cast it back to an array or `List<T>` and mutate it.
- A library that *adds* a span overload changes which overload existing callers bind to on recompilation — check expression-tree callers (EF Core, Moq `Setup(...)`, AutoMapper projections).
- Types with implicit conversions to both `T` and `T[]` become ambiguous: `string.Join(",", stringValues)` with an ASP.NET Core `StringValues` is `CS0121` on .NET 9 (verified); write `string.Join(",", (string?[])stringValues)` or `stringValues.ToString()`.

***

#### New in C# 13 / .NET 9: the `Lock` Type

.NET 9 adds `System.Threading.Lock`. When the operand of a `lock` statement has the static type `Lock`, C# 13 emits `using (x.EnterScope()) { … }` instead of `Monitor.Enter/Exit`.

```csharp
var counter = new Counter();
Parallel.For(0, 100_000, _ => counter.Increment());
Console.WriteLine(counter.Value);                         // 100000

var gate = new Lock();
lock (gate) { lock (gate) { Console.WriteLine(gate.IsHeldByCurrentThread); } } // True — re-entrant
using (gate.EnterScope()) { Console.WriteLine("explicit scope"); }
if (gate.TryEnter()) { try { Console.WriteLine("TryEnter"); } finally { gate.Exit(); } }

sealed class Counter
{
    private readonly Lock _gate = new();   // GOOD: dedicated, private, readonly Lock
    private int _value;

    public void Increment() { lock (_gate) { _value++; } }
    public int Value { get { lock (_gate) return _value; } }
}
```

Rules and pitfalls (all verified on .NET 9.0.21):

- **The field must be typed `Lock`.** Converting it to `object` (or to a generic `T`) silently falls back to `Monitor` and loses the new semantics. The compiler warns: `CS9216: A value of type 'System.Threading.Lock' converted to a different type will use likely unintended monitor-based locking in 'lock' statement.` Never store a `Lock` in an `object` field, never `lock (this)` or `lock (typeof(T))`.
- **`lock (Lock)` needs C# 13.** With `LangVersion=12` the compiler reports `CS9202: Feature 'Lock object' is not available in C# 12.0` rather than silently using `Monitor`.
- **`Lock` is re-entrant** for the owning thread, like `Monitor`. `Lock` and `Monitor` are independent: while one thread holds `lock (gate)`, `Monitor.TryEnter(gate)` on another thread still succeeds (verified). Mixing them on one instance breaks mutual exclusion; the compiler reports `CS9216` for the `Monitor` calls too.
- **No `await` inside the lock.** `lock (gate) { await …; }` is `CS1996`; `using (gate.EnterScope()) { await …; }` is `CS4007: Instance of type 'System.Threading.Lock.Scope' cannot be preserved across 'await' or 'yield' boundary.` For async mutual exclusion use `SemaphoreSlim(1, 1)` with `WaitAsync` and `try/finally`.
- Migrating `private readonly object _sync = new();` to `private readonly Lock _sync = new();` is a safe, recommended change in .NET 9 code — unless the object is also used with `Monitor.Wait/Pulse` (`Lock` has no condition variables) or is exposed outside the class.

```csharp
// GOOD: async-compatible mutual exclusion
using var mutex = new SemaphoreSlim(1, 1);
await mutex.WaitAsync();
try
{
    await Task.Delay(1);   // awaiting while holding the semaphore is fine
}
finally
{
    mutex.Release();
}
Console.WriteLine("released");
```

***

#### New in C# 13: Small Syntax Features

##### `\e` escape

`'\e'` is U+001B (ESC). Prefer it over `\x1b`: the `\x` escape takes up to **four** hex digits, so `"\x1bBold"` is `U+01BB` followed by `"old"`.

```csharp
string red = "\e[31mred\e[0m";
Console.WriteLine($"{(int)red[0]:X2} {red.Length}");     // 1B 12

string bad = "\x1bBold";                                  // BAD: greedy \x escape
Console.WriteLine($"U+{(int)bad[0]:X4} {bad.Length}");   // U+01BB 4
string good = "\u001bBold";                               // OK in any version; "\eBold" in C# 13
Console.WriteLine(good.Length);                           // 5
```

##### Implicit "from the end" index in object initializers

```csharp
var countdown = new Countdown { Buffer = { [^1] = 0, [^2] = 1, [^3] = 2 } };
Console.WriteLine(string.Join(",", countdown.Buffer));   // 0,0,0,0,0,0,0,2,1,0

var list = new Holder { Items = { [^1] = 99 } };          // also works for List<T>
Console.WriteLine(string.Join(",", list.Items));         // 1,2,99

class Countdown { public int[] Buffer { get; } = new int[10]; }
class Holder { public List<int> Items { get; } = [1, 2, 3]; }
```

The initializer *assigns* into an existing collection; it does not add elements. On an empty `List<T>` the same code throws `ArgumentOutOfRangeException` at runtime. `LangVersion=12` reports `CS9202: Feature 'implicit indexer initializer' …`.

***

#### New in C# 13: `ref` Locals, `ref struct` Locals and `unsafe` in Async Methods and Iterators

`async` methods and iterators may now declare `ref` locals and locals of `ref struct` types (`Span<T>`, `ReadOnlySpan<T>`, `Utf8JsonReader`, …), and iterators may contain `unsafe` blocks. The compiler still forbids keeping such a value alive across an `await` or `yield return`.

```csharp
Console.WriteLine(await CountFieldsAsync("a,b,,c"));       // 4
foreach (int n in Lengths(["ab", "cde"])) Console.Write($"{n} ");   // 2 3
Console.WriteLine();

static async Task<int> CountFieldsAsync(string csv)
{
    await Task.Yield();
    ReadOnlySpan<char> span = csv.AsSpan();   // allowed in C# 13 (CS9202 in C# 12)
    return span.Count(',') + 1;               // span is dead before the next await
}

static IEnumerable<int> Lengths(string[] items)
{
    foreach (string s in items)
    {
        ReadOnlySpan<char> span = s;          // allowed in C# 13
        int length = span.Length;
        yield return length;                  // span is not used after the yield
    }
}
```

```csharp
// Does not compile — span used across an await:
static async Task<int> BadAsync(string csv)
{
    ReadOnlySpan<char> span = csv.AsSpan();
    await Task.Yield();
    return span.Length;
    // error CS4007: Instance of type 'System.ReadOnlySpan<char>' cannot be preserved across 'await' or 'yield' boundary.
}
```

Notes:

- The same rule now lets top-level programs that contain `await` use span locals (the generated `Main` is async); with C# 12 that is `CS9202`.
- In iterators, `yield return`/`yield break` must stay in a safe context: an `unsafe { … }` block may compute a value, the `yield` goes outside it.
- **Compiler breaking change (Roslyn 17.11+, C# 13):** iterators now introduce a safe context, so a local function inside an iterator of an `unsafe class` no longer inherits `unsafe`. Code that declared pointers there compiles with C# 12 but fails with `CS0214` in C# 13 — add `unsafe` to the local function.

***

#### New in C# 13: `ref struct` Interfaces and `allows ref struct`

A `ref struct` may implement interfaces, and a generic type parameter can opt in to `ref struct` type arguments with the `allows ref struct` anti-constraint. The compiler then enforces ref-safety rules on all values of that type parameter.

```csharp
var reader = new SpanReader("hello".AsSpan());
Console.WriteLine(Describe(reader));          // SpanReader(5)
Console.WriteLine(Describe(new Num(42)));     // Num(42)

static string Describe<T>(T value) where T : IDescribable, allows ref struct
    => value.Describe();                      // constrained call, no boxing

interface IDescribable { string Describe(); }

ref struct SpanReader(ReadOnlySpan<char> data) : IDescribable
{
    private readonly ReadOnlySpan<char> _data = data;
    public string Describe() => $"SpanReader({_data.Length})";
}

readonly struct Num(int value) : IDescribable
{
    public string Describe() => $"Num({value})";
}
```

Restrictions to check in review:

- A `ref struct` can **never** be converted to the interface type: `IDescribable d = reader;` is `CS0029`. Interface methods are reachable only through a generic parameter that `allows ref struct`.
- Passing a `ref struct` to a generic method *without* `allows ref struct` is `CS9244`.
- A `ref struct` must implement every interface member, including members with default implementations.
- Inside a method whose `T` `allows ref struct`, `T` cannot be boxed, stored in a non-ref-struct field, captured by a lambda, or used across `await`.
- .NET 9 annotates many BCL APIs with `allows ref struct` — for example `string.Create<TState>`'s `TState` and the type parameters of `Func<…>`/`Action<…>` — so spans can flow through them. `Func<ReadOnlySpan<char>, int> f = s => s.Length;` compiles on .NET 9 with C# 13; on .NET 8 it is `CS9244`, and on .NET 9 with `LangVersion=12` it is `CS9202: Feature 'allows ref struct constraint' …`.

***

#### New in C# 13: Partial Properties and Indexers

Properties and indexers can be split into one *declaring* declaration (no bodies, looks like an auto-property) and one *implementing* declaration (with accessor bodies). The main consumers are source generators — for example `[GeneratedRegex]` on a property, which is new in .NET 9.

```csharp
using System.Text.RegularExpressions;

var person = new Person { Name = "  Ada " };
Console.WriteLine($"[{person.Name}]");               // [Ada]
Console.WriteLine(Patterns.Digits.IsMatch("abc123")); // True

public partial class Person
{
    public partial string Name { get; set; }          // declaring declaration
}

public partial class Person
{
    private string _name = "";
    public partial string Name                         // implementing declaration
    {
        get => _name;
        set => _name = value.Trim();
    }
}

static partial class Patterns
{
    [GeneratedRegex(@"\d+")]
    public static partial Regex Digits { get; }       // .NET 9 regex source generator on a property
}
```

- The implementing declaration cannot be an auto-property: two body-less declarations give `CS9250: A partial property may not have multiple defining declarations, and cannot be an auto-property.` and `CS9248: Partial property 'C.X' must have an implementation part.`
- Signatures, accessibility and accessor sets must match between the two parts.
- In C# 12 `partial` on a property is `CS8703: The modifier 'partial' is not valid for this item in C# 12.0.` — including in generator output.

***

#### New in C# 13: `[OverloadResolutionPriority]`

`System.Runtime.CompilerServices.OverloadResolutionPriorityAttribute` lets an API author prefer one overload over another among otherwise applicable candidates (higher wins, default 0).

```csharp
using System.Runtime.CompilerServices;

Log.Write("text");          // span overload — chosen because of the priority
Log.Sum([1, 2, 3]);         // ReadOnlySpan overload

static class Log
{
    [OverloadResolutionPriority(1)]
    public static void Write(ReadOnlySpan<char> message) => Console.WriteLine("span overload");
    public static void Write(string message) => Console.WriteLine("string overload");

    public static void Sum(int[] values) => Console.WriteLine("array");
    [OverloadResolutionPriority(1)]
    public static void Sum(ReadOnlySpan<int> values) => Console.WriteLine("span");
}
```

- It is a tool for **library authors** evolving public APIs. In application code, flag it: it hides which overload runs and surprises readers.
- Priority only compares overloads declared in the same type; it is applied before "better function member" rules.
- Applying it with `LangVersion=12` is `CS9202: Feature 'overload resolution priority' …`.

***

#### New in C# 13: Method Group Natural Type

When a method group needs a natural (delegate) type — for example `var d = obj.M;` — C# 13 prunes inapplicable candidates (wrong generic arity, unsatisfied constraints) scope by scope, like normal overload resolution.

```csharp
var action = new Widget().Run;      // C# 12: CS8917 The delegate type could not be inferred
action();                           // C# 13: binds to the extension method
Console.WriteLine(action.GetType()); // System.Action

class Widget { public void Run<T>() => Console.WriteLine("generic"); }
static class WidgetExtensions { public static void Run(this Widget w) => Console.WriteLine("extension"); }
```

In reviews prefer an explicit delegate type (`Action a = w.Run;`) whenever more than one overload exists — inference rules also changed in compiler servicing (`params`/default-parameter candidates now produce an ambiguity error instead of an order-dependent pick).

***

#### The `field` Keyword Is Preview in C# 13 — Do Not Use

`field` (access to the compiler-synthesized backing field inside an accessor) compiles on the .NET 9 SDK only with `<LangVersion>preview</LangVersion>`; with C# 13 it is `CS8652: The feature 'field keyword' is currently in Preview and *unsupported*.` It is stable only in C# 14.

```csharp
// GOOD in C# 13: explicit backing field
var c = new Customer { Name = "  Ada  " };
Console.WriteLine($"[{c.Name}]");   // [Ada]

class Customer
{
    private string _name = "";
    public string Name
    {
        get => _name;
        set => _name = value?.Trim() ?? throw new ArgumentNullException(nameof(value));
    }
}
```

**Upgrade hazard to remember:** inside a property accessor, an identifier `field` that today refers to a *member named `field`* silently changes meaning in C# 14 — it then binds to a new synthesized backing field. When the code is compiled as C# 14 the compiler warns (`CS9258: In language version 14.0, the 'field' keyword binds to a synthesized backing field for the property…`); in C# 13 mode neither the .NET 9 nor the .NET 10 SDK warns. Flag members named `field` in C# 13 code; rename them or use `@field`/`this.field`.

***

#### C# 13 Compiler Breaking Changes

Verified with SDK 9.0.319 by compiling the same source with `LangVersion` 12 and 13:

- **Collection expressions prefer exact element types.** `M([1])` with overloads `M(ReadOnlySpan<byte>)` and `M(Span<int>)` binds to `ReadOnlySpan<byte>` in C# 12 and to `Span<int>` in C# 13. Empty `[]` no longer uses "is it a span" as a tie-breaker: `M([])` with `M(ReadOnlySpan<int>)` and `M(Span<object>)` picks the `ReadOnlySpan<int>` overload in C# 12 and is `CS0121` (ambiguous) in C# 13.
- **Expression trees + `params` span overloads** (see above).
- **Iterators introduce a safe context** (`unsafe` no longer flows into nested local functions).
- **`[InlineArray]` on a `record struct`** is `CS9259` (for every `LangVersion`, compiler 17.11+).
- **Indexers require `System.Reflection.DefaultMemberAttribute`** to exist (custom corelibs only).
- **Method-group natural type with `params`/default values**: some `var d = x.M;` that inferred `Action<long[]>` depending on declaration order are now ambiguity errors — use an explicit delegate type.

***

#### Naming and Code Style

House rules (unchanged from earlier C# versions):

```csharp
public interface IUserRepository { Task<User?> FindAsync(int id, CancellationToken ct); }   // I prefix

public sealed class UserService(IUserRepository repository, ILogger<UserService> logger)   // PascalCase types
{
    public const int MaxRetryCount = 3;                     // constants: PascalCase, not SCREAMING_CASE
    private static readonly TimeSpan s_defaultTimeout = TimeSpan.FromSeconds(30); // static: s_ prefix (optional)
    private int _cacheHits;                                 // private fields: _camelCase

    public bool IsWarm { get; private set; }                // booleans: Is/Has/Can prefix

    public async Task<User> GetUserAsync(int id, CancellationToken ct = default)   // async suffix + token last
    {
        var user = await repository.FindAsync(id, ct).ConfigureAwait(false);
        if (user is null)
        {
            logger.LogWarning("User {UserId} not found", id);  // structured template, not interpolation
            throw new KeyNotFoundException($"User {id} not found");
        }
        _cacheHits++;
        IsWarm = true;
        return user;
    }
}

public sealed record User(int Id, string Name);
```

- One public type per file; file name = type name; file-scoped namespaces (`namespace MyApp.Users;`).
- Member order: constants, static fields, instance fields, constructors, properties, public methods, private methods.
- Seal classes that are not designed for inheritance (`sealed` also lets the JIT devirtualize).
- Make types `internal` unless they are part of a public API (CA1515 in .NET 9 flags public types in executables).
- `var` when the type is evident from the right-hand side; explicit type otherwise.
- Expression-bodied members for one-liners only.
- Use `is null`/`is not null`, not `== null` (operators can be overloaded).
- Enforce style with `.editorconfig` + `<EnforceCodeStyleInBuild>true</EnforceCodeStyleInBuild>` rather than review comments.

Project structure that scales:

```
Solution/
├── Directory.Build.props        # shared TargetFramework, Nullable, analyzers, TreatWarningsAsErrors
├── Directory.Packages.props     # Central Package Management
├── global.json                  # pinned SDK (9.0.3xx)
├── src/
│   ├── MyApp.Api/               # ASP.NET Core host
│   ├── MyApp.Core/              # domain + application logic, no infrastructure references
│   └── MyApp.Infrastructure/    # EF Core, HTTP clients, messaging
└── tests/
    ├── MyApp.UnitTests/
    └── MyApp.IntegrationTests/
```

***

#### Project Configuration

```xml
<!-- Directory.Build.props -->
<Project>
  <PropertyGroup>
    <TargetFramework>net9.0</TargetFramework>
    <!-- LangVersion omitted: net9.0 implies C# 13. Never "preview"; never "latest" on a shared build agent. -->
    <Nullable>enable</Nullable>
    <ImplicitUsings>enable</ImplicitUsings>
    <TreatWarningsAsErrors>true</TreatWarningsAsErrors>
    <AnalysisLevel>9.0-recommended</AnalysisLevel>
    <EnforceCodeStyleInBuild>true</EnforceCodeStyleInBuild>
    <NuGetAuditMode>all</NuGetAuditMode>       <!-- default for net9.0 is "direct" -->
    <WarningsNotAsErrors>$(WarningsNotAsErrors);NU1901;NU1902</WarningsNotAsErrors>
    <InvariantGlobalization>true</InvariantGlobalization> <!-- only if the app never needs culture data -->
  </PropertyGroup>
</Project>
```

```json
{
  "sdk": {
    "version": "9.0.319",
    "rollForward": "latestPatch"
  }
}
```

- `global.json` with `rollForward: latestPatch` keeps the feature band (9.0.3xx) and accepts newer patches. `"rollForward": "disable"` demands that exact SDK, so builds fail on machines and images that only have a newer patch.
- **NuGet audit**: on `net9.0` the default `NuGetAuditMode` is `direct` (top-level packages only); `all` is the default only for `net10.0`+ projects. Set `all` explicitly to see vulnerable transitive packages (`NU1901`–`NU1904`).
- Central Package Management (`Directory.Packages.props` with `<ManagePackageVersionsCentrally>true</ManagePackageVersionsCentrally>`) avoids version drift between projects.
- Keep ASP.NET Core / EF Core / `Microsoft.Extensions.*` 9.x packages on the same patch as the runtime (9.0.21 in October 2026). Out-of-band `Microsoft.Extensions.*` libraries such as `Microsoft.Extensions.Http.Resilience` and `Microsoft.Extensions.Caching.Hybrid` use their own minor versions: the last 9.x release is 9.10.0 (October 14, 2025); the 10.x line (10.11.0 in October 2026) follows .NET 10.
- `InvariantGlobalization` changes culture-sensitive behaviour: all cultures behave like the invariant culture, and `new CultureInfo("de-DE")` throws `CultureNotFoundException` (verified on 9.0.21; `PredefinedCulturesOnly` defaults to `true` in invariant mode). Flag it in apps that format dates/currencies for users.

***
#### Nullable Reference Types

Enable `<Nullable>enable</Nullable>` everywhere and treat nullable warnings as errors. Annotations are a compile-time contract; they do not validate data that arrives at runtime.

```csharp
using System.Diagnostics.CodeAnalysis;

Console.WriteLine(FindUser(1)?.Name ?? "unknown");                  // safe access
Console.WriteLine(TryGetEmail("b", out var email) ? email.Length : -1); // email is non-null here

static User? FindUser(int id) => id == 2 ? new User("Bo") : null;

static bool TryGetEmail(string key, [NotNullWhen(true)] out string? email)
{
    email = key == "b" ? "b@example.com" : null;
    return email is not null;
}

sealed record User(string Name);
```

Rules:

- **`!` (null-forgiving) is a code smell.** Each use must be justified by an invariant the compiler cannot see. `default!` for DI-populated or EF-populated properties is the usual exception — prefer `required` or constructor parameters.
- **Validate at the boundary.** JSON, configuration binding, EF materialization of nullable columns, reflection and `Activator` all ignore annotations. `System.Text.Json` does *not* enforce them unless `RespectNullableAnnotations = true` (.NET 9, see JSON section).
- **Use the attributes** (`[NotNullWhen]`, `[MaybeNullWhen]`, `[NotNull]`, `[MemberNotNull]`, `[DoesNotReturn]`) on `Try*` methods and guard helpers so callers get correct flow analysis.
- **`ArgumentNullException.ThrowIfNull(x)`** for public API arguments; `ArgumentException.ThrowIfNullOrEmpty/ThrowIfNullOrWhiteSpace`, `ArgumentOutOfRangeException.ThrowIfNegative/ThrowIfZero/ThrowIfGreaterThan` (.NET 8) for the rest. Do not pass non-nullable value types (`CA2264`, on by default in .NET 9) or `Nullable<T>` (`CA1871`) to `ThrowIfNull`.
- A `#nullable disable` region, or `<Nullable>disable</Nullable>` in a new project, needs a reason.
- Generic code: `T?` on an unconstrained `T` means "default value may be returned" (it is not `Nullable<T>` for value types).

***

#### Types: Classes, Records, Structs and Primary Constructors

##### Records

```csharp
var a = new Order(1, ["x"]);
var b = new Order(1, ["x"]);
Console.WriteLine(a == b);                 // False — arrays compare by reference
var c = a with { Id = 2 };
c.Tags[0] = "mutated";
Console.WriteLine(a.Tags[0]);              // mutated — 'with' is a shallow copy

var p = new Point(1, 2);
Console.WriteLine(p == new Point(1, 2));   // True — value equality for value members

record Order(int Id, string[] Tags);
readonly record struct Point(int X, int Y);
```

- Use `record`/`record struct` for immutable data with value semantics (DTOs, messages, value objects). Use `readonly record struct` for small value types.
- Records with collection members need custom `Equals`/`GetHashCode` (or `ImmutableArray<T>`/`IReadOnlyList<T>` plus explicit comparison) if value equality matters.
- Positional record properties are `init`-only, but referenced objects stay mutable.
- Do not use records as EF Core entities that need identity-based tracking; value equality confuses change tracking and `HashSet` membership after mutation.

##### `required` and `init`

```csharp
var user = new Account { Email = "a@example.com" };
Console.WriteLine(user.Email);

sealed class Account
{
    public required string Email { get; init; }   // must be set in the object initializer
    public string? DisplayName { get; init; }
}
```

Omitting a required member is `CS9035`. `[SetsRequiredMembers]` on a constructor disables the check — only use it on constructors that really set every required member.

##### Primary constructors (C# 12) — pitfalls

```csharp
var s = new Service("a");
s.Rename();
Console.WriteLine($"{s.Name} vs {s.Current}");   // a vs changed

class Service(string name)
{
    public string Name { get; } = name;          // copies the value once
    public string Current => name;               // captures the parameter into a hidden mutable field
    public void Rename() => name = "changed";    // parameters are not readonly
}
```

The compiler warns `CS9124: Parameter 'string name' is captured into the state of the enclosing type and its value is also used to initialize a field, property, or event.` — two copies of the same state that can diverge. In review:

- Use the parameter either to initialize a member **or** capture it, not both.
- For DI services, `class OrderService(IOrderRepository repository, ILogger<OrderService> logger)` is idiomatic; if the dependency must be immutable, assign it to a `private readonly` field explicitly.
- Primary constructor parameters are not validated: add `ArgumentNullException.ThrowIfNull` in a field initializer or keep a regular constructor when arguments come from untrusted callers.

##### Structs

- Keep structs small (≤ 16–24 bytes), immutable (`readonly struct`), and override `Equals`/`GetHashCode` (or use `record struct`) — the default `ValueType.Equals` uses reflection for some layouts.
- Mark methods that do not mutate state `readonly` on mutable structs to avoid defensive copies.
- `[InlineArray(n)]` structs (C# 12) cannot be `record struct`s, and the default `Equals`/`GetHashCode` throw for them in .NET 9.

***

#### Pattern Matching

```csharp
object o = new[] { 1, 2, 3 };
Console.WriteLine(o switch
{
    int[] and [1, .. var rest] => $"starts with 1, {rest.Length} more",
    int[] { Length: 0 } => "empty",
    null => "null",
    _ => "other",
});

Console.WriteLine(Classify(15));          // big
Console.WriteLine(Shipping(new(150m, true)));  // Express

static string Classify(int n) => n switch
{
    < 0 => "negative",
    0 => "zero",
    > 0 and < 10 => "small",
    _ => "big",
};

static string Shipping(Parcel p) => p switch
{
    { Total: > 100m, IsPriority: true } => "Express",
    { Total: > 100m } => "Standard",
    { IsPriority: true } => "Priority",
    _ => "Economy",
};

record Parcel(decimal Total, bool IsPriority);
```

- A `switch` expression without a discard that does not cover all inputs produces `CS8509` and throws `SwitchExpressionException` at runtime — treat the warning as an error.
- Order matters: the compiler reports subsumed arms (`CS8510`), but `when` guards hide coverage problems.
- `is not null` and `is { } x` are the idiomatic null checks; `is var x` always matches (including `null`).
- For enums, a `_ => throw new UnreachableException()` arm documents intent; values outside the declared members are legal at runtime.

***

#### Collections and Collection Expressions

##### Collection expressions (C# 12) and their runtime types

```csharp
using System.Collections.Immutable;

IEnumerable<int> e = [1, 2];
IReadOnlyList<int> ro = [1, 2];
IList<int> mutable = [1, 2];
mutable.Add(3);                                 // OK: IList<T>/ICollection<T> targets get a List<T>
List<int> list = [.. e, 4];                     // spread
ImmutableArray<int> imm = [1, 2, 3];            // via [CollectionBuilder]
int[] empty = [];

Console.WriteLine(e.GetType().Name);            // <>z__ReadOnlyArray`1 (compiler-synthesized, read-only)
Console.WriteLine(ReferenceEquals(empty, Array.Empty<int>())); // True
try { ((ICollection<int>)ro).Add(9); }
catch (NotSupportedException) { Console.WriteLine("read-only"); }
Console.WriteLine($"{string.Join(",", list)} {imm.Length}");
```

- `IEnumerable<T>`/`IReadOnlyCollection<T>`/`IReadOnlyList<T>` targets produce a read-only compiler-synthesized type; casting it to `ICollection<T>` and mutating throws `NotSupportedException`.
- `[]` for arrays reuses `Array.Empty<T>()` — do not lock on or compare identity of empty collections.
- Prefer collection expressions over `new List<int> { … }` / `new[] { … }` in C# 12+ code; they pick the most efficient construction and work with spans.

##### New in .NET 9: collection types and lookups

```csharp
using System.Collections.ObjectModel;

OrderedDictionary<string, int> ordered = new() { ["b"] = 2, ["c"] = 3 };
ordered.Insert(0, "a", 1);                                 // index-based insert
Console.WriteLine($"{string.Join(",", ordered.Keys)} idx(c)={ordered.IndexOf("c")}"); // a,b,c idx(c)=2

var set = new HashSet<int> { 1, 2 };
ReadOnlySet<int> view = new(set);                          // read-only *view*, not a copy
set.Add(3);
Console.WriteLine(view.Count);                             // 3

var queue = new PriorityQueue<string, int>();
queue.Enqueue("job", 5);
queue.Remove("job", out _, out int oldPriority);           // O(n) removal, enables re-prioritization
queue.Enqueue("job", 1);
Console.WriteLine($"{oldPriority} -> {queue.Peek()}");

// Alternate lookup: query a string-keyed dictionary with a span, no string allocation
var counts = new Dictionary<string, int>(StringComparer.OrdinalIgnoreCase) { ["key"] = 7 };
var lookup = counts.GetAlternateLookup<ReadOnlySpan<char>>();
ReadOnlySpan<char> slice = "my KEY".AsSpan(3);
Console.WriteLine(lookup[slice]);                          // 7
```

- `GetAlternateLookup<ReadOnlySpan<char>>()` works on `Dictionary`, `HashSet`, `ConcurrentDictionary` and `FrozenDictionary`/`FrozenSet` with `string` keys when the comparer implements `IAlternateEqualityComparer` (the built-in `StringComparer.Ordinal*` comparers and the default comparer do). A custom comparer throws `InvalidOperationException`.
- Use `FrozenDictionary`/`FrozenSet` (.NET 8) for read-mostly lookup tables built once at startup.
- `ReadOnlySet<T>`, like `ReadOnlyCollection<T>`, reflects later changes to the wrapped set — it is not a snapshot. Use `ToFrozenSet()`/`ToImmutableHashSet()` for snapshots.
- `OrderedDictionary<TKey,TValue>` (generic) is new in .NET 9; the non-generic `System.Collections.Specialized.OrderedDictionary` boxes and is not type-safe.

##### Collection review rules

- Never mutate a `List<T>`/`Dictionary` while enumerating it (`InvalidOperationException: Collection was modified`).
- `Dictionary<TKey,TValue>` is not thread-safe even for "mostly reads" while any writer exists; use `ConcurrentDictionary` or immutable snapshots. `ConcurrentDictionary.GetOrAdd(key, factory)` may run the factory more than once.
- Expose `IReadOnlyList<T>`/`IReadOnlyCollection<T>` from APIs, not `List<T>`.
- Specify a comparer for string keys (`StringComparer.Ordinal` or `OrdinalIgnoreCase`), never rely on culture-sensitive defaults for identifiers.

***

#### LINQ

##### New in .NET 9: `CountBy`, `AggregateBy`, `Index`

```csharp
string[] words = ["a", "b", "a", "c", "a", "b"];

foreach (var (word, count) in words.CountBy(w => w))        // no intermediate groupings
    Console.Write($"{word}={count} ");                      // a=3 b=2 c=1
Console.WriteLine();

(string Id, int Score)[] scores = [("0", 42), ("1", 5), ("0", 25)];
foreach (var total in scores.AggregateBy(s => s.Id, seed: 0, (acc, s) => acc + s.Score))
    Console.Write($"{total} ");                             // [0, 67] [1, 5]
Console.WriteLine();

foreach (var (index, word) in words.Take(2).Index())        // (int Index, T Item)
    Console.Write($"{index}:{word} ");                      // 0:a 1:b
Console.WriteLine();

var mostFrequent = words.CountBy(w => w).MaxBy(kv => kv.Value);
Console.WriteLine(mostFrequent.Key);                        // a
```

`CountBy`/`AggregateBy` return `IEnumerable<KeyValuePair<TKey, TValue>>`. .NET 9 also adds `Queryable` counterparts, but EF Core 9 cannot translate them: `dbContext.Items.CountBy(i => i.Category).ToList()` throws `InvalidOperationException: The LINQ expression … could not be translated`. Use `GroupBy(i => i.Category).Select(g => new { g.Key, Count = g.Count() })` for database queries.

##### LINQ pitfalls a reviewer must catch

```csharp
int threshold = 1;
List<int> numbers = [1, 2, 3, 4];
var query = numbers.Where(x => x > threshold);   // deferred: nothing runs yet
threshold = 3;
Console.WriteLine(string.Join(",", query));      // 4 — the lambda sees the *current* threshold

try { new[] { "a", "A" }.ToDictionary(s => s.ToLowerInvariant()); }
catch (ArgumentException) { Console.WriteLine("duplicate key"); }

try { new[] { 1, 1 }.Single(x => x == 1); }
catch (InvalidOperationException) { Console.WriteLine("more than one element"); }

Console.WriteLine(Array.Empty<int>().FirstOrDefault());     // 0 — default(T), not null
Console.WriteLine(Array.Empty<int>().FirstOrDefault(-1));   // -1 — explicit default (.NET 6+)
```

- **Multiple enumeration** of an `IEnumerable<T>` re-runs the query (and any I/O or side effects). Materialize once with `ToList()`/`ToArray()` when a sequence is used more than once.
- **`Count() > 0`** on a lazy sequence enumerates everything — use `Any()`. On `List<T>`/arrays use the `Count`/`Length` property.
- **`First` vs `Single` vs `FirstOrDefault`**: `Single` documents and enforces uniqueness; `FirstOrDefault` returns `default(T)` (0 for `int`) which can be confused with a real value.
- **`ToDictionary`/`ToHashSet`/`GroupBy`** without a comparer use the default (ordinal, case-sensitive) string comparison; `ToDictionary` throws `ArgumentException` on duplicate keys, `ToHashSet` silently drops duplicates.
- **`OrderBy` is stable**; `OrderBy(x => x).OrderBy(y => y)` discards the first ordering — use `ThenBy`.
- **`Select` with side effects** combined with deferred execution runs the side effects at enumeration time, possibly twice.
- **`IQueryable` vs `IEnumerable`**: calling a method that takes `IEnumerable<T>` (or `AsEnumerable()`) switches to client-side evaluation for the rest of the query.
- **`.Result` inside `Select`** (`items.Select(i => GetAsync(i).Result)`) blocks threads; use `Task.WhenAll(items.Select(GetAsync))` with bounded concurrency.
- Prefer method syntax; query syntax is fine for joins and `let`.

***

#### Strings, Spans and Memory

```csharp
using System.Buffers;
using System.Text;
using System.Text.RegularExpressions;

// Span splitting without allocations (.NET 9 enumerator)
ReadOnlySpan<char> csv = "a,b,,c";
int fields = 0;
foreach (Range r in csv.Split(',')) fields++;
Console.WriteLine(fields);                                   // 4

// SearchValues<string> (.NET 9): multi-substring search
SearchValues<string> animals = SearchValues.Create(["cat", "dog"], StringComparison.OrdinalIgnoreCase);
Console.WriteLine("hot DOG stand".AsSpan().IndexOfAny(animals)); // 4

// Regex.EnumerateSplits (.NET 9): ranges instead of string[]
foreach (Range r in Regex.EnumerateSplits("1-2-3", "-")) Console.Write($"[{"1-2-3"[r]}]");
Console.WriteLine();

// StringBuilder for loops; interpolation for one-offs
var sb = new StringBuilder();
for (int i = 0; i < 3; i++) sb.Append(i).Append(';');
Console.WriteLine(sb.ToString());                            // 0;1;2;

// Pool large temporary buffers
byte[] buffer = ArrayPool<byte>.Shared.Rent(4096);
try { Console.WriteLine(buffer.Length >= 4096); }
finally { ArrayPool<byte>.Shared.Return(buffer); }           // never use buffer after Return
```

Rules:

- **Comparisons**: use `StringComparison.Ordinal`/`OrdinalIgnoreCase` for identifiers, keys, paths, protocol tokens; culture-aware comparison only for user-visible sorting. `string.Equals("straße", "STRASSE", StringComparison.OrdinalIgnoreCase)` is `false`. `ToLower()`/`ToUpper()` without culture is culture-sensitive (the Turkish-I problem); use `ToLowerInvariant()` or comparison overloads.
- **`Span<T>`** is stack-only: not in fields of classes, not in lambdas, not across `await`. Use `Memory<T>`/`ReadOnlyMemory<T>` for async or heap storage.
- **`stackalloc`** only for small, bounded sizes (`stackalloc byte[256]` or `n <= 256 ? stackalloc … : new …`); unbounded `stackalloc` of user-controlled size is a stack-overflow DoS.
- **`ArrayPool<T>`**: always return in `finally`; rented arrays may be larger than requested and contain old data — slice to the requested length; clear them (`clearArray: true`) when they held secrets.
- **Comparing a span to `null`/`default`** is always "is empty" (`CA2265`, enabled by default in .NET 9) — use `IsEmpty`.
- **`BitConverter.ToString(bytes).Replace("-", "")`** → `Convert.ToHexString(bytes)` / `Convert.ToHexStringLower(bytes)` (.NET 9; `CA1872`).
- **Base64 in URLs/JWTs**: `Base64Url.EncodeToString(bytes)` / `Base64Url.DecodeFromChars(text)` (.NET 9, `System.Buffers.Text`) instead of `Convert.ToBase64String(...).Replace('+','-')…`.
- **`CompositeFormat.Parse`** (.NET 8) for format strings used repeatedly with `string.Format`.
- **`Regex`**: use `[GeneratedRegex]` (method or, in .NET 9, property) for constant patterns; always pass a `matchTimeout` (or `RegexOptions.NonBacktracking`) for patterns applied to untrusted input to prevent ReDoS.

***

#### Numbers, Dates and GUIDs

```csharp
int max = int.MaxValue;
Console.WriteLine(max + 1);                               // -2147483648 (unchecked wraparound)
try { _ = checked(max + 1); } catch (OverflowException) { Console.WriteLine("overflow"); }

Console.WriteLine(0.1 + 0.2 == 0.3);                      // False — use decimal for money
Console.WriteLine(0.1m + 0.2m == 0.3m);                   // True

// .NET 9: integer TimeSpan factories avoid floating-point rounding
Console.WriteLine(TimeSpan.FromSeconds(101.832));         // 00:01:41.8319999
Console.WriteLine(TimeSpan.FromSeconds(101, 832));        // 00:01:41.8320000

double big = 1e20, nan = double.NaN;                     // variables: constant casts are compile-time checked
Console.WriteLine((int)big);                              // .NET 9 x64: 2147483647 (saturates)
Console.WriteLine((int)nan);                              // .NET 9 x64: 0
```

- **`TimeSpan.FromMilliseconds(integer)` breaks expression trees on .NET 9.** An `int`/`long` argument now binds to the new `FromMilliseconds(long milliseconds, long microseconds = 0)` overload, and expression trees cannot contain calls that rely on optional arguments: `Expression<Func<int, TimeSpan>> e = x => TimeSpan.FromMilliseconds(x);` compiles on .NET 8 and .NET 10 but fails on .NET 9 with `CS0854: An expression tree may not contain a call or invocation that uses optional arguments` — this hits EF Core `Where`/`Select` lambdas. `FromDays`/`FromHours`/`FromMinutes`/`FromSeconds`/`FromMicroseconds` have single-argument integer overloads and are not affected. Fix: pass a `double` (`TimeSpan.FromMilliseconds((double)x)`) or compute the `TimeSpan` outside the expression. (Verified on .NET 8.0.32, 9.0.21 and 10.0.13.)
- **Float → integer conversions saturate on x86/x64 in .NET 9** (out-of-range values clamp to `MinValue`/`MaxValue`, `NaN` → 0). On .NET 8.0.32 x64 `(int)1e20`, `(int)-1e20` and `(int)double.NaN` all returned `int.MinValue` (`0x80000000`). Code that relied on the sentinel, or tests asserting it, change behaviour; validate ranges explicitly instead.
- Use `checked` (or `<CheckForOverflowUnderflow>true</CheckForOverflowUnderflow>`) for arithmetic on sizes, money in minor units, and anything derived from user input.
- **Dates**: store and compare `DateTimeOffset`/UTC; `new DateTime(...)` has `Kind = Unspecified`. Inject `TimeProvider` (.NET 8) instead of calling `DateTime.UtcNow`/`DateTimeOffset.UtcNow` in logic that needs testing.
- **GUIDs — New in .NET 9**: `Guid.CreateVersion7()` produces RFC 9562 version-7 GUIDs with a 48-bit Unix-millisecond timestamp prefix.

```csharp
var tight = Enumerable.Range(0, 1000).Select(_ => Guid.CreateVersion7()).ToList();
int outOfOrder = 0;
for (int i = 1; i < tight.Count; i++) if (tight[i - 1].CompareTo(tight[i]) > 0) outOfOrder++;
Console.WriteLine(outOfOrder > 0);   // True — no ordering within the same millisecond

var v7 = Guid.CreateVersion7(DateTimeOffset.UtcNow);
Console.WriteLine(v7.Version);       // 7
```

`CreateVersion7()` is *not* monotonic within a millisecond (the remaining bits are random; in a tight loop about half of adjacent pairs compared out of order). It sorts by time only across milliseconds, and SQL Server's `uniqueidentifier` ordering uses a different byte order, so v7 GUIDs are not automatically "sequential" there. Do not use them as the sole ordering key, and do not expose them where creation time is sensitive (the timestamp is readable).

***

#### Error Handling and Exceptions

```csharp
try { Rethrow(useThrowEx: true); }
catch (Exception e) { Console.WriteLine(e.StackTrace!.Split('\n')[0].Trim()); } // points at Rethrow — origin lost

try { Rethrow(useThrowEx: false); }
catch (Exception e) { Console.WriteLine(e.StackTrace!.Split('\n')[0].Trim()); } // points at Thrower — preserved

try { throw new HttpStatusException(503); }
catch (HttpStatusException e) when (e.Status >= 500)        // filter: does not unwind when false
{
    Console.WriteLine("retryable");
}

static void Rethrow(bool useThrowEx)
{
    try { Thrower(); }
    catch (InvalidOperationException ex)
    {
        if (useThrowEx) throw ex;   // BAD: resets the stack trace (CA2200)
        throw;                      // GOOD
    }
}

static void Thrower() => throw new InvalidOperationException("boom");

sealed class HttpStatusException(int status) : Exception($"HTTP {status}")
{
    public int Status { get; } = status;
}
```

Rules:

- Catch the most specific exception you can handle; never `catch (Exception) { }` silently. Top-level handlers (middleware, `BackgroundService.ExecuteAsync` wrapper) may catch `Exception` to log and fail the request/job.
- Wrap with context: `throw new OrderProcessingException($"Order {id} failed", ex);` keeps the inner exception.
- `throw;` to rethrow; `ExceptionDispatchInfo.Capture(ex).Throw()` to rethrow later from another context.
- Do not use exceptions for expected control flow (parsing user input → `TryParse`, missing keys → `TryGetValue`). .NET 9's managed exception handling is 2–4× faster, but throwing is still far more expensive than returning a result.
- `OperationCanceledException` is not an error when the caller's token was cancelled — let it propagate or handle it separately from failures; do not log it as an error.
- `catch (Exception ex) when (ex is not OperationCanceledException)` is the idiomatic way to keep cancellation flowing.
- Custom exceptions: derive from `Exception`, end the name with `Exception`, provide the standard constructors; do not add `[Serializable]`/serialization constructors (`BinaryFormatter` is gone; the `Exception(SerializationInfo, StreamingContext)` constructor is obsolete, `SYSLIB0051`).

***

#### Resource Management: `IDisposable` and `IAsyncDisposable`

```csharp
await using (var outer = new Resource("outer"))
await using (var inner = new Resource("inner"))
{
    Console.WriteLine("work");
}   // disposes inner, then outer

using var file = File.CreateText(Path.Combine(Path.GetTempPath(), "demo.txt")); // using declaration
file.WriteLine("x");

sealed class Resource(string name) : IAsyncDisposable
{
    public ValueTask DisposeAsync()
    {
        Console.WriteLine($"dispose {name}");
        return ValueTask.CompletedTask;
    }
}
```

- Every `IDisposable` you create and own must be disposed — `using` declarations are the default. Types you receive (DI-injected, `HttpClient` from the factory) are disposed by their owner, not by you.
- Prefer `await using` for types that implement `IAsyncDisposable` (`DbContext`, `Stream`s doing async I/O, `ServiceProvider` scopes with async-disposable services). Disposing an async-only service synchronously throws `InvalidOperationException` from the DI container.
- Full `Dispose(bool)` pattern only for unsealed classes that own unmanaged resources directly; otherwise `sealed class … : IDisposable` with a simple `Dispose()`. Never add a finalizer just to call `Dispose`; use `SafeHandle` for native handles.
- `ObjectDisposedException.ThrowIf(_disposed, this)` (.NET 7+) at the top of public members of disposable types.
- `CancellationTokenSource`, `Timer`, `SemaphoreSlim`, `HttpResponseMessage`, `JsonDocument`, `Process`, `RSA`/`ECDsa` instances are disposable — check that they are disposed.

***
#### Async/Await and Concurrency

##### Fundamentals

```csharp
var all = Task.WhenAll(FailAsync("A"), FailAsync("B"));
try { await all; }
catch (Exception ex)
{
    // await rethrows only one exception (the first recorded — usually A, but B under load); the Task keeps all of them
    Console.WriteLine($"{ex.Message}; total={all.Exception!.InnerExceptions.Count}");   // A; total=2
}

ValueTask<int> vt = GetValueAsync();
Console.WriteLine(await vt);    // await a ValueTask exactly once; never store and await twice

static async Task FailAsync(string message) { await Task.Yield(); throw new InvalidOperationException(message); }
static async ValueTask<int> GetValueAsync() { await Task.Yield(); return 42; }
```

Review rules:

- **No sync-over-async.** `.Result`, `.Wait()`, `.GetAwaiter().GetResult()` on incomplete tasks block a thread-pool thread and can deadlock under a `SynchronizationContext` (WinForms/WPF/MAUI, legacy ASP.NET). In ASP.NET Core they cause thread-pool starvation under load.
- **No `async void`** except event handlers, and those must catch all exceptions — an exception escaping `async void` crashes the process.
- **Pass `CancellationToken` through every async call chain**, as the last parameter, defaulted only on public entry points. ASP.NET Core supplies `HttpContext.RequestAborted` as a `CancellationToken` parameter in minimal APIs and controller actions.
- **`ConfigureAwait(false)`** in general-purpose library code; it is unnecessary in ASP.NET Core application code (no `SynchronizationContext`) but harmless.
- **Fire-and-forget** (`_ = DoWorkAsync();`) loses exceptions and outlives the request scope (and its `DbContext`). Use a `BackgroundService` + `Channel<T>` queue or a hosted job system.
- **Do not wrap synchronous CPU work in `Task.Run`** inside ASP.NET Core request handlers; it only moves the work to another pool thread.
- **`ValueTask`**: await once, never concurrently, never `.Result` before completion. Convert with `.AsTask()` if it must be awaited more than once.
- **`TaskCompletionSource`**: create with `TaskCreationOptions.RunContinuationsAsynchronously` so that `SetResult` does not run arbitrary continuations inline on the completing thread.
- **Avoid `Task.Factory.StartNew`** with async lambdas (it returns `Task<Task>`); use `Task.Run`.

##### Cancellation and timeouts

```csharp
using var cts = new CancellationTokenSource(TimeSpan.FromMilliseconds(50));
try { await Task.Delay(5000, cts.Token); }
catch (OperationCanceledException e) when (e.CancellationToken == cts.Token)
{
    Console.WriteLine(e.GetType().Name);                // TaskCanceledException (derives from OperationCanceledException)
}

// WaitAsync stops *waiting*; it does not stop the work
var slow = Task.Delay(300).ContinueWith(_ => "done");
try { await slow.WaitAsync(TimeSpan.FromMilliseconds(20)); }
catch (TimeoutException) { Console.WriteLine($"timed out, work still running: {!slow.IsCompleted}"); }

using var a = new CancellationTokenSource();
using var b = new CancellationTokenSource();
using var linked = CancellationTokenSource.CreateLinkedTokenSource(a.Token, b.Token); // params span overload in .NET 9
b.Cancel();
Console.WriteLine(linked.IsCancellationRequested);      // True
```

- Catch `OperationCanceledException`, not `TaskCanceledException`.
- Dispose `CancellationTokenSource`s that use timers (`CancelAfter`, timeout constructor) or are linked; undisposed linked sources leak registrations on long-lived parent tokens.
- `cts.Cancel()` runs registered callbacks synchronously and can throw `AggregateException`; `CancelAsync()` (.NET 8) avoids blocking.
- Check `ct.ThrowIfCancellationRequested()` in long CPU loops; pass the token to every awaited call.

##### New in .NET 9: `Task.WhenEach`

```csharp
Task<string>[] tasks = [DelayAsync(30, "slow"), DelayAsync(5, "fast")];
await foreach (Task<string> completed in Task.WhenEach(tasks))
{
    Console.Write(await completed + " ");                // fast slow — completion order
}
Console.WriteLine();

static async Task<string> DelayAsync(int ms, string result) { await Task.Delay(ms); return result; }
```

`Task.WhenEach` replaces the O(n²) `while (tasks.Count > 0) { var t = await Task.WhenAny(tasks); tasks.Remove(t); }` loop. Each yielded task is already complete; `await completed` (not `.Result`) still surfaces its exception correctly.

##### `IAsyncEnumerable<T>` and cancellation

```csharp
using System.Runtime.CompilerServices;

using var cts = new CancellationTokenSource();
int seen = 0;
try
{
    await foreach (int n in NumbersAsync().WithCancellation(cts.Token))
    {
        seen++;
        if (n == 2) cts.Cancel();
    }
}
catch (OperationCanceledException) { Console.WriteLine($"cancelled after {seen} items"); }   // 3

static async IAsyncEnumerable<int> NumbersAsync([EnumeratorCancellation] CancellationToken ct = default)
{
    for (int i = 0; ; i++)
    {
        await Task.Delay(1, ct);
        yield return i;
    }
}
```

- Without `[EnumeratorCancellation]`, `WithCancellation(token)` does not reach the iterator — the token parameter stays `default`. The compiler warns (`CS8425`) when an async iterator has an unannotated `CancellationToken` parameter.
- LINQ operators over `IAsyncEnumerable<T>` are *not* in the .NET 9 BCL (`System.Linq.AsyncEnumerable` is .NET 10). Use the `System.Linq.Async` package or plain `await foreach`; when upgrading to .NET 10, remove `System.Linq.Async` to avoid ambiguous-call errors.

##### Bounded parallelism

```csharp
int current = 0, peak = 0;
await Parallel.ForEachAsync(Enumerable.Range(0, 20),
    new ParallelOptions { MaxDegreeOfParallelism = 3 },
    async (item, ct) =>
    {
        int now = Interlocked.Increment(ref current);
        int seen;
        do { seen = peak; } while (now > seen && Interlocked.CompareExchange(ref peak, now, seen) != seen);
        await Task.Delay(5, ct);
        Interlocked.Decrement(ref current);
    });
Console.WriteLine(peak);   // 3
```

- `Task.WhenAll(items.Select(CallApiAsync))` over thousands of items opens thousands of concurrent requests — bound it with `Parallel.ForEachAsync` (`MaxDegreeOfParallelism`), a `SemaphoreSlim`, or a `Channel<T>` with N consumers.
- `Parallel.For/ForEach` (synchronous) are for CPU-bound work only; never put blocking I/O or `.Result` inside them.

##### Channels — New in .NET 9: prioritized channel

```csharp
using System.Threading.Channels;

var channel = Channel.CreateUnboundedPrioritized<int>();   // smallest value first (Comparer<T>.Default)
foreach (int x in new[] { 3, 1, 2 }) await channel.Writer.WriteAsync(x);
channel.Writer.Complete();
await foreach (int x in channel.Reader.ReadAllAsync()) Console.Write(x);   // 123
Console.WriteLine();

var bounded = Channel.CreateBounded<string>(new BoundedChannelOptions(100)
{
    FullMode = BoundedChannelFullMode.Wait,      // back-pressure instead of unbounded memory growth
    SingleReader = true,
});
await bounded.Writer.WriteAsync("job");
Console.WriteLine(await bounded.Reader.ReadAsync());
```

Prefer bounded channels for producer/consumer pipelines fed by external input; an unbounded channel is an unbounded queue.

##### Thread safety primitives

- `Lock` (.NET 9) for short synchronous critical sections; `SemaphoreSlim` for async sections; `ReaderWriterLockSlim` only after measuring.
- `Interlocked` for counters and flags. .NET 9 adds `Interlocked.Exchange`/`CompareExchange` for `byte`, `sbyte`, `short`, `ushort` and lets the generic overloads accept enums and other primitive types (previously `where T : class`).
- `volatile` and `Volatile.Read/Write` only for simple publication; `Thread.VolatileRead/Write` are obsolete (`SYSLIB0054`).
- `static` mutable state in ASP.NET Core apps is shared by all requests — flag unsynchronized `static List<T>`/`Dictionary` fields.
- `[ThreadStatic]`/`ThreadLocal<T>` do not flow across `await`; use `AsyncLocal<T>` for ambient async context, sparingly.

##### Testable time: `TimeProvider` (.NET 8)

```csharp
var service = new TokenValidator(TimeProvider.System);
Console.WriteLine(service.IsExpired(DateTimeOffset.UtcNow.AddMinutes(-20)));   // True

using var timer = TimeProvider.System.CreateTimer(_ => { }, null, TimeSpan.FromSeconds(1), Timeout.InfiniteTimeSpan);
await Task.Delay(TimeSpan.FromMilliseconds(1), TimeProvider.System);          // Delay overload taking a TimeProvider

sealed class TokenValidator(TimeProvider time)
{
    private static readonly TimeSpan Lifetime = TimeSpan.FromMinutes(15);
    public bool IsExpired(DateTimeOffset issuedAt) => time.GetUtcNow() - issuedAt > Lifetime;
}
```

Register `TimeProvider.System` as a singleton and inject it; tests use `FakeTimeProvider` from `Microsoft.Extensions.TimeProvider.Testing` (see Testing).

***

#### Dependency Injection, Options and Configuration

##### Lifetimes and registration

```csharp
var builder = WebApplication.CreateBuilder(args);

builder.Services.AddSingleton(TimeProvider.System);
builder.Services.AddScoped<IOrderRepository, InMemoryOrderRepository>();   // one per request
builder.Services.AddTransient<IPriceCalculator, PriceCalculator>();         // new per resolution
builder.Services.AddKeyedSingleton<INotifier, EmailNotifier>("email");      // keyed services (.NET 8)
builder.Services.AddKeyedSingleton<INotifier, SmsNotifier>("sms");

var app = builder.Build();
app.MapPost("/orders/{id:int}/notify", (int id, [FromKeyedServices("email")] INotifier notifier) =>
    notifier.Notify($"order {id}"));
app.Run();

public interface IOrderRepository { }
public sealed class InMemoryOrderRepository : IOrderRepository { }
public interface IPriceCalculator { }
public sealed class PriceCalculator : IPriceCalculator { }
public interface INotifier { string Notify(string message); }
public sealed class EmailNotifier : INotifier { public string Notify(string m) => $"email: {m}"; }
public sealed class SmsNotifier : INotifier { public string Notify(string m) => $"sms: {m}"; }
```

Review rules:

- **Captive dependencies**: a singleton must not depend on a scoped service (e.g. a `DbContext`). In the `Development` environment `WebApplication`/the default host validate scopes and the whole service graph on `Build()` and throw `AggregateException` ("Cannot consume scoped service … from singleton …"); in `Production` the same registration builds and runs, silently sharing one scoped instance forever. Verified on .NET 9.0.21.
- To use scoped services from a singleton or `BackgroundService`, inject `IServiceScopeFactory` and create a scope per unit of work (`await using var scope = scopeFactory.CreateAsyncScope();`).
- **Do not call `BuildServiceProvider()`** in application code (`ASP0000`): it creates a second container with duplicate singletons.
- **Keyed services**: `[FromKeyedServices("key")]` throws `InvalidOperationException` when no service is registered under that key — since .NET 9 (and 8.0.9) it no longer falls back to the non-keyed registration. Middleware constructors and `Invoke`/`InvokeAsync` parameters accept `[FromKeyedServices]` in ASP.NET Core 9.
- Disposable transients resolved from the root provider are kept alive until the container is disposed — a memory leak. Resolve them from a scope or make them scoped.
- Async-only disposables (`IAsyncDisposable` without `IDisposable`) must live in scopes disposed with `await using`/`CreateAsyncScope()`; a synchronous `scope.Dispose()` throws `InvalidOperationException: 'X' type only implements IAsyncDisposable`.
- Constructor injection only; no service locator (`IServiceProvider.GetService` in business code) except in factories.
- `ActivatorUtilities.CreateInstance` constructor selection changed in .NET 9 so that `[ActivatorUtilitiesConstructor]` is always honoured; check types with multiple public constructors.

##### Options

```csharp
using System.ComponentModel.DataAnnotations;
using Microsoft.Extensions.Options;

var builder = WebApplication.CreateBuilder(args);
builder.Services.AddOptions<SmtpOptions>()
    .Bind(builder.Configuration.GetSection(SmtpOptions.Section))
    .ValidateDataAnnotations()
    .Validate(o => o.Port != 25 || o.AllowPlainText, "Port 25 requires AllowPlainText")
    .ValidateOnStart();                        // fail at startup (e.g. missing Smtp:Host), not on first use
builder.Services.AddSingleton<Mailer>();

var app = builder.Build();
app.MapGet("/smtp", (Mailer m) => m.Describe());
app.Run();

public sealed class SmtpOptions
{
    public const string Section = "Smtp";
    [Required] public string Host { get; set; } = "";
    [Range(1, 65535)] public int Port { get; set; } = 587;
    public bool AllowPlainText { get; set; }
}

public sealed class Mailer(IOptions<SmtpOptions> options)
{
    private readonly SmtpOptions _options = options.Value;
    public string Describe() => $"{_options.Host}:{_options.Port}";
}
```

- Without `ValidateOnStart()`, validation runs on the first `.Value` access and throws `OptionsValidationException` in the middle of a request.
- `IOptions<T>` is a singleton snapshot; `IOptionsSnapshot<T>` is per scope (reloads per request, cannot be injected into singletons); `IOptionsMonitor<T>` for singletons that react to reloads.
- Never bind secrets into types that get logged or serialized. Never put secrets in `appsettings.json`; use environment variables, user-secrets in development, or a vault provider.
- For Native AOT/trimmed apps enable the configuration binding source generator (`<EnableConfigurationBindingGenerator>true</EnableConfigurationBindingGenerator>`) and the options validation source generator (`[OptionsValidator]`).
- **Environment variables take precedence** in `runtimeconfig.json`-style app runtime settings (for example `DOTNET_gcServer`) since .NET 9.

***

#### Logging and Observability

```csharp
using var loggerFactory = LoggerFactory.Create(b => b.AddSimpleConsole(o => o.SingleLine = true));
var logger = loggerFactory.CreateLogger<OrderWorker>();
new OrderWorker(logger).Process(42, "eu-west");
// info: OrderWorker[...] Processing order 42 in eu-west

public partial class OrderWorker(ILogger<OrderWorker> logger)   // primary constructor: supported by the generator in .NET 9
{
    public void Process(int orderId, string region)
    {
        LogProcessing(orderId, region);
        logger.LogInformation("Order {OrderId} done in {Region}", orderId, region);   // template, not $"..."
    }

    [LoggerMessage(Level = LogLevel.Information, Message = "Processing order {OrderId} in {Region}")]
    private partial void LogProcessing(int orderId, string region);
}
```

Review rules:

- **Message templates, not interpolation**: `logger.LogInformation($"User {id}")` allocates even when the level is disabled and loses structured properties (`CA2254`). Use `{Named}` placeholders; names should be stable PascalCase.
- **`[LoggerMessage]` source generator** for hot paths (zero allocation when disabled). In .NET 9 it works in classes with a primary constructor that takes an `ILogger`.
- **Never log secrets or personal data**: tokens, passwords, connection strings, full request bodies, `Authorization` headers. Since .NET 9, `IHttpClientFactory` no longer logs header *values* at `Trace` by default and redacts URI query strings in its logs and `HttpClient` EventSource events; re-enabling them (`RedactLoggedHeaders(_ => false)`) needs a justification.
- Use `logger.LogError(ex, "…")` — pass the exception object, not `ex.Message`.
- Log levels: `Trace`/`Debug` for diagnostics, `Information` for business events, `Warning` for recoverable anomalies, `Error` for failed operations, `Critical` for process-wide failures.

##### Metrics and tracing

```csharp
using System.Diagnostics;
using System.Diagnostics.Metrics;

var meter = new Meter("Shop.Orders", "1.0");
Counter<long> ordersPlaced = meter.CreateCounter<long>("shop.orders.placed");
Gauge<int> queueDepth = meter.CreateGauge<int>("shop.queue.depth");      // New in .NET 9
ordersPlaced.Add(1, new KeyValuePair<string, object?>("region", "eu"));
queueDepth.Record(17);

var source = new ActivitySource("Shop.Orders");
using var listener = new ActivityListener
{
    ShouldListenTo = s => s.Name == "Shop.Orders",
    Sample = (ref ActivityCreationOptions<ActivityContext> _) => ActivitySamplingResult.AllData,
};
ActivitySource.AddActivityListener(listener);

using (Activity? activity = source.StartActivity("PlaceOrder"))
{
    activity?.SetTag("order.id", 42);
    activity?.AddLink(new ActivityLink(new ActivityContext(ActivityTraceId.CreateRandom(), ActivitySpanId.CreateRandom(), ActivityTraceFlags.None))); // .NET 9
    Console.WriteLine(activity?.Links.Count());   // 1
}
```

- `StartActivity` returns `null` when nobody listens — always use `?.`.
- Keep metric tag cardinality low (no user IDs, no raw URLs).
- `Gauge<T>` records non-additive current values; `UpDownCounter<T>` for values that go up and down by deltas; `ObservableGauge<T>` for values pulled on collection.
- Use OpenTelemetry (`OpenTelemetry.Extensions.Hosting`) for export; .NET's `System.Diagnostics` APIs are the instrumentation layer.
- `[DisableHttpMetrics]` / `.DisableHttpMetrics()` (ASP.NET Core 9) to exclude health-check endpoints from request metrics.

***

#### HttpClient

```csharp
using System.Net.Http.Json;
using Microsoft.Extensions.Http.Resilience;

var builder = WebApplication.CreateBuilder(args);
builder.Services.AddHttpClient<GitHubClient>(client =>
{
    client.BaseAddress = new Uri("https://api.github.com/");
    client.DefaultRequestHeaders.UserAgent.ParseAdd("my-app/1.0");
    client.Timeout = TimeSpan.FromSeconds(30);
})
.AddStandardResilienceHandler();          // Microsoft.Extensions.Http.Resilience: retry, circuit breaker, timeouts

var app = builder.Build();
app.MapGet("/repo/{owner}/{name}", (string owner, string name, GitHubClient gh, CancellationToken ct) =>
    gh.GetRepoAsync(owner, name, ct));
app.Run();

public sealed class GitHubClient(HttpClient http)
{
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

Verified behaviours (.NET 9.0.21):

- `HttpClient.Timeout` expiry throws `TaskCanceledException` whose `InnerException` is `TimeoutException`; cancellation through the caller's token throws `TaskCanceledException` without a `TimeoutException`. Distinguish them with the inner exception or with `ct.IsCancellationRequested`.
- `GetAsync`/`SendAsync` do not throw on 4xx/5xx; `GetStringAsync`/`GetFromJsonAsync`/`EnsureSuccessStatusCode` throw `HttpRequestException` with `StatusCode` set.
- `AddStandardResilienceHandler()` retried two `503` responses to a `GET` and returned the third (successful) response.

Review rules:

- **Never `new HttpClient()` per request** (socket exhaustion) and never a static client without `PooledConnectionLifetime` (stale DNS). Use `IHttpClientFactory` (typed/named clients) or a singleton `SocketsHttpHandler { PooledConnectionLifetime = TimeSpan.FromMinutes(2) }`. In .NET 9 the factory's default primary handler is `SocketsHttpHandler`, so casting the primary handler to `HttpClientHandler` in `ConfigurePrimaryHttpMessageHandler` callbacks fails.
- Do not capture typed clients in singletons — the factory rotates handlers; a captured `HttpClient` never sees DNS changes.
- Escape user input in URLs (`Uri.EscapeDataString`) or build them with a query builder; never concatenate raw input.
- Dispose `HttpResponseMessage`; use `HttpCompletionOption.ResponseHeadersRead` + streaming for large bodies.
- Do not disable TLS validation (`ServerCertificateCustomValidationCallback = (_, _, _, _) => true`, `DangerousAcceptAnyServerCertificateValidator`) outside tests.
- Retries only for idempotent requests. The standard handler retries every method by default — a `POST` answered with `503` was sent 4 times (1 + 3 retries) in a test; with `o.Retry.DisableForUnsafeHttpMethods()` it was sent once. Use that option when duplicate side effects are harmful.
- `HttpClientHandler.MaxResponseHeadersLength` is in **kilobytes** (`CA2262`).

***

#### System.Text.Json

##### Defaults that surprise (verified on .NET 9.0.21)

```csharp
using System.Text.Json;
using System.Text.Json.Serialization;

// 1. Nullable annotations are NOT enforced by default
var book = JsonSerializer.Deserialize<Book>("""{"Title":null}""")!;
Console.WriteLine(book.Title is null);                         // True, despite 'string Title'

// 2. Opt in (New in .NET 9)
var strict = new JsonSerializerOptions
{
    RespectNullableAnnotations = true,
    RespectRequiredConstructorParameters = true,
    UnmappedMemberHandling = JsonUnmappedMemberHandling.Disallow,
};
try { JsonSerializer.Deserialize<Book>("""{"Title":null}""", strict); }
catch (JsonException e) { Console.WriteLine(e.Message[..60]); }

// 3. Missing constructor parameters silently become default
var person = JsonSerializer.Deserialize<Person>("{}")!;
Console.WriteLine($"{person.Name is null} {person.Age}");      // True 0
try { JsonSerializer.Deserialize<Person>("{}", strict); }
catch (JsonException) { Console.WriteLine("missing required constructor parameter"); }

// 4. Default options are case-sensitive and do not read numbers from strings
var web = JsonSerializer.Deserialize<Person>("""{"name":"a","age":"5"}""", JsonSerializerOptions.Web)!; // New in .NET 9
Console.WriteLine(web);                                          // Person { Name = a, Age = 5 }

// 5. Enum member names (New in .NET 9)
Console.WriteLine(JsonSerializer.Serialize(Color.DarkRed));     // "dark-red"

public class Book { public string Title { get; set; } = ""; }
public record Person(string Name, int Age);

[JsonConverter(typeof(JsonStringEnumConverter<Color>))]
public enum Color { [JsonStringEnumMemberName("dark-red")] DarkRed }
```

- `JsonSerializerOptions.Web` (camelCase, case-insensitive, numbers allowed as strings) is the cached singleton for ASP.NET Core-style defaults — use it instead of `new JsonSerializerOptions(JsonSerializerDefaults.Web)` per call.
- **Cache `JsonSerializerOptions` instances.** Creating a new instance per call discards the metadata cache (large CPU and allocation cost). Instances become immutable after first use; mutating them later throws `InvalidOperationException`.
- **`RespectNullableAnnotations`/`RespectRequiredConstructorParameters`** are off by default for compatibility; enable them for new APIs (or validate DTOs explicitly). `required` C# members *are* enforced by default.
- **Polymorphism**: use `[JsonPolymorphic]` + `[JsonDerivedType]` (allow-list). Never deserialize into `object` with custom converters that resolve type names from the payload, and never use `TypeNameHandling.All`-style settings in Newtonsoft.Json on untrusted input.
- **Untrusted input limits**: `MaxDepth` defaults to 64; for request bodies also bound the size (Kestrel `MaxRequestBodySize`).
- `JsonDocument` is `IDisposable` (pooled buffers) — dispose it, or use `JsonNode`/`JsonElement.Clone()` for data that outlives the document.
- Other .NET 9 additions: `IndentCharacter`/`IndentSize`, `JsonSchemaExporter.GetJsonSchemaAsNode(options, typeof(T))`, ordered `JsonObject` APIs (`IndexOf`, `Insert`), and `JsonReaderOptions.AllowMultipleValues` for whitespace-separated documents.
- .NET 9 behaviour changes: nullable `JsonDocument` properties deserialize JSON `null` to a `JsonDocument` whose `RootElement.ValueKind` is `Null` (previously the property was `null`); metadata property names such as `$type` are now unescaped before comparison.

##### Source generation (required for Native AOT and trimming)

```csharp
using System.Text.Json;
using System.Text.Json.Serialization;

var json = JsonSerializer.Serialize(new Person("Ada", 36), AppJsonContext.Default.Person);
Console.WriteLine(json);                                                    // {"name":"Ada","age":36}
Console.WriteLine(JsonSerializer.Deserialize(json, AppJsonContext.Default.Person));

public record Person(string Name, int Age);

[JsonSourceGenerationOptions(JsonSerializerDefaults.Web)]
[JsonSerializable(typeof(Person))]
internal partial class AppJsonContext : JsonSerializerContext;
```

With `<IsAotCompatible>true</IsAotCompatible>` (or `PublishAot`/`PublishTrimmed`), the reflection-based `JsonSerializer.Serialize<T>(value)` produces `IL2026` and `IL3050` warnings; the `JsonTypeInfo` overloads above do not.

***
#### ASP.NET Core 9 Essentials

##### Minimal API shape

```csharp
using Microsoft.AspNetCore.Http.HttpResults;

var builder = WebApplication.CreateBuilder(args);
builder.Services.AddProblemDetails();
builder.Services.AddOpenApi();                           // New in .NET 9: Microsoft.AspNetCore.OpenApi
builder.Services.AddSingleton<ProductStore>();

var app = builder.Build();
app.UseExceptionHandler(new ExceptionHandlerOptions     // StatusCodeSelector is New in .NET 9
{
    StatusCodeSelector = ex => ex is TimeoutException
        ? StatusCodes.Status503ServiceUnavailable
        : StatusCodes.Status500InternalServerError,
});
app.MapOpenApi();                                        // GET /openapi/v1.json (OpenAPI 3.0 document)

var products = app.MapGroup("/products").ProducesProblem(StatusCodes.Status500InternalServerError); // on groups: New in .NET 9

products.MapGet("/{id:int}", Results<Ok<Product>, NotFound> (int id, ProductStore store) =>
    store.Find(id) is { } p ? TypedResults.Ok(p) : TypedResults.NotFound());

products.MapPost("/", Results<Created<Product>, ValidationProblem> (CreateProduct body, ProductStore store) =>
{
    if (string.IsNullOrWhiteSpace(body.Name))
        return TypedResults.ValidationProblem(new Dictionary<string, string[]> { ["name"] = ["Name is required."] });
    var created = store.Add(body.Name, body.Price);
    return TypedResults.Created($"/products/{created.Id}", created);
});

app.MapGet("/fail", () => TypedResults.InternalServerError("unexpected"));   // New in .NET 9
app.Run();

public sealed record Product(int Id, string Name, decimal Price);
public sealed record CreateProduct(string Name, decimal Price);

public sealed class ProductStore
{
    private readonly Lock _gate = new();
    private readonly List<Product> _items = [];
    public Product? Find(int id) { lock (_gate) return _items.Find(p => p.Id == id); }
    public Product Add(string name, decimal price)
    {
        lock (_gate)
        {
            var p = new Product(_items.Count + 1, name, price);
            _items.Add(p);
            return p;
        }
    }
}
```

Verified with an in-process Kestrel run on ASP.NET Core 9.0.21: `GET /openapi/v1.json` returned `"openapi": "3.0.4"`; an endpoint throwing `TimeoutException` returned `503 application/problem+json`; `TypedResults.InternalServerError("…")` returned `500` with a JSON string body.

Review rules:

- Return `TypedResults` with a `Results<…>` union so the OpenAPI document and tests see the real response types.
- **Minimal APIs do not validate DataAnnotations on .NET 9** (`AddValidation()` is .NET 10). Validate request DTOs explicitly (manual checks, FluentValidation, or an endpoint filter) and return `ValidationProblem`. MVC controllers with `[ApiController]` still validate automatically.
- `app.UseExceptionHandler()` + `AddProblemDetails()` in production; never `UseDeveloperExceptionPage()` outside Development (it leaks stack traces and configuration).
- Middleware order: `UseForwardedHeaders` → `UseExceptionHandler` → `UseHsts`/`UseHttpsRedirection` → `UseRouting` (implicit) → `UseCors` → `UseAuthentication` → `UseAuthorization` → `UseRateLimiter` → endpoints. With `UseAuthorization()` before `UseAuthentication()` the policy is evaluated before the user is authenticated: in a test, a request with valid credentials got `401` instead of `200`. (Endpoints without `RequireAuthorization`/`[Authorize]` and without a fallback policy are anonymous regardless of order.)
- `MapStaticAssets()` (New in .NET 9) replaces `UseStaticFiles()` for assets known at build time: it adds build-time compression and fingerprinted ETags. Files added at runtime still need `UseStaticFiles()`.
- Swashbuckle is no longer part of the template; `Microsoft.AspNetCore.OpenApi` 9 generates OpenAPI **3.0** documents (3.1 arrives with .NET 10). Do not expose `MapOpenApi()` publicly in production unless intended — guard it with `if (app.Environment.IsDevelopment())` or `RequireAuthorization()`.

##### Security-relevant middleware

```csharp
using System.Net;
using Microsoft.AspNetCore.HttpOverrides;
using Microsoft.AspNetCore.RateLimiting;

var builder = WebApplication.CreateBuilder(args);
builder.Services.Configure<ForwardedHeadersOptions>(o =>
{
    o.ForwardedHeaders = ForwardedHeaders.XForwardedFor | ForwardedHeaders.XForwardedProto;
    o.KnownProxies.Add(IPAddress.Parse("10.0.0.10"));          // only trust your proxy
});
builder.Services.AddCors(o => o.AddPolicy("spa", p => p
    .WithOrigins("https://app.example.com")                    // explicit origins when credentials are allowed
    .AllowCredentials()
    .WithMethods("GET", "POST")
    .WithHeaders("Content-Type")));
builder.Services.AddRateLimiter(o =>
{
    o.RejectionStatusCode = StatusCodes.Status429TooManyRequests;
    o.AddFixedWindowLimiter("api", w => { w.PermitLimit = 100; w.Window = TimeSpan.FromMinutes(1); });
});
builder.Services.AddAuthentication().AddJwtBearer();
builder.Services.AddAuthorizationBuilder()
    .SetFallbackPolicy(new Microsoft.AspNetCore.Authorization.AuthorizationPolicyBuilder().RequireAuthenticatedUser().Build());

var app = builder.Build();
app.UseForwardedHeaders();
app.UseHttpsRedirection();
app.UseCors("spa");
app.UseAuthentication();
app.UseAuthorization();
app.UseRateLimiter();
app.MapGet("/health", () => "ok").AllowAnonymous();
app.MapGet("/me", (System.Security.Claims.ClaimsPrincipal user) => user.Identity?.Name).RequireRateLimiting("api");
app.Run();
```

- **Forwarded headers**: since ASP.NET Core 9.0.6 (and 8.0.17) `X-Forwarded-*` headers from proxies not listed in `KnownProxies`/`KnownNetworks` are ignored. A deployment behind a TLS-terminating proxy that is not configured sees `http` requests (redirect loops, wrong `Request.IsHttps`). The fix is to list the proxy, not to clear `KnownProxies`/`KnownNetworks` or set `ASPNETCORE_FORWARDEDHEADERS_ENABLED=true` without network isolation.
- **CORS**: `AllowAnyOrigin()` together with `AllowCredentials()` is rejected by ASP.NET Core with `InvalidOperationException` ("The CORS protocol does not allow specifying a wildcard (any) origin and credentials at the same time"). `SetIsOriginAllowed(_ => true).AllowCredentials()` compiles and works — and is a vulnerability (any site can make credentialed requests). Flag it.
- **Authorization**: a fallback policy that requires authenticated users makes endpoints secure by default; anonymous endpoints opt out explicitly. Analyzer `ASP0026` (New in .NET 9) warns when an `[Authorize]` is overridden by an `[AllowAnonymous]` "farther away" (e.g. on the controller class).
- **Antiforgery**: minimal API endpoints that bind forms (`[FromForm]`, `IFormFile`) validate antiforgery tokens when antiforgery services are registered (.NET 8+); `DisableAntiforgery()` on cookie-authenticated form endpoints enables CSRF.
- **Data Protection**: in multi-instance deployments persist keys (`PersistKeysToFileSystem`/Azure/Redis) and protect them at rest; otherwise cookies and antiforgery tokens break on scale-out or restart.
- **Kestrel limits**: keep defaults or tighten `MaxRequestBodySize` (default 30,000,000 bytes, about 28.6 MiB) and request header limits; raising them or setting `null` (unlimited) needs a reason.

##### Caching — `HybridCache` (GA since Microsoft.Extensions.Caching.Hybrid 9.3.0)

```csharp
using Microsoft.Extensions.Caching.Hybrid;

var builder = WebApplication.CreateBuilder(args);
builder.Services.AddHybridCache(o =>
{
    o.DefaultEntryOptions = new HybridCacheEntryOptions { Expiration = TimeSpan.FromMinutes(5), LocalCacheExpiration = TimeSpan.FromMinutes(1) };
});
var app = builder.Build();

app.MapGet("/weather/{city}", async (string city, HybridCache cache, CancellationToken ct) =>
    await cache.GetOrCreateAsync(
        $"weather:{city.ToLowerInvariant()}",
        async token => { await Task.Delay(10, token); return new Forecast(city, 21); },   // factory runs once per key (stampede protection)
        cancellationToken: ct));
app.Run();

public sealed record Forecast(string City, int TemperatureC);
```

- `HybridCache` combines an in-process cache with an optional `IDistributedCache` (Redis, SQL Server) and coalesces concurrent misses for the same key. It was in preview at .NET 9 GA; the first stable package is 9.3.0 (March 2025). The package ships `net9.0`, `net8.0`, `netstandard2.0`/`2.1` and `net462` builds, so it also works in libraries that target older frameworks.
- Cached objects are deserialized per call by default (safe against mutation). Only mark types immutable (`[ImmutableObject(true)]` + `sealed`) when they truly are, to enable instance reuse.
- Cache keys built from user input must be normalized and bounded in length; include tenant/user identity in keys of per-user data.
- `IMemoryCache` without `SizeLimit` and per-entry `Size` is unbounded memory.

***

#### EF Core 9

##### Breaking changes to check during upgrade or review

- **Pending model changes now throw.** `Migrate()`/`MigrateAsync()`/`dotnet ef database update` fail with `PendingModelChangesWarning` as an error when the model differs from the last migration's snapshot — commonly caused by non-deterministic `HasData` (`DateTime.Now`, `Guid.NewGuid()`) or by forgetting to add a migration. Fix the model; suppress (`ConfigureWarnings(w => w.Ignore(RelationalEventId.PendingModelChangesWarning))`) only for the documented false-positive cases.
- **Migrations inside a user transaction now throw** (`MigrationsUserTransactionWarning`). EF 9 starts its own transaction and takes a migration lock to protect against concurrent application; remove the `BeginTransaction`/`CreateExecutionStrategy` wrapper around `MigrateAsync`.
- **`Microsoft.EntityFrameworkCore.Design` not found** by `dotnet ef` with SDK 9.0.200+ when the reference is `PrivateAssets=all` — add `<Publish>true</Publish>` to the `PackageReference` as the documented workaround.
- **All pending migrations run in one transaction** (part of migration locking): if any migration fails, all of them roll back. EF 10 reverts to one transaction per migration; on EF 9 do not rely on earlier migrations of a batch having been committed.
- **EF tools no longer support .NET Framework startup projects**; `EF.Constant()`/`EF.Parameter()` inside compiled queries throw `InvalidCastException`; `AsNoTrackingWithIdentityResolution` is rejected for some queries over JSON collections. The Azure Cosmos DB provider has many high-impact changes (discriminator renamed to `$type`, `id` mapping, sync I/O blocked) — read the list before upgrading Cosmos apps.
- `EF.Functions.Unhex()` returns `byte[]?`; `ToString()` on a null value translates to `''`; compiled models reference value-converter methods directly, so those methods must not be `private`; SQL Server `IndexOf` propagates `NULL`.
- EF Core 9 targets `net8.0`; it is supported until November 10, 2026 like .NET 9. EF Core 10 requires `net10.0`.

##### Query and tracking pitfalls

```csharp
using Microsoft.Data.Sqlite;
using Microsoft.EntityFrameworkCore;

await using var connection = new SqliteConnection("DataSource=:memory:");
await connection.OpenAsync();
var options = new DbContextOptionsBuilder<ShopDb>()
    .UseSqlite(connection)
    .UseAsyncSeeding(async (context, _, ct) =>              // New in EF Core 9: runs on EnsureCreated/Migrate
    {
        var set = context.Set<Product>();
        if (!await set.AnyAsync(ct))
        {
            set.AddRange(new Product { Name = "a", Price = 1 }, new Product { Name = "b", Price = 2 });
            await context.SaveChangesAsync(ct);
        }
    })
    .UseSeeding((context, _) => { /* synchronous counterpart, used by EnsureCreated() */ })
    .Options;

await using var db = new ShopDb(options);
await db.Database.EnsureCreatedAsync();

// Read-only queries: AsNoTracking + projection
var names = await db.Products.AsNoTracking().OrderBy(p => p.Id).Select(p => p.Name).ToListAsync();
var set = await db.Products.Select(p => p.Name).ToHashSetAsync();     // New in EF Core 9
Console.WriteLine($"{string.Join(",", names)} {set.Count}");

// SQL injection: FromSql/FromSqlInterpolated parameterize; FromSqlRaw + concatenation does not
string term = "x' OR 1=1 --";
int safe = await db.Products.FromSql($"SELECT * FROM Products WHERE Name = {term}").CountAsync();
Console.WriteLine(safe);                                             // 0
// BAD: db.Products.FromSqlRaw("SELECT * FROM Products WHERE Name = '" + term + "'")

// Set-based updates without loading entities
int updated = await db.Products.ExecuteUpdateAsync(s => s.SetProperty(p => p.Price, p => p.Price * 2));
Console.WriteLine(updated);                                          // 2

// Untranslatable predicates throw instead of silently running on the client
try { await db.Products.Where(p => Helpers.IsSpecial(p.Name)).ToListAsync(); }
catch (InvalidOperationException) { Console.WriteLine("could not be translated"); }

// Changes to AsNoTracking entities are not saved
var detached = await db.Products.AsNoTracking().FirstAsync();
detached.Name = "lost";
Console.WriteLine(await db.SaveChangesAsync());                      // 0

public static class Helpers { public static bool IsSpecial(string s) => s.EndsWith('7'); }
public sealed class Product { public int Id { get; set; } public required string Name { get; set; } public decimal Price { get; set; } }
public sealed class ShopDb(DbContextOptions<ShopDb> options) : DbContext(options)
{
    public DbSet<Product> Products => Set<Product>();
    protected override void OnModelCreating(ModelBuilder b) => b.Entity<Product>().Property(p => p.Price).HasConversion<double>();
}
```

Review rules:

- **`DbContext` is not thread-safe** and is scoped per request. Never share one across concurrent tasks (`Task.WhenAll(db.A.ToListAsync(), db.B.ToListAsync())`), never register it as a singleton, never capture it in a background task that outlives the request. Use `IDbContextFactory<T>` for parallel work or background services.
- **N+1 queries**: lazy loading or per-item queries inside loops. Use projection (`Select`), `Include`, or `AsSplitQuery()` for multiple collection includes (avoids cartesian explosion).
- **`FromSqlRaw`/`ExecuteSqlRaw` with string concatenation or interpolation into the SQL text is SQL injection.** `FromSql`/`ExecuteSql` (and the `*Interpolated` variants) turn interpolation holes into parameters. `FromSqlRaw` with an interpolated `$"…"` string compiles and is vulnerable.
- **`ExecuteUpdate`/`ExecuteDelete`** bypass the change tracker, interceptors on `SaveChanges`, and concurrency tokens unless included in the predicate; tracked entities in the same context become stale.
- **`AsNoTracking()`** for read-only queries; `AsNoTrackingWithIdentityResolution()` when the graph has duplicates.
- **Pagination**: always `OrderBy` before `Skip`/`Take`; prefer keyset pagination for deep pages.
- **`Find` vs `FirstOrDefault`**: `FindAsync` checks the change tracker first; both are fine, but do not use `SingleOrDefault` on non-unique columns.
- **Migrations in production**: apply from a deployment step or a migration bundle, not from every app instance's startup. EF 9's migration lock prevents corruption but still serializes startup.
- **Primitive collections** (`ids.Contains(p.Id)` with a `List<int>`) are sent as a single JSON parameter since EF 8; `EF.Constant()` (EF 8.0.2), and in EF 9 `EF.Parameter()` and the `TranslateParameterizedCollectionsToConstants()` context option, control it — use them only after measuring query plans. In EF 9, `EF.Constant`/`EF.Parameter` inside `EF.CompileQuery`/`EF.CompileAsyncQuery` throw `InvalidCastException` at runtime.
- **Precompiled queries and Native AOT** in EF Core 9 are explicitly experimental — not for production.
- `HasData` seeding must be deterministic; use `UseSeeding`/`UseAsyncSeeding` (EF 9) for data that depends on runtime values.

***

#### Security

##### Serialization and deserialization

- **`BinaryFormatter` is gone in .NET 9**: every call throws `PlatformNotSupportedException`, and using the type is a compile-time error (`SYSLIB0011`) unless suppressed. Do not suppress it and do not add the unsupported `System.Runtime.Serialization.Formatters` compatibility package to "fix" the build — migrate to `System.Text.Json`, `DataContractSerializer` with known types, MessagePack/protobuf with explicit contracts. Existing persisted `BinaryFormatter` payloads can be parsed without instantiating arbitrary types with `NrbfDecoder` from the `System.Formats.Nrbf` NuGet package (9.0.x; not part of the shared framework, and marked experimental — using it reports `SYSLIB5005` as an error until suppressed).
- **`TypeNameHandling`** other than `None` in Newtonsoft.Json on untrusted data is remote code execution; for type names from untrusted input use an allow-list (the .NET 9 `System.Reflection.Metadata.TypeName.TryParse` helps parse them safely).
- **XML**: `XmlReader` with default settings is safe in .NET Core (`DtdProcessing.Prohibit`); flag `DtdProcessing.Parse` with an `XmlUrlResolver`.

##### Certificates and cryptography

```csharp
using System.Security.Cryptography;
using System.Security.Cryptography.X509Certificates;

using var rsa = RSA.Create(2048);
var request = new CertificateRequest("CN=test", rsa, HashAlgorithmName.SHA256, RSASignaturePadding.Pkcs1);
using var cert = request.CreateSelfSigned(DateTimeOffset.UtcNow, DateTimeOffset.UtcNow.AddDays(1));
byte[] der = cert.Export(X509ContentType.Cert);
byte[] pfx = cert.Export(X509ContentType.Pfx, "pw");

using var publicOnly = X509CertificateLoader.LoadCertificate(der);            // New in .NET 9
using var withKey = X509CertificateLoader.LoadPkcs12(pfx, "pw");             // New in .NET 9
// BAD (SYSLIB0057): new X509Certificate2(der), new X509Certificate2(pfx, "pw")
Console.WriteLine($"{publicOnly.Subject} {withKey.HasPrivateKey}");

// Password hashing: PBKDF2 one-shot with SHA-256 and a high iteration count
byte[] salt = RandomNumberGenerator.GetBytes(16);
byte[] hash = Rfc2898DeriveBytes.Pbkdf2("correct horse"u8, salt, 600_000, HashAlgorithmName.SHA256, 32);
byte[] again = Rfc2898DeriveBytes.Pbkdf2("correct horse"u8, salt, 600_000, HashAlgorithmName.SHA256, 32);
Console.WriteLine(CryptographicOperations.FixedTimeEquals(hash, again));   // constant-time comparison

// Algorithm chosen at runtime (New in .NET 9)
Console.WriteLine(Convert.ToHexStringLower(CryptographicOperations.HashData(HashAlgorithmName.SHA256, "abc"u8)));

// Tokens and identifiers
Console.WriteLine(RandomNumberGenerator.GetHexString(32, lowercase: true));  // .NET 8
```

- `X509CertificateLoader` loads exactly one format per method (no content sniffing). The old constructors and `X509Certificate2Collection.Import` are obsolete (`SYSLIB0057`) because sniffing was a source of security issues.
- Use `RandomNumberGenerator` for anything security-relevant; `Random`/`Random.Shared` are predictable.
- Compare secrets (HMACs, tokens) with `CryptographicOperations.FixedTimeEquals`, never `==`/`SequenceEqual`.
- AES: `AesGcm` (with a unique 12-byte nonce per key and message) or `Aes` + HMAC; never ECB, never a constant IV. MD5/SHA-1 only for non-security checksums.
- `Rfc2898DeriveBytes.Pbkdf2(...)` static methods are the preferred API; the constructors without a hash algorithm are obsolete (`SYSLIB0041`, SHA-1 + 1000 iterations). ASP.NET Core Identity's `PasswordHasher<T>` is a good default for user passwords.
- KMAC (`Kmac128`, `Kmac256`, `KmacXof128`, `KmacXof256`) is new in .NET 9 but platform-dependent (OpenSSL 3.0+, recent Windows 11) — check `IsSupported`.

##### Injection, paths and processes

```csharp
string root = Path.GetFullPath("/srv/files/");          // trailing separator matters
foreach (string input in new[] { "a.txt", "../etc/passwd", "/etc/passwd", "../files-evil/x" })
{
    string full = Path.GetFullPath(Path.Combine(root, input));
    bool naive = full.StartsWith(Path.GetFullPath("/srv/files"), StringComparison.Ordinal);   // BAD
    bool safe = full.StartsWith(root, StringComparison.Ordinal);                              // GOOD
    Console.WriteLine($"{input,-16} -> {full,-22} naive={naive} safe={safe}");
}
// ../files-evil/x -> /srv/files-evil/x naive=True safe=False
// /etc/passwd     -> /etc/passwd  (Path.Combine discards root when the input is absolute)
```

- **Path traversal**: `Path.Combine(root, userInput)` returns `userInput` when it is absolute. Canonicalize with `Path.GetFullPath` and check the prefix *including the trailing separator* (on Windows use `OrdinalIgnoreCase`). Reject file names with `Path.GetFileName(input) != input` when only a name is expected.
- **SQL**: parameters only (ADO.NET `DbParameter`, Dapper anonymous objects, EF `FromSql`). Never interpolate into SQL text.
- **Processes**: `ProcessStartInfo` with `ArgumentList` (each argument separate) and `UseShellExecute = false`; never build a shell command line from user input.
- **Open redirects**: validate return URLs with `Url.IsLocalUrl` / `LocalRedirect`.
- **ReDoS**: regex on untrusted input needs a timeout or `RegexOptions.NonBacktracking`.
- **Secrets**: no secrets in source, `appsettings*.json` or logs; `dotnet user-secrets` in development, environment variables/vaults in production.

##### Named mutexes on Unix — changed in 9.0.21

Starting with .NET 9.0.21 (and 8.0.32, 10.0.13), named `Mutex` objects on Unix are **user-scoped by default** instead of global; backing files move from `/tmp` to `$TMPDIR` where that is per-user (macOS), otherwise `$XDG_RUNTIME_DIR`, otherwise a directory under the home directory from the user database (not `$HOME`; on 9.0.21 without `XDG_RUNTIME_DIR` it was `~/.dotnet/corefx/Runtime/SharedMemory/`). Cross-user coordination via named mutexes stops working, and processes running as a UID without a passwd entry (common with `docker run -u <uid>` or Kubernetes `runAsUser`) fail to create named mutexes unless `XDG_RUNTIME_DIR` points to a writable directory. The opt-out is the `System.Threading.Mutex.UnsafeDefaultMutexScopeGlobal` runtime configuration switch.

***

#### Performance

.NET 9 runtime improvements that apply without code changes: dynamic PGO now optimizes type checks/casts, loop optimizations (induction-variable widening, strength reduction, counting down), inlining of shared generics needing runtime lookups, stack allocation of boxes that do not escape, Arm64 SVE (experimental) and AVX10.1 intrinsics, a 2–4× faster exception-handling implementation, and Server GC with DATAS enabled by default.

Review rules:

- **Measure first** — BenchmarkDotNet for micro-benchmarks, `dotnet-counters`/`dotnet-trace` for services. Reject "optimizations" without numbers when they hurt readability.
- **Allocation hot spots**: LINQ in tight loops, closures capturing locals, boxing (`object`/non-generic interfaces on structs), `string` concatenation in loops, `params` arrays (C# 13 span overloads remove many), `ToList()` just to call `Count`.
- **Strings**: `StringBuilder` or `string.Create` for building; `AsSpan()` slicing instead of `Substring` for parsing; `SearchValues` for repeated `IndexOfAny`.
- **Collections**: pre-size `List<T>`/`Dictionary` when the count is known; `FrozenDictionary` for read-only lookups; `CollectionsMarshal.AsSpan(list)` only when the list is not modified meanwhile.
- **Async**: `ValueTask` only for hot paths that usually complete synchronously; avoid `async` wrappers that only `return await` a single call (but keep them when `using`/`try` needs the await).
- **DATAS**: Server GC on .NET 9 starts with one heap and adds heaps with load (`GC.GetConfigurationVariables()["GCDynamicAdaptationMode"]` is `1` on 9.0.21 and `0` on 8.0.32; on 9.0.21 the variable reads `1` under Workstation GC as well, but DATAS only takes effect with Server GC). Containers with memory limits benefit; latency-critical services with steady high allocation can opt out with `<GarbageCollectionAdaptationMode>0</GarbageCollectionAdaptationMode>` — only after measuring.
- **Exceptions on hot paths** remain expensive even after .NET 9's speed-up; use `Try*` patterns.
- **Regex**: `[GeneratedRegex]` or a `static readonly Regex` with `RegexOptions.Compiled`; never construct the same `Regex` per call.
- **JSON**: reuse `JsonSerializerOptions`; source generation for startup time and AOT.
- **EF Core**: projection + `AsNoTracking`, compiled queries (`EF.CompileAsyncQuery`) only for proven hot queries, `DbContext` pooling (`AddDbContextPool`) for high-throughput services.

***

#### Trimming and Native AOT

```xml
<PropertyGroup>
  <PublishAot>true</PublishAot>               <!-- implies trimming; console apps, minimal APIs, worker services -->
  <IsAotCompatible>true</IsAotCompatible>     <!-- for libraries: enables trim/AOT/single-file analyzers -->
  <InvariantGlobalization>true</InvariantGlobalization>
</PropertyGroup>
```

- Treat `IL2026` (RequiresUnreferencedCode), `IL2067`/`IL2070`-family (`DynamicallyAccessedMembers` mismatches) and `IL3050` (RequiresDynamicCode) warnings as errors in AOT-targeted projects; suppressing them hides runtime failures.
- Use source generators: `System.Text.Json` contexts, `[LoggerMessage]`, `[GeneratedRegex]`, configuration binding and options validation generators, `WebApplication.CreateSlimBuilder` for minimal APIs.
- .NET 9 adds `[FeatureSwitchDefinition("Name")]` (trimmer treats the property as a constant when the switch is set) and `[FeatureGuard(typeof(RequiresDynamicCodeAttribute))]` (calls to annotated APIs inside the guarded branch do not warn).
- Not supported in Native AOT on .NET 9: MVC controllers/Razor Pages, Blazor Server, `System.Reflection.Emit`, dynamic assembly loading; EF Core support is experimental. SignalR supports trimming and Native AOT since .NET 9 with limitations (JSON protocol with source-generated contexts only, no strongly typed hubs under `PublishAot`). `TypeDescriptor` trimming support needs `TypeDescriptor.RegisterType<T>()` + `*FromRegisteredType` APIs (New in .NET 9).
- Native AOT publish requires the platform native toolchain on the build machine (clang and the zlib development package on Linux, the MSVC C++ build tools on Windows).

***

#### Testing

```csharp
using System.Net;
using Microsoft.AspNetCore.Mvc.Testing;
using Microsoft.Extensions.DependencyInjection;
using Microsoft.Extensions.DependencyInjection.Extensions;
using Microsoft.Extensions.Time.Testing;
using Xunit;

public class TokenServiceTests
{
    [Theory]
    [InlineData(14, false)]
    [InlineData(15, false)]
    [InlineData(16, true)]
    public void IsExpired_UsesInjectedClock(int minutesLater, bool expected)
    {
        var issued = new DateTimeOffset(2026, 10, 22, 12, 0, 0, TimeSpan.Zero);
        var clock = new FakeTimeProvider(issued);                 // Microsoft.Extensions.TimeProvider.Testing
        var service = new TokenService(clock);
        clock.Advance(TimeSpan.FromMinutes(minutesLater));
        Assert.Equal(expected, service.IsExpired(issued));
    }
}

public class ApiTests(WebApplicationFactory<Program> factory) : IClassFixture<WebApplicationFactory<Program>>
{
    [Fact]
    public async Task ExpiredToken_Returns410()
    {
        var issued = new DateTimeOffset(2026, 10, 22, 12, 0, 0, TimeSpan.Zero);
        var clock = new FakeTimeProvider(issued.AddHours(1));
        using var client = factory.WithWebHostBuilder(b => b.ConfigureServices(s =>
            s.Replace(ServiceDescriptor.Singleton<TimeProvider>(clock)))).CreateClient();

        var response = await client.GetAsync($"/token/{issued.UtcTicks}");

        Assert.Equal(HttpStatusCode.Gone, response.StatusCode);
    }
}
```

The application under test (top-level `Program.cs`):

```csharp
var builder = WebApplication.CreateBuilder(args);
builder.Services.AddSingleton(TimeProvider.System);
builder.Services.AddSingleton<TokenService>();
var app = builder.Build();
app.MapGet("/token/{issuedTicks:long}", (long issuedTicks, TokenService tokens) =>
    tokens.IsExpired(new DateTimeOffset(issuedTicks, TimeSpan.Zero)) ? Results.StatusCode(410) : Results.Ok("valid"));
app.Run();

public sealed class TokenService(TimeProvider time)
{
    public static readonly TimeSpan Lifetime = TimeSpan.FromMinutes(15);
    public bool IsExpired(DateTimeOffset issuedAt) => time.GetUtcNow() - issuedAt > Lifetime;
}

public partial class Program;   // required on .NET 9 for WebApplicationFactory<Program> in another assembly
```

Verified with xUnit 2.9.3, Microsoft.AspNetCore.Mvc.Testing 9.0.21 and Microsoft.Extensions.TimeProvider.Testing 9.10.0: without `public partial class Program;` the test project fails with `CS0122: 'Program' is inaccessible due to its protection level`; with it, all four tests pass. (.NET 10 generates the public declaration automatically — do not remove the line while the app targets `net9.0`.)

Review rules:

- Name tests `Method_Scenario_ExpectedResult`; Arrange/Act/Assert; one behaviour per test; table-driven `[Theory]` for rule-heavy logic.
- No `Thread.Sleep`/real clocks/real network in unit tests: inject `TimeProvider`, `IHttpClientFactory`/`HttpMessageHandler` fakes.
- `async` tests return `Task`, never `async void`.
- Integration tests: `WebApplicationFactory<Program>`; replace infrastructure with `ConfigureTestServices`; real databases via Testcontainers rather than the EF in-memory provider (which does not enforce relational constraints or translate SQL).
- Do not assert on exception *messages* from the BCL or on log text; assert types, status codes, and structured data.
- `dotnet test` in .NET 9 runs test projects for multiple target frameworks in parallel (`TestTfmsInParallel=false` to opt out) — tests that share files, ports or databases across TFMs must be isolated.

***

#### Analyzers and Tooling

- **Defaults**: the .NET 9 SDK enables a small set of CA rules as warnings by default; in a plain `net9.0` project `CA2022` (inexact `Stream.Read`), `CA2264` (non-nullable value passed to `ThrowIfNull`) and `CA2265` (span compared to `null`/`default`) fire without configuration. `AnalysisLevel` `9.0-recommended` (or `latest-recommended`) additionally enabled `CA1871`, `CA1872` and `CA2262` in the same test file (verified on SDK 9.0.319). Recommend `<AnalysisLevel>9.0-recommended</AnalysisLevel>` + `TreatWarningsAsErrors`.
- **New .NET 9 rules**: CA1514 (redundant length argument), CA1515 (public types in executables), CA1871 (nullable struct to `ThrowIfNull`), CA1872 (`BitConverter.ToString` → `Convert.ToHexString`), CA2022 (inexact `Stream.Read`, on by default — reported for variables typed `Stream`, not for `MemoryStream`), CA2262 (`MaxResponseHeadersLength` is in KB), CA2263 (prefer generic overload), CA2264, CA2265.
- **Nullable warnings** (`CS86xx`) are part of the compiler, not of `AnalysisLevel` — `<Nullable>enable</Nullable>` plus `TreatWarningsAsErrors` makes them errors.
- **BuildChecks** (`dotnet build /check`) catch MSBuild script problems (e.g. `BC0101` shared output path, `BC0102` double writes).
- **Terminal Logger** is the default for `build`/`test`/`publish` in the .NET 9 SDK; CI logs that parse MSBuild output may need `--tl:off` or `MSBUILDTERMINALLOGGER=off`.
- **Vulnerable packages**: `dotnet list package --vulnerable --include-transitive`; NuGet audit warnings `NU1901`–`NU1904` (low…critical). Do not blanket-suppress them; use `NuGetAuditSuppress` items for analysed advisories.
- **Formatting**: `dotnet format` with `.editorconfig`; `dotnet format --verify-no-changes` in CI.

***

#### Removed / Changed in .NET 9 — Migration Notes

From .NET 8 / C# 12 to .NET 9 / C# 13:

| Area | Change | Action |
|---|---|---|
| C# | `params` span overloads chosen over arrays | Fix expression-tree call sites (`CS8640`/`CS9226`) with explicit arrays |
| C# | Collection-expression overload resolution prefers exact element type | Re-check overloads taking `Span<int>` vs `ReadOnlySpan<byte>` etc.; ambiguous `M([])` now errors |
| C# | Iterators are safe contexts | Add `unsafe` to local functions in iterators that use pointers |
| Serialization | `BinaryFormatter` always throws | Migrate data formats; plan for existing persisted binary blobs (the experimental `System.Formats.Nrbf` package can parse them) |
| JSON | `JsonDocument?` properties get a `Null`-kind document; metadata names unescaped | Check `null` handling and `$type`/`$id` with escaped names |
| Crypto | `X509Certificate2` loading ctors obsolete (`SYSLIB0057`) | `X509CertificateLoader` |
| JIT | Floating-point → integer casts saturate (x86/x64) | Validate ranges; update tests asserting `int.MinValue` sentinels |
| BCL | New integer `TimeSpan.From*` overloads; `FromMilliseconds(int/long)` inside expression trees → `CS0854` | Cast to `double` or hoist out of EF/LINQ-provider lambdas |
| BCL | `BinaryReader.ReadString()` yields U+FFFD for malformed data; `String.Trim(ReadOnlySpan<char>)` preview overloads removed | Validate input explicitly; recompile anything built against .NET 9 previews |
| Runtime | Empty environment variables are kept (`SetEnvironmentVariable("X", "")` no longer deletes `X` on Unix — verified: `""` on 9.0.21, `null` on 8.0.32) | Use `null` to delete; treat `""` as set |
| Diagnostics | `Debug.Assert` messages include the condition text | Tests matching assert output change |
| DI | `[FromKeyedServices]` no longer falls back to non-keyed services | Register keyed services explicitly |
| Networking | `IHttpClientFactory` uses `SocketsHttpHandler`; header values and query strings redacted in logs | Update `ConfigurePrimaryHttpMessageHandler` casts; opt in to logging specific headers deliberately |
| Inline arrays | Default `Equals`/`GetHashCode` throw `NotSupportedException` | Implement equality explicitly |
| ZIP | `ZipArchiveEntry` names/comments respect the UTF-8 flag | Re-test archives with non-ASCII names |
| Containers | .NET 9 images no longer install `zlib` (the runtime bundles zlib-ng) | Add `zlib` explicitly if native code needs it |
| SDK | Terminal Logger default; `dotnet test` parallel across TFMs; `NETSDK1138` for `net7.0` executables, `NETSDK1215` for `netstandard1.x` | Adjust CI parsing; retarget |
| ASP.NET Core | Dev-cert export, `ValidateOnBuild`/`ValidateScopes` in Development for `HostBuilder`, middleware with multiple constructors, forwarded headers from unknown proxies ignored (9.0.6) | Review host setup and proxy configuration |
| EF Core 9 | Pending model changes / user transactions throw on migrate | See EF Core section |
| 9.0.21 | Named mutexes user-scoped on Unix | See Security section |

##### Moving on to .NET 10 (recommended before November 10, 2026)

- Change `TargetFramework` to `net10.0` (C# 14 by default), update `global.json` to a 10.0.1xx/4xx SDK, and update all `Microsoft.*` packages to 10.0.x.
- Review the .NET 10 breaking changes, especially: the `field` keyword now binds to a synthesized backing field (members named `field` change meaning), first-class span conversions can change overload resolution (some calls now bind to span-based `MemoryExtensions` methods; expression trees may fail), `System.Linq.Async` conflicts with the built-in `System.Linq.AsyncEnumerable`, and `NuGetAuditMode` defaults to `all`.
- C# 14 features (extension members, null-conditional assignment, `field`) become available — do not introduce them while the project still builds for `net9.0`.

***

#### Production / Review Checklist

##### Version and configuration

- [ ] `TargetFramework` is `net9.0` (or multi-targets with guarded .NET 9-only APIs); no `LangVersion` `preview`/`latest`/`14`
- [ ] `global.json` pins a 9.0.3xx (or 9.0.1xx) SDK with `rollForward: latestPatch`; runtime and `Microsoft.*` packages on 9.0.21
- [ ] Upgrade to .NET 10 LTS is planned — .NET 9 support ends November 10, 2026
- [ ] `Nullable` enabled; `TreatWarningsAsErrors`; `AnalysisLevel` set; `NuGetAuditMode=all`
- [ ] No C# 14 / .NET 10 APIs suggested or used (extension blocks, `field`, `?.` assignment, `LeftJoin`, `Shuffle`, `AsyncEnumerable`, `AddValidation`, `KnownIPNetworks`, `JsonSerializerOptions.Strict`)

##### C# 13 features

- [ ] `params ReadOnlySpan<T>` for new read-only variadic APIs; no span `params` captured, stored or used across `await`
- [ ] Expression-tree call sites compile against the new span overloads (explicit arrays where needed); no `TimeSpan.FromMilliseconds(int/long)` inside EF/LINQ-provider lambdas (`CS0854`)
- [ ] `Lock` fields typed as `Lock` (no conversion to `object`, no `CS9216`); no `await` in locks; `SemaphoreSlim` for async
- [ ] `\e` instead of `\x1b`; no members named `field`
- [ ] `[OverloadResolutionPriority]` only in libraries, with a documented reason
- [ ] `allows ref struct`/`ref struct` interfaces used only where spans must flow through generics

##### Correctness

- [ ] No `.Result`/`.Wait()`/`async void`; `CancellationToken` passed through; `OperationCanceledException` not logged as an error
- [ ] Bounded concurrency for fan-out (`Parallel.ForEachAsync`, `SemaphoreSlim`, bounded `Channel`)
- [ ] `IDisposable`/`IAsyncDisposable` disposed by their owner; `CancellationTokenSource`s disposed
- [ ] No multiple enumeration of lazy sequences; `Single` vs `First` chosen deliberately; comparers specified for string keys
- [ ] Records with collection members do not rely on value equality; `with` treated as shallow
- [ ] Float → int casts validated (saturation in .NET 9); `checked` arithmetic for sizes/money
- [ ] `Guid.CreateVersion7()` not treated as strictly monotonic; `TimeProvider` injected for time logic
- [ ] `throw;` not `throw ex;`; exceptions wrapped with context

##### ASP.NET Core / EF Core

- [ ] DI lifetimes correct (no scoped in singleton); no `BuildServiceProvider()`; keyed services registered as keyed
- [ ] Options validated with `ValidateOnStart()`; no secrets in config files or logs
- [ ] Minimal API input validated explicitly (no automatic DataAnnotations validation on .NET 9)
- [ ] Exception handler + ProblemDetails; no developer exception page in production; OpenAPI endpoint not public unless intended
- [ ] Forwarded headers configured with `KnownProxies`/`KnownNetworks`; CORS has explicit origins with credentials; fallback authorization policy; rate limiting on public endpoints
- [ ] `HttpClient` via `IHttpClientFactory`; timeouts; resilience only for idempotent calls; TLS validation not disabled
- [ ] `DbContext` scoped, never shared across threads; `AsNoTracking` + projection for reads; no N+1
- [ ] Only `FromSql`/`ExecuteSql` with interpolation — no `FromSqlRaw` concatenation
- [ ] Migrations applied by a deployment step; no explicit transaction around `MigrateAsync`; deterministic `HasData`

##### Security

- [ ] No `BinaryFormatter`/`TypeNameHandling`/type names from untrusted input
- [ ] `X509CertificateLoader` for certificates; `RandomNumberGenerator` for secrets; `FixedTimeEquals` for comparisons; PBKDF2-SHA256 (or Identity's hasher) for passwords
- [ ] Path traversal checked with `GetFullPath` + separator-terminated prefix; no shell command lines from input
- [ ] Regex on untrusted input has a timeout or `NonBacktracking`
- [ ] Named mutex usage on Unix reviewed for 9.0.21's user scope

##### Performance and deployment

- [ ] Hot paths measured; `JsonSerializerOptions` cached; `[GeneratedRegex]`; no per-call `HttpClient`
- [ ] Trim/AOT warnings (`IL2026`, `IL3050`) fixed, not suppressed, in AOT/trimmed apps; source-generated JSON
- [ ] Container images on a supported 9.0.21 tag (or migrated to 10.0); non-root user; `zlib` added if native code needs it
