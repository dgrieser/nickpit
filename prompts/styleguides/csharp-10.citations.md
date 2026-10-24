### Citations: C# 10 on .NET 6: The Complete Production Developer Guide

Sources for `csharp-10.md`. The guide carries no footnote markers; each entry lists where it was cited (section path, block, and the words just before the citation). Not sent to the model; kept for maintainers.

[^1]: .NET 6.0 release notes and `releases.json` (6.0.36 / SDK 6.0.428 and 6.0.136, November 12, 2024). https://github.com/dotnet/core/blob/main/release-notes/6.0/6.0.36/6.0.36.md , https://github.com/dotnet/core/blob/main/release-notes/6.0/releases.json
    - C# 10 on .NET 6: The Complete Production Developer Guide — quote block: "…and EF Core 6 ended on the same 6.0.36 version."
    - Version-Aware Review › How the version is determined — bullet 8: "…6 SDKs are 6.0.428 (6.0.4xx band) and 6.0.136 (6.0.1xx band)."
[^2]: .NET and .NET Core official support policy (release dates, latest patches, end-of-support dates; .NET 11 RC2 go-live). https://dotnet.microsoft.com/en-us/platform/support/policy/dotnet-core
    - C# 10 on .NET 6: The Complete Production Developer Guide — quote block: "…and EF Core 6 ended on the same 6.0.36 version."
    - C# 10 on .NET 6: The Complete Production Developer Guide — quote block: "…2025 Kestrel request-smuggling fix, CVE-2025-55315) went only to supported versions."
    - C# 10 on .NET 6: The Complete Production Developer Guide — quote block: "…2** (October 13, 2026) and is not a stable release."
[^3]: Microsoft Security Advisory CVE-2025-55315: .NET Security Feature Bypass Vulnerability (fixed October 14, 2025 for supported versions only). https://github.com/dotnet/aspnetcore/issues/64033
    - C# 10 on .NET 6: The Complete Production Developer Guide — quote block: "…2025 Kestrel request-smuggling fix, CVE-2025-55315) went only to supported versions."
[^4]: The history of C# — C# version 10 (feature list, preview features). https://learn.microsoft.com/en-us/dotnet/csharp/whats-new/csharp-version-history#c-version-10 ; What's new in C# 10. https://learn.microsoft.com/en-us/dotnet/csharp/whats-new/csharp-10
    - Overview — paragraph 1: "Its features:"
[^5]: What's new in .NET 6. https://learn.microsoft.com/en-us/dotnet/core/whats-new/dotnet-6
    - Overview — paragraph 3: ".NET 6 itself adds, among others:"
[^6]: What's new in ASP.NET Core 6.0. https://learn.microsoft.com/en-us/aspnet/core/release-notes/aspnetcore-6.0
    - Overview — paragraph 4: "…compiled models, pre-convention configuration (`ConfigureConventions`), `[Precision]`/`[Unicode]`/`[EntityTypeConfiguration]` attributes, and migration bundles."
[^7]: What's New in EF Core 6.0, and EF Core releases and planning (support dates, EF Core 10 requires .NET 10). https://learn.microsoft.com/en-us/ef/core/what-is-new/ef-core-6.0/whatsnew , https://learn.microsoft.com/en-us/ef/core/what-is-new/
    - Overview — paragraph 4: "…compiled models, pre-convention configuration (`ConfigureConventions`), `[Precision]`/`[Unicode]`/`[EntityTypeConfiguration]` attributes, and migration bundles."
    - EF Core 6 — paragraph 1: "EF Core 10 requires .NET 10."
[^8]: Central Package Management (NuGet). https://learn.microsoft.com/en-us/nuget/consume-packages/central-package-management
    - Project Setup and Build Hygiene › Recommended project settings — bullet 3: "`GlobalPackageReference` needs NuGet 6.4 (SDK 7.0.100 or later)."
