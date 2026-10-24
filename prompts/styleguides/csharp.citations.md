### Citations: C# Style Guide

Sources for `csharp.md`. The guide carries no footnote markers; each entry lists where it was cited (section path, block, and the words just before the citation). Not sent to the model; kept for maintainers.

[^1]: Announcing .NET 10. .NET Blog, November 11, 2025. https://devblogs.microsoft.com/dotnet/announcing-dotnet-10/
    - C# Style Guide — quote block: "…SDK 10.0.402, and it is supported until **November 14, 2028**."
[^2]: .NET release metadata (releases-index.json and per-version releases.json: release dates, latest patches, SDKs, CVEs, end-of-support dates). https://builds.dotnet.microsoft.com/dotnet/release-metadata/releases-index.json ; https://github.com/dotnet/core/blob/main/release-notes/10.0/10.0.13/10.0.13.md
    - C# Style Guide — quote block: "…SDK 10.0.402, and it is supported until **November 14, 2028**."
    - C# Style Guide — quote block: "…a go-live license; general availability is planned for November 2026."
    - Release and Support Status (October 2026) — paragraph 1: "…22, 2026 (dates from Microsoft's release metadata and lifecycle pages):"
    - Release and Support Status (October 2026) — bullet 3: "…`global.json` pinned to an older patch is a security finding."
[^3]: .NET 11 Release Candidate 2 is now available. .NET Blog, October 13, 2026. https://devblogs.microsoft.com/dotnet/dotnet-11-rc-2/ ; What's new in C# 15. https://learn.microsoft.com/en-us/dotnet/csharp/whats-new/csharp-15
    - C# Style Guide — quote block: "…a go-live license; general availability is planned for November 2026."
    - Do Not Use or Recommend Without a Version Check — bullet 1: "…these for production code; .NET 11 is a release candidate."
[^4]: C# language versioning — defaults per target framework. https://learn.microsoft.com/en-us/dotnet/csharp/language-reference/language-versioning
    - Determine the Version First › Where the version comes from — paragraph 1: "…(verified with SDK 11.0.100-rc.2 and SDK 10.0.402, `dotnet msbuild -getProperty:LangVersion`):"
[^5]: Configure language version (LangVersion values, "using a C# version newer than the TFM's default isn't supported", warning against `latest`, `#error version`). https://learn.microsoft.com/en-us/dotnet/csharp/language-reference/configure-language-version
    - Determine the Version First › Rules for version-aware review — bullet 4: "`net10.0` adds an `Enumerable.Reverse(T[])` overload that restores the old binding."
    - Determine the Version First › Rules for version-aware review — bullet 5: "…own documentation warns against `latest`; flag `preview` in production code."
    - Determine the Version First › Rules for version-aware review — bullet 8: "…version (CS8304) — a quick way to confirm in doubt."
