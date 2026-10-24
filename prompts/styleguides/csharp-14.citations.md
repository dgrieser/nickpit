### Citations: C# 14 on .NET 10 (LTS): The Complete Production Developer Guide

Sources for `csharp-14.md`. The guide carries no footnote markers; each entry lists where it was cited (section path, block, and the words just before the citation). Not sent to the model; kept for maintainers.

[^1]: Announcing .NET 10. .NET Blog, November 11, 2025. https://devblogs.microsoft.com/dotnet/announcing-dotnet-10/
    - C# 14 on .NET 10 (LTS): The Complete Production Developer Guide — quote block: "…released **November 11, 2025** (SDK 10.0.100, Visual Studio 2026 18.0)."
[^2]: What's new in .NET 10 (overview). https://learn.microsoft.com/en-us/dotnet/core/whats-new/dotnet-10/overview
    - C# 14 on .NET 10 (LTS): The Complete Production Developer Guide — quote block: "…released **November 11, 2025** (SDK 10.0.100, Visual Studio 2026 18.0)."
[^3]: .NET 10.0 release metadata (releases.json: 10.0.0 on 2025-11-11 … 10.0.13 on 2026-10-13, SDKs, CVE lists, EOL date). https://builds.dotnet.microsoft.com/dotnet/release-metadata/10.0/releases.json ; release notes: https://github.com/dotnet/core/blob/main/release-notes/10.0/10.0.13/10.0.13.md
    - C# 14 on .NET 10 (LTS): The Complete Production Developer Guide — quote block: "…44 CVEs, so anything older than 10.0.13 is a finding."
    - C# 14 on .NET 10 (LTS): The Complete Production Developer Guide — quote block: "…release, in the *active* phase, supported until **November 14, 2028**."
    - Version-Aware Review — bullet 5: "…rotate the key ring if an affected version was deployed."
[^4]: .NET and .NET Core Support Policy / release index (support phases and end-of-support dates for .NET 8, 9, 10). https://dotnet.microsoft.com/platform/support/policy/dotnet-core ; https://builds.dotnet.microsoft.com/dotnet/release-metadata/releases-index.json
    - C# 14 on .NET 10 (LTS): The Complete Production Developer Guide — quote block: "…release, in the *active* phase, supported until **November 14, 2028**."
    - C# 14 on .NET 10 (LTS): The Complete Production Developer Guide — quote block: "…(STS) both reach end of support on **November 10, 2026**."
