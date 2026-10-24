### Citations: C# 9 on .NET 5 (End of Life): The Complete Production Developer Guide

Sources for `csharp-9.md`. The guide carries no footnote markers; each entry lists where it was cited (section path, block, and the words just before the citation). Not sent to the model; kept for maintainers.

[^1]: The history of C# — C# version 9 (released November 2020 with .NET 5). https://learn.microsoft.com/dotnet/csharp/whats-new/csharp-version-history#c-version-9
    - C# 9 on .NET 5 (End of Life): The Complete Production Developer Guide — quote block: "…on November 10, 2020** (SDK 5.0.100, Visual Studio 2019 16.8)."
    - Overview and Global Truths — paragraph 1: "…interop and source generators, which also debuted with .NET 5."
    - What's New in C# 9 — paragraph 1: "…8` (only `AllowUnsafeBlocks` is required; verified via `MethodBody.InitLocals == false`)."
[^2]: .NET 5.0 release metadata (release dates, 5.0.17, SDK 5.0.408/5.0.214, CVEs, EOL 2022-05-10, VS 16.8). https://builds.dotnet.microsoft.com/dotnet/release-metadata/5.0/releases.json
    - C# 9 on .NET 5 (End of Life): The Complete Production Developer Guide — quote block: "…on November 10, 2020** (SDK 5.0.100, Visual Studio 2019 16.8)."
    - C# 9 on .NET 5 (End of Life): The Complete Production Developer Guide — quote block: "…SDKs **5.0.408** and **5.0.214**; it fixed CVE-2022-29117, CVE-2022-23267 and CVE-2022-29145."
    - C# 9 on .NET 5 (End of Life): The Complete Production Developer Guide — quote block: "…it has gone more than four years without security fixes."
    - Support Status and Upgrade Path — paragraph 1: "Sources: the official release metadata (`releases-index.json` and per-channel `releases.json`)."
[^3]: .NET releases index (support phases, latest patches, EOL dates for 6.0–11.0). https://builds.dotnet.microsoft.com/dotnet/release-metadata/releases-index.json ; .NET and .NET Core support policy. https://dotnet.microsoft.com/platform/support/policy/dotnet-core
    - C# 9 on .NET 5 (End of Life): The Complete Production Developer Guide — quote block: "…it has gone more than four years without security fixes."
    - C# 9 on .NET 5 (End of Life): The Complete Production Developer Guide — quote block: "…11.0.100-rc.2) shipped on October 13, 2026 with a go-live license."
    - Support Status and Upgrade Path — paragraph 1: "Sources: the official release metadata (`releases-index.json` and per-channel `releases.json`)."
[^4]: C# language versioning — defaults per target framework. https://learn.microsoft.com/dotnet/csharp/language-reference/language-versioning
    - C# 9 on .NET 5 (End of Life): The Complete Production Developer Guide — quote block: "…`LangVersion`, because C# 9 is the default for .NET 5."
    - How C# 9 Is Selected: LangVersion, TFMs and Down-Level Targets › Default language version per target framework — paragraph 1: "The default is chosen by the compiler from the TFM."
[^5]: C# 9.0 feature proposals (records, init, top-level statements, patterns3, native integers, function pointers, skip-localsinit, module initializers, extending partial methods, covariant returns, target-typed new/conditional, static lambdas, lambda discards, extension GetEnumerator, local function attributes). https://github.com/dotnet/csharplang/tree/main/proposals/csharp-9.0
    - Overview and Global Truths — paragraph 1: "…interop and source generators, which also debuted with .NET 5."
    - What's New in C# 9 — paragraph 1: "…8` (only `AllowUnsafeBlocks` is required; verified via `MethodBody.InitLocals == false`)."
[^6]: .NET globalization and ICU (behaviour changes, `UseNls`). https://learn.microsoft.com/dotnet/core/extensions/globalization-icu
    - Overview and Global Truths — bullet 10: "…so culture-sensitive string APIs behave differently from .NET Framework's NLS."
    - What's New in .NET 5 for Reviewers › Runtime and library changes [New in .NET 5] ★★★ — bullet 2: "…grapheme), but `6` with `StringComparison.Ordinal` or with the `char` overload."
[^7]: Covariant return types proposal and `RuntimeFeature.CovariantReturnsOfClasses`. https://github.com/dotnet/csharplang/blob/main/proposals/csharp-9.0/covariant-returns.md
    - How C# 9 Is Selected: LangVersion, TFMs and Down-Level Targets › Which C# 9 features need what (down-level targets) — paragraph 2: "…`RuntimeFeature.CovariantReturnsOfClasses`, which only the .NET 5+ runtime reports as supported."
