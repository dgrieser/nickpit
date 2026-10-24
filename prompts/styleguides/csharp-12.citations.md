### Citations: C# 12 on .NET 8 (LTS): The Complete Production Developer Guide

Sources for `csharp-12.md`. The guide carries no footnote markers; each entry lists where it was cited (section path, block, and the words just before the citation). Not sent to the model; kept for maintainers.

[^1]: Announcing .NET 8. Microsoft .NET Blog, November 14, 2023. https://devblogs.microsoft.com/dotnet/announcing-dotnet-8/
    - C# 12 on .NET 8 (LTS): The Complete Production Developer Guide — quote block: "…November 14, 2023 as a Long Term Support (LTS) release."
[^2]: .NET 8.0.32 release notes (October 13, 2026) and the .NET 8 release metadata (latest release, SDKs, CVE lists incl. CVE-2025-55315 in 8.0.21). https://github.com/dotnet/core/blob/main/release-notes/8.0/8.0.32/8.0.32.md , https://builds.dotnet.microsoft.com/dotnet/release-metadata/8.0/releases.json
    - C# 12 on .NET 8 (LTS): The Complete Production Developer Guide — quote block: "EF Core 8.0.32 shipped the same day."
    - C# 12 on .NET 8 (LTS): The Complete Production Developer Guide — paragraph 1: "…fixed CVE-2025-55315, a critical HTTP request smuggling flaw in Kestrel."
    - Security — bullet 10: "…`mcr.microsoft.com/dotnet/aspnet:8.0.x` pinned to an older tag) is missing security fixes."
[^3]: .NET and .NET Core support policy; .NET releases index (support phases and end-of-support dates for .NET 8, 9, 10, 11). https://dotnet.microsoft.com/en-us/platform/support/policy/dotnet-core , https://builds.dotnet.microsoft.com/dotnet/release-metadata/releases-index.json
    - C# 12 on .NET 8 (LTS): The Complete Production Developer Guide — quote block: "…— a preview with go-live support, not a stable release."
    - Removed / Changed in .NET 8 — Migration Notes › Planning the move to .NET 10 LTS — paragraph 1: "….NET 9 (STS, also ending November 10, 2026) is normal."
[^4]: What's new in C# 12. https://learn.microsoft.com/en-us/dotnet/csharp/whats-new/csharp-12
    - What's New in C# 12 and .NET 8 › Language (C# 12) — paragraph 1: "…the language version — they also work with `LangVersion 11`)."
    - New in C# 12: Primary Constructors — paragraph 1: "…a member is *captured* into a compiler-generated, unnamed, **mutable** field."
    - New in C# 12: Collection Expressions — paragraph 1: "`..e` spreads an enumerable."
    - New in C# 12: Inline Arrays — paragraph 1: "It converts implicitly to `Span<T>`/`ReadOnlySpan<T>`, supports indexing and `foreach`."
    - New in C# 12: `ref readonly` Parameters — paragraph 1: "…passing an rvalue (a constant or temporary) gives `warning CS9193`."
    - New in C# 12: `ExperimentalAttribute` — paragraph 1: "Experimental(...)]` are experimental."
    - New in C# 12: Interceptors (preview only) — paragraph 1: "…project property `<InterceptorsPreviewNamespaces>$(InterceptorsPreviewNamespaces);My.Generated</InterceptorsPreviewNamespaces>`; without it the build fails with `CS9137`."