[^6]: C# compiler breaking changes since C# 13 (field keyword, first-class spans, `Enumerable.Reverse`). https://learn.microsoft.com/en-us/dotnet/csharp/whats-new/breaking-changes/compiler%20breaking%20changes%20-%20dotnet%2010
    - Determine the Version First › Rules for version-aware review — bullet 4: "`net10.0` adds an `Enumerable.Reverse(T[])` overload that restores the old binding."
    - Types: Classes, Records, Structs › The `field` keyword (C# 14) — paragraph 1: "…treat CS9258 as an error when moving to C# 14."
    - Upgrade Pitfalls Across Versions — Migration Notes — bullet 6: "`Rfc2898DeriveBytes` constructors obsolete (SYSLIB0060)."
    - Upgrade Pitfalls Across Versions — Migration Notes — bullet 7: "C# 12 prints `1,2,3`, C# 14 prints `3,2,1`)."
[^7]: .NET and .NET Core Support Policy (LTS/STS, 24-month STS since .NET 9). https://dotnet.microsoft.com/platform/support/policy/dotnet-core
    - Release and Support Status (October 2026) — paragraph 1: "…22, 2026 (dates from Microsoft's release metadata and lifecycle pages):"
    - Release and Support Status (October 2026) — bullet 2: "….NET 9 ends on the same day as .NET 8."
[^8]: Microsoft .NET Framework lifecycle (4.8.1, 4.6.2 end of support January 12, 2027; 4.5.2/4.6/4.6.1 retired April 26, 2022; 3.5 SP1 until January 9, 2029). https://learn.microsoft.com/en-us/lifecycle/products/microsoft-net-framework ; https://dotnet.microsoft.com/platform/support/policy/dotnet-framework
    - Release and Support Status (October 2026) — paragraph 1: "…22, 2026 (dates from Microsoft's release metadata and lifecycle pages):"
    - Release and Support Status (October 2026) — bullet 5: "Microsoft states that new development should use modern .NET."
[^9]: The history of C# (features per version, release dates and Visual Studio versions). https://learn.microsoft.com/en-us/dotnet/csharp/whats-new/csharp-version-history
    - Release and Support Status (October 2026) — bullet 1: "….NET Core 3.0 and Visual Studio 2019 16.3 (September 2019)."
    - C# Language Features by Version — paragraph 1: "…table to decide whether a suggestion compiles in the project."
[^10]: Roslyn Language Feature Status. https://github.com/dotnet/roslyn/blob/main/docs/Language%20Feature%20Status.md
    - C# Language Features by Version — paragraph 1: "…table to decide whether a suggestion compiles in the project."
[^11]: BinaryFormatter migration guide (obsolete in .NET 5, removed in .NET 9). https://learn.microsoft.com/en-us/dotnet/standard/serialization/binaryformatter-migration-guide/
    - Deprecated, Obsolete and Removed APIs — paragraph 1: "…the detected version (with the version where they became obsolete):"
    - Security › Deserialization — bullet 1: "On .NET 9+ `BinaryFormatter` always throws."
[^12]: Obsolete features in .NET 5+ (SYSLIB0001 onward). https://learn.microsoft.com/en-us/dotnet/fundamentals/syslib-diagnostics/obsoletions-overview
    - Deprecated, Obsolete and Removed APIs — paragraph 1: "…the detected version (with the version where they became obsolete):"
[^13]: Auditing package dependencies for security vulnerabilities (NuGet Audit). https://learn.microsoft.com/en-us/nuget/concepts/auditing-packages
    - Project Setup, Build and Analyzers › Baseline SDK-style project — bullet 4: "Do not suppress NU1901–NU1904 globally."
[^14]: Code analysis in .NET — AnalysisLevel and AnalysisMode. https://learn.microsoft.com/en-us/dotnet/fundamentals/code-analysis/overview
    - Project Setup, Build and Analyzers › Baseline SDK-style project — bullet 6: "`-none`, `-default`, `-minimum`, `-recommended`, `-all`."
[^15]: Breaking change: System.Linq.AsyncEnumerable in .NET 10. https://learn.microsoft.com/en-us/dotnet/core/compatibility/core-libraries/10.0/asyncenumerable
    - Async/Await › Async streams (C# 8, .NET Core 3.0) — paragraph 1: "…upgrade to 7.0.0, and use `<ExcludeAssets>compile</ExcludeAssets>` when it arrives transitively."
[^16]: Behavior changes when comparing strings on .NET 5+ (ICU on Windows). https://learn.microsoft.com/en-us/dotnet/standard/base-types/string-comparison-net-5-plus ; .NET globalization and ICU. https://learn.microsoft.com/en-us/dotnet/core/extensions/globalization-icu
    - Strings, Culture and Encoding — paragraph 1: "…between developer machines and containers, or after a base-image update."
    - Upgrade Pitfalls Across Versions — Migration Notes — bullet 2: "…version can change such results again; ordinal comparison is immune."
[^17]: Breaking changes in .NET 10 (configuration null values, SIGTERM handler, BackgroundService). https://learn.microsoft.com/en-us/dotnet/core/compatibility/10
    - Dependency Injection, Options and Hosting › Options pattern — bullet 3: "….NET 10's binder writes explicit JSON `null` values into properties)."
    - Dependency Injection, Options and Hosting › Hosted and background services — bullet 2: "….NET 10 the whole method runs on a background thread."
    - Dependency Injection, Options and Hosting › Hosted and background services — bullet 5: "Use the Generic Host or `PosixSignalRegistration` (.NET 6)."
    - Upgrade Pitfalls Across Versions — Migration Notes — bullet 6: "`Rfc2898DeriveBytes` constructors obsolete (SYSLIB0060)."
[^18]: What's new in .NET libraries for .NET 10 (JsonSerializerOptions.Strict, AllowDuplicateProperties). https://learn.microsoft.com/en-us/dotnet/core/whats-new/dotnet-10/libraries
    - JSON Serialization — bullet 3: "…(one component checks the first `"role"`, another uses the last)."
[^19]: What's new in ASP.NET Core in .NET 10 (minimal API validation, 401/403 for API endpoints). https://learn.microsoft.com/en-us/aspnet/core/release-notes/aspnetcore-10.0
    - ASP.NET Core — paragraph 1: "`IExceptionHandler` .NET 8; built-in minimal API validation (`AddValidation()`) .NET 10."
    - ASP.NET Core — bullet 15: "SPAs that relied on following redirects need updating."
[^20]: Microsoft Security Advisory CVE-2025-55315: .NET Security Feature Bypass Vulnerability (GHSA-5rrx-jjjq-q2r5, Kestrel request smuggling). https://github.com/dotnet/aspnetcore/security/advisories/GHSA-5rrx-jjjq-q2r5
    - ASP.NET Core — bullet 14: "…unsupported versions (.NET 6/7, .NET Core 3.1) received no fix."
[^21]: EF Core breaking changes per release (EF Core 3.0 … 10). https://learn.microsoft.com/en-us/ef/core/what-is-new/ef-core-10.0/breaking-changes ; https://learn.microsoft.com/en-us/ef/core/what-is-new/ef-core-8.0/breaking-changes ; https://learn.microsoft.com/en-us/ef/core/what-is-new/ef-core-3.x/breaking-changes
    - EF Core — paragraph 1: "…`LeftJoin`/`RightJoin` translation and `ExecuteUpdate` with statement lambdas (EF Core 10)."
    - EF Core — bullet 10: "…large lists can hit parameter limits (2,100 on SQL Server)."
    - Upgrade Pitfalls Across Versions — Migration Notes — bullet 8: "Read the breaking-changes page of every major version."
[^22]: xUnit.net v3 release notes; NUnit framework release notes (5.0.0, September 27, 2026). https://xunit.net/releases/ ; https://docs.nunit.org/articles/nunit/release-notes/framework.html
    - Testing — bullet 2: "…changed `Assert.ThrowsAsync` to return a `Task` that must be awaited."
    - Testing — bullet 3: "…v3 APIs; v3 test projects are executables and offer `TestContext.Current.CancellationToken`."
