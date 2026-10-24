### Citations: C# 13 on .NET 9: The Complete Production Developer Guide

Sources for `csharp-13.md`. The guide carries no footnote markers; each entry lists where it was cited (section path, block, and the words just before the citation). Not sent to the model; kept for maintainers.

[^1]: .NET 9.0 release metadata (releases.json: 9.0.0 GA, 9.0.21 / SDK 9.0.319 and 9.0.122 on 2026-10-13, end of life 2026-11-10). https://builds.dotnet.microsoft.com/dotnet/release-metadata/9.0/releases.json
    - C# 13 on .NET 9: The Complete Production Developer Guide — quote block: "…is the matching SDK in the older 9.0.1xx feature band."
[^2]: .NET releases index (11.0.0-rc.2, 10.0.13 LTS until 2028-11-14, 9.0 and 8.0 EOL 2026-11-10). https://builds.dotnet.microsoft.com/dotnet/release-metadata/releases-index.json
    - C# 13 on .NET 9: The Complete Production Developer Guide — quote block: "…is the matching SDK in the older 9.0.1xx feature band."
    - C# 13 on .NET 9: The Complete Production Developer Guide — quote block: "…(11.0.0-rc.2, October 13, 2026) and is not a stable release."
[^3]: .NET Blog — .NET 8 and .NET 9 reach end of support on November 10, 2026 (STS extended to 24 months; .NET 9 released November 12, 2024). https://devblogs.microsoft.com/dotnet/dotnet-8-9-end-of-support/
    - C# 13 on .NET 9: The Complete Production Developer Guide — quote block: "…is known; after that there are no more security fixes."
[^4]: .NET and .NET Core Support Policy. https://dotnet.microsoft.com/platform/support/policy/dotnet-core
    - C# 13 on .NET 9: The Complete Production Developer Guide — quote block: "…is known; after that there are no more security fixes."
[^5]: What's new in C# 13. https://learn.microsoft.com/dotnet/csharp/whats-new/csharp-13
    - Overview — paragraph 1: "…C# 13; it became a stable feature in C# 14."
    - New in C# 13: `params` Collections — paragraph 1: "…concrete collection types with an accessible `Add` (for example `List<T>`)."
    - New in C# 13 / .NET 9: the `Lock` Type — paragraph 1: "…13 emits `using (x.EnterScope()) { … }` instead of `Monitor.Enter/Exit`."
    - New in C# 13: Small Syntax Features › `\e` escape — paragraph 1: "…**four** hex digits, so `"\x1bBold"` is `U+01BB` followed by `"old"`."
    - New in C# 13: `ref` Locals, `ref struct` Locals and `unsafe` in Async Methods and Iterators — paragraph 1: "…such a value alive across an `await` or `yield return`."
    - New in C# 13: `ref struct` Interfaces and `allows ref struct` — paragraph 1: "…enforces ref-safety rules on all values of that type parameter."
    - New in C# 13: Partial Properties and Indexers — paragraph 1: "…`[GeneratedRegex]` on a property, which is new in .NET 9."
    - New in C# 13: `[OverloadResolutionPriority]` — paragraph 1: "…over another among otherwise applicable candidates (higher wins, default 0)."
    - New in C# 13: Method Group Natural Type — paragraph 1: "…arity, unsatisfied constraints) scope by scope, like normal overload resolution."
    - The `field` Keyword Is Preview in C# 13 — Do Not Use — paragraph 1: "…Preview and *unsupported*.` It is stable only in C# 14."
[^6]: What's new in .NET libraries for .NET 9. https://learn.microsoft.com/dotnet/core/whats-new/dotnet-9/libraries
    - Overview — paragraph 2: "…lock, `ToHashSetAsync`, and turns forgotten migrations into a hard error."
    - What's New in C# 13 and .NET 9 › Libraries and runtime (.NET 9) — bullet 11: "CET shadow stacks on Windows."
    - New in C# 13: `ref struct` Interfaces and `allows ref struct` — bullet 5: "Feature 'allows ref struct constraint' …`."
    - New in C# 13: Partial Properties and Indexers — paragraph 1: "…`[GeneratedRegex]` on a property, which is new in .NET 9."
    - Async/Await and Concurrency › Thread safety primitives — bullet 2: "…enums and other primitive types (previously `where T : class`)."
    - Dependency Injection, Options and Configuration › Lifetimes and registration — bullet 8: "…`[ActivatorUtilitiesConstructor]` is always honoured; check types with multiple public constructors."
    - Logging and Observability — bullet 2: "…in classes with a primary constructor that takes an `ILogger`."
    - Trimming and Native AOT — bullet 4: "…support needs `TypeDescriptor.RegisterType<T>()` + `*FromRegisteredType` APIs (New in .NET 9)."