[^9]: Roslyn: Compiler breaking changes in .NET 6 (C# 10). https://github.com/dotnet/roslyn/blob/main/docs/compilers/CSharp/Compiler%20Breaking%20Changes%20-%20DotNet%206.md
    - New in C# 10: Lambda Improvements › Breaking change: overload resolution with natural types — paragraph 1: "…chosen when code moves from C# 9 to C# 10:"
    - Removed / Changed in .NET 6 — Migration Notes (from .NET 5 / .NET Core 3.1) › C# 10 compiler breaking changes — section heading
[^10]: Breaking change: Partial and zero-byte reads in DeflateStream, GZipStream, and CryptoStream (.NET 6). https://learn.microsoft.com/en-us/dotnet/core/compatibility/core-libraries/6.0/partial-byte-reads-in-streams
    - Resource Management: `IDisposable` and `IAsyncDisposable` › `Stream` reads can be partial (changed in .NET 6) — paragraph 1: "…that ignores the return value of `Read`/`ReadAsync` silently truncates data."
[^11]: Performance Improvements in .NET 8 (dynamic PGO enabled by default). https://devblogs.microsoft.com/dotnet/performance-improvements-in-net-8/
    - Performance — bullet 7: "It is on by default from .NET 8."
[^12]: Breaking changes in .NET 6. https://learn.microsoft.com/en-us/dotnet/core/compatibility/6.0
    - Removed / Changed in .NET 6 — Migration Notes (from .NET 5 / .NET Core 3.1) — paragraph 1: "…moved to `net6.0`, or that was written with older assumptions."
[^13]: Breaking change: System.Drawing.Common only supported on Windows. https://learn.microsoft.com/en-us/dotnet/core/compatibility/core-libraries/6.0/system-drawing-common-windows-only
    - Removed / Changed in .NET 6 — Migration Notes (from .NET 5 / .NET Core 3.1) › Runtime and libraries — bullet 6: "Use ImageSharp, SkiaSharp or similar for cross-platform imaging."
[^14]: Breaking change: Unhandled exceptions from a BackgroundService. https://learn.microsoft.com/en-us/dotnet/core/compatibility/core-libraries/6.0/hosting-exception-handling
    - Removed / Changed in .NET 6 — Migration Notes (from .NET 5 / .NET Core 3.1) › Runtime and libraries — bullet 7: "In .NET 5 they were swallowed."
[^15]: Obsolete features in .NET 5+ (SYSLIB0001–SYSLIB00xx). https://learn.microsoft.com/en-us/dotnet/fundamentals/syslib-diagnostics/obsoletions-overview
    - Removed / Changed in .NET 6 — Migration Notes (from .NET 5 / .NET Core 3.1) › Runtime and libraries — bullet 11: "….NET 5 include SYSLIB0011 (`BinaryFormatter`), SYSLIB0006 (`Thread.Abort`) and SYSLIB0012 (`Assembly.CodeBase`)."
[^16]: Breaking changes in .NET (index for .NET 7, 8, 9 and 10) and EF Core breaking changes. https://learn.microsoft.com/en-us/dotnet/core/compatibility/breaking-changes , https://learn.microsoft.com/en-us/ef/core/what-is-new/ef-core-7.0/breaking-changes
    - Upgrading to .NET 10 (the recommended target) — paragraph 1: "Key steps and the most common review points:"
[^17]: C# compiler warning waves (wave 6: CS8826). https://learn.microsoft.com/en-us/dotnet/csharp/language-reference/compiler-messages/warning-waves
    - Analyzers and Warnings — bullet 4: "…method declarations with signature differences, for example different parameter names)."
[^18]: System.Text.Json source generation and supported types (DateOnly/TimeOnly from .NET 7). https://learn.microsoft.com/en-us/dotnet/standard/serialization/system-text-json/source-generation , https://learn.microsoft.com/en-us/dotnet/standard/serialization/system-text-json/supported-types
    - System.Text.Json (.NET 6) › Source generation (New in .NET 6) — bullet 1: "…compile time and throws `InvalidOperationException` at runtime (verified on 6.0.36)."
[^19]: Minimal APIs parameter binding (ASP.NET Core 6.0). https://learn.microsoft.com/en-us/aspnet/core/fundamentals/minimal-apis/parameter-binding?view=aspnetcore-6.0
    - ASP.NET Core 6 › Minimal hosting and minimal APIs (New in .NET 6) — paragraph 1: "…do and do not do (verified on ASP.NET Core 6.0.36):"
[^20]: System.Random thread safety. https://learn.microsoft.com/en-us/dotnet/api/system.random#thread-safety
    - Concurrency and Thread Safety — bullet 4: "…threads** can return 0 forever after concurrent use corrupts it."
[^21]: HttpClient guidelines for .NET (lifetime, DNS, IHttpClientFactory). https://learn.microsoft.com/en-us/dotnet/fundamentals/networking/http/httpclient-guidelines
    - HttpClient › Use `IHttpClientFactory` (typed or named clients) — bullet 1: "…pool, so this leads to socket exhaustion (`TIME_WAIT`) under load."
[^22]: DbContext lifetime, configuration, and initialization — avoiding DbContext threading issues. https://learn.microsoft.com/en-us/ef/core/dbcontext-configuration/
    - EF Core 6 › `DbContext` lifetime and thread safety — bullet 1: "**A `DbContext` is not thread-safe.**"
[^23]: What's New in EF Core 8.0 (DateOnly/TimeOnly on SQL Server). https://learn.microsoft.com/en-us/ef/core/what-is-new/ef-core-8.0/whatsnew
    - Dates, Times and Numbers › `DateOnly` and `TimeOnly` (New in .NET 6) — bullet 3: "…The SQL Server provider added them in EF Core 8."
[^24]: Breaking changes in EF Core 6.0. https://learn.microsoft.com/en-us/ef/core/what-is-new/ef-core-6.0/breaking-changes
    - Removed / Changed in .NET 6 — Migration Notes (from .NET 5 / .NET Core 3.1) › EF Core 6 breaking changes worth checking — section heading
