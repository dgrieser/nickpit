### C# 9 on .NET 5 (End of Life): The Complete Production Developer Guide

> **Version**: C# 9 shipped with **.NET 5.0 on November 10, 2020** (SDK 5.0.100, Visual Studio 2019 16.8).[^1][^2] The final servicing release is **.NET 5.0.17 (May 10, 2022)** with SDKs **5.0.408** and **5.0.214**; it fixed CVE-2022-29117, CVE-2022-23267 and CVE-2022-29145.[^2]
> **Support**: .NET 5 was a short-term ("Current", now called STS) release. It reached **end of support on May 10, 2022** — as of October 2026 it has gone more than four years without security fixes.[^2][^3] The recommended target is **.NET 10 (LTS)**: latest servicing release **10.0.13 (October 13, 2026)**, SDK 10.0.402, supported until **November 14, 2028**. .NET 8 (LTS, 8.0.32) and .NET 9 (STS, 9.0.21) both reach end of support on **November 10, 2026**, so they are not sensible migration targets any more. **.NET 11 / C# 15 is not released**: 11.0.0-rc.2 (SDK 11.0.100-rc.2) shipped on October 13, 2026 with a go-live license.[^3]
> **Scope**: The C# 9 language, the .NET 5 base class library and runtime, ASP.NET Core 5 and EF Core 5 as they affect production code and code review: idioms, pitfalls, security, performance, testing and analyzers — plus C# 9 used with `<LangVersion>9</LangVersion>` on `netstandard2.0`, .NET Framework and newer TFMs.
> **Selection**: This guide applies when the project's C# language version is 9: an explicit `<LangVersion>9</LangVersion>` (or `9.0`), or `<TargetFramework>net5.0</TargetFramework>` (also `net5.0-windows`) with no `LangVersion`, because C# 9 is the default for .NET 5.[^4]
> **Note**: This document is self-contained. Sections marked **New in C# 9** or **New in .NET 5** cover version-specific behaviour; the rest applies to all C# 9 code. Every example and behavioural claim was compiled and run with the original toolchain — **SDK 5.0.408 / runtime 5.0.17 / ASP.NET Core 5.0.17** (official `mcr.microsoft.com/dotnet/sdk:5.0` image, Linux x64) and EF Core 5.0.17. "New in C# 9" boundaries were checked by compiling the same code with `LangVersion` 8; "not available" claims for C# 10–14 were checked with SDK 10.0.402 at `LangVersion` 9 and 10; `LangVersion` 9 on `net6.0` was run with SDK 6.0.428 / runtime 6.0.36, and on `netstandard2.0` with SDK 5.0.408. Windows-only behaviour (WinForms/WPF, NLS vs ICU on Windows, registry) was not executed.

---

#### Table of Contents