[^7]: What's new in ASP.NET Core in .NET 9. https://learn.microsoft.com/aspnet/core/release-notes/aspnetcore-9.0
    - Overview — paragraph 2: "…lock, `ToHashSetAsync`, and turns forgotten migrations into a hard error."
    - What's New in C# 13 and .NET 9 › ASP.NET Core 9, EF Core 9, SDK — bullet 1: "Swashbuckle is no longer in the Web API template."
    - ASP.NET Core 9 Essentials › Security-relevant middleware — bullet 3: "…by an `[AllowAnonymous]` "farther away" (e.g. on the controller class)."
[^8]: Microsoft.Extensions.Caching.Hybrid on NuGet (first stable version 9.3.0, 2025-03-11). https://www.nuget.org/packages/Microsoft.Extensions.Caching.Hybrid
    - Overview — paragraph 2: "…lock, `ToHashSetAsync`, and turns forgotten migrations into a hard error."
    - What's New in C# 13 and .NET 9 › ASP.NET Core 9, EF Core 9, SDK — bullet 2: "Native AOT/precompiled queries (experimental)."
[^9]: What's New in EF Core 9 (seeding, migration locking, ToHashSetAsync, experimental AOT/precompiled queries). https://learn.microsoft.com/ef/core/what-is-new/ef-core-9.0/whatsnew
    - Overview — paragraph 2: "…lock, `ToHashSetAsync`, and turns forgotten migrations into a hard error."
    - What's New in C# 13 and .NET 9 › ASP.NET Core 9, EF Core 9, SDK — bullet 2: "Native AOT/precompiled queries (experimental)."
    - EF Core 9 › Query and tracking pitfalls — bullet 10: "…EF Core 9 are explicitly experimental — not for production."
[^10]: C# language versioning (default language version per target framework). https://learn.microsoft.com/dotnet/csharp/language-reference/configure-language-version
    - Overview — bullet 1: "…on `net9.0` is unsupported (and requires the .NET 10 SDK)."
[^11]: What's new in the .NET 9 runtime (DATAS, CET, exceptions, JIT, feature switches, UnsafeAccessor generics). https://learn.microsoft.com/dotnet/core/whats-new/dotnet-9/runtime
    - Overview — bullet 4: "…heap sizes now track the live data size more closely."
    - What's New in C# 13 and .NET 9 › Libraries and runtime (.NET 9) — bullet 11: "CET shadow stacks on Windows."
    - Error Handling and Exceptions — bullet 4: "…throwing is still far more expensive than returning a result."
    - Performance — paragraph 1: "…exception-handling implementation, and Server GC with DATAS enabled by default."
    - Trimming and Native AOT — bullet 3: "…to annotated APIs inside the guarded branch do not warn)."
[^12]: What's new in the SDK and tooling for .NET 9 (Terminal Logger, parallel test, BuildChecks, new CA rules). https://learn.microsoft.com/dotnet/core/whats-new/dotnet-9/sdk
    - What's New in C# 13 and .NET 9 › ASP.NET Core 9, EF Core 9, SDK — bullet 3: "…rules (CA1514, CA1515, CA1871, CA1872, CA2022, CA2262–CA2265), NuGet audit sources."
    - Testing — bullet 6: "…share files, ports or databases across TFMs must be isolated."
    - Analyzers and Tooling — bullet 2: "…(`MaxResponseHeadersLength` is in KB), CA2263 (prefer generic overload), CA2264, CA2265."