[^5]: What's new in the .NET 8 runtime and libraries. https://learn.microsoft.com/en-us/dotnet/core/whats-new/dotnet-8/runtime
    - What's New in C# 12 and .NET 8 › Runtime and libraries (.NET 8) — bullet 1: "…`GetElapsedTime()`, `CreateTimer()`), `ITimer`, and `Task.Delay`/`CancellationTokenSource` overloads that take a `TimeProvider`."
    - What's New in C# 12 and .NET 8 › Runtime and libraries (.NET 8) — bullet 2: "`System.Collections.Frozen` (`FrozenDictionary<TKey,TValue>`, `FrozenSet<T>`), `System.Buffers.SearchValues<T>`, `System.Text.CompositeFormat`, `IUtf8SpanFormattable`, `Utf8.TryWrite`, `Encoding.TryGetBytes`."
    - What's New in C# 12 and .NET 8 › Runtime and libraries (.NET 8) — bullet 3: "`Random.GetItems`, `Random.Shuffle`, `RandomNumberGenerator.GetItems/Shuffle/GetString/GetHexString`."
    - What's New in C# 12 and .NET 8 › Runtime and libraries (.NET 8) — bullet 4: "`ArgumentException.ThrowIfNullOrWhiteSpace`, `ArgumentOutOfRangeException.ThrowIfNegative/ThrowIfZero/ThrowIfNegativeOrZero/ThrowIfGreaterThan/ThrowIfLessThan/…` (analyzers CA1510–CA1513 suggest them)."
    - What's New in C# 12 and .NET 8 › Runtime and libraries (.NET 8) — bullet 6: "…helpers (`GetFromJsonAsAsyncEnumerable`), `Half`/`Int128`/`UInt128`/`Memory<T>` support, an option to disable reflection (`JsonSerializerIsReflectionEnabledByDefault`)."
    - What's New in C# 12 and .NET 8 › Runtime and libraries (.NET 8) — bullet 7: "…(`[OptionsValidator]`), configuration-binding source generator, `LoggerMessageAttribute` shorter constructors, `IMeterFactory` and `MetricCollector<T>`."
    - What's New in C# 12 and .NET 8 › Runtime and libraries (.NET 8) — bullet 8: "`[Length]`, `[AllowedValues]`, `[DeniedValues]`, `[Base64String]`, `RangeAttribute.MinimumIsExclusive/MaximumIsExclusive`."
    - What's New in C# 12 and .NET 8 › Runtime and libraries (.NET 8) — bullet 9: "…an `https://` proxy), `TensorPrimitives` (System.Numerics.Tensors package), source-generated COM interop (`[GeneratedComInterface]`)."
    - New in .NET 8: `TimeProvider` — paragraph 1: "…timers so code is testable without `Thread.Sleep` or wall-clock waits."
    - System.Text.Json › Source generation, enums, polymorphism — bullet 6: "…(`JsonSerializerIsReflectionEnabledByDefault=false`) — every serialized type must be in a `JsonSerializerContext`."
    - Security — bullet 3: "…it depends on OpenSSL 1.1.1+ or a recent Windows build."
    - Performance — bullet 2: "…compilation or PGO in `runtimeconfig` usually makes steady-state code slower."
    - Performance — bullet 9: "…8**) applies a changed container memory limit at run time."
[^6]: ConfigureAwaitOptions enum; CancellationTokenSource.CancelAsync. https://learn.microsoft.com/en-us/dotnet/api/system.threading.tasks.configureawaitoptions , https://learn.microsoft.com/en-us/dotnet/api/system.threading.cancellationtokensource.cancelasync
    - What's New in C# 12 and .NET 8 › Runtime and libraries (.NET 8) — bullet 5: "`CancellationTokenSource.CancelAsync()`, `ConfigureAwaitOptions` (`SuppressThrowing`, `ForceYielding`, `ContinueOnCapturedContext`)."
    - Async/Await › New in .NET 8: `ConfigureAwaitOptions` and `CancelAsync` — paragraph 1: "`ConfigureAwait(true)` to `ContinueOnCapturedContext`."
[^7]: What's new in ASP.NET Core in .NET 8. https://learn.microsoft.com/en-us/aspnet/core/release-notes/aspnetcore-8.0
    - What's New in C# 12 and .NET 8 › Runtime and libraries (.NET 8) — bullet 10: "…timeouts middleware, new built-in metrics, Blazor Web App unified model."
    - New in C# 12: Interceptors (preview only) — bullet 1: "…generator (`<EnableConfigurationBindingGenerator>true</EnableConfigurationBindingGenerator>`) use interceptors; the SDK adds their namespaces automatically."
    - Dependency Injection › New in .NET 8: keyed services — bullet 3: "…API parameters, MVC action parameters and Blazor `[Inject(Key = "key")]`."
    - ASP.NET Core 8 Essentials › Minimal API service skeleton — bullet 7: "…to the next handler; multiple handlers run in registration order."
    - ASP.NET Core 8 Essentials › Hosting, ports and proxies — bullet 1: "`HTTP_PORTS`/`HTTPS_PORTS` configuration keys are new in .NET 8."
    - ASP.NET Core 8 Essentials › Forms and antiforgery (new in .NET 8 for minimal APIs) — paragraph 1: "For token-authenticated APIs (no cookies), opt out explicitly with `.DisableAntiforgery()`."
    - ASP.NET Core 8 Essentials › Other ASP.NET Core 8 changes reviewers meet — bullet 1: "…OIDC; review that registration/confirmation flows and email sending are configured."
    - ASP.NET Core 8 Essentials › Other ASP.NET Core 8 changes reviewers meet — bullet 5: "…Kestrel and SignalR publish `System.Diagnostics.Metrics` instruments (`http.server.request.duration`, …) for OpenTelemetry."
    - Trimming and Native AOT — bullet 4: "`ConfigureHttpJsonOptions(o => o.SerializerOptions.TypeInfoResolverChain.Insert(0, AppJsonContext.Default))` registers source-generated metadata."