[^8]: Records (C# reference). https://learn.microsoft.com/dotnet/csharp/language-reference/builtin-types/record
    - What's New in C# 9 › 1. Records [New in C# 9] ★★★ — paragraph 1: "…constructor, and a hidden `<Clone>$` method that powers `with` expressions."
[^9]: Patterns (C# reference) — relational, logical, type and parenthesized patterns. https://learn.microsoft.com/dotnet/csharp/language-reference/operators/patterns
    - What's New in C# 9 › 4. Pattern matching enhancements [New in C# 9] ★★★ — paragraph 1: "…patterns** (a bare type, `int => ...`, without a designation)."
[^10]: Breaking changes in .NET 5 (core libraries, SDK, serialization, code analysis, globalization, networking, cryptography). https://learn.microsoft.com/dotnet/core/compatibility/5.0
    - What's New in .NET 5 for Reviewers › Runtime and library changes [New in .NET 5] ★★★ — bullet 1: "`NETCOREAPP3_1` is not defined for `net5.0` (all verified)."
    - What's New in .NET 5 for Reviewers › Runtime and library changes [New in .NET 5] ★★★ — bullet 3: "…— changed from `Length - 1` in .NET Core 3.1."
    - What's New in .NET 5 for Reviewers › Runtime and library changes [New in .NET 5] ★★★ — bullet 4: "ASP.NET Core 5 correlates with W3C `traceparent` headers by default."
    - What's New in .NET 5 for Reviewers › Runtime and library changes [New in .NET 5] ★★★ — bullet 9: "…— side-effecting predicates behave differently after an upgrade from 3.1."
    - What's New in .NET 5 for Reviewers › Runtime and library changes [New in .NET 5] ★★★ — bullet 11: "…officially supported; in single-file apps `Assembly.Location` returns an empty string."
    - Project Setup, Build and Analyzers › Recommended project file — bullet 1: "…CA2013, CA2014 and CA2247 were triggered in a test build."
    - Project Setup, Build and Analyzers › Recommended project file — bullet 5: "…management with `ManagePackageVersionsCentrally` itself only works with later NuGet versions."
    - Logging and Observability — bullet 5: "…true)`, `AddJsonConsole()`, `AddSystemdConsole()`; the older `ConsoleLoggerOptions` formatting properties are obsolete."
    - ASP.NET Core 5 › Application shape — bullet 6: "…accepted in ASP.NET Core 5 apps (breaking change from 3.1)."
    - Security — bullet 12: "….NET 5 uses a restricted default cipher-suite list (OpenSSL configuration)."
    - Removed / Changed in .NET 5 — Migration Notes › From .NET Core 3.1 to .NET 5 — bullet 7: "…TLS cipher suites on Linux — behaviour changes to re-test."
    - Removed / Changed in .NET 5 — Migration Notes › From .NET Core 3.1 to .NET 5 — bullet 8: "ASP.NET Core accepts quoted numbers."
    - Removed / Changed in .NET 5 — Migration Notes › From .NET Core 3.1 to .NET 5 — bullet 10: "P/Invoke no longer probes `A`/`W` suffixes on non-Windows platforms."
[^11]: HttpClient: throw TimeoutException as inner exception on timeout (dotnet/runtime#21965, shipped in .NET 5). https://github.com/dotnet/runtime/issues/21965
    - What's New in .NET 5 for Reviewers › Runtime and library changes [New in .NET 5] ★★★ — bullet 5: "…— distinguish timeouts from caller cancellation by checking that (verified)."
[^12]: BinaryFormatter serialization methods are obsolete and prohibited in ASP.NET apps (.NET 5). https://learn.microsoft.com/dotnet/core/compatibility/serialization/5.0/binaryformatter-serialization-obsolete
    - What's New in .NET 5 for Reviewers › Runtime and library changes [New in .NET 5] ★★★ — bullet 7: "…`NotSupportedException` in ASP.NET Core apps** (`Microsoft.NET.Sdk.Web`) by default — verified."
[^13]: Obsolete features in .NET 5+ (SYSLIB0001–SYSLIB0041 list). https://learn.microsoft.com/dotnet/fundamentals/syslib-diagnostics/obsoletions-overview
    - What's New in .NET 5 for Reviewers › Runtime and library changes [New in .NET 5] ★★★ — bullet 8: "…obsolete (SYSLIB0001); code-access-security, GAC, remoting, CER APIs are obsolete (SYSLIB0002–0012)."
    - Deprecated, Obsoleted and Removed APIs in .NET 5 — paragraph 1: "…7; they default to **SHA-1 and 1,000 iterations** — verified)."
    - Removed / Changed in .NET 5 — Migration Notes › From .NET 5 to .NET 10 (the recommended upgrade) — bullet 5: "…are obsolete (SYSLIB0060) in .NET 10 — move to `Rfc2898DeriveBytes.Pbkdf2`."
[^14]: What's new in .NET 5. https://learn.microsoft.com/dotnet/core/whats-new/dotnet-5
    - What's New in .NET 5 for Reviewers › Runtime and library changes [New in .NET 5] ★★★ — bullet 10: "…is annotated, so upgrading from 3.1 surfaces new nullable warnings."
[^15]: System.Text.Json overview and how-to articles (immutable types/records, fields, ignore conditions, quoted numbers, reference preservation, web defaults). https://learn.microsoft.com/dotnet/standard/serialization/system-text-json/overview
    - What's New in .NET 5 for Reviewers › System.Text.Json 5.0 [New in .NET 5] ★★ — paragraph 1: "…a copy constructor for `JsonSerializerOptions`, and `HttpClient`/`HttpContent` JSON extensions (`System.Net.Http.Json`)."
[^16]: What's new in ASP.NET Core 5.0. https://learn.microsoft.com/aspnet/core/release-notes/aspnetcore-5.0
    - What's New in .NET 5 for Reviewers › ASP.NET Core 5 [New in .NET 5] — paragraph 1: "…ASP.NET Core app, JSON request bodies accept quoted numbers (`JsonSerializerDefaults.Web`)."
    - Logging and Observability — bullet 5: "…true)`, `AddJsonConsole()`, `AddSystemdConsole()`; the older `ConsoleLoggerOptions` formatting properties are obsolete."
[^17]: What's new in EF Core 5.0. https://learn.microsoft.com/ef/core/what-is-new/ef-core-5.0/whatsnew
    - What's New in .NET 5 for Reviewers › EF Core 5 [New in .NET 5] — paragraph 1: "…targets `netstandard2.1`, so it **does not run on .NET Framework**."
[^18]: Breaking changes in EF Core 5.0. https://learn.microsoft.com/ef/core/what-is-new/ef-core-5.0/breaking-changes
    - What's New in .NET 5 for Reviewers › EF Core 5 [New in .NET 5] — paragraph 1: "…targets `netstandard2.1`, so it **does not run on .NET Framework**."
    - EF Core 5 — bullet 13: "EF Core 6+ requires .NET 6+."
    - Removed / Changed in .NET 5 — Migration Notes › From .NET Core 3.1 to .NET 5 — bullet 12: "…to a table", discriminators are read-only, `IndexBuilder.HasName` obsolete (use `HasDatabaseName`)."
[^19]: Code analysis in .NET (EnableNETAnalyzers, AnalysisLevel, AnalysisMode). https://learn.microsoft.com/dotnet/fundamentals/code-analysis/overview
    - Project Setup, Build and Analyzers › Recommended project file — bullet 1: "…CA2013, CA2014 and CA2247 were triggered in a test build."
[^20]: Async ValueTask Pooling in .NET 5 (`DOTNET_SYSTEM_THREADING_POOLASYNCVALUETASKS`). https://devblogs.microsoft.com/dotnet/async-valuetask-pooling-in-net-5/
    - Async/Await, Cancellation and Concurrency — bullet 8: "…state machines (opt-in via `DOTNET_SYSTEM_THREADING_POOLASYNCVALUETASKS`) are new in .NET 5."
[^21]: Trimming options (.NET 5 options, trim analyzer in .NET 6+, feature switches). https://learn.microsoft.com/dotnet/core/deploying/trimming/trimming-options
    - Project Setup, Build and Analyzers › Single-file and trimming — paragraph 1: "…arrives with .NET 6, so .NET 5 gives little warning."
[^22]: Unhandled exceptions from a BackgroundService stop the host (.NET 6 breaking change). https://learn.microsoft.com/dotnet/core/compatibility/core-libraries/6.0/hosting-exception-handling
    - Dependency Injection, Options, Configuration and Hosting › Hosting and background services — bullet 1: "…stop the host and added `HostOptions.BackgroundServiceExceptionBehavior` (absent in 5.0, verified)."
[^23]: Breaking changes in ASP.NET Core 5. https://learn.microsoft.com/aspnet/core/breaking-changes/5/overview
    - ASP.NET Core 5 › Application shape — bullet 15: "…— handlers that cast it to `Endpoint` silently stop matching."
    - ASP.NET Core 5 › Application shape — bullet 16: "…defaults) in 5.0; code that pins `Tls11`/`Tls` re-enables obsolete protocols."
    - ASP.NET Core 5 › Application shape — bullet 17: "…error handler path returns 404, instead of returning the 404."
    - Removed / Changed in .NET 5 — Migration Notes › From .NET Core 3.1 to .NET 5 — bullet 11: "…are obsolete, `UseSignalR`/`UseConnections` are removed, `AzureAD.UI`/`AzureADB2C.UI` are obsolete (use Microsoft.Identity.Web)."