[^13]: What's new in C# 14. https://learn.microsoft.com/dotnet/csharp/whats-new/csharp-14
    - Do Not Use or Recommend (Newer Than C# 13 / .NET 9) — paragraph 2: "**C# 14 (.NET 10) language features**"
    - The `field` Keyword Is Preview in C# 13 — Do Not Use — paragraph 1: "…Preview and *unsupported*.` It is stable only in C# 14."
[^14]: What's new in .NET 10 (libraries, ASP.NET Core 10, EF Core 10, SDK). https://learn.microsoft.com/dotnet/core/whats-new/dotnet-10/overview
    - Do Not Use or Recommend (Newer Than C# 13 / .NET 9) — paragraph 3: "**.NET 10 APIs (not in the .NET 9 BCL)**"
[^15]: BinaryFormatter migration guide / Breaking change: BinaryFormatter always throws. https://learn.microsoft.com/dotnet/core/compatibility/serialization/9.0/binaryformatter-removal
    - Do Not Use or Recommend (Newer Than C# 13 / .NET 9) › Deprecated or removed as of .NET 9 — flag when seen — bullet 1: "…remain so that code compiles (with `SYSLIB0011` as an error)."
    - Security › Serialization and deserialization — bullet 1: "…to `System.Text.Json`, `DataContractSerializer` with known types, MessagePack/protobuf with explicit contracts."
[^16]: .NET 9 obsoletions with custom IDs (SYSLIB0009, 0014, 0054–0057). https://learn.microsoft.com/dotnet/core/compatibility/core-libraries/9.0/obsolete-apis-with-custom-diagnostics
    - Do Not Use or Recommend (Newer Than C# 13 / .NET 9) › Deprecated or removed as of .NET 9 — flag when seen — bullet 2: "…X509Certificate2(byte[]/string …)` and `X509Certificate2Collection.Import(…)` loading overloads — `SYSLIB0057`; use `X509CertificateLoader.LoadCertificate*`/`LoadPkcs12*`."
    - Do Not Use or Recommend (Newer Than C# 13 / .NET 9) › Deprecated or removed as of .NET 9 — flag when seen — bullet 3: "…has no effect on `HttpClient`), `AuthenticationManager` (`SYSLIB0009`), `Assembly.LoadFrom(…, AssemblyHashAlgorithm)` (`SYSLIB0056`)."
    - Security › Certificates and cryptography — bullet 1: "…obsolete (`SYSLIB0057`) because sniffing was a source of security issues."
[^17]: Breaking changes in .NET 9. https://learn.microsoft.com/dotnet/core/compatibility/9.0
    - What's New in C# 13 and .NET 9 › Libraries and runtime (.NET 9) — bullet 14: "`TimeSpan.FromMilliseconds(int/long)` in expression trees is `CS0854` (see Numbers)."
    - Do Not Use or Recommend (Newer Than C# 13 / .NET 9) › Deprecated or removed as of .NET 9 — flag when seen — bullet 5: "`netstandard1.x` targets produce `NETSDK1215` in every project type."
    - New in C# 13: `params` Collections › Review notes for `params` collections — bullet 5: "…`CS0121` on .NET 9 (verified); write `string.Join(",", (string?[])stringValues)` or `stringValues.ToString()`."
    - Types: Classes, Records, Structs and Primary Constructors › Structs — bullet 3: "…and the default `Equals`/`GetHashCode` throw for them in .NET 9."
    - Dependency Injection, Options and Configuration › Options — bullet 5: "…`runtimeconfig.json`-style app runtime settings (for example `DOTNET_gcServer`) since .NET 9."
    - System.Text.Json › Defaults that surprise (verified on .NET 9.0.21) — bullet 8: "…property names such as `$type` are now unescaped before comparison."
    - Removed / Changed in .NET 9 — Migration Notes — paragraph 1: "…8 / C# 12 to .NET 9 / C# 13:"
[^18]: Breaking change: C# overload resolution prefers params span-type overloads. https://learn.microsoft.com/dotnet/core/compatibility/core-libraries/9.0/params-overloads
    - New in C# 13: `params` Collections › Overload resolution prefers the span overload — paragraph 1: "…others, so recompiling existing calls removes the hidden array allocation."
[^19]: System.Threading.Lock class. https://learn.microsoft.com/dotnet/api/system.threading.lock
    - New in C# 13 / .NET 9: the `Lock` Type — paragraph 1: "…13 emits `using (x.EnterScope()) { … }` instead of `Monitor.Enter/Exit`."
[^20]: C# compiler breaking changes since C# 12 (Roslyn, .NET 9). https://learn.microsoft.com/dotnet/csharp/whats-new/breaking-changes/compiler%20breaking%20changes%20-%20dotnet%209
    - New in C# 13: `ref` Locals, `ref struct` Locals and `unsafe` in Async Methods and Iterators — bullet 3: "…in C# 13 — add `unsafe` to the local function."
    - New in C# 13: Method Group Natural Type — paragraph 2: "…now produce an ambiguity error instead of an order-dependent pick)."
    - C# 13 Compiler Breaking Changes — paragraph 1: "…by compiling the same source with `LangVersion` 12 and 13:"
    - Removed / Changed in .NET 9 — Migration Notes — paragraph 1: "…8 / C# 12 to .NET 9 / C# 13:"
[^21]: Constraints on type parameters — allows ref struct. https://learn.microsoft.com/dotnet/csharp/programming-guide/generics/constraints-on-type-parameters#allows-ref-struct
    - New in C# 13: `ref struct` Interfaces and `allows ref struct` — paragraph 1: "…enforces ref-safety rules on all values of that type parameter."
[^22]: Auditing package dependencies for security vulnerabilities (NuGetAuditMode defaults). https://learn.microsoft.com/nuget/concepts/auditing-packages
    - Project Configuration — bullet 2: "Set `all` explicitly to see vulnerable transitive packages (`NU1901`–`NU1904`)."
[^23]: Breaking change: Floating point-to-integer conversions are saturating. https://learn.microsoft.com/dotnet/core/compatibility/jit/9.0/fp-to-integer
    - Numbers, Dates and GUIDs — bullet 2: "…or tests asserting it, change behaviour; validate ranges explicitly instead."
[^24]: Breaking change: FromKeyedServicesAttribute no longer injects non-keyed parameter. https://learn.microsoft.com/dotnet/core/compatibility/core-libraries/9.0/non-keyed-params
    - Dependency Injection, Options and Configuration › Lifetimes and registration — bullet 4: "…8.0.9) it no longer falls back to the non-keyed registration."
[^25]: Breaking changes: HttpClientFactory uses SocketsHttpHandler as primary handler; HttpClientFactory logging redacts header values by default. https://learn.microsoft.com/dotnet/core/compatibility/networking/9.0/default-handler , https://learn.microsoft.com/dotnet/core/compatibility/networking/9.0/redact-headers
    - Logging and Observability — bullet 3: "…EventSource events; re-enabling them (`RedactLoggedHeaders(_ => false)`) needs a justification."
    - HttpClient — bullet 4: "…casting the primary handler to `HttpClientHandler` in `ConfigurePrimaryHttpMessageHandler` callbacks fails."
[^26]: System.Text.Json — respect nullable annotations, required constructor parameters, JSON schema exporter. https://learn.microsoft.com/dotnet/standard/serialization/system-text-json/nullable-annotations , https://learn.microsoft.com/dotnet/standard/serialization/system-text-json/extract-schema
    - System.Text.Json › Defaults that surprise (verified on .NET 9.0.21) — bullet 7: "…ordered `JsonObject` APIs (`IndexOf`, `Insert`), and `JsonReaderOptions.AllowMultipleValues` for whitespace-separated documents."
[^27]: Breaking change: Forwarded headers middleware ignores X-Forwarded-* headers from unknown proxies (ASP.NET Core 8.0.17 / 9.0.6). https://learn.microsoft.com/aspnet/core/breaking-changes/8/forwarded-headers-unknown-proxies
    - ASP.NET Core 9 Essentials › Security-relevant middleware — bullet 1: "…not to clear `KnownProxies`/`KnownNetworks` or set `ASPNETCORE_FORWARDEDHEADERS_ENABLED=true` without network isolation."
[^28]: Breaking changes in EF Core 9. https://learn.microsoft.com/ef/core/what-is-new/ef-core-9.0/breaking-changes
    - EF Core 9 › Breaking changes to check during upgrade or review — section heading
    - EF Core 9 › Query and tracking pitfalls — bullet 9: "In EF 9, `EF.Constant`/`EF.Parameter` inside `EF.CompileQuery`/`EF.CompileAsyncQuery` throw `InvalidCastException` at runtime."
[^29]: Breaking change: Named mutexes default to user scope on Unix (.NET 8.0.32, 9.0.21, 10.0.13, 11 RC 2). https://learn.microsoft.com/dotnet/core/compatibility/core-libraries/11/named-mutexes-unix-default-scope
    - Security › Named mutexes on Unix — changed in 9.0.21 — paragraph 1: "The opt-out is the `System.Threading.Mutex.UnsafeDefaultMutexScopeGlobal` runtime configuration switch."
[^30]: Breaking change: New TimeSpan.From*() overloads that take integers; TimeSpan.FromMilliseconds API reference. https://learn.microsoft.com/dotnet/core/compatibility/core-libraries/9.0/timespan-from-overloads , https://learn.microsoft.com/dotnet/api/system.timespan.frommilliseconds
    - Numbers, Dates and GUIDs — bullet 1: "(Verified on .NET 8.0.32, 9.0.21 and 10.0.13.)"
