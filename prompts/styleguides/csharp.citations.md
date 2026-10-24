### Citations: C# Style Guide

Footnote sources for `csharp.md`. Not sent to the model; kept for maintainers.

[^1]: Announcing .NET 10. .NET Blog, November 11, 2025. https://devblogs.microsoft.com/dotnet/announcing-dotnet-10/
[^2]: .NET release metadata (releases-index.json and per-version releases.json: release dates, latest patches, SDKs, CVEs, end-of-support dates). https://builds.dotnet.microsoft.com/dotnet/release-metadata/releases-index.json ; https://github.com/dotnet/core/blob/main/release-notes/10.0/10.0.13/10.0.13.md
[^3]: .NET 11 Release Candidate 2 is now available. .NET Blog, October 13, 2026. https://devblogs.microsoft.com/dotnet/dotnet-11-rc-2/ ; What's new in C# 15. https://learn.microsoft.com/en-us/dotnet/csharp/whats-new/csharp-15
[^4]: C# language versioning — defaults per target framework. https://learn.microsoft.com/en-us/dotnet/csharp/language-reference/language-versioning
[^5]: Configure language version (LangVersion values, "using a C# version newer than the TFM's default isn't supported", warning against `latest`, `#error version`). https://learn.microsoft.com/en-us/dotnet/csharp/language-reference/configure-language-version
[^6]: C# compiler breaking changes since C# 13 (field keyword, first-class spans, `Enumerable.Reverse`). https://learn.microsoft.com/en-us/dotnet/csharp/whats-new/breaking-changes/compiler%20breaking%20changes%20-%20dotnet%2010
[^7]: .NET and .NET Core Support Policy (LTS/STS, 24-month STS since .NET 9). https://dotnet.microsoft.com/platform/support/policy/dotnet-core
[^8]: Microsoft .NET Framework lifecycle (4.8.1, 4.6.2 end of support January 12, 2027; 4.5.2/4.6/4.6.1 retired April 26, 2022; 3.5 SP1 until January 9, 2029). https://learn.microsoft.com/en-us/lifecycle/products/microsoft-net-framework ; https://dotnet.microsoft.com/platform/support/policy/dotnet-framework
[^9]: The history of C# (features per version, release dates and Visual Studio versions). https://learn.microsoft.com/en-us/dotnet/csharp/whats-new/csharp-version-history
[^10]: Roslyn Language Feature Status. https://github.com/dotnet/roslyn/blob/main/docs/Language%20Feature%20Status.md
[^11]: BinaryFormatter migration guide (obsolete in .NET 5, removed in .NET 9). https://learn.microsoft.com/en-us/dotnet/standard/serialization/binaryformatter-migration-guide/
[^12]: Obsolete features in .NET 5+ (SYSLIB0001 onward). https://learn.microsoft.com/en-us/dotnet/fundamentals/syslib-diagnostics/obsoletions-overview
[^13]: Auditing package dependencies for security vulnerabilities (NuGet Audit). https://learn.microsoft.com/en-us/nuget/concepts/auditing-packages
[^14]: Code analysis in .NET — AnalysisLevel and AnalysisMode. https://learn.microsoft.com/en-us/dotnet/fundamentals/code-analysis/overview
[^15]: Breaking change: System.Linq.AsyncEnumerable in .NET 10. https://learn.microsoft.com/en-us/dotnet/core/compatibility/core-libraries/10.0/asyncenumerable
[^16]: Behavior changes when comparing strings on .NET 5+ (ICU on Windows). https://learn.microsoft.com/en-us/dotnet/standard/base-types/string-comparison-net-5-plus ; .NET globalization and ICU. https://learn.microsoft.com/en-us/dotnet/core/extensions/globalization-icu
[^17]: Breaking changes in .NET 10 (configuration null values, SIGTERM handler, BackgroundService). https://learn.microsoft.com/en-us/dotnet/core/compatibility/10
[^18]: What's new in .NET libraries for .NET 10 (JsonSerializerOptions.Strict, AllowDuplicateProperties). https://learn.microsoft.com/en-us/dotnet/core/whats-new/dotnet-10/libraries
[^19]: What's new in ASP.NET Core in .NET 10 (minimal API validation, 401/403 for API endpoints). https://learn.microsoft.com/en-us/aspnet/core/release-notes/aspnetcore-10.0
[^20]: Microsoft Security Advisory CVE-2025-55315: .NET Security Feature Bypass Vulnerability (GHSA-5rrx-jjjq-q2r5, Kestrel request smuggling). https://github.com/dotnet/aspnetcore/security/advisories/GHSA-5rrx-jjjq-q2r5
[^21]: EF Core breaking changes per release (EF Core 3.0 … 10). https://learn.microsoft.com/en-us/ef/core/what-is-new/ef-core-10.0/breaking-changes ; https://learn.microsoft.com/en-us/ef/core/what-is-new/ef-core-8.0/breaking-changes ; https://learn.microsoft.com/en-us/ef/core/what-is-new/ef-core-3.x/breaking-changes
[^22]: xUnit.net v3 release notes; NUnit framework release notes (5.0.0, September 27, 2026). https://xunit.net/releases/ ; https://docs.nunit.org/articles/nunit/release-notes/framework.html
