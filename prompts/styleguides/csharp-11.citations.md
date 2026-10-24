### Citations: C# 11 on .NET 7: The Complete Production Developer Guide

Sources for `csharp-11.md`. The guide carries no footnote markers; each entry lists where it was cited (section path, block, and the words just before the citation). Not sent to the model; kept for maintainers.

[^1]: .NET 7.0 release notes and release metadata (7.0.0 on 2022-11-08 … 7.0.20 / SDK 7.0.410 on 2024-05-28). https://github.com/dotnet/core/blob/main/release-notes/7.0/README.md , https://dotnetcli.blob.core.windows.net/dotnet/release-metadata/7.0/releases.json
    - Overview — paragraph 1: "…SDK** and Visual Studio 2022 17.4 on **November 8, 2022**."
    - Overview — paragraph 1: "…as a security release (CVE-2024-20672, a macOS-only X.509 denial-of-service issue)."
[^2]: .NET and .NET Core official support policy; release index with support phases and end-of-support dates (7.0 EOL 2024-05-14; 8.0 and 9.0 2026-11-10; 10.0 2028-11-14; 11.0 RC2 go-live). https://dotnet.microsoft.com/platform/support/policy/dotnet-core , https://dotnetcli.blob.core.windows.net/dotnet/release-metadata/releases-index.json
    - Overview — paragraph 1: "…as a security release (CVE-2024-20672, a macOS-only X.509 denial-of-service issue)."
    - Overview — paragraph 1: "…7.0.x packages reached end of support on the same date."
    - Overview — paragraph 2: "Status of the .NET line as of October 2026:"
    - EF Core 7 — paragraph 1: "…release and is out of support together with .NET 7."
[^3]: .NET 7.0.20 – May 28, 2024 (CVE-2024-20672). https://github.com/dotnet/core/blob/main/release-notes/7.0/7.0.20/7.0.20.md
    - Overview — paragraph 1: "…as a security release (CVE-2024-20672, a macOS-only X.509 denial-of-service issue)."
[^4]: The history of C# — C# version 11. https://learn.microsoft.com/dotnet/csharp/whats-new/csharp-version-history#c-version-11 ; csharplang Language Version History. https://github.com/dotnet/csharplang/blob/main/Language-Version-History.md
    - Overview — paragraph 1: "…SDK** and Visual Studio 2022 17.4 on **November 8, 2022**."
    - Overview — paragraph 4: "…patterns, lambda natural types, `CallerArgumentExpression`, constant interpolated strings (C# 10)."
    - What's New in C# 11 and .NET 7 › Language (C# 11) — every item verified on LangVersion 10 vs 11 — paragraph 1: "Sources: the C# version history and the csharplang language-version history."
    - New in C# 11: Raw String Literals — paragraph 1: "…has its common indentation removed, measured from the closing delimiter."
    - New in C# 11: List Patterns — paragraph 1: "…var rest`) also require a range indexer or `Slice(int, int)`."