[^5]: .NET 11 Release Candidate 2 is now available. .NET Blog, October 13, 2026. https://devblogs.microsoft.com/dotnet/dotnet-11-rc-2/
    - C# 14 on .NET 10 (LTS): The Complete Production Developer Guide — quote block: "GA is planned for November 2026."
    - What's New in C# 14 › 10. File-based apps and `#:` directives [New in .NET 10 SDK] — paragraph 1: "…quoted directive values arrive only with the .NET 11 SDK."
    - Do Not Use or Recommend (C# 15 / .NET 11) — paragraph 1: "…of the following, and the reviewer must never suggest them:"
[^6]: What's new in C# 14. Microsoft Learn. https://learn.microsoft.com/en-us/dotnet/csharp/whats-new/csharp-14
    - C# 14 on .NET 10 (LTS): The Complete Production Developer Guide — quote block: "…no `LangVersion`, because C# 14 is the default for `net10.0`."
    - Overview and Global Truths — paragraph 1: "…in expression trees, and the `#:` directives of **file-based apps**."
    - What's New in C# 14 › 1. Extension members (`extension` blocks) [New in C# 14] ★★★ — paragraph 1: "…declares **static** members of the extended type, including **user-defined operators**."
    - What's New in C# 14 › 2. The `field` keyword [New in C# 14] ★★★ — paragraph 1: "…so validation or normalization no longer needs a hand-written field."
    - What's New in C# 14 › 3. Null-conditional assignment [New in C# 14] ★★ — paragraph 1: "…right-hand side is evaluated **only if** the receiver is non-null."
    - What's New in C# 14 › 4. `nameof` with unbound generic types [New in C# 14] ★ — paragraph 1: "Before C# 14 a closed type (`nameof(List<int>)`) was required."
    - What's New in C# 14 › 5. First-class span conversions [New in C# 14] ★★★ — paragraph 1: "…that participate in **extension-method receiver lookup** and **generic type inference**."
    - What's New in C# 14 › 6. Modifiers on simple lambda parameters [New in C# 14] ★ — paragraph 1: "`params` still requires explicit types."
    - What's New in C# 14 › 7. Partial constructors and partial events [New in C# 14] ★ — paragraph 1: "…`this(...)`/`base(...)` initializer; the implementing event declaration must have `add`/`remove` accessors."
[^7]: C# compiler breaking changes since C# 13 (Roslyn "Compiler Breaking Changes - DotNet 10"). https://learn.microsoft.com/en-us/dotnet/csharp/whats-new/breaking-changes/compiler%20breaking%20changes%20-%20dotnet%2010
    - Overview and Global Truths — paragraph 1: "…in expression trees, and the `#:` directives of **file-based apps**."
    - Overview and Global Truths — bullet 4: "…is unsupported and can change overload resolution (see `Enumerable.Reverse` below)."
    - What's New in C# 14 › 1. Extension members (`extension` blocks) [New in C# 14] ★★★ — bullet 4: "…type parameter named `extension` no longer compiles (escape as `@extension`)."
    - What's New in C# 14 › 2. The `field` keyword [New in C# 14] ★★★ — paragraph 3: "…reports it as **CS9273** (the breaking-changes document still says CS9272)."
    - What's New in C# 14 › 5. First-class span conversions [New in C# 14] ★★★ — paragraph 3: "Callers can force the old binding with `o.AsEnumerable()`."
    - What's New in C# 14 › 5. First-class span conversions [New in C# 14] ★★★ — bullet 3: "…the new `Enumerable.Reverse<T>(this T[])` overload wins and behavior is unchanged."
    - What's New in C# 14 › 6. Modifiers on simple lambda parameters [New in C# 14] ★ — paragraph 2: "…scoped s) => …` with a type named `scoped` breaks)."
    - What's New in C# 14 › 7. Partial constructors and partial events [New in C# 14] ★ — paragraph 2: "…be used as a method return type without escaping (`@partial`)."
    - What's New in C# 14 › 7. Partial constructors and partial events [New in C# 14] ★ — paragraph 2: "…they were not), which can change which implementation is called."
    - What's New in C# 14 › 11. Other compiler changes that surface in review — bullet 1: "C# 13 gives no warning."
    - What's New in C# 14 › 11. Other compiler changes that surface in review — bullet 2: "…compiler-generated enumerator now returns `false` instead of resuming user code."
    - What's New in C# 14 › 11. Other compiler changes that surface in review — bullet 3: "**`[Obsolete]` on pattern-based `DisposeAsync`/`Dispose` is now reported in `foreach`/`await foreach`.**"
    - What's New in C# 14 › Breaking changes in C# 14 (must know for upgrades) — paragraph 1: "Roslyn "Compiler breaking changes since C# 13"."
[^8]: What's new in .NET libraries for .NET 10. https://learn.microsoft.com/en-us/dotnet/core/whats-new/dotnet-10/libraries
    - Overview and Global Truths — paragraph 2: "…and returns **401/403 instead of login redirects** for API endpoints."
    - What's New in C# 14 › 1. Extension members (`extension` blocks) [New in C# 14] ★★★ — bullet 5: "…(the .NET 10 `System.Numerics.Tensors` package uses extension operators this way)."
    - What's New in .NET 10 for Reviewers › New BCL APIs worth recommending [New in .NET 10] — paragraph 1: "…to exist on `net10.0` and to be absent on `net9.0`:"
    - What's New in .NET 10 for Reviewers › New BCL APIs worth recommending [New in .NET 10] — paragraph 3: "…a fallback; never assume PQC works because the API compiles."
    - Logging and Observability › Metrics and tracing — bullet 1: "…with OpenTelemetry. .NET 10 adds `ActivitySourceOptions` and telemetry schema URLs."
    - System.Text.Json › Defaults that surprise reviewers (verified on .NET 10) — bullet 1: "…(one component validates the first `"role"`, another uses the last)."
    - System.Text.Json › Source generation (required for trimming/AOT, faster startup) — paragraph 1: "…o.SerializerOptions.TypeInfoResolverChain.Insert(0, AppJsonContext.Default))`. .NET 10 allows `ReferenceHandler = JsonKnownReferenceHandler.Preserve` on `[JsonSourceGenerationOptions]`."
[^9]: Breaking changes in .NET 10. https://learn.microsoft.com/en-us/dotnet/core/compatibility/10
    - Overview and Global Truths — paragraph 2: "…and returns **401/403 instead of login redirects** for API endpoints."
    - What's New in C# 14 › 10. File-based apps and `#:` directives [New in .NET 10 SDK] — paragraph 1: "…quoted directive values arrive only with the .NET 11 SDK."
    - What's New in .NET 10 for Reviewers › Runtime and hosting behavior changes [New in .NET 10] ★★★ — paragraph 5: "**Other runtime/library behavior changes** (all .NET 10):"
    - What's New in .NET 10 for Reviewers › SDK and tooling changes [New in .NET 10] — bullet 2: "`dotnet new sln` creates **`.slnx`** files by default."
    - What's New in .NET 10 for Reviewers › SDK and tooling changes [New in .NET 10] — bullet 3: "NU1510 flags direct references that the framework already provides."
    - What's New in .NET 10 for Reviewers › SDK and tooling changes [New in .NET 10] — bullet 8: "…sources reject plain HTTP; .NET tool packages can be RuntimeIdentifier-specific."
    - Dependency Injection, Options, Configuration and Hosting › Lifetimes and captive dependencies — bullet 5: "…S3Store>("s3")` + `[FromKeyedServices("s3")]`. .NET 10 fixed `KeyedService.AnyKey` behavior in `GetKeyedService`/`GetKeyedServices`."
    - Logging and Observability › Source-generated logging — bullet 6: "…console formatter no longer duplicates the message in the output."
    - Security › Other injection and parsing risks — bullet 7: "…KB limit in .NET 10 — validate input lengths explicitly."
    - Testing › xUnit.net v3 — bullet 9: "`coverlet.collector` or `Microsoft.Testing.Extensions.CodeCoverage`; in .NET 10 `EnableDynamicNativeInstrumentation` defaults to `false`."
    - Trimming and Native AOT — bullet 6: "`PublishTrimmed` disables HTTP/3 by default in .NET 10."
[^10]: What's new in ASP.NET Core in .NET 10. https://learn.microsoft.com/en-us/aspnet/core/release-notes/aspnetcore-10.0
    - Overview and Global Truths — paragraph 2: "…and returns **401/403 instead of login redirects** for API endpoints."
    - What's New in .NET 10 for Reviewers › ASP.NET Core 10 highlights [New in .NET 10] — bullet 1: "Without `AddValidation()` nothing is validated."
    - What's New in .NET 10 for Reviewers › ASP.NET Core 10 highlights [New in .NET 10] — bullet 2: "…endpoint returns `401`, while an HTML endpoint still returns `302`."
    - What's New in .NET 10 for Reviewers › ASP.NET Core 10 highlights [New in .NET 10] — bullet 7: "…through `PipeReader`; custom `JsonConverter`s that ignore `Utf8JsonReader.HasValueSequence` can now fail."
    - What's New in .NET 10 for Reviewers › ASP.NET Core 10 highlights [New in .NET 10] — bullet 10: "…Patch documents from clients must still be authorization-checked per path."
    - Logging and Observability › Metrics and tracing — bullet 3: "…10 emits new metrics for authentication/authorization, Identity and memory pools."
    - ASP.NET Core 10 Minimal APIs — bullet 1: "…generator only discovers types in the assembly that calls `AddValidation()`."
[^11]: The history of C# (C# 9–13 feature lists). https://learn.microsoft.com/en-us/dotnet/csharp/whats-new/csharp-version-history
    - Overview and Global Truths — paragraph 3: "…`ref struct` interfaces and `allows ref struct`, `[OverloadResolutionPriority]` (C# 13)."
[^12]: Extension members — feature specification and `extension` keyword reference. https://learn.microsoft.com/en-us/dotnet/csharp/language-reference/proposals/csharp-14.0/extensions ; https://learn.microsoft.com/en-us/dotnet/csharp/language-reference/keywords/extension
    - What's New in C# 14 › 1. Extension members (`extension` blocks) [New in C# 14] ★★★ — paragraph 1: "…declares **static** members of the extended type, including **user-defined operators**."
[^13]: First-class Span types — feature specification. https://learn.microsoft.com/en-us/dotnet/csharp/language-reference/proposals/csharp-14.0/first-class-span-types
    - What's New in C# 14 › 5. First-class span conversions [New in C# 14] ★★★ — paragraph 1: "…that participate in **extension-method receiver lookup** and **generic type inference**."
[^14]: User-defined compound assignment operators — feature specification. https://learn.microsoft.com/en-us/dotnet/csharp/language-reference/proposals/csharp-14.0/user-defined-compound-assignment
    - What's New in C# 14 › 8. User-defined compound assignment and increment operators [New in C# 14] ★★ — paragraph 1: "…place** instead of producing a new instance through `operator +`."
[^15]: Roslyn Language Feature Status (C# 14 and C# 15 feature tables). https://github.com/dotnet/roslyn/blob/main/docs/Language%20Feature%20Status.md
    - What's New in C# 14 › 9. Optional and named arguments in expression trees [New in C# 14] — paragraph 1: "…14 compiles `() => M.F(1)` as `() => F(1, 10)`."
[^16]: What's new in the SDK and tooling for .NET 10. https://learn.microsoft.com/en-us/dotnet/core/whats-new/dotnet-10/sdk
    - What's New in C# 14 › 10. File-based apps and `#:` directives [New in .NET 10 SDK] — paragraph 1: "`dotnet publish app.cs` produces a native executable."
    - What's New in .NET 10 for Reviewers › SDK and tooling changes [New in .NET 10] — bullet 1: "…treat it like `npx`: pin versions in CI (`dnx tool@1.2.3`)."
    - What's New in .NET 10 for Reviewers › SDK and tooling changes [New in .NET 10] — bullet 3: "NU1510 flags direct references that the framework already provides."
    - What's New in .NET 10 for Reviewers › SDK and tooling changes [New in .NET 10] — bullet 4: "…on **Microsoft.Testing.Platform** natively (`"test": { "runner": "Microsoft.Testing.Platform" }` in `global.json`)."
    - Project Setup, Build and Analyzers › Central package management and supply chain — paragraph 1: "…reproducible restores, and `packageSourceMapping` in `nuget.config` to prevent dependency confusion."
    - Trimming and Native AOT — bullet 5: "…app.cs`) publish as Native AOT by default in .NET 10."
[^17]: Breaking change: .NET runtime no longer provides default termination signal handlers. https://learn.microsoft.com/en-us/dotnet/core/compatibility/core-libraries/10.0/sigterm-signal-handler
    - What's New in .NET 10 for Reviewers › Runtime and hosting behavior changes [New in .NET 10] ★★★ — paragraph 1: "…on `docker stop`/Kubernetes termination now gets killed without running it."
[^18]: Breaking change: BackgroundService runs all of ExecuteAsync as a Task. https://learn.microsoft.com/en-us/dotnet/core/compatibility/extensions/10.0/backgroundservice-executeasync-task
    - What's New in .NET 10 for Reviewers › Runtime and hosting behavior changes [New in .NET 10] ★★★ — paragraph 2: "…it ran on a pool thread and `StartAsync` returned first."
[^19]: Breaking change: Null values preserved in configuration. https://learn.microsoft.com/en-us/dotnet/core/compatibility/extensions/10.0/configuration-null-values-preserved
    - What's New in .NET 10 for Reviewers › Runtime and hosting behavior changes [New in .NET 10] ★★★ — paragraph 3: "…a non-nullable value type now yields `default(T)` instead of throwing."
[^20]: Breaking change: System.Linq.AsyncEnumerable in .NET 10. https://learn.microsoft.com/en-us/dotnet/core/compatibility/core-libraries/10.0/asyncenumerable
    - What's New in .NET 10 for Reviewers › Runtime and hosting behavior changes [New in .NET 10] ★★★ — paragraph 4: "…become `Select` overloads that take `async` lambdas with a `CancellationToken`."
[^21]: What's new in the .NET 10 runtime. https://learn.microsoft.com/en-us/dotnet/core/whats-new/dotnet-10/runtime
    - What's New in .NET 10 for Reviewers › JIT and GC improvements (no code change needed) — paragraph 1: "AVX10.2 intrinsics exist but are disabled by default."
    - Performance — bullet 9: "…of abstractions — benchmark before replacing LINQ/`IEnumerable<T>` with hand-written loops."
[^22]: Breaking changes in ASP.NET Core 10. https://learn.microsoft.com/en-us/aspnet/core/breaking-changes/10/overview
    - What's New in .NET 10 for Reviewers › ASP.NET Core 10 highlights [New in .NET 10] — bullet 2: "…endpoint returns `401`, while an HTML endpoint still returns `302`."
    - What's New in .NET 10 for Reviewers › ASP.NET Core 10 highlights [New in .NET 10] — bullet 5: "…middleware no longer emits diagnostics/logs for it (`ExceptionHandlerOptions.SuppressDiagnosticsCallback` restores them)."
    - What's New in .NET 10 for Reviewers › ASP.NET Core 10 highlights [New in .NET 10] — bullet 14: "…and `ForwardedHeadersOptions.KnownNetworks` (use `KnownIPNetworks` with `System.Net.IPNetwork`), Razor runtime compilation, `WithOpenApi`."
    - Deprecated, Obsoleted and Removed APIs — bullet 5: "…`IWebHost`, `WebHost`, `IActionContextAccessor`, `IPNetwork`/`KnownNetworks`, Razor runtime compilation, `WithOpenApi()` are obsolete."
    - ASP.NET Core 10 Minimal APIs — bullet 7: "…with `KnownProxies`/`KnownIPNetworks` (`KnownNetworks` and `Microsoft.AspNetCore.HttpOverrides.IPNetwork` are obsolete in .NET 10)."
    - ASP.NET Core 10 Minimal APIs — bullet 10: "…diagnostics in .NET 10 — make sure the handler logs."
[^23]: Breaking changes in EF Core 10. https://learn.microsoft.com/en-us/ef/core/what-is-new/ef-core-10.0/breaking-changes
    - What's New in .NET 10 for Reviewers › EF Core 10 highlights [New in .NET 10] — bullet 2: "…(code that built expression trees by hand no longer compiles)."
    - What's New in .NET 10 for Reviewers › EF Core 10 highlights [New in .NET 10] — bullet 3: "…JSON array parameter; tune with `UseParameterizedCollectionMode(ParameterTranslationMode.…)` or `EF.Parameter`/`EF.Constant`/`EF.MultipleParameters` per query."
    - What's New in .NET 10 for Reviewers › EF Core 10 highlights [New in .NET 10] — bullet 4: "…text break; a one-time plan-cache recompilation spike happens after deployment."
    - What's New in .NET 10 for Reviewers › EF Core 10 highlights [New in .NET 10] — bullet 5: "…to columns and JSON (`ComplexProperty(...).ToJson()`); named query filters; vector search."
    - What's New in .NET 10 for Reviewers › EF Core 10 highlights [New in .NET 10] — bullet 6: "`DateTimeOffset` written to REAL columns is converted to UTC."
    - What's New in .NET 10 for Reviewers › EF Core 10 highlights [New in .NET 10] — bullet 8: "…it), and Microsoft.Data.SqlClient 6.1 pulls in `Microsoft.Identity.Client.Broker` (larger publish output)."
    - EF Core 10 › Query pitfalls — bullet 11: "…`EF.Parameter(ids)` (JSON parameter) or a temp table for big sets."
    - EF Core 10 › Migrations and model — bullet 1: "…when moving to `UseAzureSql`/compatibility level 170 in EF Core 10."
    - EF Core 10 › Migrations and model — bullet 4: "…10 redacts inlined constants in logs unless it is enabled."
[^24]: What's new in EF Core 10. https://learn.microsoft.com/en-us/ef/core/what-is-new/ef-core-10.0/whatsnew
    - What's New in .NET 10 for Reviewers › EF Core 10 highlights [New in .NET 10] — bullet 5: "…to columns and JSON (`ComplexProperty(...).ToJson()`); named query filters; vector search."
[^25]: What's new in C# 15 (preview). https://learn.microsoft.com/en-us/dotnet/csharp/whats-new/csharp-15
    - Do Not Use or Recommend (C# 15 / .NET 11) — paragraph 1: "…of the following, and the reviewer must never suggest them:"
[^26]: What's new in .NET libraries for .NET 11 (preview). https://learn.microsoft.com/en-us/dotnet/core/whats-new/dotnet-11/libraries
    - Do Not Use or Recommend (C# 15 / .NET 11) — paragraph 1: "…of the following, and the reviewer must never suggest them:"
[^27]: Breaking change: .NET 10 obsoletions with custom IDs (SYSLIB0058–SYSLIB0062). https://learn.microsoft.com/en-us/dotnet/core/compatibility/core-libraries/10.0/obsolete-apis
    - Deprecated, Obsoleted and Removed APIs — paragraph 1: "…10 (custom diagnostic IDs; suppressing CS0618 does not suppress them):"
[^28]: BinaryFormatter migration guide (removed in .NET 9). https://learn.microsoft.com/en-us/dotnet/standard/serialization/binaryformatter-migration-guide/
    - Deprecated, Obsoleted and Removed APIs — bullet 1: "Replace with System.Text.Json, protobuf, or `System.Formats.Nrbf` for reading legacy payloads."
    - Security › Deserialization — bullet 1: "…9); do not re-enable them through the unsupported compatibility package."
[^29]: .NET 10.0.7 release notes (CVE-2026-40372, Microsoft.AspNetCore.DataProtection). https://github.com/dotnet/core/blob/main/release-notes/10.0/10.0.7/10.0.7.md
    - Version-Aware Review — bullet 5: "…rotate the key ring if an affected version was deployed."
    - Security › Cryptography — bullet 7: "…≥ 10.0.7 (CVE-2026-40372) and rotate keys if 10.0.0–10.0.6 was deployed."
[^30]: Code analysis in .NET — AnalysisLevel and AnalysisMode. https://learn.microsoft.com/en-us/dotnet/fundamentals/code-analysis/overview
    - Project Setup, Build and Analyzers › Baseline project file — bullet 3: "`AnalysisMode` alone sets the mode."
[^31]: Breaking change: System.Text.Json checks for property name conflicts. https://learn.microsoft.com/en-us/dotnet/core/compatibility/serialization/10/property-name-validation
    - System.Text.Json › Defaults that surprise reviewers (verified on .NET 10) — bullet 4: "Rename or `[JsonIgnore]` the property."
[^32]: xUnit.net v3 4.0.0 release notes. https://xunit.net/releases/v3/4.0.0
    - Testing › xUnit.net v3 — bullet 1: "…removed), **MSTest 4.5.1**, **NUnit 5.0.0** (September 27, 2026), Microsoft.NET.Test.Sdk 18.11.0."
[^33]: NUnit framework release notes (5.0.0, September 27, 2026). https://docs.nunit.org/articles/nunit/release-notes/framework.html
    - Testing › xUnit.net v3 — bullet 1: "…removed), **MSTest 4.5.1**, **NUnit 5.0.0** (September 27, 2026), Microsoft.NET.Test.Sdk 18.11.0."
    - Testing › xUnit.net v3 — bullet 2: "NUnit.Analyzers 4.14+ helps convert existing code."
