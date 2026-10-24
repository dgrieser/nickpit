### Citations: C# 15 on .NET 11: The Complete Production Developer Guide

Sources for `csharp-15.md`. The guide carries no footnote markers; each entry lists where it was cited (section path, block, and the words just before the citation). Not sent to the model; kept for maintainers.

[^1]: .NET 11.0 downloads — release list with dates, SDK/runtime versions and go-live status (RC 2 2026-10-13, RC 1 2026-09-08). https://dotnet.microsoft.com/en-us/download/dotnet/11.0
    - C# 15 on .NET 11: The Complete Production Developer Guide — quote block: "…(the launch is planned for .NET Conf 2026, November 10–12)."
[^2]: What's new in .NET 11 (updated for RC 2; GA expected November 2026; C# 15 is the default for net11.0). https://learn.microsoft.com/en-us/dotnet/core/whats-new/dotnet-11/overview
    - C# 15 on .NET 11: The Complete Production Developer Guide — quote block: "…(the launch is planned for .NET Conf 2026, November 10–12)."
    - What's New in C# 15 and .NET 11 — paragraph 1: "…SDK compiles `union` and `closed` declarations without any `<LangVersion>` element."
[^3]: .NET Conf 2026 announcement (November 10–12, 2026, .NET 11 launch). https://devblogs.microsoft.com/dotnet/dotnet-conf-2026/
    - C# 15 on .NET 11: The Complete Production Developer Guide — quote block: "…(the launch is planned for .NET Conf 2026, November 10–12)."
[^4]: .NET and .NET Core official support policy (versions, latest patches 10.0.13/9.0.21/8.0.32 on 2026-10-13, end-of-support dates, go-live RC support windows). https://dotnet.microsoft.com/en-us/platform/support/policy/dotnet-core
    - C# 15 on .NET 11: The Complete Production Developer Guide — quote block: "…November 2028; the exact end date is published at GA."
    - C# 15 on .NET 11: The Complete Production Developer Guide — quote block: "….NET 8 (LTS) both reach end of support on **2026-11-10**."
    - Version-Aware Review — bullet 2: "RC 2's go-live support ends 2026-11-10."
[^5]: .NET STS releases extended to 24 months starting with .NET 9 (heise report of the September 2025 announcement); .NET 8 and .NET 9 end of support on November 10, 2026. https://heise.de/-10661376 , https://devblogs.microsoft.com/dotnet/dotnet-8-9-end-of-support/
    - C# 15 on .NET 11: The Complete Production Developer Guide — quote block: "…November 2028; the exact end date is published at GA."
[^6]: What's new in C# 15. https://learn.microsoft.com/en-us/dotnet/csharp/whats-new/csharp-15
    - What's New in C# 15 and .NET 11 › Language features (C# 15) — paragraph 1: "…**C# 15.0** and "Unsafe evolution" as "Merged as preview feature"."
    - What's New in C# 15 and .NET 11 › Language features (C# 15) — paragraph 3: "…"the latest C# preview release" until .NET 11 reaches GA."
    - Version-Aware Review — bullet 5: "…yet implemented"); behaviour described here is that of RC 2."
    - Extension Members and Extension Indexers — paragraph 1: "**C# 15 adds extension indexers**."
    - Memory Safety Preview (Not for Production) — paragraph 1: "`unsafe(expression)` and a `safe` modifier are added."