[^8]: What's new in EF Core 8. https://learn.microsoft.com/en-us/ef/core/what-is-new/ef-core-8.0/whatsnew
    - What's New in C# 12 and .NET 8 › Runtime and libraries (.NET 8) — bullet 11: "…queries, `HierarchyId`, `DateOnly`/`TimeOnly` on SQL Server, sentinel values for defaults."
[^9]: Breaking changes in .NET 8 (containers, core libraries, cryptography, extensions, globalization, SDK, serialization). https://learn.microsoft.com/en-us/dotnet/core/compatibility/8.0
    - What's New in C# 12 and .NET 8 › Runtime and libraries (.NET 8) — bullet 12: "…packages (NuGet audit), `BinaryFormatter` is disabled for most project types."
    - What's New in C# 12 and .NET 8 › Deprecated or removed in .NET 8 — bullet 1: "…a security finding; it is removed entirely in .NET 9."
    - What's New in C# 12 and .NET 8 › Deprecated or removed in .NET 8 — bullet 2: "`XmlSecureResolver` (SYSLIB0047) was already obsolete in .NET 7."
    - What's New in C# 12 and .NET 8 › Deprecated or removed in .NET 8 — bullet 4: "…pack` default to the `Release` configuration in .NET 8 projects."
    - Project Setup › Recommended project file — bullet 5: "Do not suppress these warnings without an issue reference."
    - Project Setup › SDK-style behavior changes that surprise reviewers — bullet 2: "…longer implies `--self-contained`; add `--self-contained` explicitly when it is required."
    - Strings, Formatting and Culture — bullet 4: "…2049 on .NET 8 (1949 on .NET 7) — verified."
    - Exceptions and Guard Clauses — bullet 7: "…that constructor and `GetObjectData` are obsolete in .NET 8 (SYSLIB0051)."
    - LINQ — bullet 3: "Use `long` accumulation for values near `int` limits."
    - Dependency Injection › Options — bullet 3: "…which now also throws for such mismatches (verified on 8.0.32)."
    - ASP.NET Core 8 Essentials › Hosting, ports and proxies — bullet 1: "Kubernetes manifests and Dockerfiles that still expose port 80 break."
    - Containers and Deployment — bullet 1: "…add it for Negotiate/Kerberos auth), and multi-platform tags are Linux-only."
    - Containers and Deployment — bullet 2: "…default tag is `latest` — pin explicit tags in CI."
[^10]: What's new in C# 13. https://learn.microsoft.com/en-us/dotnet/csharp/whats-new/csharp-13
    - What's New in C# 12 and .NET 8 › Do not use or recommend (newer than C# 12 / .NET 8) — paragraph 2: "**C# 13 (.NET 9, November 2024)**:"
[^11]: What's new in C# 14. https://learn.microsoft.com/en-us/dotnet/csharp/whats-new/csharp-14
    - What's New in C# 12 and .NET 8 › Do not use or recommend (newer than C# 12 / .NET 8) — paragraph 3: "**C# 14 (.NET 10, November 2025)**:"
    - Removed / Changed in .NET 8 — Migration Notes › Planning the move to .NET 10 LTS — bullet 1: "…first-class span conversions that change which extension overload is chosen."
[^12]: What's new in C# 15 (preview). https://learn.microsoft.com/en-us/dotnet/csharp/whats-new/csharp-15
    - What's New in C# 12 and .NET 8 › Do not use or recommend (newer than C# 12 / .NET 8) — paragraph 4: "…15 (.NET 11, preview — RC2 as of October 2026)**:"
[^13]: What's new in .NET 9 libraries. https://learn.microsoft.com/en-us/dotnet/core/whats-new/dotnet-9/libraries
    - What's New in C# 12 and .NET 8 › Do not use or recommend (newer than C# 12 / .NET 8) — paragraph 5: "…9 APIs** (absent from `net8.0`, verified by compiling against it):"
[^14]: EF Core releases and planning (target frameworks and support dates for EF Core 8, 9, 10). https://learn.microsoft.com/en-us/ef/core/what-is-new/
    - What's New in C# 12 and .NET 8 › Do not use or recommend (newer than C# 12 / .NET 8) — bullet 22: "…are available only if the project references EF Core 9.x."
    - EF Core 8 — paragraph 1: "EF Core 10 requires .NET 10."