[^5]: Roslyn: Breaking changes in the compiler after .NET 6 (C# 11 / .NET 7). https://github.com/dotnet/roslyn/blob/main/docs/compilers/CSharp/Compiler%20Breaking%20Changes%20-%20DotNet%207.md
    - What's New in C# 11 and .NET 7 › Language (C# 11) — every item verified on LangVersion 10 vs 11 — paragraph 1: "Sources: the C# version history and the csharplang language-version history."
    - Naming and Code Style — paragraph 2: "…`file` as type names (`CS9029` for a type named `required`)."
    - New in C# 11 / .NET 7: `nint`, `nuint` and Numeric `IntPtr` — paragraph 1: "…is controlled by the **target framework** (`RuntimeFeature.NumericIntPtr`), not by `LangVersion`."
    - New in C# 11: List Patterns — bullet 4: "…Count: < 0 }` are reported as errors (subsumed patterns)."
    - New in C# 11: `ref` Fields and `scoped` — paragraph 2: "…arguments, which turns some formerly accepted span code into errors."
    - Removed / Changed in .NET 7 and C# 11 — Migration Notes — paragraph 1: "…reviewer of a freshly migrated code base should look for):"
[^6]: What's new in .NET 7. https://learn.microsoft.com/dotnet/core/whats-new/dotnet-7
    - What's New in C# 11 and .NET 7 › Runtime and libraries (.NET 7) — highlights for reviewers — bullet 1: "…`INumberBase<T>`, `IBinaryInteger<T>`, `IFloatingPoint<T>`, …) on all numeric types, plus `Int128`/`UInt128`."
    - What's New in C# 11 and .NET 7 › Runtime and libraries (.NET 7) — highlights for reviewers — bullet 2: "…span APIs `Regex.EnumerateMatches`, `Regex.Count`, `IsMatch(ReadOnlySpan<char>)`, analyzer `SYSLIB1045` suggests the generator."
    - What's New in C# 11 and .NET 7 › Runtime and libraries (.NET 7) — highlights for reviewers — bullet 4: "…(verified: the type is absent from the 7.0.20 base runtime)."
    - What's New in C# 11 and .NET 7 › Runtime and libraries (.NET 7) — highlights for reviewers — bullet 5: "`System.Formats.Tar` (`TarFile`, `TarReader`, `TarWriter`)."
    - What's New in C# 11 and .NET 7 › Runtime and libraries (.NET 7) — highlights for reviewers — bullet 6: "`Stream.ReadExactly`/`ReadAtLeast` (+ async)."
    - What's New in C# 11 and .NET 7 › Runtime and libraries (.NET 7) — highlights for reviewers — bullet 7: "`UnixFileMode`, `File.GetUnixFileMode`/`SetUnixFileMode`, `Directory.CreateDirectory(path, mode)`, `FileStreamOptions.UnixCreateMode`."
    - What's New in C# 11 and .NET 7 › Runtime and libraries (.NET 7) — highlights for reviewers — bullet 8: "**Microsecond/nanosecond** members on `DateTime`, `DateTimeOffset`, `TimeOnly`, `TimeSpan`."
    - What's New in C# 11 and .NET 7 › Runtime and libraries (.NET 7) — highlights for reviewers — bullet 9: "…P/Invoke source generator, **`ObjectDisposedException.ThrowIf`**, **`ArgumentException.ThrowIfNullOrEmpty`**, **`UnreachableException`**, **`Enumerable.Order`/`OrderDescending`**, **`Stopwatch.GetElapsedTime`**, **`MemoryCache.GetCurrentStatistics()`**, **`Activity.CurrentChanged`**."
    - What's New in C# 11 and .NET 7 › Runtime and libraries (.NET 7) — highlights for reviewers — bullet 10: "…by default** (`<TieredPGO>true</TieredPGO>`; on by default only from .NET 8)."
    - What's New in C# 11 and .NET 7 › Runtime and libraries (.NET 7) — highlights for reviewers — bullet 11: "…SDK 6.0.300 — `GlobalPackageReference` needs SDK 7.0.100+ / NuGet 6.4."
    - What's New in C# 11 and .NET 7 › Runtime and libraries (.NET 7) — highlights for reviewers — bullet 12: "ASP.NET Core gRPC JSON transcoding."
    - Performance — bullet 2: "…is enabled, so long-running loops in Tier-0 methods get optimized."
    - Tooling, Analyzers and Build Hygiene — bullet 3: "`CentralPackageTransitivePinningEnabled` pins vulnerable transitive packages."
    - Observability — bullet 3: "`Activity.CurrentChanged` and `Activity.EnumerateTagObjects()` are **New in .NET 7**."
[^7]: Breaking changes in .NET 7. https://learn.microsoft.com/dotnet/core/compatibility/7.0
    - Configuration and Options — bullet 4: "…values in .NET 7 rather than replacing them (breaking change)."
    - New in .NET 7: Regular Expressions — bullet 4: "…behaviours under `IgnoreCase` (breaking change "Regex patterns with ranges corrected")."
    - Removed / Changed in .NET 7 and C# 11 — Migration Notes — paragraph 1: "…reviewer of a freshly migrated code base should look for):"
[^8]: What's new in ASP.NET Core in .NET 7. https://learn.microsoft.com/aspnet/core/release-notes/aspnetcore-7.0
    - What's New in C# 11 and .NET 7 › Runtime and libraries (.NET 7) — highlights for reviewers — bullet 12: "ASP.NET Core gRPC JSON transcoding."
    - What's New in C# 11 and .NET 7 › Runtime and libraries (.NET 7) — highlights for reviewers — bullet 13: "…scheme becomes the default scheme, API controllers infer `[FromServices]` parameters."
    - New in C# 11: Generic Math and Static Abstract Interface Members — bullet 3: "…use for model binding (`static bool TryParse(string?, IFormatProvider?, out T)`)."
    - ASP.NET Core 7 Essentials › Minimal APIs: route groups, typed results, filters (New in .NET 7) — paragraph 2: "Other minimal-API facts for .NET 7:"
    - ASP.NET Core 7 Essentials › Middleware order — bullet 3: "…registered, .NET 7 uses it as the default scheme automatically;"
[^9]: Breaking changes in ASP.NET Core 7. https://learn.microsoft.com/aspnet/core/breaking-changes/7/overview
    - What's New in C# 11 and .NET 7 › Runtime and libraries (.NET 7) — highlights for reviewers — bullet 13: "…scheme becomes the default scheme, API controllers infer `[FromServices]` parameters."
    - Dependency Injection — bullet 3: "…registered in DI is no longer read from the body."
    - ASP.NET Core 7 Essentials › Middleware order — bullet 2: "…default in .NET 7 (breaking change) — configure endpoints explicitly."
    - ASP.NET Core 7 Essentials › Controllers — bullet 1: "…type is registered in DI are now inferred as `[FromServices]`."
    - ASP.NET Core 7 Essentials › Controllers — bullet 3: "Clients must send proper headers."
    - Removed / Changed in .NET 7 and C# 11 — Migration Notes — paragraph 1: "…reviewer of a freshly migrated code base should look for):"
[^10]: What's new in EF Core 7.0. https://learn.microsoft.com/ef/core/what-is-new/ef-core-7.0/whatsnew
    - What's New in C# 11 and .NET 7 › Runtime and libraries (.NET 7) — highlights for reviewers — bullet 14: "…scalar types, `ExecuteSql`, interceptors for materialization, stored-procedure mapping for inserts/updates/deletes."
    - EF Core 7 › New in EF Core 7: `ExecuteUpdate` / `ExecuteDelete` — paragraph 1: "Review rules for bulk operations:"
[^11]: Breaking changes in EF Core 7.0. https://learn.microsoft.com/ef/core/what-is-new/ef-core-7.0/breaking-changes
    - EF Core 7 — paragraph 1: "…7 targets `net6.0`, so it also runs on .NET 7."
    - EF Core 7 › EF Core 7 breaking changes to check during review — section heading
    - Removed / Changed in .NET 7 and C# 11 — Migration Notes — paragraph 1: "…reviewer of a freshly migrated code base should look for):"
[^12]: C# language versioning (defaults per target framework). https://learn.microsoft.com/dotnet/csharp/language-reference/configure-language-version
    - Version-Aware Review › How to determine the language version — paragraph 1: "…default `LangVersion` from the target framework, not from the SDK:"
    - Version-Aware Review › How to determine the language version — bullet 1: "…Microsoft does not support language versions above the TFM default."
[^13]: What's new in C# 12. https://learn.microsoft.com/dotnet/csharp/whats-new/csharp-12
    - Do Not Use or Recommend (not available in C# 11 / .NET 7) › C# 12 (.NET 8, November 2023) — section heading
[^14]: What's new in C# 13. https://learn.microsoft.com/dotnet/csharp/whats-new/csharp-13
    - Do Not Use or Recommend (not available in C# 11 / .NET 7) › C# 13 (.NET 9, November 2024) — section heading
[^15]: What's new in C# 14. https://learn.microsoft.com/dotnet/csharp/whats-new/csharp-14
    - Do Not Use or Recommend (not available in C# 11 / .NET 7) › C# 14 (.NET 10, November 2025) — section heading
[^16]: What's new in .NET 8 and breaking changes in .NET 8. https://learn.microsoft.com/dotnet/core/whats-new/dotnet-8/overview , https://learn.microsoft.com/dotnet/core/compatibility/8.0
    - Removed / Changed in .NET 7 and C# 11 — Migration Notes — paragraph 2: "…Core 8+ changes `Contains` translation with JSON/OPENJSON on SQL Server)."
[^17]: What's new in .NET 9 and breaking changes in .NET 9. https://learn.microsoft.com/dotnet/core/whats-new/dotnet-9/overview , https://learn.microsoft.com/dotnet/core/compatibility/9.0
    - Removed / Changed in .NET 7 and C# 11 — Migration Notes — paragraph 2: "…Core 8+ changes `Contains` translation with JSON/OPENJSON on SQL Server)."
[^18]: What's new in .NET 10 and breaking changes in .NET 10. https://learn.microsoft.com/dotnet/core/whats-new/dotnet-10/overview , https://learn.microsoft.com/dotnet/core/compatibility/10.0
    - Removed / Changed in .NET 7 and C# 11 — Migration Notes — paragraph 2: "…Core 8+ changes `Contains` translation with JSON/OPENJSON on SQL Server)."
[^19]: What's new in System.Text.Json in .NET 7. https://devblogs.microsoft.com/dotnet/system-text-json-in-dotnet-7/
    - What's New in C# 11 and .NET 7 › Runtime and libraries (.NET 7) — highlights for reviewers — bullet 3: "…(`DefaultJsonTypeInfoResolver` + `Modifiers`), polymorphism (`[JsonPolymorphic]`, `[JsonDerivedType]`), `required` members and `[JsonRequired]`."
[^20]: Regular expression improvements in .NET 7; .NET regular expression source generators. https://devblogs.microsoft.com/dotnet/regular-expression-improvements-in-dotnet-7/ , https://learn.microsoft.com/dotnet/standard/base-types/regular-expression-source-generators
    - What's New in C# 11 and .NET 7 › Runtime and libraries (.NET 7) — highlights for reviewers — bullet 2: "…span APIs `Regex.EnumerateMatches`, `Regex.Count`, `IsMatch(ReadOnlySpan<char>)`, analyzer `SYSLIB1045` suggests the generator."
    - New in .NET 7: Regular Expressions — bullet 1: "…`RightToLeft`) or a match timeout; never an unbounded backtracking regex."
[^21]: Rate limiting middleware in ASP.NET Core. https://learn.microsoft.com/aspnet/core/performance/rate-limit?view=aspnetcore-7.0
    - What's New in C# 11 and .NET 7 › Runtime and libraries (.NET 7) — highlights for reviewers — bullet 4: "…(verified: the type is absent from the 7.0.20 base runtime)."
    - ASP.NET Core 7 Essentials › New in .NET 7: rate limiting middleware — paragraph 1: "Set `429` and consider `OnRejected` to add `Retry-After`."
[^22]: Output caching middleware in ASP.NET Core (7.0 version: default policy, limits, ordering). https://learn.microsoft.com/aspnet/core/performance/caching/output?view=aspnetcore-7.0
    - ASP.NET Core 7 Essentials › Middleware order — bullet 1: "…`UseAuthorization`; otherwise cached content can be served to unauthorized callers."
    - ASP.NET Core 7 Essentials › New in .NET 7: output caching — paragraph 1: "The Redis output-cache store is .NET 8."
[^23]: Generic math. https://learn.microsoft.com/dotnet/standard/generics/math
    - What's New in C# 11 and .NET 7 › Runtime and libraries (.NET 7) — highlights for reviewers — bullet 1: "…`INumberBase<T>`, `IBinaryInteger<T>`, `IFloatingPoint<T>`, …) on all numeric types, plus `Int128`/`UInt128`."
    - New in C# 11: Generic Math and Static Abstract Interface Members — paragraph 1: "…(`INumber<T>`, `IAdditionOperators<,,>`, `IBinaryInteger<T>`, `IFloatingPointIeee754<T>`, …) on every primitive numeric type."
[^24]: C# compiler warning waves (CS8981, wave 7). https://learn.microsoft.com/dotnet/csharp/language-reference/compiler-messages/warning-waves
    - Naming and Code Style — paragraph 2: "`csc -warn:6` does not)."
[^25]: Obsolete features in .NET 5+ (SYSLIB0001–SYSLIB0047). https://learn.microsoft.com/dotnet/fundamentals/syslib-diagnostics/obsoletions-overview
    - Do Not Use or Recommend (not available in C# 11 / .NET 7) › Deprecated or removed in .NET 7 (do not keep using) — bullet 2: "…obsoletions with custom diagnostic IDs (verified as warnings on 7.0.410):"
[^26]: MSBuild properties for code analysis (AnalysisLevel, AnalysisMode). https://learn.microsoft.com/dotnet/core/project-sdk/msbuild-props#code-analysis-properties
    - Tooling, Analyzers and Build Hygiene — bullet 1: "…undisposed `MemoryStream` local, so do not rely on it alone)."
[^27]: Guidelines for using HttpClient. https://learn.microsoft.com/dotnet/fundamentals/networking/http/httpclient-guidelines
    - HttpClient — bullet 1: "…(typed or named clients) or one shared client with `SocketsHttpHandler.PooledConnectionLifetime`."
[^28]: Performance improvements in .NET 7; Performance improvements in .NET 8 (Dynamic PGO on by default). https://devblogs.microsoft.com/dotnet/performance_improvements_in_net_7/ , https://devblogs.microsoft.com/dotnet/performance-improvements-in-net-8/
    - What's New in C# 11 and .NET 7 › Runtime and libraries (.NET 7) — highlights for reviewers — bullet 10: "…by default** (`<TieredPGO>true</TieredPGO>`; on by default only from .NET 8)."
    - Performance — bullet 2: "…is enabled, so long-running loops in Tier-0 methods get optimized."
[^29]: Native AOT deployment overview. https://learn.microsoft.com/dotnet/core/deploying/native-aot/
    - Performance — bullet 3: "…Core), requires trimming-compatible code and reflection-free serialization (STJ source generation)."
[^30]: How to serialize properties of derived classes with System.Text.Json (polymorphism). https://learn.microsoft.com/dotnet/standard/serialization/system-text-json/polymorphism
    - System.Text.Json › New in .NET 7: polymorphism — paragraph 1: "STJ only allows the derived types you register."
[^31]: Customize a JSON contract. https://learn.microsoft.com/dotnet/standard/serialization/system-text-json/custom-contracts
    - System.Text.Json › New in .NET 7: contract customization — paragraph 1: "`TypeInfoResolverChain` and `WithAddedModifier` are .NET 8."
[^33]: Low-level struct improvements (ref fields, scoped, UnscopedRef) proposal. https://github.com/dotnet/csharplang/blob/main/proposals/csharp-11.0/low-level-struct-improvements.md
    - New in C# 11: `ref` Fields and `scoped` — paragraph 1: "`[UnscopedRef]` widens it."
[^34]: Required members proposal. https://github.com/dotnet/csharplang/blob/main/proposals/csharp-11.0/required-members.md
    - New in C# 11: Required Members — paragraph 1: "…in an object initializer; a constructor marked `[SetsRequiredMembers]` opts out."
[^35]: Integration tests in ASP.NET Core (WebApplicationFactory, `public partial class Program`). https://learn.microsoft.com/aspnet/core/test/integration-tests?view=aspnetcore-7.0
    - Testing — paragraph 1: "…at the end of `Program.cs` (or `InternalsVisibleTo` the test assembly)."
[^37]: SQL Queries — EF Core (FromSql in EF 7, SqlQuery unmapped types in EF 8). https://learn.microsoft.com/ef/core/querying/sql-queries
    - EF Core 7 › Raw SQL and injection — bullet 1: "`SqlQuery<T>` for unmapped entity-like types is EF Core 8."
[^38]: Breaking change: BinaryFormatter serialization APIs produce compiler errors (.NET 7). https://learn.microsoft.com/dotnet/core/compatibility/serialization/7.0/binaryformatter-apis-produce-errors
    - Do Not Use or Recommend (not available in C# 11 / .NET 7) › Deprecated or removed in .NET 7 (do not keep using) — bullet 1: "`error SYSLIB0011`), not just a warning."
    - Security › Deserialization and XML — bullet 1: "…`<NoWarn>SYSLIB0011</NoWarn>` or `EnableUnsafeBinaryFormatterSerialization` in a diff is a red flag."
[^39]: Auditing package dependencies for security vulnerabilities (NuGetAudit, .NET 8 SDK). https://learn.microsoft.com/nuget/concepts/auditing-packages
    - Tooling, Analyzers and Build Hygiene — bullet 4: "…7; automatic NuGet audit during restore (`NuGetAudit`) needs SDK 8+."
[^40]: GitLab advisory database — Moq 4.20.0/4.20.1 SponsorLink data collection, removed in 4.20.2. https://advisories.gitlab.com/pkg/nuget/moq/
    - Testing — bullet 3: "…git config); 4.20.2 removed it — flag pins to 4.20.0/4.20.1."