[^7]: Roslyn — Language Feature Status (C# 15.0 and working set). https://github.com/dotnet/roslyn/blob/main/docs/Language%20Feature%20Status.md
    - What's New in C# 15 and .NET 11 › Language features (C# 15) — paragraph 1: "…**C# 15.0** and "Unsafe evolution" as "Merged as preview feature"."
    - What's New in C# 15 and .NET 11 › Language features (C# 15) — paragraph 2: "…smaller C# 15 items that the "What's new" page omits:"
    - Do Not Use or Recommend › Features that are not in C# 15 / .NET 11 — paragraph 1: "Do not suggest them for C# 15 code:"
[^8]: C# compiler breaking changes since C# 14 (.NET 11). https://learn.microsoft.com/en-us/dotnet/csharp/whats-new/breaking-changes/compiler%20breaking%20changes%20-%20dotnet%2011
    - What's New in C# 15 and .NET 11 › Language features (C# 15) — paragraph 1: "…behaviour, which confirms they are not part of C# 15."
    - Do Not Use or Recommend › Deprecated or removed in .NET 11 (flag in review) — bullet 8: "…`@union` (verified; a type *declared* as `class union` still compiles)."
    - Collection Expression Arguments (New in C# 15) — bullet 5: "…upgrading to a 10.0.4xx or 11 SDK, regardless of `LangVersion`."
    - Memory Safety Preview (Not for Production) — paragraph 3: "…to delegates with pointer parameters), so expect new ambiguities (`CS0121`)."
    - Span, Memory and Allocation-Free Code — bullet 5: "…now an error; use an array or move the declaration."
    - Removed / Changed in .NET 11 — Migration Notes › Language / compiler — paragraph 1: "Roslyn "breaking changes since C# 14"."
[^9]: What's new in the .NET 11 runtime (Runtime Async, JIT, NativeAOT, ReadyToRun). https://learn.microsoft.com/en-us/dotnet/core/whats-new/dotnet-11/runtime
    - What's New in C# 15 and .NET 11 › Runtime and libraries (.NET 11) — bullet 1: "See [Async/Await and Runtime Async](#asyncawait-and-runtime-async)."
    - What's New in C# 15 and .NET 11 › Runtime and libraries (.NET 11) — bullet 2: "…is missing one or more of the baseline instruction sets.""
    - Do Not Use or Recommend › Features that are not in C# 15 / .NET 11 — bullet 5: "Do not recommend the removed `DOTNET_RuntimeAsync` environment variable."
    - Async/Await and Runtime Async › Runtime Async (New in .NET 11) — paragraph 1: "…is compiled this way; application and library code **opt in**:"
    - Async/Await and Runtime Async › Runtime Async (New in .NET 11) — paragraph 3: "No code change is needed."
    - Performance — bullet 4: "…(opt-in), no `ExecutionContext` capture when unused, JIT intrinsics for `Task.FromResult`/`ValueTask.FromResult`."
    - Performance — bullet 5: "…code into unreadable forms for things the JIT now does."
    - Performance — bullet 7: "`ReadyToRun` now specializes `EqualityComparer<T>.Default`/`Comparer<T>.Default`."
    - Trimming and Native AOT — bullet 3: "…data is available, and behaves differently when `StackTraceSupport` is disabled."
[^10]: Breaking change: Minimum hardware requirements updated (.NET 11). https://learn.microsoft.com/en-us/dotnet/core/compatibility/jit/11/minimum-hardware-requirements
    - What's New in C# 15 and .NET 11 › Runtime and libraries (.NET 11) — bullet 2: "…is missing one or more of the baseline instruction sets.""
    - Performance — bullet 6: "…(code may be re-jitted on older v2 CPUs, costing startup)."
[^11]: Breaking changes in .NET 11 (index of core library, crypto, extensions, deployment, interop, JIT, networking and SDK changes). https://learn.microsoft.com/en-us/dotnet/core/compatibility/11
    - What's New in C# 15 and .NET 11 › Runtime and libraries (.NET 11) — bullet 3: "`IHost.RunAsync`/`StopAsync` **throw** when a `BackgroundService` fails."
    - Concurrency and Synchronization — bullet 8: "…running as different users no longer share them by default."
    - Numerics, Dates and Times › Behaviour changes in .NET 11 (verified 10.0.13 vs 11 RC 2) — bullet 3: "`Complex` special values follow C23 Annex G."
    - Logging and Observability — bullet 5: "`Microsoft.Extensions.Diagnostics` can configure `Activity` tracing declaratively with rules."
    - HttpClient — bullet 9: "…AIA fetching is limited to two fetches per chain build."
    - Security — bullet 13: "ZIP reading validates CRC32."
    - Trimming and Native AOT — bullet 3: "…data is available, and behaves differently when `StackTraceSupport` is disabled."
    - Removed / Changed in .NET 11 — Migration Notes › Runtime and libraries — paragraph 1: "Sources: .NET 11 breaking changes index and linked articles."
[^12]: What's new in the .NET 11 libraries. https://learn.microsoft.com/en-us/dotnet/core/whats-new/dotnet-11/libraries
    - What's New in C# 15 and .NET 11 › Runtime and libraries (.NET 11) — bullet 4: "…Lines, async validation (`AsyncValidationAttribute`, `IAsyncValidatableObject`, `IAsyncValidateOptions<T>`), request-body compression content types."
    - Union Types (New in C# 15) › Unions over the wire — paragraph 1: "OpenAPI describes them as `anyOf`."
    - IAsyncEnumerable and Async LINQ — bullet 3: "`AsyncEnumerable` gains `FullJoin` and tuple-returning `Join`/`GroupJoin` like `Enumerable`."
    - IAsyncEnumerable and Async LINQ — bullet 5: "…source, topLevelValues: true)` writes JSON Lines (one value per line)."
    - Span, Memory and Allocation-Free Code — bullet 6: "`BitArray` gains span constructors."
    - Collections and Collection Expressions — bullet 7: "…int Age)>.Create(p => p.Name)`); .NET 8 already had `EqualityComparer<T>.Create(equals, getHashCode)`."
    - LINQ — bullet 7: "EF Core 11 translates `FullJoin` to `FULL JOIN`."
    - Numerics, Dates and Times › Behaviour changes in .NET 11 (verified 10.0.13 vs 11 RC 2) — bullet 4: "…types; do not replace `decimal` for money without a reason."
    - Dependency Injection and Options › Async options validation (New in .NET 11) — bullet 4: "`System.ComponentModel.DataAnnotations` gains `AsyncValidationAttribute`, `IAsyncValidatableObject` and `Validator.ValidateObjectAsync`/`TryValidateObjectAsync`."
    - Logging and Observability — bullet 5: "`Microsoft.Extensions.Diagnostics` can configure `Activity` tracing declaratively with rules."
    - HttpClient — bullet 8: "…request bodies for servers known to accept `Content-Encoding` on requests."
    - System.Text.Json › Unions, closed hierarchies and JSON (New in .NET 11) — bullet 2: "…format — use explicit `[JsonDerivedType(typeof(Circle), "circle")]` for stable public contracts."
    - System.Text.Json › Unions, closed hierarchies and JSON (New in .NET 11) — bullet 4: "…types/members; type-level `[JsonIgnore(Condition = ...)]` defaults; built-in converters for `Decimal32`/`Decimal64`/`Decimal128`."
    - Security — bullet 3: "…ML-KEM and HPKE APIs (SYSLIB5006/SYSLIB5009) are not for production yet."
[^13]: Breaking change: Some Microsoft.Extensions packages included in shared framework (.NET 11). https://learn.microsoft.com/en-us/dotnet/core/compatibility/extensions/11/extensions-in-shared-framework
    - What's New in C# 15 and .NET 11 › Runtime and libraries (.NET 11) — bullet 5: "…`Primitives`, …) are part of `Microsoft.NETCore.App`; explicit `PackageReference`s produce **NU1510**."
    - Do Not Use or Recommend › Deprecated or removed in .NET 11 (flag in review) — bullet 7: "…now in the shared framework (NU1510) when targeting only `net11.0`."
    - Project Setup › Recommended project file — bullet 3: "Keep them, conditioned on the TFM, in multi-targeted projects."
[^14]: What's new in ASP.NET Core in .NET 11. https://learn.microsoft.com/en-us/aspnet/core/release-notes/aspnetcore-11
    - What's New in C# 15 and .NET 11 › Runtime and libraries (.NET 11) — bullet 6: "…OpenAPI 3.2 default, **`ConcurrencyLimiter` middleware removed**, stricter Kestrel protocol compliance."
    - Union Types (New in C# 15) › Unions over the wire — paragraph 1: "OpenAPI describes them as `anyOf`."
    - ASP.NET Core Minimal APIs — bullet 2: "…attributes run here too, and `ValidatableTypeAttribute`/`SkipValidationAttribute` are no longer experimental."
    - ASP.NET Core Minimal APIs — bullet 3: "…their arguments were bound — check `context.HttpContext.Response.StatusCode` before reading `context.Arguments`."
[^15]: Breaking changes in ASP.NET Core 11. https://learn.microsoft.com/en-us/aspnet/core/breaking-changes/11/overview
    - What's New in C# 15 and .NET 11 › Runtime and libraries (.NET 11) — bullet 6: "…OpenAPI 3.2 default, **`ConcurrencyLimiter` middleware removed**, stricter Kestrel protocol compliance."
    - Do Not Use or Recommend › Deprecated or removed in .NET 11 (flag in review) — bullet 4: "…use the rate limiting middleware (`AddRateLimiter` with a concurrency limiter)."
    - Do Not Use or Recommend › Deprecated or removed in .NET 11 (flag in review) — bullet 5: "…obsolete `ImageTagHelper` members and an obsolete `ModelMetadataIdentity.ForProperty` overload are removed."
    - Logging and Observability — bullet 5: "`Microsoft.Extensions.Diagnostics` can configure `Activity` tracing declaratively with rules."
    - ASP.NET Core Minimal APIs — bullet 7: "…use a concurrency limiter policy in the rate limiting middleware."
    - ASP.NET Core Minimal APIs — bullet 9: "…strictly (malformed requests that used to be tolerated are rejected)."
    - ASP.NET Core Minimal APIs — bullet 10: "…that only understands 3.0/3.1 needs `options.OpenApiVersion = OpenApiSpecVersion.OpenApi3_1` or similar."
    - Removed / Changed in .NET 11 — Migration Notes › ASP.NET Core 11 and EF Core 11 — bullet 1: "SqlClient Entra ID authentication moved to a separate package."
[^16]: What's new in EF Core 11. https://learn.microsoft.com/en-us/ef/core/what-is-new/ef-core-11.0/whatsnew
    - What's New in C# 15 and .NET 11 › Runtime and libraries (.NET 11) — bullet 7: "…`Microsoft.Data.SqlClient` 7.0 (Entra ID auth moved to a separate package)."
    - IAsyncEnumerable and Async LINQ — bullet 4: "…Core 11 ships an analyzer and code fix for this."
    - LINQ — bullet 7: "EF Core 11 translates `FullJoin` to `FULL JOIN`."
    - EF Core 11 — paragraph 1: "…on .NET 11 (not on .NET 10 or .NET Framework)."
    - EF Core 11 — bullet 11: "…in one step (review: generated migrations still need code review)."
[^17]: Breaking changes in EF Core 11. https://learn.microsoft.com/en-us/ef/core/what-is-new/ef-core-11.0/breaking-changes
    - What's New in C# 15 and .NET 11 › Runtime and libraries (.NET 11) — bullet 7: "…`Microsoft.Data.SqlClient` 7.0 (Entra ID auth moved to a separate package)."
    - Do Not Use or Recommend › Deprecated or removed in .NET 11 (flag in review) — bullet 6: "`SqlAuthenticationMethod.ActiveDirectoryPassword` is obsolete."
    - EF Core 11 — bullet 9: "…contains no migrations (suppress with `ConfigureWarnings(w => w.Ignore(RelationalEventId.MigrationsNotFound))` if intentional)."
    - EF Core 11 — bullet 10: "`Property(...)` no longer configures primitive collections (use `PrimitiveCollection`)."
    - EF Core 11 — bullet 12: "`Microsoft.Data.Sqlite` 11 targets `net10.0` only (no .NET Framework)."
    - Trimming and Native AOT — bullet 2: "…publish when `PublishAot` is `true`; the `EFOptimizeContext` property was removed."
    - Removed / Changed in .NET 11 — Migration Notes › ASP.NET Core 11 and EF Core 11 — bullet 2: "…`__jObject` removed, sync I/O impossible, `id` characters no longer escaped."
[^18]: What's new in the SDK and tooling for .NET 11. https://learn.microsoft.com/en-us/dotnet/core/whats-new/dotnet-11/sdk
    - What's New in C# 15 and .NET 11 › Runtime and libraries (.NET 11) — bullet 8: "…in RC 2 (`--xunit-version v3` opts into v3 with Microsoft.Testing.Platform)."
    - Project Setup › Recommended project file — bullet 4: "…supply-chain guard), and `dotnet run -e NAME=value` passes environment variables."
    - Logging and Observability — bullet 2: "…arguments that are expensive to evaluate when logging is disabled."
    - Testing — bullet 3: "…`global.json` needs `"test": { "runner": "Microsoft.Testing.Platform" }` for `dotnet test`."
    - Testing — bullet 8: "MSTest/NUnit templates support `--test-runner`."
[^19]: What's new in C# 14. https://learn.microsoft.com/en-us/dotnet/csharp/whats-new/csharp-14
    - What's New in C# 15 and .NET 11 › What C# 14 / .NET 10 already gave you (still current) — paragraph 1: "…`LeftJoin`/`RightJoin`, `Shuffle`, `JsonSerializerOptions.Strict`, `AllowDuplicateProperties`, and minimal API validation via `AddValidation()`."
    - Extension Members and Extension Indexers — paragraph 1: "**C# 15 adds extension indexers**."
    - C# 14 Features Available in C# 15 — bullet 3: "…`Enumerable.Contains(a, 1)` explicitly in expressions handed to providers or interpreters."
[^20]: Breaking change: .NET 11 obsoletions with custom IDs (SYSLIB0064, SYSLIB0066). https://learn.microsoft.com/en-us/dotnet/core/compatibility/core-libraries/11/obsolete-apis
    - Do Not Use or Recommend › Deprecated or removed in .NET 11 (flag in review) — bullet 1: "…use the overloads that take `RSAEncryptionPadding` (prefer `RSA.Create()` with `RSAEncryptionPadding.OaepSHA256`)."
    - Do Not Use or Recommend › Deprecated or removed in .NET 11 (flag in review) — bullet 2: "`IStartupValidator` — obsolete **SYSLIB0066**; implement `IAsyncStartupValidator`."
    - Dependency Injection and Options › Async options validation (New in .NET 11) — bullet 3: "`IStartupValidator` is obsolete (SYSLIB0066); implement `IAsyncStartupValidator`."
    - Security — bullet 3: "…ML-KEM and HPKE APIs (SYSLIB5006/SYSLIB5009) are not for production yet."
[^21]: Union types — C# reference. https://learn.microsoft.com/en-us/dotnet/csharp/language-reference/builtin-types/union
    - Union Types (New in C# 15) › Declaration and generated shape — paragraph 1: "…are existing types; the union does not define new data."
    - Union Types (New in C# 15) › Declaration and generated shape — bullet 1: "…11, so unions cannot be used from a `net10.0` target."
    - Union Types (New in C# 15) › Declaration and generated shape — bullet 4: "…write `pet.Value is not null` or `pet is not null`."
    - Union Types (New in C# 15) › The `default` trap — always handle `null` — paragraph 2: "…`CS8655` fires for every switch without a `null` arm (verified)."
    - Union Types (New in C# 15) › Custom unions and `IUnion` — paragraph 1: "…wrong pattern-matching results — review custom unions like unsafe code."
    - Union Types (New in C# 15) › Custom unions and `IUnion` — paragraph 2: "Review these with the same soundness rules."
    - Nullable Reference Types — bullet 6: "…therefore invisible to the compiler — see the union section."
[^22]: What's new in C# 15 — Closed hierarchies; closed modifier reference. https://learn.microsoft.com/en-us/dotnet/csharp/whats-new/csharp-15#closed-hierarchies
    - Closed Hierarchies (New in C# 15) — paragraph 1: "…handles each direct subclass is exhaustive without a discard arm."
[^23]: Collection expressions — collection expression arguments. https://learn.microsoft.com/en-us/dotnet/csharp/language-reference/operators/collection-expressions#collection-expression-arguments
    - Collection Expression Arguments (New in C# 15) — paragraph 1: "…passes arguments to the target collection's constructor or builder method:"
[^24]: Jump statements — labeled break and continue. https://learn.microsoft.com/en-us/dotnet/csharp/language-reference/statements/jump-statements
    - Labeled break and continue (New in C# 15) — paragraph 1: "This replaces flag variables and `goto` for nested loops."
[^25]: IDE0410 — Use labeled jump statement. https://learn.microsoft.com/en-us/dotnet/fundamentals/code-analysis/style-rules/ide0410
    - Labeled break and continue (New in C# 15) — bullet 3: "…labeled jump can replace (verified with `EnforceCodeStyleInBuild` and severity `warning`)."
    - Analyzers and Build Hygiene — bullet 3: "…and `dotnet_diagnostic.IDE0410.severity = warning`, the flag-variable nested-loop pattern reports IDE0410."
[^26]: Unsafe code, pointer types, and function pointers — the updated memory safety model (preview). https://learn.microsoft.com/en-us/dotnet/csharp/language-reference/unsafe-code
    - Memory Safety Preview (Not for Production) — paragraph 1: "`unsafe(expression)` and a `safe` modifier are added."
[^27]: Breaking change: DateOnly and TimeOnly TryParse methods throw for invalid input. https://learn.microsoft.com/en-us/dotnet/core/compatibility/core-libraries/11/dateonly-timeonly-tryparse-argumentexception
    - Exceptions — bullet 6: "A `Try*` method throwing is now expected for *programming* errors."
[^28]: Breaking change: System.Linq.AsyncEnumerable in .NET 10. https://learn.microsoft.com/en-us/dotnet/core/compatibility/core-libraries/10.0/asyncenumerable
    - IAsyncEnumerable and Async LINQ — bullet 2: "…7.x, whose `net10.0` asset no longer duplicates the BCL operators."
[^29]: Breaking change: Floating-point conversions to small integral types are saturating (.NET 11). https://learn.microsoft.com/en-us/dotnet/core/compatibility/jit/11/fp-to-small-integer
    - Numerics, Dates and Times › Behaviour changes in .NET 11 (verified 10.0.13 vs 11 RC 2) — bullet 1: "Mono is not included in this change."
[^30]: Breaking change: Math.Round and MathF.Round return correctly rounded results (.NET 11). https://learn.microsoft.com/en-us/dotnet/core/compatibility/core-libraries/11/math-round-digits
    - Numerics, Dates and Times › Behaviour changes in .NET 11 (verified 10.0.13 vs 11 RC 2) — bullet 2: "…must use `decimal` (`Math.Round(2.675m, 2)` is `2.68` on every version)."
[^31]: Breaking change: Synchronous access to async-validated options throws (.NET 11 RC 1). https://learn.microsoft.com/en-us/dotnet/core/compatibility/extensions/11/async-options-validation-sync-access
    - Dependency Injection and Options › Async options validation (New in .NET 11) — bullet 1: "…options are re-created synchronously (`IOptionsMonitor` reload, `IOptionsSnapshot` in later scopes)."
[^32]: Breaking change: IHost.RunAsync and IHost.StopAsync throw when a BackgroundService fails (.NET 11). https://learn.microsoft.com/en-us/dotnet/core/compatibility/extensions/11/ihost-runasync-stopasync-throw-backgroundservice-failure
    - Hosting and Background Services — bullet 1: "…around `RunAsync` that logged "graceful shutdown" now sees the failure."
[^33]: Breaking change: .NET 11 obsoletions in cryptography (SYSLIB0065). https://learn.microsoft.com/en-us/dotnet/core/compatibility/cryptography/11/obsolete-apis
    - Do Not Use or Recommend › Deprecated or removed in .NET 11 (flag in review) — bullet 3: "Verified: all three IDs (SYSLIB0064/0065/0066) warn on RC 2."