[^15]: What's new in .NET 10. https://learn.microsoft.com/en-us/dotnet/core/whats-new/dotnet-10/overview
    - What's New in C# 12 and .NET 8 › Do not use or recommend (newer than C# 12 / .NET 8) — paragraph 6: "…built-in minimal API validation (`builder.Services.AddValidation()`), file-based apps (`dotnet run app.cs`)."
[^16]: Code quality rules (CA) reference. https://learn.microsoft.com/en-us/dotnet/fundamentals/code-analysis/quality-rules/
    - Project Setup › Recommended project file — bullet 2: "…(`SearchValues`) and CA2021 (impossible `Cast<T>`/`OfType<T>`), all present in SDK 8.0.426."
    - Analyzers and Tooling — bullet 2: "…`JsonSerializerOptions`), CA1870 (`SearchValues`), CA2021 (impossible casts), CA2261 (`ConfigureAwaitOptions.SuppressThrowing` on `Task<T>`)."
[^17]: Tutorial: Explore primary constructors. https://learn.microsoft.com/en-us/dotnet/csharp/whats-new/tutorials/primary-constructors
    - New in C# 12: Primary Constructors — paragraph 1: "…a member is *captured* into a compiler-generated, unnamed, **mutable** field."
[^18]: Collection expressions — C# language reference. https://learn.microsoft.com/en-us/dotnet/csharp/language-reference/operators/collection-expressions
    - New in C# 12: Collection Expressions — paragraph 1: "`..e` spreads an enumerable."
[^19]: Interceptors feature specification (Roslyn). https://github.com/dotnet/roslyn/blob/main/docs/features/interceptors.md
    - New in C# 12: Interceptors (preview only) — paragraph 1: "…project property `<InterceptorsPreviewNamespaces>$(InterceptorsPreviewNamespaces);My.Generated</InterceptorsPreviewNamespaces>`; without it the build fails with `CS9137`."
[^20]: Build resilient HTTP apps (Microsoft.Extensions.Http.Resilience defaults, DisableFor/DisableForUnsafeHttpMethods). https://learn.microsoft.com/en-us/dotnet/core/resilience/http-resilience
    - HttpClient — bullet 3: "…set `options.Retry.ShouldHandle` to exclude unsafe methods, or use idempotency keys."
[^21]: Request timeouts middleware in ASP.NET Core. https://learn.microsoft.com/en-us/aspnet/core/performance/timeouts?view=aspnetcore-8.0
    - ASP.NET Core 8 Essentials › Minimal API service skeleton — bullet 8: "…the middleware does not trigger while a debugger is attached."
[^22]: Breaking changes in EF Core 8. https://learn.microsoft.com/en-us/ef/core/what-is-new/ef-core-8.0/breaking-changes
    - EF Core 8 › EF Core 8 behavior changes to check when upgrading from EF Core 7 — section heading
[^23]: What's new in EF Core 9 (experimental Native AOT and precompiled queries). https://learn.microsoft.com/en-us/ef/core/what-is-new/ef-core-9.0/whatsnew
    - Trimming and Native AOT — paragraph 1: "…(experimental support with precompiled queries starts in EF Core 9)."
[^24]: ASP.NET Core support for Native AOT (.NET 8 compatibility table). https://learn.microsoft.com/en-us/aspnet/core/fundamentals/native-aot?view=aspnetcore-8.0
    - Trimming and Native AOT — paragraph 1: "…middleware and authentication other than JWT bearer are **not** AOT-compatible."
[^25]: Breaking changes in ASP.NET Core 8 (security token events return a JsonWebToken, ISystemClock obsolete, AddHttpLogging/AddRateLimiter required, TrimMode full). https://learn.microsoft.com/en-us/aspnet/core/breaking-changes/8/overview
    - ASP.NET Core 8 Essentials › Middleware order — bullet 3: "…set `options.UseSecurityTokenValidators = true` only as a temporary migration aid."
    - ASP.NET Core 8 Essentials › Other ASP.NET Core 8 changes reviewers meet — bullet 4: "Web SDK projects that trim default to `TrimMode=full`."
[^26]: Breaking change: Forwarded headers middleware ignores X-Forwarded-* headers from unknown proxies (ASP.NET Core 8.0.17 / 9.0.6). https://learn.microsoft.com/en-us/aspnet/core/breaking-changes/8/forwarded-headers-unknown-proxies
    - ASP.NET Core 8 Essentials › Hosting, ports and proxies — bullet 2: "…`Microsoft.AspNetCore.HttpOverrides.IgnoreUnknownProxiesWithoutFor` switch, restores the insecure behavior and is a finding."