1. [Overview and Global Truths](#overview-and-global-truths)
2. [Support Status and Upgrade Path](#support-status-and-upgrade-path)
3. [How C# 9 Is Selected: LangVersion, TFMs and Down-Level Targets](#how-c-9-is-selected-langversion-tfms-and-down-level-targets)
4. [What's New in C# 9](#whats-new-in-c-9)
5. [What's New in .NET 5 for Reviewers](#whats-new-in-net-5-for-reviewers)
6. [Do Not Use or Recommend (C# 10+ / .NET 6+)](#do-not-use-or-recommend-c-10--net-6)
7. [Deprecated, Obsoleted and Removed APIs in .NET 5](#deprecated-obsoleted-and-removed-apis-in-net-5)
8. [Version-Aware Review](#version-aware-review)
9. [Project Setup, Build and Analyzers](#project-setup-build-and-analyzers)
10. [Naming and Code Style](#naming-and-code-style)
11. [Nullable Reference Types](#nullable-reference-types)
12. [Records, Classes and Structs](#records-classes-and-structs)
13. [Pattern Matching](#pattern-matching)
14. [Strings, Globalization and Numbers](#strings-globalization-and-numbers)
15. [Collections, Span and Memory](#collections-span-and-memory)
16. [LINQ](#linq)
17. [Exceptions and Error Handling](#exceptions-and-error-handling)
18. [Async/Await, Cancellation and Concurrency](#asyncawait-cancellation-and-concurrency)
19. [IAsyncEnumerable and Channels](#iasyncenumerable-and-channels)
20. [IDisposable and IAsyncDisposable](#idisposable-and-iasyncdisposable)
21. [Dependency Injection, Options, Configuration and Hosting](#dependency-injection-options-configuration-and-hosting)
22. [Logging and Observability](#logging-and-observability)
23. [HttpClient and IHttpClientFactory](#httpclient-and-ihttpclientfactory)
24. [System.Text.Json (5.0 Behaviour)](#systemtextjson-50-behaviour)
25. [ASP.NET Core 5](#aspnet-core-5)
26. [EF Core 5](#ef-core-5)
27. [Security](#security)
28. [Performance](#performance)
29. [Testing](#testing)
30. [Removed / Changed in .NET 5 — Migration Notes](#removed--changed-in-net-5--migration-notes)
31. [Production / Review Checklist](#production--review-checklist)

---

#### Overview and Global Truths

C# 9 is the language of .NET 5, the release that merged .NET Core and the "one .NET" naming (`netcoreapp3.1` → `net5.0`). Its headline features are **records**, **init-only setters**, **top-level statements**, the **pattern-matching upgrade** (relational `<`/`>=`, logical `and`/`or`/`not`, type patterns, parenthesized patterns), **target-typed `new()`**, and a set of low-level features (`nint`/`nuint`, function pointers, `[SkipLocalsInit]`, module initializers, extended partial methods) aimed at interop and source generators, which also debuted with .NET 5.[^1][^5]

Global truths for C# 9 / .NET 5 code:

- **The runtime is unsupported.** Any production service on `net5.0` runs on a runtime that has not been patched since May 2022. Flag it in every review of deployment-relevant code (TFM, Dockerfile, global.json, CI images) and recommend .NET 10 LTS.
- **C# 9 ≠ .NET 5.** The language version and the library version are separate. C# 9 can be compiled for `netstandard2.0`, `net48` or `net6.0`; the BCL APIs available depend on the target framework, not on `LangVersion`. Several C# 9 features depend on types (`IsExternalInit`, `ModuleInitializerAttribute`, `SkipLocalsInitAttribute`) or runtime capabilities (covariant returns) that only `net5.0`+ provides.
- **Records are reference types.** C# 9 has only `record` (= record class). `record struct` and `record class` keywords are C# 10.
- **Nothing is `required`.** `init` makes a property settable only during initialization; it does not force callers to set it. `required` is C# 11.
- **Nullable reference types are opt-in.** The .NET 5 project templates do not set `<Nullable>enable</Nullable>` (the .NET 6 templates were the first to do so); check the project file. Annotations are compile-time only — `null!` and `default!` silence the compiler, not the runtime.
- **Patterns are evaluated against constants only.** Relational patterns need a constant on the right (`> 0`, not `> limit`).
- **No implicit usings, no global usings, no file-scoped namespaces.** Every file has its own `using` directives and a block-bodied `namespace { }`.
- **ASP.NET Core 5 uses `Startup` + the generic host.** Minimal hosting (`WebApplication`) and minimal APIs are .NET 6.
- **System.Text.Json is the default serializer** for ASP.NET Core and `System.Net.Http.Json`; Newtonsoft.Json is opt-in via `Microsoft.AspNetCore.Mvc.NewtonsoftJson`.
- **Globalization uses ICU** on Linux and macOS (and, new in .NET 5, on Windows 10 May 2019 Update and later), so culture-sensitive string APIs behave differently from .NET Framework's NLS.[^6]

---

#### Support Status and Upgrade Path

| Version | Released | Latest patch (date) | End of support | Status (October 2026) |
|---|---|---|---|---|
| .NET 5 (C# 9) | Nov 10, 2020 | 5.0.17 (May 10, 2022) | May 10, 2022 | **End of life** |
| .NET 6 (C# 10) | Nov 8, 2021 | 6.0.36 (Nov 12, 2024) | Nov 12, 2024 | End of life |
| .NET 7 (C# 11) | Nov 8, 2022 | 7.0.20 | May 14, 2024 | End of life |
| .NET 8 (C# 12, LTS) | Nov 14, 2023 | 8.0.32 (Oct 13, 2026) | **Nov 10, 2026** | Maintenance, ends in weeks |
| .NET 9 (C# 13, STS) | Nov 12, 2024 | 9.0.21 (Oct 13, 2026) | **Nov 10, 2026** | Maintenance, ends in weeks |
| .NET 10 (C# 14, LTS) | Nov 11, 2025 | 10.0.13 (Oct 13, 2026) | Nov 14, 2028 | **Active — recommended** |
| .NET 11 (C# 15) | — | 11.0.0-rc.2 (Oct 13, 2026) | — | Release candidate (go-live), not GA |

Sources: the official release metadata (`releases-index.json` and per-channel `releases.json`).[^2][^3]

Review guidance:

- **Treat `net5.0` as a security finding** in services, containers and anything internet-facing: the runtime, ASP.NET Core 5 and the `mcr.microsoft.com/dotnet/*:5.0` images receive no fixes. Libraries that *multi-target* `net5.0` alongside supported TFMs are a smaller issue (drop the `net5.0` target; consumers on `net5.0` should upgrade).
- **Recommend .NET 10 LTS** as the migration target, not .NET 8 or 9 (both end on November 10, 2026). Upgrading the TFM also raises the default language to C# 14; that is fine, but newer analyzers and nullable annotations will surface new warnings — budget for them.
- **Do not "fix" EOL by pinning a newer SDK** while keeping `net5.0`: the SDK 6+ compiler can still build `net5.0` (with warning `NETSDK1138`), but the app still runs on the unpatched 5.0 runtime.
- **Pin the SDK** with `global.json` while the project is on .NET 5 (e.g. `"version": "5.0.408"`, `"rollForward": "latestFeature"`) so CI and developer machines use the same compiler; an unpinned newer SDK compiles C# 9 with a newer Roslyn whose behaviour differs in small ways (see "Top-level statements").
- Library packages last published for .NET 5 (e.g. `Microsoft.Extensions.*` 5.0.x, EF Core 5.0.17, `Microsoft.AspNetCore.*` 5.0.17) are also out of support. Upgrading `Microsoft.Extensions.*` packages beyond 5.0 on a `net5.0` app is possible for some packages but not a substitute for upgrading the runtime.

---

#### How C# 9 Is Selected: LangVersion, TFMs and Down-Level Targets

##### Default language version per target framework

| Target framework | Default C# version |
|---|---|
| `net5.0` | **C# 9** |
| `netcoreapp3.x`, `netstandard2.1` | C# 8 |
| `netcoreapp2.x`, `netstandard2.0`, `netstandard1.x`, all .NET Framework | C# 7.3 |
| `net6.0` / `net7.0` / `net8.0` / `net9.0` / `net10.0` | C# 10 / 11 / 12 / 13 / 14 |

The default is chosen by the compiler from the TFM.[^4] So code is "C# 9" when it targets `net5.0` without `LangVersion`, or when any project sets `<LangVersion>9</LangVersion>` / `9.0` — typically a `netstandard2.0` or .NET Framework library that wants records and init-only properties, or a newer-TFM project held back on purpose.

```xml
<!-- Library that ships to .NET Framework and modern .NET, written in C# 9 -->
<PropertyGroup>
  <TargetFrameworks>netstandard2.0;net5.0</TargetFrameworks>
  <LangVersion>9.0</LangVersion>
  <Nullable>enable</Nullable>
</PropertyGroup>
```

Rules for reviewers:

- **Never accept `<LangVersion>latest</LangVersion>` or `default`/`latestMajor`** in a project that is supposed to be C# 9: the language then follows whatever SDK is installed, so the same source builds on one machine and fails on another. Pin `9.0` explicitly when down-level targets need it.
- **`LangVersion` does not add APIs.** `LangVersion 9` on `netstandard2.0` gives the syntax, not `IsExternalInit`, `ModuleInitializerAttribute`, `SkipLocalsInitAttribute`, `MemberNotNullAttribute`, `Index`/`Range` or `IAsyncEnumerable<T>` — those come from the target framework or from polyfills/packages.
- `#error version` makes the compiler print its version and the selected language version (CS8304) — useful when a build behaves unexpectedly.

##### Which C# 9 features need what (down-level targets)

Measured with SDK 5.0.408, `netstandard2.0`, `<LangVersion>9.0</LangVersion>`:

| Feature | `netstandard2.0` / .NET Framework result | Fix |
|---|---|---|
| Records, `init` accessors | **CS0518** `IsExternalInit is not defined or imported` | Polyfill `IsExternalInit` (below) |
| Module initializers | **CS0246** `ModuleInitializer` not found | Polyfill the attribute |
| `[SkipLocalsInit]` | **CS0246** not found | Polyfill the attribute (also needs `AllowUnsafeBlocks`) |
| Covariant return types | **CS8830** "Target runtime doesn't support covariant return types in overrides" | No fix — needs the .NET 5+ runtime |
| `[NotNullWhen]`, `[MemberNotNull]` on local functions or anywhere | **CS0246** not found (attributes ship in .NET Core 3.0+ / 5) | Polyfill, or the `Nullable` package |
| Relational/logical/type patterns, `nint`/`nuint`, managed function pointers, target-typed `new`/conditional, static lambdas, lambda discards, extension `GetEnumerator`, extended partial methods | Compile and work | — |
| Top-level statements | Work in an `Exe` project | — |
| (C# 8) default interface members | CS8701 — runtime does not support them | No fix |

Covariant returns are the one C# 9 feature that cannot be back-ported: the compiler checks `RuntimeFeature.CovariantReturnsOfClasses`, which only the .NET 5+ runtime reports as supported.[^7]

The compiler only needs types with the right *names* in the right namespace. Declare polyfills `internal` so they never clash with the real types when a consumer targets `net5.0`+:

```csharp
// Polyfills.cs — only for netstandard2.0 / .NET Framework targets.
// Wrap in #if !NET5_0_OR_GREATER when multi-targeting.
namespace System.Runtime.CompilerServices
{
    internal static class IsExternalInit { }

    [AttributeUsage(AttributeTargets.Method, Inherited = false)]
    internal sealed class ModuleInitializerAttribute : Attribute { }

    [AttributeUsage(AttributeTargets.Module | AttributeTargets.Class | AttributeTargets.Struct
        | AttributeTargets.Interface | AttributeTargets.Constructor | AttributeTargets.Method
        | AttributeTargets.Property | AttributeTargets.Event, Inherited = false)]
    internal sealed class SkipLocalsInitAttribute : Attribute { }
}
```

With these polyfills, a `netstandard2.0` library containing a record with a `with` expression, a `[ModuleInitializer]` method and a `[SkipLocalsInit]` method compiles, and when consumed by a `net5.0` app the record works, the module initializer runs exactly once, and the method body's `InitLocals` flag is `false` (verified). Ready-made packages exist (`IsExternalInit`, `PolySharp`, `Nullable` on NuGet); prefer one consistent approach per solution.

Review points:

- A **public** `IsExternalInit` in a library causes type conflicts in consumers — it must be `internal`.
- `init` setters carry a `modreq(IsExternalInit)`. Consumers compiling with C# 8 or earlier (the default for every .NET Framework and `netstandard2.0` project) cannot assign them in object initializers; consumers with C# 9+ can. Mention it when a `netstandard2.0` library's public API exposes `init` properties to .NET Framework consumers.
- **C# 9 on `net6.0`+** (held back with `LangVersion 9`) compiles everything in this guide; the BCL is the newer one, so .NET 6+ APIs exist there. Review API usage against the *target framework*, and syntax against C# 9.

---

#### What's New in C# 9

Every feature below fails to compile with `LangVersion 8` (CS8400 "Feature '…' is not available in C# 8.0. Please use language version 9.0 or greater.") — except `[SkipLocalsInit]`, which the SDK 5.0.408 compiler honours even at `LangVersion 8` (only `AllowUnsafeBlocks` is required; verified via `MethodBody.InitLocals == false`).[^1][^5]

##### 1. Records [New in C# 9] ★★★

A `record` is a class with compiler-synthesized value equality (`Equals`, `GetHashCode`, `==`, `!=`, `IEquatable<T>`), a virtual `EqualityContract`, `ToString()` that prints public members, `Deconstruct` (positional records), a protected copy constructor, and a hidden `<Clone>$` method that powers `with` expressions.[^8]

```csharp
public record Person(string First, string Last);              // positional: init-only props + ctor + Deconstruct
public record Employee(string First, string Last, string Dept) : Person(First, Last);

var p = new Person("Ada", "Lovelace");
var q = p with { Last = "King" };                                // non-destructive mutation
Console.WriteLine(p);                                           // Person { First = Ada, Last = Lovelace }
Console.WriteLine(p == new Person("Ada", "Lovelace"));          // True
var (first, last) = p;                                          // Deconstruct
```

Pitfalls a reviewer must catch (all verified on 5.0.17):

- **`with` is a shallow copy.** Reference-type members are shared:
  ```csharp
  public record Order(int Id, List<string> Lines);

  var a = new Order(1, new List<string> { "x" });
  var b = a with { };
  b.Lines.Add("y");          // a.Lines.Count == 2 — both records share one list
  ```
  Use immutable collections (`ImmutableArray<T>`, `ImmutableList<T>`) or copy explicitly: `a with { Lines = new List<string>(a.Lines) }`.
- **Collection members break value equality.** `List<T>`/arrays compare by reference, so two records with equal contents are *not* equal, and `ToString()` prints `System.String[]`:
  ```csharp
  public record Tags(int Id, string[] Values);
  new Tags(1, new[] { "a" }) == new Tags(1, new[] { "a" });   // False
  ```
  Either override equality (below) or keep collection members out of records used as keys/values.
- **Equality includes the runtime type.** `EqualityContract` makes a `Person` never equal to an `Employee` with the same `First`/`Last` — in both directions.
- **`with` preserves the runtime type.** `Person x = new Employee(...); x with { First = "B" }` returns an `Employee` (the virtual `<Clone>$` is called).
- **`with` bypasses constructor validation.** Only the copy constructor runs, then the `init` setters:
  ```csharp
  public record Validated
  {
      public Validated(string name) =>
          Name = string.IsNullOrWhiteSpace(name) ? throw new ArgumentException("required", nameof(name)) : name;
      public string Name { get; init; }
  }

  var bad = new Validated("ok") with { Name = "" };   // no exception: Name == ""
  ```
  Put invariants into the `init` accessor (with a backing field) if they must hold after `with`.
- **Mutable records in hash-based collections.** A record with `set` properties changes its hash code when mutated; `HashSet.Contains` then returns `false` for the same instance. Records used as keys must be immutable (positional or `init`-only).
- **Base `ToString()` overrides are not inherited.** Each derived record synthesizes its own `ToString()`, so `record Shape { public override string ToString() => "custom"; } record Circle(double R) : Shape;` prints `Circle { R = 1 }`. `sealed override ToString()` in a record (to stop this) is C# 10. Customize per record, or override `PrintMembers`.
- **Secrets leak through `ToString()`.** Logging a record prints all public properties, including passwords and tokens. Redact by implementing `PrintMembers` (in a `sealed` record it must be `private bool PrintMembers(StringBuilder)`; in a non-sealed record `protected virtual`):
  ```csharp
  public sealed record Credentials(string User, string Password)
  {
      private bool PrintMembers(System.Text.StringBuilder b)
      {
          b.Append("User = ").Append(User);
          return true;
      }
  }
  // Credentials { User = ann }
  ```
- **Attributes on positional parameters target the parameter.** `public record R([Required] string? Name)` puts `[Required]` on the constructor parameter only. ASP.NET Core 5 MVC model validation understands that, but `Validator.TryValidateObject`, many third-party validators and serializers look at *properties* and see nothing (verified: 0 errors vs 1 with `[property: Required]`). Attributes that are not valid on parameters (e.g. `[JsonPropertyName]`) fail to compile (CS0592) unless written as `[property: JsonPropertyName("n")]`.
- **Overriding equality.** You may declare `public virtual bool Equals(R? other)` (or non-virtual in a `sealed` record) and `GetHashCode()`; the compiler then uses yours:
  ```csharp
  public sealed record Basket(string Id, IReadOnlyList<string> Items)
  {
      public bool Equals(Basket? other) =>
          other is not null && Id == other.Id && Items.SequenceEqual(other.Items);
      public override int GetHashCode() => HashCode.Combine(Id, Items.Count);
  }
  ```
- **Records are classes** — every `with` allocates. In hot paths prefer a `readonly struct` (C# 9 has no `record struct`).
- **EF Core entities should not be records.** Value equality conflicts with identity-based change tracking; use records for DTOs, messages, value objects and configuration.
- Seal records that are not designed for inheritance (`sealed record`): it makes the intent explicit, keeps equality symmetric (no derived types with extra members) and lets the JIT devirtualize calls.

##### 2. Init-only setters [New in C# 9] ★★★

```csharp
public class ServerOptions
{
    public string Host { get; init; } = "localhost";
    public int Port { get; init; } = 80;
}

var o = new ServerOptions { Port = 8080 };   // OK in object initializer
// o.Port = 9090;                            // error CS8852: Init-only property ... can only be assigned in an object initializer, ...
```

- `init` does **not** make a property required. A caller can omit it and get the default (often `null` for strings, despite `string` being non-nullable — the nullable analysis in C# 9 warns at the declaration (CS8618) unless you initialize it). Use constructor parameters for mandatory values.
- Reflection, model binders and serializers can still call `init` setters after construction (`PropertyInfo.SetValue` works — verified). `init` is a compile-time contract, not a security boundary.
- The setter is emitted with `modreq(System.Runtime.CompilerServices.IsExternalInit)`; see the polyfill section for down-level targets.
- `System.Text.Json` 5.0 can deserialize into `init` properties; `Microsoft.Extensions.Configuration` binding (`Bind`/`Get<T>`) uses reflection and also sets them.

##### 3. Top-level statements [New in C# 9] ★★

```csharp
// Program.cs — the whole entry point
using System;
using System.Threading.Tasks;

Console.WriteLine($"Running on {Environment.Version}");
await Task.Delay(10);
return 0;                        // optional exit code
```

- Only **one file** per project may contain top-level statements (CS8802). `args` is available implicitly. `await` makes the synthesized `Main` return `Task`/`Task<int>`.
- Types must be declared **after** the statements (in C# 9, a namespace or type before statements is CS8803).
- **The synthesized class is named `<Program>$`** with method `<Main>$` when compiled by the C# 9 compiler (SDK 5.0.x) — verified via `Assembly.EntryPoint`. It cannot be referenced by name; ASP.NET Core 5 integration tests use `WebApplicationFactory<Startup>` (the documented pattern for 5.0) rather than `WebApplicationFactory<Program>`. The same source compiled by SDK 6.0.428 with `LangVersion 9` produces a class named `Program` (verified) — the name follows the compiler, not the language version, which is another reason to pin the SDK.
- Local functions declared among top-level statements are local to the synthesized `Main`; they cannot be called from other files.
- Keep top-level `Program.cs` thin in ASP.NET Core 5: build the host and run it. Logic belongs in `Startup` and services.

##### 4. Pattern matching enhancements [New in C# 9] ★★★

New pattern forms: **relational** (`<`, `<=`, `>`, `>=` against a constant), **logical** (`and`, `or`, `not`), **parenthesized** patterns, and **type patterns** (a bare type, `int => ...`, without a designation).[^9]

```csharp
static string Classify(int t) => t switch
{
    < 0 => "negative",
    >= 0 and < 10 => "small",
    _ => "large",
};

static bool IsLetter(char c) => c is (>= 'a' and <= 'z') or (>= 'A' and <= 'Z');

static string Kind(object o) => o switch
{
    int => "int",               // type pattern (C# 9); C# 8 needed `int _`
    string => "string",
    null => "null",
    _ => "other",
};

if (input is not null) { /* ... */ }
```

See "Pattern Matching" below for precedence and NaN pitfalls.

##### 5. Target-typed `new` expressions [New in C# 9] ★★

```csharp
public record Point(int X, int Y);

private readonly Dictionary<string, List<int>> _index = new();   // field: type is on the left
ServerOptions options = new() { Port = 8080 };
var points = new List<Point>();
points.Add(new(1, 2));                                           // target type from the parameter
```

- Use it when the type is obvious from the left-hand side (fields, typed locals). Avoid it in method arguments where the reader cannot see the type, and with overloaded methods (ambiguity errors CS0121).
- `var x = new();` is an error (CS8754 "There is no target type for 'new()'").
- `throw new();` compiles (it creates a plain `System.Exception`) — flag it: it throws an exception with no type information and no message.

##### 6. Target-typed conditional expressions [New in C# 9]

`b ? 1 : null` now converts to a target type (`int?`) instead of failing (with `LangVersion 8` it is CS8400 "Feature 'target-typed conditional expression' is not available in C# 8.0").

```csharp
int? x = isSet ? value : null;
```

When there *is* a natural type, it wins over the target type: `object o = flag ? 1 : 2L;` boxes a `long` for both branches (natural type `long`). Be explicit when mixing numeric types.

##### 7. Covariant return types [New in C# 9]

```csharp
public class Animal { public virtual Animal Clone() => new Animal(); }
public class Dog : Animal { public override Dog Clone() => new Dog(); }   // narrower return type
```

Requires runtime support (`RuntimeFeature.IsSupported("CovariantReturnsOfClasses")` is `true` on .NET 5); on `netstandard2.0`/.NET Framework targets it is error CS8830. A library multi-targeting `netstandard2.0` cannot use covariant returns in shared code.

##### 8. Static anonymous functions [New in C# 9]

```csharp
Func<int, int> addOne = static x => x + 1;   // cannot capture locals, parameters or `this`
```

Use `static` lambdas in hot paths and in callbacks that must not capture state accidentally (e.g. `ConcurrentDictionary.GetOrAdd(key, static (k, arg) => ..., arg)`). Capturing produces CS8820.

##### 9. Lambda discard parameters [New in C# 9]

```csharp
Func<int, int, int> constant = (_, _) => 42;
button.Click += (_, _) => Refresh();
```

A **single** `_` is still an ordinary parameter named `_` (back-compat): `Func<int,int> f = _ => _ * 2;` compiles and returns `42` for `21` (verified). Only two or more `_` parameters are discards.

##### 10. Extension `GetEnumerator` in `foreach` [New in C# 9]

```csharp
public readonly struct Range3
{
    public readonly int N;
    public Range3(int n) => N = n;
}

public static class Range3Extensions
{
    public static IEnumerator<int> GetEnumerator(this Range3 r)
    {
        for (int i = 0; i < r.N; i++) yield return i;
    }
}

foreach (var i in new Range3(4)) { /* 0..3 */ }
```

Useful for adapting types you do not own. Do not use it to make `foreach` over a type with surprising semantics (e.g. `foreach (var i in 10)`); reviewers should flag cute extensions on primitives.

##### 11. Attributes on local functions [New in C# 9]

```csharp
public static bool HasValue(string? s)
{
    return Check(s) && s.Length > 0;       // no CS8602: NotNullWhen flows from the local function

    static bool Check([NotNullWhen(true)] string? v) => v != null;
}
```

Nullable attributes, `[MethodImpl]` and `[Conditional]` now work on local functions (`[Conditional]` only on `static` local functions — CS8783 otherwise). C# 8 rejects them with CS8400 "Feature 'local function attributes' is not available in C# 8.0".

##### 12. Native-sized integers `nint` / `nuint` [New in C# 9]

```csharp
nint a = 5;
nuint b = 6;
nint sum = a + (nint)b;
Console.WriteLine(nint.MaxValue);    // 9223372036854775807 on x64
```

- `nint`/`nuint` compile to `System.IntPtr`/`UIntPtr` but get full arithmetic, conversions and `checked` semantics (`checked(nint.MaxValue + 1)` throws `OverflowException` — verified). In C# 9, `IntPtr` keeps only its BCL operators (`IntPtr + int` works, `IntPtr * IntPtr` is CS0019); `nint` and `IntPtr` become full synonyms only in C# 11 with .NET 7.
- Size is platform-dependent (8 on x64, 4 on x86/ARM32). Use for interop and pointer arithmetic, not for business values or anything serialized.

##### 13. Function pointers [New in C# 9]

```csharp
public static unsafe class Calc
{
    private static int Twice(int x) => x * 2;

    public static int Run()
    {
        delegate*<int, int> f = &Twice;            // managed function pointer, no delegate allocation
        return f(21);
    }
}
```

- Requires `unsafe` and `<AllowUnsafeBlocks>true</AllowUnsafeBlocks>`. `delegate* unmanaged[Cdecl]<...>` targets native code; methods called from native code via a function pointer are marked `[UnmanagedCallersOnly]` (.NET 5 API).
- No null check, no type safety beyond the signature, no GC tracking of the target object. Restrict to interop layers; prefer delegates or `static` lambdas elsewhere.

##### 14. `[SkipLocalsInit]` [New in C# 9 / .NET 5]

```csharp
[System.Runtime.CompilerServices.SkipLocalsInit]
public static int SumFirst(ReadOnlySpan<int> data)
{
    Span<int> scratch = stackalloc int[64];          // NOT zeroed: contains stack garbage
    int n = Math.Min(data.Length, scratch.Length);
    data.Slice(0, n).CopyTo(scratch);                // write before read
    int sum = 0;
    for (int i = 0; i < n; i++) sum += scratch[i];   // only the written prefix is read
    return sum;
}
```

- The method's `.locals init` flag is dropped: `stackalloc` buffers and locals are not zeroed. Reading before writing leaks stale stack data — a real **information-disclosure** risk if the buffer is ever returned or written out. Only accept it in measured hot paths with obvious write-before-read.
- Requires `AllowUnsafeBlocks` (CS0227 otherwise) even when the method has no `unsafe` code. Can be applied to a type or the whole module (`[module: SkipLocalsInit]`) — a module-wide attribute deserves a careful review.

##### 15. Module initializers [New in C# 9 / .NET 5]

```csharp
internal static class Startup
{
    [ModuleInitializer]
    internal static void Init() => AppContext.SetSwitch("MyLib.UseFastPath", true);
}
```

- The method must be `static`, parameterless, return `void`, be non-generic, not in a generic type, and accessible from the module (`internal` or `public`; `private` is CS8814). The runtime runs all module initializers before any other code in the assembly touches the module.
- Intended for source generators and low-level registration. In application code they are hidden control flow: an exception there becomes a `TypeInitializationException` for `<Module>`; in an executable it terminates the process before `Main` runs and cannot be caught there (verified). Prefer explicit registration in `Main`/DI.

##### 16. Extended partial methods [New in C# 9]

```csharp
public partial class Generated
{
    public partial int Compute(int x);                 // accessibility / non-void / out => must be implemented
}

public partial class Generated
{
    public partial int Compute(int x) => x * 3;
}
```

Before C# 9, partial methods had to be `private void` and could be omitted. Now any partial method with an accessibility modifier, a non-`void` return, `out` parameters, or `virtual`/`override`/`sealed`/`new`/`extern` **must** have an implementation (CS8795). This is the hook that source generators (also new with the .NET 5 SDK) fill in.

##### 17. Other compiler changes in C# 9

- **Pattern `is not null`** replaces `!(x is null)`/`x != null` and is not affected by overloaded `==`/`!=` operators.
- **Unconstrained `T?`** (new in C# 9): `public static T? FirstOrNull<T>(IEnumerable<T> s)` compiles for any `T` and means "may be default" (C# 8: CS8627). Overrides and explicit implementations that need to disambiguate use the new `where T : default` constraint (C# 8: CS8400 "default type parameter constraints"). Note that for a value type `T`, `T?` here is still `T` (default `0`), not `Nullable<T>`.
- **New warning CS8524**: a `switch` expression over an enum that handles every *named* member still warns, because unnamed values like `(Color)5` are possible; at runtime such a value throws `SwitchExpressionException` (verified). Add a discard arm that throws a meaningful exception.
- **CS8425**: an async iterator with a `CancellationToken` parameter that is not marked `[EnumeratorCancellation]` — the token passed via `WithCancellation` is silently ignored (see IAsyncEnumerable).
- **Source generators** (`ISourceGenerator`) shipped with the .NET 5 SDK compiler (SDK 5.0.408 contains C# compiler 3.11.0 — `#error version` prints it). They are an SDK/compiler feature, not a language feature; the incremental generator API (`IIncrementalGenerator`) is .NET 6 SDK / Roslyn 4.0+.

---

#### What's New in .NET 5 for Reviewers

##### Runtime and library changes [New in .NET 5] ★★★

- **One TFM: `net5.0`** (plus OS-specific `net5.0-windows`); `netcoreapp`/`netstandard` targeting for new apps is replaced. `NET5_0`, `NET5_0_OR_GREATER` and `NETCOREAPP3_1_OR_GREATER` (the `_OR_GREATER` symbols are new in the .NET 5 SDK) are defined; `NETCOREAPP3_1` is not defined for `net5.0` (all verified).[^10]
- **ICU on Windows**: on Windows 10 May 2019 Update or later, .NET 5 uses ICU instead of NLS for globalization. Culture-sensitive `string.IndexOf(string)` etc. now behave like Linux. Example (verified on Linux 5.0.17): `"line1\r\nline2".IndexOf("\n")` returns **-1** with the current culture (because `\r\n` is one grapheme), but `6` with `StringComparison.Ordinal` or with the `char` overload.[^6]
- **`string.LastIndexOf("")`** returns the string length (`3` for `"abc"`) — changed from `Length - 1` in .NET Core 3.1.[^10]
- **Default `ActivityIdFormat` is W3C** (trace context) instead of hierarchical; ASP.NET Core 5 correlates with W3C `traceparent` headers by default.[^10]
- **HttpClient timeouts** throw `TaskCanceledException` whose `InnerException` is a `TimeoutException` (new in .NET 5) — distinguish timeouts from caller cancellation by checking that (verified).[^11]
- **`HttpResponseMessage.Content` is never null** in .NET 5 (an empty content is created by default) — verified.
- **BinaryFormatter** is obsolete (SYSLIB0011 warning) and **throws `NotSupportedException` in ASP.NET Core apps** (`Microsoft.NET.Sdk.Web`) by default — verified.[^12]
- **`Thread.Abort`** is obsolete (SYSLIB0006) and throws `PlatformNotSupportedException`; **UTF-7** (`Encoding.UTF7`) is obsolete (SYSLIB0001); code-access-security, GAC, remoting, CER APIs are obsolete (SYSLIB0002–0012).[^13]
- **`OrderBy(...).First(predicate)`/`FirstOrDefault(predicate)`** is O(N log N) again (full sort, as in .NET Framework) instead of O(N) in .NET Core 1.x–3.x, but the predicate runs fewer times (2 calls for 1,000 elements in a test on 5.0.17, instead of N) — side-effecting predicates behave differently after an upgrade from 3.1.[^10]
- **Nullable annotations**: about 80% of the BCL is annotated, so upgrading from 3.1 surfaces new nullable warnings.[^14]
- **Single-file apps and trimming** (`PublishSingleFile`, `PublishTrimmed`) are officially supported; in single-file apps `Assembly.Location` returns an empty string.[^10]
- **New types**: `System.Half`, `System.Runtime.CompilerServices.IsExternalInit`/`ModuleInitializerAttribute`/`SkipLocalsInitAttribute`, `UnmanagedCallersOnlyAttribute`, `MemberNotNull[When]Attribute`, non-generic `TaskCompletionSource`, `ValueTask.CompletedTask`, `Process.WaitForExitAsync`, `Convert.ToHexString`/`FromHexString`, `SHA256.HashData(...)` (static one-shot), `Enum.GetValues<T>()`, `Environment.ProcessId`, `GC.AllocateUninitializedArray<T>`, `CollectionsMarshal.AsSpan(List<T>)`, `Interlocked.And`/`Or`, `Encoding.Latin1`, `HttpClient.Send` (synchronous), `HttpVersionPolicy`, `X509Certificate2.CreateFromPemFile`, `ActivitySource` — all verified to compile on `net5.0`.

##### System.Text.Json 5.0 [New in .NET 5] ★★

Records and immutable types via constructors (`[JsonConstructor]` or the single public constructor), `IncludeFields`, `[JsonInclude]` for non-public accessors, `JsonIgnoreCondition` (`WhenWritingNull`, `WhenWritingDefault`, `Never`), `JsonNumberHandling` (quoted numbers), `ReferenceHandler.Preserve`, non-string dictionary keys, `JsonSerializerDefaults.Web`, a copy constructor for `JsonSerializerOptions`, and `HttpClient`/`HttpContent` JSON extensions (`System.Net.Http.Json`).[^15] Details and pitfalls are in the System.Text.Json section.

##### ASP.NET Core 5 [New in .NET 5]

Model binding and validation of C# 9 records; `DateTime` binding of UTC strings as UTC; OpenAPI (Swashbuckle) on by default in the `webapi` template; `HttpRequest.ReadFromJsonAsync` / `HttpResponse.WriteAsJsonAsync`; `[FromBody(EmptyBodyBehavior = ...)]`; `IAuthorizationMiddlewareResultHandler`; endpoint `AllowAnonymous()`; console logger formatters including JSON (`AddJsonConsole`); Kestrel endpoint-specific configuration; HTTP/2 and gRPC performance work; and in an ASP.NET Core app, JSON request bodies accept quoted numbers (`JsonSerializerDefaults.Web`).[^16]

##### EF Core 5 [New in .NET 5]

Many-to-many without a join entity, split queries (`AsSplitQuery`), filtered `Include`, `LogTo` simple logging, `ToQueryString()`, `DbContextFactory` (`AddDbContextFactory`), table-per-type mapping, `HasPrecision`, property bags/shared-type entities, SQLite table rebuilds in migrations, and event counters. EF Core 5 targets `netstandard2.1`, so it **does not run on .NET Framework**.[^17][^18]

---

#### Do Not Use or Recommend (C# 10+ / .NET 6+)

The project's compiler and runtime do not have these. The syntax items were checked with SDK 10.0.402 at `LangVersion 9` (most fail with CS8773 "Feature '…' is not available in C# 9.0. Please use language version N or greater"; static abstract members with CS8703) and every API item to be missing from the `net5.0` reference assemblies (or the 5.0 packages). On a C# 9 project that targets `net6.0`+ the *APIs* may exist — check the TFM — but the *syntax* is still unavailable.

##### Language (C# 10+)

| Do not suggest | Introduced in |
|---|---|
| `record struct`, `readonly record struct`, `record class` | C# 10 |
| `global using`, implicit usings (`<ImplicitUsings>`) | C# 10 / .NET 6 SDK |
| File-scoped namespaces (`namespace Foo;`) | C# 10 |
| Extended property patterns (`{ Inner.Value: 1 }`) — write `{ Inner: { Value: 1 } }` | C# 10 |
| Constant interpolated strings (`const string B = $"{A}b";`) | C# 10 |
| Lambda natural type (`var f = (int x) => x;`), explicit lambda return types, attributes on lambdas | C# 10 |
| Mixed declaration/assignment in deconstruction (`(x, int y) = ...`) | C# 10 |
| Parameterless struct constructors, field initializers in structs | C# 10 |
| `sealed override ToString()` in records | C# 10 |
| `[CallerArgumentExpression]` taking effect (the attribute type exists in .NET 5, but C# 9 callers get the parameter's default value) | C# 10 |
| Interpolated string handlers (`DefaultInterpolatedStringHandler`, `string.Create(provider, $"...")`) | C# 10 / .NET 6 |
| `required` members, `[SetsRequiredMembers]` | C# 11 / .NET 7 |
| Raw string literals (`"""..."""`), UTF-8 literals (`"x"u8`) | C# 11 |
| List patterns (`[1, .., 3]`), slice patterns | C# 11 |
| Generic attributes (`[Foo<int>]`), `file`-scoped types | C# 11 |
| Static abstract/virtual interface members, generic math (`INumber<T>`) | C# 11 / .NET 7 |
| `nint`/`IntPtr` full unification, `>>>` operator | C# 11 |
| Primary constructors for classes/structs, collection expressions (`[1, 2, 3]`), `using` aliases for any type, lambda default parameters | C# 12 |
| `params` collections, `\e` escape, `System.Threading.Lock` semantics, `ref struct` locals such as `Span<T>` in async methods (CS4012 in C# 9) | C# 13 / .NET 9 |
| `field` keyword, `extension` blocks, null-conditional assignment (`a?.B = c`), first-class span conversions | C# 14 |

`field` deserves a note: in C# 9, `get => field;` compiles only if a member named `field` exists, and then it refers to *that member* (CS0103 otherwise).

##### Libraries and frameworks (.NET 6+)

| Do not suggest | Use in .NET 5 instead | Introduced in |
|---|---|---|
| `ArgumentNullException.ThrowIfNull(x)` | `x ?? throw new ArgumentNullException(nameof(x))` or an `if` guard | .NET 6 |
| `ArgumentException.ThrowIfNullOrEmpty`, `ObjectDisposedException.ThrowIf` | explicit `if`/`throw` | .NET 7 |
| `ArgumentOutOfRangeException.ThrowIfNegative` etc. | explicit `if`/`throw` | .NET 8 |
| `DateOnly`, `TimeOnly` | `DateTime` (`.Date`), `TimeSpan` | .NET 6 |
| `PriorityQueue<TElement,TPriority>` | `SortedSet<T>` with a tie-breaker, or a library | .NET 6 |
| LINQ `Chunk`, `MaxBy`, `MinBy`, `DistinctBy`, `ExceptBy`, `IntersectBy`, `UnionBy`, `TryGetNonEnumeratedCount`, `Take(Range)`, `FirstOrDefault(defaultValue)` | `GroupBy`/`Select` helpers, `OrderBy(...).First()`, `Aggregate` | .NET 6 |
| `Order()`/`OrderDescending()` | `OrderBy(x => x)` | .NET 7 |
| `CountBy`, `AggregateBy`, `Index()` | `GroupBy(...).Select(g => (g.Key, g.Count()))` | .NET 9 |
| `Parallel.ForEachAsync` | `SemaphoreSlim` + `Task.WhenAll`, or `Channel<T>` workers | .NET 6 |
| `Task.WaitAsync(TimeSpan/CancellationToken)` | `Task.WhenAny(task, Task.Delay(timeout, ct))` | .NET 6 |
| `PeriodicTimer` | `Task.Delay` loop in a `BackgroundService` | .NET 6 |
| `Random.Shared` | a `[ThreadStatic]`/`ThreadLocal<Random>` instance, or `RandomNumberGenerator` | .NET 6 |
| `CancellationTokenSource.TryReset` / `CancelAsync` | new `CancellationTokenSource` / `Cancel()` | .NET 6 / .NET 8 |
| `string.ReplaceLineEndings` | `Replace("\r\n", "\n")` | .NET 6 |
| `Stream.ReadExactly` / `ReadAtLeast` | loop on `Read`/`ReadAsync` until the count is reached | .NET 7 |
| `File.ReadLinesAsync` | `StreamReader.ReadLineAsync` loop | .NET 7 |
| `Directory.CreateTempSubdirectory` | `Path.Combine(Path.GetTempPath(), Path.GetRandomFileName())` + `CreateDirectory` | .NET 7 |
| `RandomNumberGenerator.GetBytes(int)`, `HMACSHA256.HashData`, `Rfc2898DeriveBytes.Pbkdf2` (static) | `RandomNumberGenerator.Fill(span)`, `new HMACSHA256(key).ComputeHash`, `new Rfc2898DeriveBytes(pw, salt, iter, HashAlgorithmName.SHA256)` | .NET 6 |
| `System.Text.Json.Nodes` (`JsonNode`, `JsonObject`), `ReferenceHandler.IgnoreCycles`, source generation (`JsonSerializerContext`), `SerializeAsyncEnumerable`/`DeserializeAsyncEnumerable` | `JsonDocument`/`JsonElement`, DTOs without cycles, reflection serializer | .NET 6 |
| `[JsonPolymorphic]`/`[JsonDerivedType]`, `[JsonRequired]` | custom `JsonConverter<T>` | .NET 7 |
| `JsonNamingPolicy.SnakeCaseLower`, `UnmappedMemberHandling` | custom `JsonNamingPolicy` | .NET 8 |
| `JsonSerializerOptions.Web` (static), `RespectNullableAnnotations` | `new JsonSerializerOptions(JsonSerializerDefaults.Web)`, manual validation | .NET 9 |
| `IServiceProvider.CreateAsyncScope()` / `AsyncServiceScope` | cast the scope to `IAsyncDisposable` (see DI section) | .NET 6 |
| `OptionsBuilder.ValidateOnStart()` | resolve `IOptions<T>.Value` at startup in a hosted service | .NET 6 |
| `HostOptions.BackgroundServiceExceptionBehavior` | catch-all in `ExecuteAsync` + `IHostApplicationLifetime.StopApplication()` | .NET 6 |
| `[LoggerMessage]` source generator | `LoggerMessage.Define<...>` | .NET 6 |
| `WebApplication`/`WebApplicationBuilder`, minimal APIs with parameter binding (`app.MapGet("/x", (int id) => ...)`), `Results`/`TypedResults` | `Startup` + controllers, or `endpoints.MapGet("/x", async context => ...)` | .NET 6 / .NET 7 |
| `UseRateLimiter`, `UseOutputCache`, `AddProblemDetails` | third-party middleware, `ResponseCaching`, `ProblemDetailsFactory` | .NET 7 |
| `System.Diagnostics.Metrics.Meter` | `EventCounters` (`EventSource`), OpenTelemetry/Prometheus libraries | .NET 6 |
| `TimeProvider`, `FrozenDictionary`, `SearchValues` | an injected clock interface, `ImmutableDictionary`/`Dictionary` | .NET 8 |
| `System.Threading.Lock`, `Task.WhenEach`, `Guid.CreateVersion7()` | `lock (object)`, `WhenAny` loop, a ULID/UUIDv7 library | .NET 9 |
| `UnreachableException`, `RegexOptions.NonBacktracking`, Regex source generator (`[GeneratedRegex]`) | `InvalidOperationException`, match timeouts, `RegexOptions.Compiled` | .NET 7 |
| `Environment.ProcessPath`, `NativeMemory`, `Stopwatch.GetElapsedTime` | `Process.GetCurrentProcess().MainModule?.FileName`, `Marshal.AllocHGlobal`, `Stopwatch.GetTimestamp` arithmetic | .NET 6 / .NET 7 |
| EF Core `ExecuteUpdate`/`ExecuteDelete`, `Database.SqlQuery<T>`, JSON columns, compiled models | load-and-save, `ExecuteSqlInterpolated`, `FromSqlInterpolated` on an entity type | EF Core 7 / 8 |

***

#### Deprecated, Obsoleted and Removed APIs in .NET 5

| API | Status in .NET 5 | Replacement |
|---|---|---|
| `BinaryFormatter.Serialize/Deserialize` | SYSLIB0011 warning; throws `NotSupportedException` in ASP.NET Core apps | `System.Text.Json`, `XmlSerializer`, protobuf |
| `Encoding.UTF7`, `UTF7Encoding` ctor | SYSLIB0001 | UTF-8 |
| `PrincipalPermissionAttribute` | SYSLIB0002 **error** | ASP.NET Core authorization |
| Code access security (`SecurityPermission`, `[PermissionSet]` …) | SYSLIB0003 | OS / container isolation |
| Constrained execution regions | SYSLIB0004 | — |
| GAC APIs (`Assembly.GlobalAssemblyCache`) | SYSLIB0005 | — |
| `Thread.Abort` | SYSLIB0006; throws `PlatformNotSupportedException` | cooperative `CancellationToken` |
| `SymmetricAlgorithm.Create()` etc. (parameterless default-implementation `Create`) | SYSLIB0007 | `Aes.Create()`, `SHA256.Create()` |
| `AuthenticationManager` | SYSLIB0009 | `HttpClient` handlers |
| Remoting APIs (`MarshalByRefObject.GetLifetimeService`) | SYSLIB0010 | gRPC, HTTP |
| `Assembly.CodeBase`/`EscapedCodeBase` | SYSLIB0012 | `Assembly.Location` (empty in single-file apps) |
| Built-in WinRT support | removed | C#/WinRT projection package |
| `WinHttpHandler` in the shared framework | removed | NuGet `System.Net.Http.WinHttpHandler` |
| `Microsoft.DotNet.PlatformAbstractions` package | removed | `RuntimeInformation` |

Not yet obsolete in .NET 5 (obsoleted later, still worth flagging): `WebRequest`/`WebClient`/`HttpWebRequest` (SYSLIB0014 in .NET 6), `RNGCryptoServiceProvider` and the derived `*CryptoServiceProvider`/`*Managed` crypto types (SYSLIB0021/0023 in .NET 6), `JsonSerializerOptions.IgnoreNullValues` (SYSLIB0020 in .NET 6 — no warning on .NET 5, verified), `Rfc2898DeriveBytes` constructors without a `HashAlgorithmName` (SYSLIB0041 in .NET 7; they default to **SHA-1 and 1,000 iterations** — verified).[^13]

***

#### Version-Aware Review

- **Determine the effective language version and TFM first**: `LangVersion` in the `.csproj`, `Directory.Build.props`, or the TFM default table. A `net5.0` project without `LangVersion` is C# 9; a `netstandard2.0` project without `LangVersion` is C# 7.3 and cannot use anything in this guide.
- **Verify an API against `net5.0` (or the actual TFM) before claiming it is unavailable**, and only say "not available in .NET 5" when the API was introduced later (see the table above). Many "modern" APIs are older than they look and *are* available: `Convert.ToHexString` (5), `SHA256.HashData` (5), `Enum.GetValues<T>()` (5), `Half` (5), `ToHashSet` (Core 2.0), `Math.Clamp` (Core 2.0), `CryptographicOperations.FixedTimeEquals` (Core 2.1), `RandomNumberGenerator.GetInt32` (Core 3.0), `Path.Join` (Core 3.0), `IAsyncEnumerable<T>`/`await foreach` (Core 3.0, C# 8), `Channel<T>` (Core 3.0), `HttpClient.GetFromJsonAsync` (5).
- **Do not "modernize" C# 9 code with C# 10+ syntax** (file-scoped namespaces, `record struct`, `required`, collection expressions, primary constructors). Suggest the C# 9 equivalent, and suggest upgrading the TFM as a separate, larger change.
- **Distinguish compile-time from runtime claims.** A C# 9 project on `net6.0` has .NET 6 APIs; a C# 9 library on `netstandard2.0` has neither .NET 5 APIs nor covariant returns.
- **Check package versions against the TFM.** Many current packages no longer ship `net5.0` assets and fall back to `netstandard2.0` builds or fail to restore. Pin `Microsoft.Extensions.*`, EF Core and ASP.NET Core packages to 5.0.x on `net5.0`; EF Core 6+ requires .NET 6.
- **Prefer "upgrade to .NET 10" over local workarounds** when a review finds a missing modern API in a long-lived service — but still provide the .NET 5-compatible fix for the code under review.

***

#### Project Setup, Build and Analyzers

##### Recommended project file

```xml
<Project Sdk="Microsoft.NET.Sdk">
  <PropertyGroup>
    <TargetFramework>net5.0</TargetFramework>
    <!-- Explicit even though it is the default: documents intent and survives TFM changes -->
    <LangVersion>9.0</LangVersion>
    <Nullable>enable</Nullable>
    <TreatWarningsAsErrors>true</TreatWarningsAsErrors>
    <!-- .NET 5 SDK values: Default | AllEnabledByDefault | AllDisabledByDefault -->
    <AnalysisMode>AllEnabledByDefault</AnalysisMode>
    <Deterministic>true</Deterministic>
  </PropertyGroup>
</Project>
```

```json
// global.json — pin the SDK the project was built and tested with
{ "sdk": { "version": "5.0.408", "rollForward": "latestFeature" } }
```

- **Analyzers are on by default for `net5.0`** (`EnableNETAnalyzers` defaults to `true`, `AnalysisLevel` to `latest`). Only a handful of CA rules are warnings out of the box; the ones a C# 9 review should expect to see are CA2200 (rethrow with `throw ex`), CA2013 (`ReferenceEquals` on value types), CA2014 (`stackalloc` in a loop), CA2247 (`TaskContinuationOptions` passed to `TaskCompletionSource`), CA1416 (platform compatibility), CA1417 and CA1831 — all listed as new .NET 5 warnings; CA2200, CA2013, CA2014 and CA2247 were triggered in a test build.[^10][^19]
- **`AnalysisMode` values differ by SDK.** The .NET 5 SDK understands `AllEnabledByDefault`/`AllDisabledByDefault`/`Default`; the .NET 6+ values `All`, `Recommended`, `Minimum`, `None` are silently ignored by SDK 5.0.408 (verified: `All` produced 1 CA rule, `AllEnabledByDefault` 25 on the same code). A project file with `<AnalysisMode>All</AnalysisMode>` on SDK 5 therefore has no extra analysis.
- `TreatWarningsAsErrors` plus `<WarningsAsErrors>nullable</WarningsAsErrors>` (or the whole set) keeps nullable warnings from accumulating.
- `<Nullable>enable</Nullable>` at project level; avoid file-level `#nullable disable` except in generated code.
- Use `Directory.Build.props` for shared settings. Note that the .NET 5 SDK **imports a `Directory.Packages.props` file by default** (a breaking change that can surprise solutions that already had a file of that name); central package management with `ManagePackageVersionsCentrally` itself only works with later NuGet versions.[^10]
- `dotnet format` is a separate global tool for .NET 5 (it became part of the SDK in .NET 6).

##### Single-file and trimming

`PublishSingleFile` and `PublishTrimmed` are supported in .NET 5. Review points: `Assembly.Location` and `AppContext.BaseDirectory` semantics (`Location` is empty in single-file apps — use `AppContext.BaseDirectory`), reflection-heavy code (DI with open generics, `System.Text.Json` reflection serialization, EF Core) is not trim-safe and needs `[DynamicallyAccessedMembers]` annotations or must not be trimmed — the trim-compatibility Roslyn analyzer only arrives with .NET 6, so .NET 5 gives little warning.[^21] `EnableUnsafeBinaryFormatterSerialization=false` and `EnableUnsafeUTF7Encoding=false` are feature switches that remove those insecure code paths.

***

#### Naming and Code Style

House rules (unchanged from the generic C# guide; all valid for C# 9):

```csharp
// PascalCase for types, methods, properties, events, constants and namespaces
public class UserService { }
public const int MaxRetryCount = 3;            // constants are PascalCase, not SCREAMING_CASE

// Interfaces start with I; async methods end with Async
public interface IUserRepository
{
    Task<User?> FindAsync(int id, CancellationToken cancellationToken = default);
}

public class Order
{
    // Private fields: _camelCase; parameters and locals: camelCase
    private readonly IOrderRepository _repository;
    private int _itemCount;

    public Order(IOrderRepository repository) =>
        _repository = repository ?? throw new ArgumentNullException(nameof(repository));

    // Booleans read as predicates
    public bool IsActive { get; set; }
    public bool HasDiscount { get; set; }

    // init for values fixed at creation (C# 9)
    public DateTime CreatedAt { get; init; }
}
```

- One top-level type per file; file name = type name. Block-scoped namespaces (`namespace X { ... }`) — file-scoped namespaces are C# 10.
- Member order: constants, static fields, instance fields, constructors, properties, public methods, private methods.
- Guard clauses use `?? throw new ArgumentNullException(nameof(x))` or `if (x is null) throw ...` — **not** `ArgumentNullException.ThrowIfNull` (.NET 6).
- Prefer `is null`/`is not null` over `== null`/`!= null`; patterns ignore user-defined operators.
- `var` when the type is obvious from the right-hand side; target-typed `new()` when the type is obvious from the left. Write the type once: `var x = new Foo();` or `Foo x = new();`, not `Foo x = new Foo();`.
- Expression-bodied members for one-liners; block bodies once there is branching.
- Records for immutable data (`public record UserDto(int Id, string Name);`), classes for entities and services.
- Solution layout: `src/MyApp.Api`, `src/MyApp.Core`, `src/MyApp.Infrastructure`, `tests/MyApp.UnitTests`, `tests/MyApp.IntegrationTests`.

***

#### Nullable Reference Types

```csharp
public class Connection
{
    public string Name { get; private set; }

    public Connection() => Init();

    [MemberNotNull(nameof(Name))]                // .NET 5 attribute: Init() guarantees Name is set
    private void Init() => Name = "default";

    public bool TryGet(string key, [NotNullWhen(true)] out string? value)
    {
        value = key == "a" ? "x" : null;
        return value is not null;
    }

    [return: NotNullIfNotNull("s")]               // nameof(s) on a parameter is C# 11; use a string here
    public static string? Trim(string? s) => s?.Trim();
}
```

- **`MemberNotNull`/`MemberNotNullWhen` are new in .NET 5**; `NotNullWhen`, `MaybeNullWhen`, `NotNullIfNotNull`, `DoesNotReturn` exist since .NET Core 3.0. On `netstandard2.0` they need a polyfill.
- **`null!` and `default!` only silence the compiler.** `string s = null!; s.Length` still throws `NullReferenceException`. Accept `= null!` only for properties set by a framework (EF Core navigation properties, `[Inject]`) and say why in a comment.
- **Arrays and `default(T)` are holes**: `new string[2]` contains nulls typed as `string`; `default(KeyValuePair<string,string>).Key` is `null` (verified). Struct defaults and array allocations are not checked.
- **Deserializers ignore annotations.** System.Text.Json 5.0 happily puts `null` into a non-nullable `string` property or constructor parameter, and a missing constructor argument becomes `default` (verified). Validate DTOs after deserialization (`[Required]` + model validation in ASP.NET Core, or explicit checks).
- **Generic `T?`** on an unconstrained type parameter is allowed in C# 9 (it means "default of T", maybe null). `where T : notnull` documents non-null generic arguments.
- Public library APIs: annotate everything; never `#nullable disable` a public surface. Review new `!` operators with suspicion — each one is an unproven claim.
- `Dictionary.TryGetValue(key, out var v)` leaves `v` as `null`/default when it returns `false`; the BCL's `[MaybeNullWhen(false)]` annotation lets the compiler catch misuse.

***

#### Records, Classes and Structs

When to use what in C# 9:

| Need | Use |
|---|---|
| DTO, message, event, value object, config snapshot | `public sealed record X(...)` (positional) or `record` with `init` properties |
| Entity with identity (EF Core), mutable aggregate | `class` |
| Small, immutable, allocation-sensitive value | `readonly struct` with `IEquatable<T>` (no `record struct` in C# 9) |
| Service/stateful component | `class` (often `sealed`) |

```csharp
// Value object with validation that also holds after `with`
public sealed record Email
{
    private readonly string _value = "";

    public Email(string value) => Value = value;

    public string Value
    {
        get => _value;
        init => _value = value is not null && value.Contains('@')
            ? value
            : throw new ArgumentException("Invalid e-mail address.", nameof(value));
    }

    public override string ToString() => Value;
}

// readonly struct instead of `record struct` (C# 10)
public readonly struct Money : IEquatable<Money>
{
    public Money(decimal amount, string currency) { Amount = amount; Currency = currency; }
    public decimal Amount { get; }
    public string Currency { get; }
    public bool Equals(Money other) => Amount == other.Amount && Currency == other.Currency;
    public override bool Equals(object? obj) => obj is Money m && Equals(m);
    public override int GetHashCode() => HashCode.Combine(Amount, Currency);
    public static bool operator ==(Money a, Money b) => a.Equals(b);
    public static bool operator !=(Money a, Money b) => !a.Equals(b);
}
```

- Structs without `IEquatable<T>` use reflection-based `ValueType.Equals` (slow, boxes). Always implement equality on structs used as keys.
- Mutable structs in collections/`readonly` fields lead to "modifying a copy" bugs; keep structs `readonly`.
- Classes: `sealed` by default unless designed for inheritance (enables devirtualization, clarifies intent).
- Records: see "What's New → Records" for shallow `with`, collection equality, `ToString` leaks, parameter-targeted attributes, and validation bypass.

***

#### Pattern Matching

```csharp
public decimal Discount(object customer) => customer switch
{
    PremiumCustomer p => p.PurchaseTotal * 0.2m,
    RegularCustomer { YearsActive: > 5 } r => r.PurchaseTotal * 0.1m,   // relational inside property pattern
    RegularCustomer r => r.PurchaseTotal * 0.05m,
    null => 0m,
    _ => throw new ArgumentException("Unknown customer type", nameof(customer)),
};

public string Shipping(Order order) => order switch
{
    { Total: > 100, IsPriority: true } => "Express",
    { Total: > 100 } => "Standard",
    { IsPriority: true } => "Priority",
    _ => "Economy",
};

// Nested property patterns: C# 9 needs the nested form — `{ Address.City: "Berlin" }` is C# 10
public bool IsBerlin(Customer c) => c is { Address: { City: "Berlin" } };
```

Pitfalls (verified on 5.0.17):

- **`not` binds tighter than `or`/`and`.** `o is not string or null` means `(not string) or null`, so it is `true` for `null` and for `1`, and `false` only for strings. Write `o is not (string or null)` when that is what you mean.
- **NaN matches no relational pattern.** `double.NaN switch { < 0 => ..., 0 => ..., > 0 => ..., _ => "NaN" }` hits the discard. A switch without `_` over `double` gets CS8509 for exactly this reason — handle `double.NaN` explicitly.
- **Enums are never exhaustive.** Handling all named members still yields warning CS8524, and an out-of-range value throws `SwitchExpressionException` at runtime. Add `_ => throw new ArgumentOutOfRangeException(nameof(value), value, null)`.
- **Relational patterns need constants.** `x is > limit` is CS0150; use `x > limit`.
- **Type patterns do not convert.** A boxed `long` is not `int`: `(object)5L is int` is `false`; `is long and > 3` is `true`.
- `is not null` checks are preferred to `!= null` for types with overloaded equality (patterns never call user operators).
- List patterns (`[1, .., 3]`) are C# 11 — use `Length`/indexers or LINQ in C# 9.
- Order switch arms from specific to general; the compiler reports error CS8510 ("The pattern is unreachable") for arms subsumed by earlier ones.

***

#### Strings, Globalization and Numbers

```csharp
// BAD: culture-sensitive by default (ICU) — "\r\n" is one text element
int i = text.IndexOf("\n");                                   // -1 for "line1\r\nline2"

// GOOD: ordinal for protocol/identifier/log text
int j = text.IndexOf("\n", StringComparison.Ordinal);          // 6
int k = text.IndexOf('\n');                                    // char overloads are ordinal
bool same = string.Equals(a, b, StringComparison.OrdinalIgnoreCase);
```

- `string.IndexOf(string)`, `StartsWith(string)`, `EndsWith(string)`, `Compare`, `CompareTo` and `ToUpper()`/`ToLower()` without arguments use the **current culture**. With ICU, ignorable characters change results: `"\u00ADabc".StartsWith("abc")` is `true` culturally (soft hyphen ignored) but `false` with `StringComparison.Ordinal` (verified). In Turkish culture `"i".ToUpper()` is `"İ"`.
- `Equals`, `==`, `Contains(string)`, `Replace(string, string)` and `Split` are ordinal.
- For identifiers, keys, file names, HTTP headers and security decisions use `StringComparison.Ordinal`/`OrdinalIgnoreCase` and `ToUpperInvariant()`. The CA1307/CA1309/CA1310 analyzers flag the culture-sensitive overloads when enabled (they are off by default).
- Apps that must keep NLS behaviour on Windows can set `System.Globalization.UseNls` — treat that as a migration crutch. Invariant mode (`InvariantGlobalization`) is common in containers; it changes culture formatting and is a behaviour change to review.
- **`DateTime.Parse("...Z")` returns `DateTimeKind.Local`** (converted to the machine's zone, verified). Use `DateTimeOffset` for timestamps, `DateTime.UtcNow` for "now", and `DateTimeStyles.AdjustToUniversal | AssumeUniversal` when parsing to UTC. There is no `DateOnly` (.NET 6).
- **Money is `decimal`.** `0.1 + 0.2 == 0.3` is `false` for `double`, `true` for `decimal`.
- `Math.Round(2.5)` is `2` (banker's rounding); pass `MidpointRounding.AwayFromZero` for commercial rounding.
- Integer overflow wraps silently in the default `unchecked` context (`int.MaxValue + 1 == int.MinValue`). Use `checked` for sizes, money in minor units, and anything security-relevant, or enable `<CheckForOverflowUnderflow>` in debug builds.
- `Half` (new in .NET 5) is a storage format: `(Half)65520f` is `Infinity` (verified). Do not compute with it.
- Format and parse with an explicit culture at system boundaries (`CultureInfo.InvariantCulture` for files/protocols).

***

#### Collections, Span and Memory

- Expose read-only abstractions (`IReadOnlyList<T>`, `IReadOnlyDictionary<TKey,TValue>`) from APIs; keep `List<T>` internal. Return empty collections (`Array.Empty<T>()`), never `null`.
- Pre-size collections when the count is known (`new List<T>(count)`, `new Dictionary<K,V>(count)`).
- `Dictionary` lookups: `TryGetValue` instead of `ContainsKey` + indexer (two lookups).
- `ConcurrentDictionary.GetOrAdd(key, factory)` may run the factory **more than once** under contention (4 calls with 8 racing threads in a test on 5.0.17); only one result is stored. Wrap expensive or side-effecting factories in `Lazy<T>` (verified: exactly one call):

```csharp
private readonly ConcurrentDictionary<string, Lazy<Task<Config>>> _cache = new();

public Task<Config> GetAsync(string key) =>
    _cache.GetOrAdd(key, k => new Lazy<Task<Config>>(() => LoadAsync(k))).Value;
```

- `Span<T>`/`ReadOnlySpan<T>` for slicing without allocation; `Memory<T>` when the buffer must live across `await` (spans cannot be locals in `async` methods in C# 9 — CS4012).
- `ArrayPool<T>.Shared.Rent` returns arrays **at least** the requested size and not cleared; always use the requested length and `Return` in `finally` (pass `clearArray: true` for sensitive data).
- `stackalloc` only for small, bounded sizes and never in loops (CA2014); fall back to the pool above a threshold:

```csharp
public static string ToHex(ReadOnlySpan<byte> data)
{
    const int MaxStack = 256;
    byte[]? rented = null;
    Span<byte> buffer = data.Length <= MaxStack
        ? stackalloc byte[MaxStack]
        : (rented = ArrayPool<byte>.Shared.Rent(data.Length));
    try
    {
        data.CopyTo(buffer);
        return Convert.ToHexString(buffer.Slice(0, data.Length));   // .NET 5 API
    }
    finally
    {
        if (rented is not null) ArrayPool<byte>.Shared.Return(rented);
    }
}
```

- `CollectionsMarshal.AsSpan(list)` (new in .NET 5) gives a span over a `List<T>`'s backing array; it is invalid after the list grows. Use only in local hot paths.
- Immutable collections (`System.Collections.Immutable`) for records and shared snapshots; `ImmutableArray<T>` has the lowest overhead but is a struct whose `default` is unusable (`IsDefault`).

***

#### LINQ

```csharp
// Method syntax for simple pipelines
var activeNames = users
    .Where(u => u.IsActive)
    .OrderBy(u => u.Name, StringComparer.Ordinal)
    .Select(u => u.Name)                  // project early
    .ToList();

// Query syntax where joins/grouping read better
var summary =
    from order in orders
    join customer in customers on order.CustomerId equals customer.Id
    where order.Total > 100
    group order by customer.Name into g
    select new { Customer = g.Key, Total = g.Sum(o => o.Total) };
```

Pitfalls (verified):

- **Deferred execution.** A query re-runs on every enumeration: calling `Any()` then `Count()` on a lazy `IEnumerable` enumerates the source twice; a `Where` over a `List<T>` sees items added later. Materialize once (`ToList()`/`ToArray()`) when the result is enumerated more than once or when the source may change.
- **`Any()` over `Count() > 0`** for lazy sequences; use the `Count`/`Length` property on materialized collections.
- **`First`/`Single` vs `...OrDefault`.** `Single` throws when more than one element matches; `FirstOrDefault` on `int` returns `0`, which may be a valid value — use `Any`, nullable projections (`.Select(x => (int?)x).FirstOrDefault()`) or `TryGet` patterns. `FirstOrDefault(defaultValue)` is .NET 6.
- **`Max`/`Min`/`Average` on an empty non-nullable sequence throw** `InvalidOperationException`; on `int?` sequences they return `null`.
- **`ToDictionary` throws on duplicate keys** (`ArgumentException`); use `GroupBy` or `ToLookup` (a missing lookup key returns an empty sequence).
- **Side effects in selectors/predicates** are executed lazily, possibly multiple times, possibly not at all (see the .NET 5 `OrderBy().First(predicate)` change).
- `MaxBy`/`DistinctBy`/`Chunk` do not exist in .NET 5 — use `OrderByDescending(x => x.Key).First()` (O(n log n)), `Aggregate`, or `GroupBy(x => x.Key).Select(g => g.First())`.
- On `IQueryable` (EF Core), everything in the expression must be translatable — see EF Core 5.
- Avoid LINQ in tight inner loops of hot paths; it allocates enumerators and delegates.

***

#### Exceptions and Error Handling

```csharp
public async Task<User> GetUserAsync(int id, CancellationToken cancellationToken)
{
    var user = await _repository.FindAsync(id, cancellationToken).ConfigureAwait(false);
    return user ?? throw new NotFoundException($"User {id} not found");
}

try
{
    await ProcessAsync(cancellationToken);
}
catch (OperationCanceledException) when (cancellationToken.IsCancellationRequested)
{
    // expected shutdown: do not log as error
}
catch (HttpRequestException ex) when (ex.StatusCode == HttpStatusCode.NotFound)   // StatusCode is new in .NET 5
{
    return null;
}
```

- **Rethrow with `throw;`**, never `throw ex;` — the latter loses the original stack frame (verified; CA2200 warns by default in .NET 5). To rethrow from elsewhere use `ExceptionDispatchInfo.Capture(ex).Throw()`.
- **Exception filters (`when`) run before unwinding**: they see the original stack; a filter that logs and returns `false` is a clean "log without catching" idiom.
- Catch specific exceptions; `catch (Exception)` only at boundaries (request pipeline, background loop, `Main`) and always log it.
- Do not use exceptions for control flow in hot paths (`int.TryParse`, `Dictionary.TryGetValue`).
- Custom exceptions: `sealed`, end in `Exception`, provide the standard constructors; do not add `[Serializable]` plumbing for `BinaryFormatter`.
- `HttpRequestException.StatusCode` (nullable) is **new in .NET 5**; `EnsureSuccessStatusCode` fills it.
- Guard clauses: `ArgumentNullException.ThrowIfNull` and friends are .NET 6+; use `param ?? throw new ArgumentNullException(nameof(param))` and explicit `ArgumentOutOfRangeException(nameof(x), x, "message")`.
- `AggregateException`: `await` surfaces only the first inner exception of `Task.WhenAll`; inspect the task's `Exception.InnerExceptions` for all of them (verified: 2 inner exceptions, the first rethrown).

***

#### Async/Await, Cancellation and Concurrency

```csharp
// Library code: no context capture, flow the token, return Task
public async Task<Data> FetchDataAsync(Uri uri, CancellationToken cancellationToken = default)
{
    using var response = await _http.GetAsync(uri, HttpCompletionOption.ResponseHeadersRead, cancellationToken)
        .ConfigureAwait(false);
    response.EnsureSuccessStatusCode();
    return (await response.Content.ReadFromJsonAsync<Data>(cancellationToken: cancellationToken)
        .ConfigureAwait(false))!;
}
```

Rules:

- **Never block on async** (`.Result`, `.Wait()`, `.GetAwaiter().GetResult()`) in request paths or UI code — deadlocks with a synchronization context and thread-pool starvation everywhere.
- **No `async void`** except event handlers, which must catch everything (an exception in `async void` crashes the process).
- **`Parallel.ForEach` with an `async` lambda is a bug**: the lambda becomes `async void`, `ForEach` returns immediately (verified: 0 of 3 items completed when it returned) and exceptions crash the process. `Parallel.ForEachAsync` is .NET 6; in .NET 5 throttle with `SemaphoreSlim`:

```csharp
public async Task ProcessAllAsync(IEnumerable<Item> items, CancellationToken cancellationToken)
{
    using var gate = new SemaphoreSlim(10);
    var tasks = items.Select(async item =>
    {
        await gate.WaitAsync(cancellationToken).ConfigureAwait(false);
        try
        {
            await ProcessItemAsync(item, cancellationToken).ConfigureAwait(false);
        }
        finally
        {
            gate.Release();
        }
    });
    await Task.WhenAll(tasks).ConfigureAwait(false);
}
```

- **Run independent operations concurrently** with `Task.WhenAll`, then read results with `await` (already completed):

```csharp
var usersTask = _users.GetActiveAsync(cancellationToken);
var ordersTask = _orders.GetRecentAsync(cancellationToken);
await Task.WhenAll(usersTask, ordersTask);
return new Dashboard(await usersTask, await ordersTask);
```

- **Timeouts without `Task.WaitAsync` (.NET 6)**: link a timeout into the token rather than abandoning work:

```csharp
using var timeout = CancellationTokenSource.CreateLinkedTokenSource(cancellationToken);
timeout.CancelAfter(TimeSpan.FromSeconds(5));
await DoWorkAsync(timeout.Token);
```

  Dispose `CancellationTokenSource` instances (they hold timers); `TryReset` is .NET 6, so create a new one per operation.
- **Cancellation surfaces as `OperationCanceledException`** (often the derived `TaskCanceledException`); catch the base type and check which token fired.
- **`ConfigureAwait(false)`** in library code; application code in ASP.NET Core (no synchronization context) may omit it but it does no harm.
- **`ValueTask`/`ValueTask<T>`**: await exactly once; never await twice, never `.Result` before completion, never `WhenAll` without `.AsTask()`. Return `ValueTask` only where synchronous completion is common and measured. `ValueTask.CompletedTask` and pooling of async `ValueTask` state machines (opt-in via `DOTNET_SYSTEM_THREADING_POOLASYNCVALUETASKS`) are new in .NET 5.[^20]
- **`lock` cannot contain `await`** (CS1996). Use `SemaphoreSlim(1, 1)` with `WaitAsync`/`Release` in `try/finally`. `System.Threading.Lock` is .NET 9.
- **`Task.Run` is for CPU-bound work**, not for wrapping async I/O in ASP.NET Core.
- **Fire-and-forget** needs explicit error handling and a lifetime owner (a `BackgroundService` or a `Channel<T>` consumer), otherwise exceptions disappear and the work dies with the request scope.
- **`TaskCompletionSource`**: create with `TaskCreationOptions.RunContinuationsAsynchronously` to avoid running continuations inline. Passing `TaskContinuationOptions.RunContinuationsAsynchronously` instead compiles because it binds to the `TaskCompletionSource<T>(object? state)` constructor — the flag becomes the task's `AsyncState` and continuations still run inline; CA2247 (on by default) catches it. The non-generic `TaskCompletionSource` is new in .NET 5.
- **`Random` is not thread-safe** and there is no `Random.Shared` in .NET 5; use one instance per thread or lock around it. For security use `RandomNumberGenerator`.
- **Thread-safety of shared state**: `Interlocked` for counters (`Interlocked.And`/`Or` are new in .NET 5), `ConcurrentDictionary`/`ImmutableInterlocked` for maps, `Channel<T>` for producer/consumer.

***

#### IAsyncEnumerable and Channels

```csharp
public async IAsyncEnumerable<Order> StreamOrdersAsync(
    [EnumeratorCancellation] CancellationToken cancellationToken = default)
{
    var page = 0;
    while (true)
    {
        var batch = await _repository.GetPageAsync(page++, 100, cancellationToken).ConfigureAwait(false);
        if (batch.Count == 0) yield break;
        foreach (var order in batch) yield return order;
    }
}

await foreach (var order in StreamOrdersAsync().WithCancellation(cancellationToken).ConfigureAwait(false))
{
    Handle(order);
}
```

- **`[EnumeratorCancellation]` is mandatory** on the token parameter of an async iterator. Without it, a token supplied through `WithCancellation(...)` is silently ignored (verified: all 5 items produced from an already-cancelled token) and the compiler only warns (CS8425). With it, cancellation stops the loop with `TaskCanceledException`.
- LINQ over `IAsyncEnumerable<T>` is not in the .NET 5 BCL; it needs the `System.Linq.Async` package. ASP.NET Core 5 MVC can return `IAsyncEnumerable<T>` from actions (System.Text.Json buffers it into an array in 5.0; streaming serialization is .NET 6).
- **`System.Threading.Channels`** (in-box) is the producer/consumer primitive: use `Channel.CreateBounded<T>(new BoundedChannelOptions(capacity) { FullMode = BoundedChannelFullMode.Wait })` for back-pressure, `Writer.Complete()` on shutdown, and `await foreach (var x in reader.ReadAllAsync(ct))` to consume. Unbounded channels can exhaust memory when consumers fall behind.

***

#### IDisposable and IAsyncDisposable

```csharp
public sealed class ReportWriter : IAsyncDisposable, IDisposable
{
    private readonly FileStream _stream;
    private bool _disposed;

    public ReportWriter(string path) =>
        _stream = new FileStream(path, FileMode.Create, FileAccess.Write, FileShare.None,
            bufferSize: 4096, useAsync: true);

    public async Task WriteAsync(ReadOnlyMemory<byte> data, CancellationToken cancellationToken = default)
    {
        if (_disposed) throw new ObjectDisposedException(nameof(ReportWriter));   // ObjectDisposedException.ThrowIf is .NET 7
        await _stream.WriteAsync(data, cancellationToken).ConfigureAwait(false);
    }

    public async ValueTask DisposeAsync()
    {
        if (_disposed) return;
        _disposed = true;
        await _stream.DisposeAsync().ConfigureAwait(false);
    }

    public void Dispose()
    {
        if (_disposed) return;
        _disposed = true;
        _stream.Dispose();
    }
}

await using var writer = new ReportWriter(path);
```

- `using var` / `await using var` (C# 8) dispose at the end of the enclosing scope — make sure that scope is not longer than intended (e.g. a whole long-running method holding a connection).
- Sealed classes without unmanaged resources need no finalizer and no `Dispose(bool)` pattern; unsealed base classes that own resources use the full pattern with `GC.SuppressFinalize(this)`.
- **Prefer `SafeHandle`** over finalizers for native resources.
- Disposing an object you did not create (e.g. an injected `HttpClient` or DI-managed service) is a bug; the owner disposes.
- **DI and `IAsyncDisposable`**: a scoped/transient service that implements only `IAsyncDisposable` makes a synchronous `scope.Dispose()` throw `InvalidOperationException` ("…type only implements IAsyncDisposable. Use DisposeAsync…", verified on Microsoft.Extensions.DependencyInjection 5.0). `CreateAsyncScope()` is .NET 6; in .NET 5 dispose the scope asynchronously yourself:

```csharp
var scope = serviceProvider.CreateScope();
try
{
    var worker = scope.ServiceProvider.GetRequiredService<IWorker>();
    await worker.RunAsync(cancellationToken);
}
finally
{
    if (scope is IAsyncDisposable asyncScope) await asyncScope.DisposeAsync();
    else scope.Dispose();
}
```

  Implementing both `IDisposable` and `IAsyncDisposable` on such services avoids the problem.

***

#### Dependency Injection, Options, Configuration and Hosting

##### Registration and lifetimes

```csharp
public void ConfigureServices(IServiceCollection services)
{
    services.AddTransient<IEmailSender, SmtpEmailSender>();          // new instance per resolve
    services.AddScoped<IUserRepository, UserRepository>();          // one per request/scope
    services.AddSingleton<IClock, SystemClock>();                   // one per container

    services.AddScoped<IDbConnection>(sp =>
        new SqlConnection(sp.GetRequiredService<IConfiguration>().GetConnectionString("Default")));
}
```

```csharp
public sealed class OrderService : IOrderService
{
    private readonly IOrderRepository _repository;
    private readonly ILogger<OrderService> _logger;

    public OrderService(IOrderRepository repository, ILogger<OrderService> logger)
    {
        _repository = repository ?? throw new ArgumentNullException(nameof(repository));
        _logger = logger ?? throw new ArgumentNullException(nameof(logger));
    }
}
```

- **Captive dependencies**: a singleton that takes a scoped service keeps one instance (often a `DbContext`) for the lifetime of the app. The default container only detects this when scope validation is on — ASP.NET Core enables `ValidateScopes`/`ValidateOnBuild` in the Development environment only. A plain `BuildServiceProvider()` resolves the captive silently (verified). Enable validation in all test hosts:

```csharp
Host.CreateDefaultBuilder(args)
    .UseDefaultServiceProvider(o => { o.ValidateScopes = true; o.ValidateOnBuild = true; });
```

- Never call `BuildServiceProvider()` inside `ConfigureServices` (creates a second container with duplicate singletons).
- Do not inject `IServiceProvider` to resolve dependencies ad hoc (service locator); inject the dependency or a factory (`Func<T>` is not supported by the default container — register a factory interface).
- Singletons must be thread-safe. Scoped services must not be captured by background threads that outlive the request; create a scope with `IServiceScopeFactory`.

##### Options

```csharp
public sealed class SmtpOptions
{
    public const string SectionName = "Smtp";
    [Required] public string Host { get; set; } = "";
    [Range(1, 65535)] public int Port { get; set; } = 587;
}

services.AddOptions<SmtpOptions>()
    .Bind(configuration.GetSection(SmtpOptions.SectionName))
    .ValidateDataAnnotations()
    .Validate(o => !o.Host.EndsWith(".invalid", StringComparison.Ordinal), "Host must be resolvable");
```

- **Validation is lazy in .NET 5**: failures throw `OptionsValidationException` the first time `IOptions<T>.Value` is read (verified), possibly in the middle of a request. `ValidateOnStart()` is .NET 6 — in .NET 5 resolve the options eagerly at startup:

```csharp
public sealed class OptionsValidationHostedService : IHostedService
{
    private readonly IOptions<SmtpOptions> _smtp;
    public OptionsValidationHostedService(IOptions<SmtpOptions> smtp) => _smtp = smtp;
    public Task StartAsync(CancellationToken cancellationToken) { _ = _smtp.Value; return Task.CompletedTask; }
    public Task StopAsync(CancellationToken cancellationToken) => Task.CompletedTask;
}
// services.AddHostedService<OptionsValidationHostedService>();
// host.StartAsync() now throws OptionsValidationException before serving requests (verified)
```

- `IOptions<T>` is a singleton snapshot (no reload); `IOptionsSnapshot<T>` is scoped and re-reads per request (cannot be injected into singletons); `IOptionsMonitor<T>` is singleton with change notifications.
- Options classes are mutable POCOs with `set` accessors; configuration binding also sets `init` properties via reflection, but binding to records with positional constructors is **not** supported by the 5.0 binder (`Get<T>()` throws `InvalidOperationException`, verified — it needs a parameterless constructor).
- Secrets come from environment variables, user secrets (development) or a secret store — never from `appsettings.json` in the repository.

##### Hosting and background services

```csharp
public sealed class QueueWorker : BackgroundService
{
    private readonly ILogger<QueueWorker> _logger;
    private readonly IHostApplicationLifetime _lifetime;

    public QueueWorker(ILogger<QueueWorker> logger, IHostApplicationLifetime lifetime)
    {
        _logger = logger;
        _lifetime = lifetime;
    }

    protected override async Task ExecuteAsync(CancellationToken stoppingToken)
    {
        try
        {
            while (!stoppingToken.IsCancellationRequested)
            {
                await ProcessNextAsync(stoppingToken);
                await Task.Delay(TimeSpan.FromSeconds(5), stoppingToken);   // PeriodicTimer is .NET 6
            }
        }
        catch (OperationCanceledException) when (stoppingToken.IsCancellationRequested)
        {
            // normal shutdown
        }
        catch (Exception ex)
        {
            _logger.LogCritical(ex, "Queue worker crashed; stopping the application");
            _lifetime.StopApplication();                                    // .NET 5 would otherwise keep running
        }
    }

    private Task ProcessNextAsync(CancellationToken ct) => Task.CompletedTask;
}
```

- **An unhandled exception in `BackgroundService.ExecuteAsync` is swallowed in .NET 5**: the host keeps running without the worker (verified: host alive 400 ms after the worker threw, no log because the exception is only stored in the task). .NET 6 changed the default to stop the host and added `HostOptions.BackgroundServiceExceptionBehavior` (absent in 5.0, verified).[^22] Every `ExecuteAsync` in .NET 5 needs the catch-all above.
- Do not block in `StartAsync`/before the first `await` in `ExecuteAsync` — the host's startup waits for it.
- Graceful shutdown timeout is `HostOptions.ShutdownTimeout` (5 seconds by default); long-running work must observe `stoppingToken`.

***

#### Logging and Observability

```csharp
public sealed partial class OrderProcessor
{
    // High-performance logging in .NET 5: LoggerMessage.Define ([LoggerMessage] source generator is .NET 6)
    private static readonly Action<ILogger, int, string, Exception?> LogOrderShipped =
        LoggerMessage.Define<int, string>(LogLevel.Information, new EventId(1001, "OrderShipped"),
            "Order {OrderId} shipped to {Country}");

    private readonly ILogger<OrderProcessor> _logger;

    public OrderProcessor(ILogger<OrderProcessor> logger) => _logger = logger;

    public void Ship(int orderId, string country)
    {
        LogOrderShipped(_logger, orderId, country, null);

        // GOOD: message template; OrderId is a structured property
        _logger.LogInformation("Order {OrderId} shipped", orderId);

        // BAD: interpolation — the rendered string becomes the template, no properties, allocates even when disabled
        _logger.LogInformation($"Order {orderId} shipped");
    }
}
```

- **Message templates, not interpolation.** With a template the logger state contains `OrderId=7` and `{OriginalFormat}`; with `$"..."` it contains only `{OriginalFormat}=Order 7 shipped` (verified by capturing the state). Interpolated strings also defeat log-level filtering of formatting cost, and interpolated string handlers that would avoid that are C# 10.
- **Arguments are evaluated even when the level is disabled** (verified). Guard expensive arguments with `if (_logger.IsEnabled(LogLevel.Debug))`.
- Pass the exception as the first argument (`_logger.LogError(ex, "...")`), never `ex.Message` in the template.
- Use scopes for correlation (`using (_logger.BeginScope("Order {OrderId}", id))`); console scopes need `IncludeScopes`.
- **New in .NET 5**: console formatters — `AddSimpleConsole(o => o.SingleLine = true)`, `AddJsonConsole()`, `AddSystemdConsole()`; the older `ConsoleLoggerOptions` formatting properties are obsolete.[^10][^16]
- Never log secrets, tokens, full request bodies or personal data; remember that records print every public property.
- **Tracing**: `Activity`/`ActivitySource` (`ActivitySource` is new in .NET 5) with the W3C ID format by default — the basis for OpenTelemetry .NET. **Metrics**: `System.Diagnostics.Metrics` is .NET 6; in .NET 5 use `EventCounters` (`dotnet-counters`) or a metrics library.
- Health checks: `services.AddHealthChecks()` + `endpoints.MapHealthChecks("/healthz")` (verified in the ASP.NET Core 5 sample below).

***

#### HttpClient and IHttpClientFactory

```csharp
public sealed class GitHubClient
{
    private readonly HttpClient _http;
    public GitHubClient(HttpClient http) => _http = http;

    public async Task<Repo?> GetRepoAsync(string owner, string name, CancellationToken cancellationToken)
    {
        using var response = await _http.GetAsync(
            $"repos/{Uri.EscapeDataString(owner)}/{Uri.EscapeDataString(name)}", cancellationToken);
        if (response.StatusCode == HttpStatusCode.NotFound) return null;
        response.EnsureSuccessStatusCode();
        return await response.Content.ReadFromJsonAsync<Repo>(cancellationToken: cancellationToken);
    }
}

public sealed record Repo(string Name, int StargazersCount);

// Registration (typed client)
services.AddHttpClient<GitHubClient>(c =>
{
    c.BaseAddress = new Uri("https://api.github.com/");   // trailing slash matters for relative URIs
    c.Timeout = TimeSpan.FromSeconds(10);
    c.DefaultRequestHeaders.UserAgent.ParseAdd("my-app/1.0");
});
```

- **Never `new HttpClient()` per request** (socket exhaustion through `TIME_WAIT`) and never a single static `HttpClient` with the default handler in a long-running process (stale DNS). Use `IHttpClientFactory` (handler pooling and rotation, default handler lifetime 2 minutes), or one long-lived `HttpClient` over a `SocketsHttpHandler { PooledConnectionLifetime = TimeSpan.FromMinutes(2) }`.
- **Typed clients are transient** (verified: two resolves give two instances). Injecting a typed client into a singleton captures its `HttpClient` — and its handler — forever, defeating handler rotation. Inject typed clients into transient/scoped services only, or inject `IHttpClientFactory` into the singleton.
- **Timeouts vs cancellation**: since .NET 5 a `HttpClient.Timeout` expiry throws `TaskCanceledException` with `InnerException` of type `TimeoutException`; a caller-token cancellation has no `TimeoutException` inner (verified). Log/handle them differently:

```csharp
catch (TaskCanceledException ex) when (ex.InnerException is TimeoutException)
{
    _logger.LogWarning("GitHub request timed out");
}
```

- Dispose `HttpResponseMessage` (`using var response`) — it holds the connection until the content is read or disposed. Use `HttpCompletionOption.ResponseHeadersRead` for large/streaming bodies.
- `System.Net.Http.Json` (in-box in .NET 5): `GetFromJsonAsync`, `PostAsJsonAsync`, `ReadFromJsonAsync` use `JsonSerializerDefaults.Web` (camelCase, case-insensitive, quoted numbers allowed). `GetFromJsonAsync` throws on non-success status codes (`HttpRequestException`) — handle 404 explicitly as above if it is expected.
- Resilience: Polly via `Microsoft.Extensions.Http.Polly` (`AddPolicyHandler`, `AddTransientHttpErrorPolicy`). Only retry idempotent requests; cap retries; combine with a per-try timeout. (`Microsoft.Extensions.Http.Resilience` is .NET 8.)
- Never disable certificate validation (`ServerCertificateCustomValidationCallback = (_, _, _, _) => true` — note the C# 9 lambda discards); flag it in any non-test code.
- `HttpClient.Send` (synchronous) is new in .NET 5; use it only where async is impossible.
- `HttpRequestMessage.Options` replaces the obsolete `Properties` bag in .NET 5.

***

#### System.Text.Json (5.0 Behaviour)

Default `JsonSerializer` settings in .NET 5 and their review consequences (all verified on 5.0.17):

| Behaviour | Default | Consequence / fix |
|---|---|---|
| Property name matching | **case-sensitive** | `{"id":1}` does not bind `Id`; use `new JsonSerializerOptions(JsonSerializerDefaults.Web)` (camelCase + case-insensitive + quoted numbers) or `PropertyNameCaseInsensitive = true`. ASP.NET Core and `System.Net.Http.Json` already use Web defaults. |
| Naming policy | as declared (PascalCase) | Web defaults write camelCase. |
| Records/immutable types | constructor binding (new in 5.0) | Parameter names must match property names (case-insensitively); missing JSON members become `default` (`null` for strings) — no error. |
| Nullable annotations | **ignored** | `null` is written into non-nullable properties; validate. (`RespectNullableAnnotations` is .NET 9.) |
| Unknown members | ignored | No "fail on unknown member" option until .NET 8 (`UnmappedMemberHandling`). Use `[JsonExtensionData]` to capture them. |
| Fields | ignored | `IncludeFields = true` or `[JsonInclude]` (new in 5.0). |
| Non-public setters | ignored | `[JsonInclude]` on the property (new in 5.0) — `private set` + `[JsonInclude]` binds (verified). |
| Enums | numbers | `JsonStringEnumConverter` for strings; numbers break when members are reordered. |
| Cycles | `JsonException` at depth 64 | `ReferenceHandler.Preserve` writes `$id`/`$ref` metadata (non-standard JSON); `IgnoreCycles` is .NET 6. Better: DTOs without cycles. |
| Max depth | 64 | `JsonException` beyond it; keep it for untrusted input. |
| Quoted numbers | rejected | `NumberHandling = JsonNumberHandling.AllowReadingFromString` (new in 5.0; part of Web defaults). |
| Comments / trailing commas | rejected | `ReadCommentHandling = JsonCommentHandling.Skip`, `AllowTrailingCommas = true` for config-like input. |
| `DateTime` | ISO 8601 only | `"10/22/2026"` throws; write a converter for other formats. |
| `object` targets | `JsonElement` | Deserializing to `object` yields `JsonElement`, not dictionaries; no `JsonNode` (.NET 6). |
| HTML-sensitive chars | escaped (`<` → `<`) | Safe default; `JavaScriptEncoder.UnsafeRelaxedJsonEscaping` only when output is never embedded in HTML. |
| Options instance | frozen after first use | Mutating after the first (de)serialization throws `InvalidOperationException`; create options once and cache them — constructing options per call also discards the metadata cache (a large performance cost). |
| `IgnoreNullValues` | works, **not obsolete** in 5.0 | Prefer `DefaultIgnoreCondition = JsonIgnoreCondition.WhenWritingNull` (new in 5.0); `IgnoreNullValues` becomes SYSLIB0020 in .NET 6. |
| `char` | single-character strings only | `"ab"` → `JsonException` (5.0 breaking change). |

```csharp
public static class Json
{
    // Create once; reuse everywhere
    public static readonly JsonSerializerOptions Options = new(JsonSerializerDefaults.Web)
    {
        DefaultIgnoreCondition = JsonIgnoreCondition.WhenWritingNull,
        Converters = { new JsonStringEnumConverter() },
    };
}

public sealed record UserDto(int Id, string Name);

var user = JsonSerializer.Deserialize<UserDto>("{\"id\":\"7\",\"name\":\"a\"}", Json.Options);
// UserDto { Id = 7, Name = a } — quoted number accepted, case-insensitive
```

- Attributes on positional record parameters need the `property:` target: `public record Dto([property: JsonPropertyName("n")] string Name);` — `[JsonPropertyName]` without it is CS0592.
- **Polymorphism**: serializing via a base-typed variable writes only base members (`JsonSerializer.Serialize<Base>(derived)`); serialize as `object` or write a converter. Deserializing polymorphic payloads requires a custom converter with an allow-list of types — never a type name from the payload.
- **Newtonsoft.Json** (`Microsoft.AspNetCore.Mvc.NewtonsoftJson`) remains common in .NET 5 code. Flag `TypeNameHandling` other than `None` on untrusted input (remote code execution gadget chains) and `MaxDepth` left unlimited on old versions (set `MaxDepth`, or use 13.0.1+ where the default is 64).

***

#### ASP.NET Core 5

##### Application shape

ASP.NET Core 5 apps use the generic host with a `Startup` class (verified with ASP.NET Core 5.0.17):

```csharp
public static class Program
{
    public static void Main(string[] args) => CreateHostBuilder(args).Build().Run();

    public static IHostBuilder CreateHostBuilder(string[] args) =>
        Host.CreateDefaultBuilder(args)
            .ConfigureWebHostDefaults(web => web.UseStartup<Startup>());
}

public record CreateOrder([Required] string Sku, [Range(1, 100)] int Quantity);
public record OrderCreated(int Id, string Sku, int Quantity);

public class Startup
{
    public void ConfigureServices(IServiceCollection services)
    {
        services.AddControllers();
        services.AddHealthChecks();
    }

    public void Configure(IApplicationBuilder app, IWebHostEnvironment env)
    {
        if (env.IsDevelopment())
        {
            app.UseDeveloperExceptionPage();
        }
        else
        {
            app.UseExceptionHandler("/error");
            app.UseHsts();
        }

        app.UseHttpsRedirection();
        app.UseRouting();                 // order matters: routing → authN → authZ → endpoints
        app.UseAuthentication();
        app.UseAuthorization();
        app.UseEndpoints(endpoints =>
        {
            endpoints.MapControllers();
            endpoints.MapHealthChecks("/healthz");
            endpoints.MapGet("/ping", async context =>
                await context.Response.WriteAsJsonAsync(new { ok = true }));   // new in 5.0
        });
    }
}

[ApiController]
[Route("api/orders")]
public class OrdersController : ControllerBase
{
    [HttpPost]
    public ActionResult<OrderCreated> Create(CreateOrder request) =>
        CreatedAtAction(nameof(Get), new { id = 42 }, new OrderCreated(42, request.Sku, request.Quantity));

    [HttpGet("{id:int}")]
    public ActionResult<OrderCreated> Get(int id) =>
        id == 42 ? new OrderCreated(42, "abc", 1) : NotFound();
}
```

With `[ApiController]`, invalid models (`quantity: 0`, missing `sku`) return 400 ValidationProblemDetails automatically — the record's parameter-targeted `[Required]`/`[Range]` attributes are honoured by MVC in 5.0 (verified by the integration tests in "Testing").

Review points:

- **Middleware order**: `UseRouting` → `UseCors` → `UseAuthentication` → `UseAuthorization` → `UseEndpoints`. Authorization before authentication, or CORS after authorization, are bugs. `UseDeveloperExceptionPage` only in Development.
- **Minimal APIs do not exist.** `endpoints.MapGet("/x", async context => ...)` is "route-to-code": no parameter binding, no automatic validation, no OpenAPI metadata — read route values from `context.Request.RouteValues`, validate manually, write with `WriteAsJsonAsync`.
- **Authorization**: put `[Authorize]` on controllers (or a fallback policy: `services.AddAuthorization(o => o.FallbackPolicy = new AuthorizationPolicyBuilder().RequireAuthenticatedUser().Build())`), and `[AllowAnonymous]` explicitly. Endpoint-level `.RequireAuthorization()`/`.AllowAnonymous()` exist in 5.0.
- **Over-posting**: bind request DTOs (records), not EF entities.
- **`[FromBody]` with an empty body** returns 400 by default; `[FromBody(EmptyBodyBehavior = EmptyBodyBehavior.Allow)]` is new in 5.0.
- **JSON**: MVC uses System.Text.Json with Web defaults (configure via `AddControllers().AddJsonOptions(o => ...)`); quoted numbers are accepted in ASP.NET Core 5 apps (breaking change from 3.1).[^10]
- **Model binding of `DateTime`**: UTC strings (`...Z`) bind as `DateTimeKind.Utc` in 5.0 (previously local).
- **No built-in rate limiting or output caching** (.NET 7) — use a reverse proxy or a library; `ResponseCaching` middleware exists.
- **Problem details**: `[ApiController]` produces `ProblemDetails` for 4xx; customize via `ProblemDetailsFactory`. `AddProblemDetails()` is .NET 7.
- **Forwarded headers** behind a proxy: `UseForwardedHeaders` with `KnownProxies`/`KnownNetworks` configured, otherwise `X-Forwarded-For` spoofing or wrong scheme in redirects.
- **Kestrel limits**: `MaxRequestBodySize` (30,000,000 bytes by default), request header limits; set explicitly for upload endpoints (`[RequestSizeLimit]`).
- **Antiforgery** for cookie-authenticated form posts (Razor Pages validate by default; MVC needs `[ValidateAntiForgeryToken]` or `AutoValidateAntiforgeryTokenAttribute`).
- **Swashbuckle** is in the 5.0 `webapi` template; expose Swagger UI only in Development or behind auth.
- **BinaryFormatter throws `NotSupportedException`** in ASP.NET Core 5 apps (verified) — legacy session/cache code using it fails at runtime.

***

#### EF Core 5

Verified with `Microsoft.EntityFrameworkCore.Sqlite` 5.0.17 on .NET 5.0.17.

```csharp
public class ShopContext : DbContext
{
    public ShopContext(DbContextOptions<ShopContext> options) : base(options) { }

    public DbSet<Blog> Blogs => Set<Blog>();
    public DbSet<Post> Posts => Set<Post>();

    protected override void OnModelCreating(ModelBuilder modelBuilder)
    {
        modelBuilder.Entity<Post>().Property(p => p.Price).HasPrecision(18, 2);   // HasPrecision is new in 5.0
    }
}

public class Blog
{
    public int Id { get; set; }
    public string Name { get; set; } = "";
    public List<Post> Posts { get; } = new();
    public List<Tag> Tags { get; } = new();          // many-to-many without a join entity (new in 5.0)
}

public class Post
{
    public int Id { get; set; }
    public string Title { get; set; } = "";
    public decimal Price { get; set; }
    public int BlogId { get; set; }
}

public class Tag
{
    public int Id { get; set; }
    public string Text { get; set; } = "";
    public List<Blog> Blogs { get; } = new();
}
```

Pitfalls a reviewer must catch:

- **SQL injection through `FromSqlRaw` + interpolation.** `FromSqlRaw($"... WHERE Name = '{input}'")` concatenates (returned all rows for `x' OR '1'='1`, verified). Use `FromSqlInterpolated($"... WHERE Name = {input}")` (parameterized as `@p0`, verified with `ToQueryString()`) or `FromSqlRaw("... = {0}", input)`. Same for `ExecuteSqlRaw` vs `ExecuteSqlInterpolated`.
- **Untranslatable predicates throw** `InvalidOperationException` ("The LINQ expression … could not be translated") — e.g. `Where(x => x.Name.Equals("A", StringComparison.OrdinalIgnoreCase))`. Client evaluation is allowed only in the final `Select` (verified). Do not "fix" by inserting `AsEnumerable()`/`ToList()` before the filter on large tables — that loads the whole table.
- **Cartesian explosion**: multiple collection `Include`s in one query log `MultipleCollectionIncludeWarning` in 5.0 (verified). Use `AsSplitQuery()` (new in 5.0) or set it globally with `UseQuerySplittingBehavior(QuerySplittingBehavior.SplitQuery)`; split queries are not consistent without a serializable/snapshot transaction.
- **Filtered include** (new in 5.0): `Include(b => b.Posts.Where(p => p.Price > 1.6m))` loads only matching children — with tracking queries, already-tracked children still appear (fix-up), so prefer `AsNoTracking()` with filtered includes.
- **Tracking**: read-only queries use `AsNoTracking()`; changes to no-tracking entities are ignored by `SaveChanges` (verified: only the tracked change was saved).
- **`DbContext` is not thread-safe.** Never share one across concurrent tasks or inject it into singletons. Concurrent-use detection is best-effort (`InvalidOperationException: A second operation was started on this context…`); a test with eight parallel queries on SQLite did not trip it, so absence of the exception proves nothing. Use `AddDbContextFactory<T>` (new in 5.0) for background or parallel work and create one context per unit of work.
- **N+1 queries**: loops that touch navigation properties with lazy loading (`UseLazyLoadingProxies`) or call the database per item. Project with `Select` or `Include` instead.
- **`decimal` without precision** on SQL Server logs a warning and truncates silently to `decimal(18,2)`; configure `HasPrecision`/`HasColumnType`. SQLite cannot `ORDER BY` or aggregate `decimal` (`NotSupportedException: SQLite cannot order by expressions of type 'decimal'`, verified).
- **Bulk updates/deletes**: `ExecuteUpdate`/`ExecuteDelete` are EF Core 7. In 5.0 use `ExecuteSqlInterpolated` for set-based changes, or load-modify-save for small sets.
- **Diagnostics** (new in 5.0): `optionsBuilder.LogTo(Console.WriteLine, LogLevel.Information)`, `query.ToQueryString()`, `ChangeTracker.DebugView`. `EnableSensitiveDataLogging()` logs parameter values — development only.
- **Migrations**: review generated migrations (column drops, type changes); SQLite migrations in 5.0 rebuild tables for unsupported `ALTER`s. Apply migrations from a deployment step, not from every app instance on startup.
- **Async all the way**: `ToListAsync`, `FirstOrDefaultAsync`, `SaveChangesAsync` with the request's `CancellationToken`.
- EF Core 5 does not run on .NET Framework (it targets `netstandard2.1`); EF Core 6+ requires .NET 6+.[^18]

***

#### Security

- **The runtime itself is the biggest finding**: .NET 5 receives no security fixes since May 10, 2022. Every CVE fixed in .NET 6+ servicing since then (TLS, HTTP/2, Kestrel, cryptography, XML, NuGet) is potentially open. Upgrade to .NET 10 LTS.
- **SQL**: parameterize everything (ADO.NET `SqlParameter`, Dapper parameters, EF `FromSqlInterpolated`); see EF Core 5.
- **Path traversal**: `Path.Combine("/srv/uploads", "/etc/passwd")` returns `/etc/passwd` (a rooted second argument wins), and `../` escapes after normalization (verified). `Path.Join` does not reset on rooted segments but does not normalize either. Validate with `GetFullPath` + an ordinal prefix check that includes the trailing separator:

```csharp
public static string ResolveUnder(string root, string userPath)
{
    var fullRoot = Path.GetFullPath(root).TrimEnd(Path.DirectorySeparatorChar) + Path.DirectorySeparatorChar;
    var candidate = Path.GetFullPath(Path.Combine(fullRoot, userPath));
    if (!candidate.StartsWith(fullRoot, StringComparison.Ordinal))   // use OrdinalIgnoreCase on Windows
        throw new UnauthorizedAccessException("Path escapes the upload directory.");
    return candidate;
}
```

- **Deserialization**: no `BinaryFormatter`, `NetDataContractSerializer`, `SoapFormatter`, `LosFormatter`, or Newtonsoft `TypeNameHandling` on untrusted data. `System.Text.Json` and `XmlSerializer` with known types are safe defaults.
- **XML**: in .NET Core/.NET 5, `XmlReader.Create` prohibits DTDs by default and `XmlDocument` has no resolver by default; flag code that sets `DtdProcessing.Parse` or assigns an `XmlUrlResolver` for untrusted input (XXE).
- **Cryptography**:
  - Passwords: `Rfc2898DeriveBytes` constructors without a hash algorithm default to **SHA-1 and 1,000 iterations** (verified). Use `new Rfc2898DeriveBytes(password, salt, iterations, HashAlgorithmName.SHA256)` with a 16+ byte random salt and a high iteration count (OWASP currently recommends 600,000 for PBKDF2-HMAC-SHA256), or ASP.NET Core Identity's `PasswordHasher<T>`. The static `Rfc2898DeriveBytes.Pbkdf2` is .NET 6.
  - Randomness for tokens/keys: `RandomNumberGenerator.Fill(span)`, `RandomNumberGenerator.GetInt32(max)`; never `System.Random`, `Guid.NewGuid()` as a secret, or `RNGCryptoServiceProvider` in new code.
  - Compare secrets/MACs with `CryptographicOperations.FixedTimeEquals`.
  - Symmetric encryption: `AesGcm` (authenticated) over `Aes` + CBC without MAC; never ECB, never static IVs.
  - Hashing: `SHA256.HashData(bytes)` (new in .NET 5); MD5/SHA-1 only for non-security checksums.
- **TLS**: do not set `ServicePointManager`/`SslProtocols` to specific old versions; leave `SslProtocols.None` (OS default). Never accept all certificates. On Linux, .NET 5 uses a restricted default cipher-suite list (OpenSSL configuration).[^10]
- **Regex DoS**: user-controlled input against backtracking patterns needs a match timeout (`new Regex(pattern, RegexOptions.None, TimeSpan.FromMilliseconds(100))`); `RegexOptions.NonBacktracking` is .NET 7. Never build a pattern from user input without `Regex.Escape`.
- **Process execution**: `ProcessStartInfo` with `UseShellExecute = false` and `ArgumentList` (each argument separately) instead of concatenating a command string.
- **ASP.NET Core**: HTTPS redirection + HSTS in production, secure/HttpOnly/SameSite cookies, antiforgery for cookie auth, CORS with explicit origins (never `AllowAnyOrigin` together with credentials — the framework throws for that combination), authorization by default via a fallback policy, Data Protection keys persisted and protected in multi-instance deployments.
- **Secrets**: never in source or `appsettings.json`; use environment variables, user secrets in development, or a vault provider.
- **Logging**: no tokens/passwords/PII; records' `ToString()` prints everything (see Records).
- **`[SkipLocalsInit]`** buffers that are returned or written out can disclose stack memory.

***

#### Performance

- **Measure first** with BenchmarkDotNet (`[MemoryDiagnoser]`), `dotnet-counters` and `dotnet-trace` (both work with .NET 5).
- **Allocation hygiene**: `Span<T>`/`stackalloc` for small temporary buffers, `ArrayPool<T>` for larger ones, `StringBuilder` for loops, `string.Create` (the `SpanAction` overload) for fixed-length strings, `Convert.ToHexString` instead of `BitConverter.ToString().Replace("-", "")`.
- **Avoid boxing**: generic collections, `IEquatable<T>` on structs, no `object` parameters in hot paths, `Enum.HasFlag` is fine on .NET Core (JIT intrinsic).
- **Async**: avoid `async` state machines for trivial pass-through methods (return the `Task` directly, but keep `async` when `using`/`try` must span the await); `ValueTask` only where synchronous completion dominates.
- **Static lambdas** (C# 9) prevent accidental closures; pass state via the overloads that take a state argument (`GetOrAdd(key, static (k, s) => ..., state)`).
- **LINQ** in hot paths: replace with loops when profiling shows it.
- **Collections**: pre-size, `TryGetValue`, `HashSet<T>` for membership, avoid `List.Remove` in loops (O(n²)), `CollectionsMarshal.AsSpan` for tight loops over lists (new in .NET 5).
- **Strings**: ordinal comparisons are much faster than culture-sensitive ones under ICU.
- **`GC.AllocateUninitializedArray<T>`** (new in .NET 5) skips zeroing for large arrays that are fully overwritten — same disclosure caveat as `SkipLocalsInit`.
- **Server GC** is the default for ASP.NET Core; in containers set memory limits so the GC heap hard limit applies.
- **JSON**: cache `JsonSerializerOptions`; stream with `JsonSerializer.SerializeAsync(stream, ...)` / `DeserializeAsync` instead of materializing strings; there is no source generator in 5.0.
- **EF Core**: `AsNoTracking`, projections, split queries for multiple collections, compiled queries (`EF.CompileAsyncQuery`) for hot queries, `DbContext` pooling (`AddDbContextPool`).
- **ReadyToRun/tiered compilation**: `PublishReadyToRun` improves startup; tiered PGO is opt-in in .NET 6 and default in .NET 8 — not available in .NET 5.

***

#### Testing

Verified on .NET 5.0.17 with xUnit 2.4.2, xunit.runner.visualstudio 2.4.5, Microsoft.NET.Test.Sdk 17.3.3, Moq 4.18.4 and Microsoft.AspNetCore.Mvc.Testing 5.0.17 (5 tests passed). Newer test packages may no longer support `net5.0` — pin versions that do.

```csharp
public class OrdersApiTests : IClassFixture<WebApplicationFactory<Startup>>   // Startup, not Program (see Top-level statements)
{
    private readonly WebApplicationFactory<Startup> _factory;

    public OrdersApiTests(WebApplicationFactory<Startup> factory) => _factory = factory;

    [Fact]
    public async Task Create_ValidRequest_Returns201()
    {
        var client = _factory.CreateClient();

        var response = await client.PostAsJsonAsync("/api/orders", new CreateOrder("abc", 2));

        Assert.Equal(HttpStatusCode.Created, response.StatusCode);
        var body = await response.Content.ReadFromJsonAsync<OrderCreated>();
        Assert.Equal(new OrderCreated(42, "abc", 2), body);             // record value equality in assertions
    }

    [Theory]
    [InlineData("{\"sku\":\"abc\",\"quantity\":0}")]
    [InlineData("{\"quantity\":1}")]
    public async Task Create_InvalidRequest_Returns400(string json)
    {
        var client = _factory.CreateClient();

        var response = await client.PostAsync("/api/orders",
            new StringContent(json, Encoding.UTF8, "application/json"));

        Assert.Equal(HttpStatusCode.BadRequest, response.StatusCode);
    }
}

public interface IClock { DateTimeOffset UtcNow { get; } }               // TimeProvider is .NET 8

public sealed record Token(string Value, DateTimeOffset Expires);

public sealed class TokenService
{
    private readonly IClock _clock;
    public TokenService(IClock clock) => _clock = clock;
    public bool IsExpired(Token token) => _clock.UtcNow >= token.Expires;
}

public class TokenServiceTests
{
    [Fact]
    public void IsExpired_UsesInjectedClock()
    {
        var now = new DateTimeOffset(2026, 10, 22, 0, 0, 0, TimeSpan.Zero);
        var clock = new Mock<IClock>();
        clock.SetupGet(c => c.UtcNow).Returns(now);
        var sut = new TokenService(clock.Object);

        Assert.True(sut.IsExpired(new Token("t", now)));
        Assert.False(sut.IsExpired(new Token("t", now.AddMinutes(1))));
    }
}
```

- Name tests `Method_Scenario_Expected`; Arrange/Act/Assert; one behaviour per test; `[Theory]` + `[InlineData]` for tables.
- **Records make assertions easy** (`Assert.Equal(expectedRecord, actual)`) — but only for scalar members; collection members compare by reference (see Records).
- Inject time (`IClock`), randomness and I/O; no `DateTime.Now` in testable code.
- Async tests return `Task` (never `async void`); use `Assert.ThrowsAsync`.
- `WebApplicationFactory<TEntryPoint>` needs a type from the app assembly — with C# 9 top-level statements compiled by the 5.0 SDK the entry class is `<Program>$` and cannot be named; use `Startup`. Override services with `WithWebHostBuilder(b => b.ConfigureTestServices(...))`.
- Test the real database provider for query behaviour; the EF Core InMemory provider does not translate SQL, enforce constraints, or reject untranslatable queries the same way. SQLite in-memory (`DataSource=:memory:` with an open connection) is closer.
- Enable `ValidateScopes`/`ValidateOnBuild` in test hosts to catch captive dependencies.

***

#### Removed / Changed in .NET 5 — Migration Notes

##### From .NET Core 3.1 to .NET 5

- TFM `netcoreapp3.1` → `net5.0`; `#if NETCOREAPP3_1` blocks no longer compile in; use `NET5_0_OR_GREATER`/`NETCOREAPP3_1_OR_GREATER`.
- Windows: ICU replaces NLS — re-test culture-sensitive string operations (`IndexOf`, `Compare`, sorting).
- New default-on analyzers (CA2200, CA2013, CA2014, CA2247, CA1416, …) may fail builds with `TreatWarningsAsErrors`.
- New nullable annotations across the BCL produce new warnings.
- `BinaryFormatter` obsolete (warning) and blocked in ASP.NET Core apps.
- `Thread.Abort`, CAS, remoting, GAC, UTF-7, `Assembly.CodeBase` obsolete (SYSLIB0001–0012); `PrincipalPermissionAttribute` is a compile **error**.
- `string.LastIndexOf("")`, `OrderBy().First(predicate)` complexity, `Activity` ID format, cookie path handling (RFC 6265), Unix UNC/non-ASCII URI parsing, `Vector<T>` `NotSupportedException` for unsupported `T`, TLS cipher suites on Linux — behaviour changes to re-test.[^10]
- System.Text.Json: non-public parameterless constructors are no longer used; `char` needs single-character strings; options honoured for `KeyValuePair`; `Serialize` with a `null` `Type` throws `ArgumentNullException`; ASP.NET Core accepts quoted numbers.[^10]
- WinForms/WPF: projects use `Microsoft.NET.Sdk` with `UseWindowsForms`/`UseWPF` and `net5.0-windows`; built-in WinRT support removed (use C#/WinRT).
- EF Core 5 breaking changes: `IProperty.GetColumnName()` obsolete (use `GetColumnBaseName()`), required navigations from principal to dependent change semantics, `ToTable(null)` means "not mapped to a table", decimals need precision, `HasName` on indexes obsolete.[^18]

##### From .NET 5 to .NET 10 (the recommended upgrade)

- Change the TFM to `net10.0` and remove `LangVersion 9` unless there is a reason to hold the language back; drop `global.json` pins to 5.0.x.
- Update all `Microsoft.Extensions.*`, ASP.NET Core and EF Core packages to 10.0.x; EF Core 6–10 contain many query and behaviour changes — read the EF Core breaking-change lists for 6.0 through 10.0.
- `BackgroundService` exceptions stop the host by default since .NET 6 (the .NET 5 catch-all remains good practice).
- `BinaryFormatter` is removed entirely (always throws) since .NET 9.
- The generic host with a `Startup` class still works on .NET 10; moving to `WebApplication` is optional. `WebApplicationBuilder` does not support `UseStartup<T>()`, so a move means relocating `ConfigureServices`/`Configure` code into `Program.cs`.
- Re-run all tests; expect new analyzer and nullable warnings.

***

#### Production / Review Checklist

##### Platform and build

- [ ] `net5.0` flagged as end-of-life (May 10, 2022); upgrade to .NET 10 LTS recommended (not .NET 8/9, which end on November 10, 2026)
- [ ] `LangVersion` explicit (`9.0`) where it is not the TFM default; never `latest`/`default`/`preview`
- [ ] `global.json` pins the SDK; Docker images and CI agree with it
- [ ] `<Nullable>enable</Nullable>`, warnings as errors, `AnalysisMode` `AllEnabledByDefault` (not the .NET 6+ value `All`)
- [ ] Down-level targets: `IsExternalInit`/attribute polyfills are `internal`; no covariant returns in `netstandard2.0` code
- [ ] No C# 10+ syntax or .NET 6+ APIs suggested or used (see the Do-Not-Use tables)

##### Language and types

- [ ] Records: no mutable collections as members of value-equal records; `with` shallow copies understood; invariants hold after `with`
- [ ] Records with secrets override `PrintMembers`; attributes on positional parameters use `property:` where properties are inspected
- [ ] `init` not mistaken for "required"; mandatory values are constructor parameters
- [ ] Patterns: `not` precedence correct; enum switches have a throwing discard arm; NaN handled
- [ ] `[SkipLocalsInit]`/`GC.AllocateUninitializedArray` buffers written before read
- [ ] Module initializers justified (prefer explicit registration)
- [ ] No `null!` without a comment; DTOs validated after deserialization

##### Async and concurrency

- [ ] No `.Result`/`.Wait()`; no `async void` outside event handlers; no async lambdas in `Parallel.ForEach`
- [ ] `CancellationToken` flowed to every async call; `[EnumeratorCancellation]` on async iterators
- [ ] `BackgroundService.ExecuteAsync` has a catch-all that logs and stops the app
- [ ] No `await` inside `lock`; `SemaphoreSlim` released in `finally`
- [ ] `ConcurrentDictionary.GetOrAdd` factories idempotent or wrapped in `Lazy<T>`

##### Services, HTTP and data

- [ ] No captive dependencies; scope validation enabled in tests
- [ ] Options validated at startup (hosted service), not lazily on first request
- [ ] `IHttpClientFactory`/typed clients; typed clients not injected into singletons; timeouts set; responses disposed
- [ ] Timeouts distinguished from cancellation (`InnerException is TimeoutException`)
- [ ] One cached `JsonSerializerOptions`; Web defaults where camelCase/case-insensitive input is expected; enum converter
- [ ] EF Core: no `FromSqlRaw`/`ExecuteSqlRaw` with interpolation; `AsNoTracking` for reads; split queries for multiple collections; `DbContext` never shared across threads; decimal precision configured
- [ ] Middleware order correct; authorization by default; HTTPS/HSTS; forwarded headers configured behind proxies

##### Security

- [ ] No `BinaryFormatter`/`TypeNameHandling`; XML parsers keep secure defaults
- [ ] PBKDF2 with SHA-256 and a high iteration count; `RandomNumberGenerator` for secrets; `FixedTimeEquals` for comparisons
- [ ] Path inputs resolved with `GetFullPath` and an ordinal prefix check
- [ ] Regex on user input has a timeout; process arguments via `ArgumentList`
- [ ] No certificate-validation bypass; no secrets in config files or logs

##### Observability and testing

- [ ] Structured logging with message templates (no interpolation); `LoggerMessage.Define` on hot paths; exceptions passed as the exception argument
- [ ] Health checks mapped; `ActivitySource`/W3C tracing where distributed tracing is used
- [ ] Integration tests via `WebApplicationFactory<Startup>`; injected clock; tests pinned to package versions that support `net5.0`

***

[^1]: The history of C# — C# version 9 (released November 2020 with .NET 5). https://learn.microsoft.com/dotnet/csharp/whats-new/csharp-version-history#c-version-9
[^2]: .NET 5.0 release metadata (release dates, 5.0.17, SDK 5.0.408/5.0.214, CVEs, EOL 2022-05-10, VS 16.8). https://builds.dotnet.microsoft.com/dotnet/release-metadata/5.0/releases.json
[^3]: .NET releases index (support phases, latest patches, EOL dates for 6.0–11.0). https://builds.dotnet.microsoft.com/dotnet/release-metadata/releases-index.json ; .NET and .NET Core support policy. https://dotnet.microsoft.com/platform/support/policy/dotnet-core
[^4]: C# language versioning — defaults per target framework. https://learn.microsoft.com/dotnet/csharp/language-reference/language-versioning
[^5]: C# 9.0 feature proposals (records, init, top-level statements, patterns3, native integers, function pointers, skip-localsinit, module initializers, extending partial methods, covariant returns, target-typed new/conditional, static lambdas, lambda discards, extension GetEnumerator, local function attributes). https://github.com/dotnet/csharplang/tree/main/proposals/csharp-9.0
[^6]: .NET globalization and ICU (behaviour changes, `UseNls`). https://learn.microsoft.com/dotnet/core/extensions/globalization-icu
[^7]: Covariant return types proposal and `RuntimeFeature.CovariantReturnsOfClasses`. https://github.com/dotnet/csharplang/blob/main/proposals/csharp-9.0/covariant-returns.md
[^8]: Records (C# reference). https://learn.microsoft.com/dotnet/csharp/language-reference/builtin-types/record
[^9]: Patterns (C# reference) — relational, logical, type and parenthesized patterns. https://learn.microsoft.com/dotnet/csharp/language-reference/operators/patterns
[^10]: Breaking changes in .NET 5 (core libraries, SDK, serialization, code analysis, globalization, networking, cryptography). https://learn.microsoft.com/dotnet/core/compatibility/5.0
[^11]: HttpClient: throw TimeoutException as inner exception on timeout (dotnet/runtime#21965, shipped in .NET 5). https://github.com/dotnet/runtime/issues/21965
[^12]: BinaryFormatter serialization methods are obsolete and prohibited in ASP.NET apps (.NET 5). https://learn.microsoft.com/dotnet/core/compatibility/serialization/5.0/binaryformatter-serialization-obsolete
[^13]: Obsolete features in .NET 5+ (SYSLIB0001–SYSLIB0041 list). https://learn.microsoft.com/dotnet/fundamentals/syslib-diagnostics/obsoletions-overview
[^14]: What's new in .NET 5. https://learn.microsoft.com/dotnet/core/whats-new/dotnet-5
[^15]: System.Text.Json overview and how-to articles (immutable types/records, fields, ignore conditions, quoted numbers, reference preservation, web defaults). https://learn.microsoft.com/dotnet/standard/serialization/system-text-json/overview
[^16]: What's new in ASP.NET Core 5.0. https://learn.microsoft.com/aspnet/core/release-notes/aspnetcore-5.0
[^17]: What's new in EF Core 5.0. https://learn.microsoft.com/ef/core/what-is-new/ef-core-5.0/whatsnew
[^18]: Breaking changes in EF Core 5.0. https://learn.microsoft.com/ef/core/what-is-new/ef-core-5.0/breaking-changes
[^19]: Code analysis in .NET (EnableNETAnalyzers, AnalysisLevel, AnalysisMode). https://learn.microsoft.com/dotnet/fundamentals/code-analysis/overview
[^20]: Async ValueTask Pooling in .NET 5 (`DOTNET_SYSTEM_THREADING_POOLASYNCVALUETASKS`). https://devblogs.microsoft.com/dotnet/async-valuetask-pooling-in-net-5/
[^21]: Trimming options (.NET 5 options, trim analyzer in .NET 6+, feature switches). https://learn.microsoft.com/dotnet/core/deploying/trimming/trimming-options
[^22]: Unhandled exceptions from a BackgroundService stop the host (.NET 6 breaking change). https://learn.microsoft.com/dotnet/core/compatibility/core-libraries/6.0/hosting-exception-handling
