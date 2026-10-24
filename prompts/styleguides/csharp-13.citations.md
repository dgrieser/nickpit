### Citations: C# 13 on .NET 9: The Complete Production Developer Guide

Footnote sources for `csharp-13.md`. Not sent to the model; kept for maintainers.

[^1]: .NET 9.0 release metadata (releases.json: 9.0.0 GA, 9.0.21 / SDK 9.0.319 and 9.0.122 on 2026-10-13, end of life 2026-11-10). https://builds.dotnet.microsoft.com/dotnet/release-metadata/9.0/releases.json
[^2]: .NET releases index (11.0.0-rc.2, 10.0.13 LTS until 2028-11-14, 9.0 and 8.0 EOL 2026-11-10). https://builds.dotnet.microsoft.com/dotnet/release-metadata/releases-index.json
[^3]: .NET Blog — .NET 8 and .NET 9 reach end of support on November 10, 2026 (STS extended to 24 months; .NET 9 released November 12, 2024). https://devblogs.microsoft.com/dotnet/dotnet-8-9-end-of-support/
[^4]: .NET and .NET Core Support Policy. https://dotnet.microsoft.com/platform/support/policy/dotnet-core
[^5]: What's new in C# 13. https://learn.microsoft.com/dotnet/csharp/whats-new/csharp-13
[^6]: What's new in .NET libraries for .NET 9. https://learn.microsoft.com/dotnet/core/whats-new/dotnet-9/libraries
[^7]: What's new in ASP.NET Core in .NET 9. https://learn.microsoft.com/aspnet/core/release-notes/aspnetcore-9.0
[^8]: Microsoft.Extensions.Caching.Hybrid on NuGet (first stable version 9.3.0, 2025-03-11). https://www.nuget.org/packages/Microsoft.Extensions.Caching.Hybrid
[^9]: What's New in EF Core 9 (seeding, migration locking, ToHashSetAsync, experimental AOT/precompiled queries). https://learn.microsoft.com/ef/core/what-is-new/ef-core-9.0/whatsnew
[^10]: C# language versioning (default language version per target framework). https://learn.microsoft.com/dotnet/csharp/language-reference/configure-language-version
[^11]: What's new in the .NET 9 runtime (DATAS, CET, exceptions, JIT, feature switches, UnsafeAccessor generics). https://learn.microsoft.com/dotnet/core/whats-new/dotnet-9/runtime
[^12]: What's new in the SDK and tooling for .NET 9 (Terminal Logger, parallel test, BuildChecks, new CA rules). https://learn.microsoft.com/dotnet/core/whats-new/dotnet-9/sdk
[^13]: What's new in C# 14. https://learn.microsoft.com/dotnet/csharp/whats-new/csharp-14
[^14]: What's new in .NET 10 (libraries, ASP.NET Core 10, EF Core 10, SDK). https://learn.microsoft.com/dotnet/core/whats-new/dotnet-10/overview
[^15]: BinaryFormatter migration guide / Breaking change: BinaryFormatter always throws. https://learn.microsoft.com/dotnet/core/compatibility/serialization/9.0/binaryformatter-removal
[^16]: .NET 9 obsoletions with custom IDs (SYSLIB0009, 0014, 0054–0057). https://learn.microsoft.com/dotnet/core/compatibility/core-libraries/9.0/obsolete-apis-with-custom-diagnostics
[^17]: Breaking changes in .NET 9. https://learn.microsoft.com/dotnet/core/compatibility/9.0
[^18]: Breaking change: C# overload resolution prefers params span-type overloads. https://learn.microsoft.com/dotnet/core/compatibility/core-libraries/9.0/params-overloads
[^19]: System.Threading.Lock class. https://learn.microsoft.com/dotnet/api/system.threading.lock
[^20]: C# compiler breaking changes since C# 12 (Roslyn, .NET 9). https://learn.microsoft.com/dotnet/csharp/whats-new/breaking-changes/compiler%20breaking%20changes%20-%20dotnet%209
[^21]: Constraints on type parameters — allows ref struct. https://learn.microsoft.com/dotnet/csharp/programming-guide/generics/constraints-on-type-parameters#allows-ref-struct
[^22]: Auditing package dependencies for security vulnerabilities (NuGetAuditMode defaults). https://learn.microsoft.com/nuget/concepts/auditing-packages
[^23]: Breaking change: Floating point-to-integer conversions are saturating. https://learn.microsoft.com/dotnet/core/compatibility/jit/9.0/fp-to-integer
[^24]: Breaking change: FromKeyedServicesAttribute no longer injects non-keyed parameter. https://learn.microsoft.com/dotnet/core/compatibility/core-libraries/9.0/non-keyed-params
[^25]: Breaking changes: HttpClientFactory uses SocketsHttpHandler as primary handler; HttpClientFactory logging redacts header values by default. https://learn.microsoft.com/dotnet/core/compatibility/networking/9.0/default-handler , https://learn.microsoft.com/dotnet/core/compatibility/networking/9.0/redact-headers
[^26]: System.Text.Json — respect nullable annotations, required constructor parameters, JSON schema exporter. https://learn.microsoft.com/dotnet/standard/serialization/system-text-json/nullable-annotations , https://learn.microsoft.com/dotnet/standard/serialization/system-text-json/extract-schema
[^27]: Breaking change: Forwarded headers middleware ignores X-Forwarded-* headers from unknown proxies (ASP.NET Core 8.0.17 / 9.0.6). https://learn.microsoft.com/aspnet/core/breaking-changes/8/forwarded-headers-unknown-proxies
[^28]: Breaking changes in EF Core 9. https://learn.microsoft.com/ef/core/what-is-new/ef-core-9.0/breaking-changes
[^29]: Breaking change: Named mutexes default to user scope on Unix (.NET 8.0.32, 9.0.21, 10.0.13, 11 RC 2). https://learn.microsoft.com/dotnet/core/compatibility/core-libraries/11/named-mutexes-unix-default-scope
[^30]: Breaking change: New TimeSpan.From*() overloads that take integers; TimeSpan.FromMilliseconds API reference. https://learn.microsoft.com/dotnet/core/compatibility/core-libraries/9.0/timespan-from-overloads , https://learn.microsoft.com/dotnet/api/system.timespan.frommilliseconds
