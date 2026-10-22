### TypeScript 5.8: The Complete Production Developer Guide

> **Version**: TypeScript 5.8 — 5.8.2 (first stable 5.8 release; there is no "5.8.0"/"5.8.1" on npm) was published on **February 28, 2025**; the latest and final patch is **5.8.3 (April 5, 2025)**.[^1][^2]
> **Status as of October 2026**: TypeScript has no LTS lines — only the newest release receives fixes, so 5.8 is unmaintained and 5.8.3 is the last 5.8 build. Newer releases: 5.9 (5.9.2, July 31, 2025; final patch 5.9.3), 6.0 (6.0.2, March 23, 2026; latest 6.0.3, April 16, 2026) and **7.0** — the native (Go) compiler — whose 7.0.2 (July 8, 2026) is `typescript@latest` on npm; 7.1 is in beta (October 20, 2026).[^2] DefinitelyTyped still publishes `@types/*` tagged `ts5.8` (it tests every TypeScript version younger than two years), so 5.8 projects still get type updates for now.[^3]
> **Scope**: The whole language as usable in 5.8, every 5.8 change relative to 5.7, compiler options, module resolution for Node.js and bundlers, reviewer-relevant bugs and pitfalls, security, performance, testing, tooling and observability.
> **Note**: This document is self-contained. Sections marked **New in 5.8** cover behaviour that differs from 5.7. Every TypeScript example was compiled with `tsc` 5.8.3 (errors shown as `// error TSnnnn` comments are the exact diagnostics 5.8.3 reports on that line); boundary claims were re-checked on 5.7.3, 5.9.3, 6.0.3 and 7.0.2, and runtime claims on Node.js 18.20.8, 20.20.2, 22.23.4, 24.22.1 and 26.12.1.

***

#### Overview

TypeScript 5.8 is a small release focused on correctness of `return` statements and on Node.js interoperability. Its changes are:[^1]

- **Granular checks for branches in `return` expressions** — each branch of a conditional expression that is the direct operand of `return` is checked against the declared return type, so an `any` in one branch no longer hides a wrong type in the other.
- **`require()` of ECMAScript modules under `--module nodenext`** — CommonJS files may `require()` ES modules (Node.js 22.12+/20.19+ behaviour); `--module node16` and the new `node18` still reject it.
- **`--module node18`** — a new, stable module mode modelling Node.js 18 (no `require(esm)`, import assertions still allowed).
- **`--erasableSyntaxOnly`** — rejects TypeScript constructs with runtime semantics (`enum`, runtime `namespace`, parameter properties, `import =`/`export =`, and — since 5.8.3 — `<T>x` angle-bracket assertions) so code can run under Node.js type stripping.
- **`--libReplacement`** — lets projects turn off the lookup of `@typescript/lib-*` replacement packages.
- **Preserved computed property names in declaration files** — `[propName]: number` instead of a degraded index signature (and no more error TS1166 for non-literal class property keys).
- **Import assertions (`assert { … }`) are an error under `--module nodenext`** — use import attributes (`with { … }`).
- **`--module node18` implies `--target es2022` and `--moduleResolution node16`** (verified with `--showConfig`); `--module nodenext` continues to imply `--target esnext`, as in 5.7.
- **`lib.d.ts`**: DOM updates; new `esnext` declarations for `Float16Array`/`Math.f16round()`/`DataView#getFloat16()` and `Promise.try()` (not mentioned in the release notes — verified by diffing the 5.7.3 and 5.8.3 lib folders).
- **Program-load and watch-mode performance** improvements (path normalization without array allocations; no re-validation of unchanged options).

One feature advertised in the 5.8 **beta** did not ship: checking `return` statements against *conditional return types* ("checked returns for conditional and indexed access types") was pulled before the RC.[^4] A function declared to return `T extends string ? string : undefined` still needs a cast on each `return` in 5.8 — and, verified, in 5.9.3, 6.0.3 and 7.0.2 as well. Do not tell authors that 5.8 narrows generic conditional return types.

Everything from earlier 5.x releases is available and should be used where it fits: standard decorators, `const` type parameters, `--verbatimModuleSyntax`, `--moduleResolution bundler`, `export type *` (5.0); `using`/`await using` declarations and decorator metadata (5.2); `import type` with `resolution-mode` and `--module preserve`, `NoInfer<T>`, `Object.groupBy`/`Map.groupBy` types and closure narrowing after the last assignment (5.4); inferred type predicates, constant-index narrowing, regular-expression syntax checking, `--isolatedDeclarations`, ES Set-method types and `${configDir}` (5.5); disallowed always-truthy/nullish checks, iterator-helper types, `--strictBuiltinIteratorReturn`, `--noUncheckedSideEffectImports`, `--noCheck` (5.6); never-initialized-variable checks, `--rewriteRelativeImportExtensions`, `--target es2024`/`--lib es2024`, generic `TypedArray`s and validated JSON imports under `nodenext` (5.7).[^5][^6]

**Do not use or recommend anything from TypeScript 5.9 or later** — the project's compiler does not have it: no `import defer` (5.9), no `--module node20` (5.9), no `--target es2025`/`--lib es2025` (6.0), no `Temporal` types, `Map#getOrInsert()`/`getOrInsertComputed()`, `RegExp.escape()`, `Error.isError()` or `Atomics.pause()` declarations, no `#/` subpath-import support, no `--stableTypeOrdering`, no `--moduleResolution bundler` with `--module commonjs`, and none of the 6.0 default changes (`strict`, `types: []`, `rootDir: "."`, `module: esnext`). The full list with versions is in [Do Not Use or Recommend](#do-not-use-or-recommend-59-and-later).

Global truths about TypeScript 5.8:

- **Types are erased.** Type annotations, interfaces, type aliases and `as` assertions have no runtime effect. Nothing checks a value's type at runtime unless code does it explicitly. Data from `JSON.parse`, `fetch().json()`, environment variables, databases and message queues is *unvalidated* no matter what type it is annotated with.
- **Structural typing.** Compatibility is decided by shape, not by declared names. Two unrelated interfaces with the same members are interchangeable; brands are needed for nominal distinctions.
- **The type system is intentionally unsound in places**: method-parameter bivariance, array covariance, index access without `noUncheckedIndexedAccess`, `any`, type assertions, non-null assertions and definite-assignment assertions. A reviewer looks hardest at these escape hatches.
- **A few constructs do emit code**: `enum`, `namespace` with values, parameter properties, decorators, `import x = require()`, `export =`, and downlevel transforms selected by `--target` (`using`, class fields, `async`, …). `--erasableSyntaxOnly` turns all of these except decorators and down-levelling into errors.
- **The compiler does not polyfill.** `--lib` and `--target` describe what the runtime provides; declaring `"lib": ["esnext"]` does not make `Promise.try` exist on Node.js 22 (it does not — it ships in Node.js 24).
- **The emitted module format is decided by `--module` plus, for `node16`/`node18`/`nodenext`, the file extension and the nearest `package.json` `"type"`** — not by the import syntax you write.

***

#### What's New in TypeScript 5.8

##### New in 5.8: Granular checks for branches in `return` expressions

Before 5.8, the type of `cond ? a : b` was the union of both branch types, and that union was then compared with the declared return type. Because `any | string` collapses to `any`, an `any`-typed branch (an untyped cache, `JSON.parse`, a `Map<any, any>`) silently disabled the check of the other branch. TypeScript 5.8 checks **each branch** of a conditional expression that is the direct operand of `return` against the declared return type.[^1][^7]

```ts
declare const untypedCache: Map<any, any>;

export function getUrlObject(urlString: string): URL {
  return untypedCache.has(urlString)
    ? untypedCache.get(urlString)
    : urlString; // error TS2322 — 5.8 only; 5.7.3 accepts this function
}

export function getUrlObjectFixed(urlString: string): URL {
  return untypedCache.has(urlString)
    ? (untypedCache.get(urlString) as URL)
    : new URL(urlString);
}
```

Where the new check applies (verified on 5.8.3, none of these error on 5.7.3):

- functions, methods and arrow functions **with an explicit return type annotation**, including concise arrow bodies (`(s: string): URL => cond ? x : s`);
- parenthesized and nested conditionals (`return a ? x : b ? y : z`);
- `async` functions (each branch checked against `T` of `Promise<T>`) and generators (`return` checked against `TReturn`).

Where it does **not** apply — review these by hand:

- the conditional is first stored in a variable (`const r = c ? anyVal : s; return r;` — `r` is `any`);
- `||`, `??` and `&&` expressions (`return (x as any) || s` is still unchecked);
- arrow functions that get their return type only *contextually* (`const f: (s: string) => URL = s => c ? anyVal : s` — no error);
- functions without a declared return type (nothing to check against).

```ts
declare const cache: Map<string, any>;

export function viaVariable(key: string): URL {
  const r = cache.has(key) ? cache.get(key) : key; // r: any
  return r; // no error — the bug is still there
}

export const contextual: (key: string) => URL = (key) =>
  cache.has(key) ? cache.get(key) : key; // no error — only contextually typed

export function viaNullish(key: string): URL {
  return cache.get(key) ?? key; // no error — `??` is not a conditional expression
}
```

Upgrading to 5.8 can therefore surface *real* bugs in existing code. Fix them; do not silence them with `as any`.

##### New in 5.8: `require()` of ES modules under `--module nodenext`

Node.js can synchronously `require()` an ES module that contains no top-level `await` (unflagged in 22.12.0 and 20.19.0, warning-free since 22.13.0/20.19.0, non-experimental since 25.4.0).[^8] TypeScript 5.8 models this under `--module nodenext` only: a CommonJS file (`.cts`, or `.ts` in a package without `"type": "module"`) may `require()`/`import x = require()` an ES module.[^1][^9]

| `--module`  | `require()` of an ESM file in 5.8.3 | TS 5.7.3                      |
| ----------- | ----------------------------------- | ----------------------------- |
| `node16`    | error TS1471                        | error TS1471                  |
| `node18`    | error TS1471 (new mode)             | `node18` does not exist (TS6046) |
| `nodenext`  | **allowed**                         | error TS1471                  |

```ts
// esm.mts
export const answer = 42;
export default function hello(): string {
  return "hi";
}
```

```ts
// main.cts — compiles under --module nodenext with 5.8; TS1471 under node16/node18 and on 5.7
import esm = require("./esm.mjs");
console.log(esm.answer, esm.default()); // 42 hi
```

Runtime results of the emitted `main.cjs`: works on Node.js 20.20.2, 22.23.4, 24.22.1 and 26.12.1; fails with `ERR_REQUIRE_ESM` on Node.js 18.20.8.

**Pitfall — top-level `await` is not detected.** If the required module (or anything it imports) uses top-level `await`, tsc 5.8.3 compiles the `require()` without complaint, and Node.js throws `ERR_REQUIRE_ASYNC_MODULE` at runtime (verified on Node.js 20, 22 and 24). Use `await import()` for such modules.

```ts
// tla.mts
export const v = await Promise.resolve(1);
```

```ts
// tla-consumer.cts — compiles cleanly under nodenext, but throws ERR_REQUIRE_ASYNC_MODULE at runtime
import t = require("./tla.mjs");
console.log(t.v);
```

Review rules:

- `nodenext` means "the newest Node.js behaviour this TypeScript knows". A library whose CommonJS entry `require()`s ESM breaks consumers on Node.js < 20.19 / < 22.12. Libraries that support older Node.js should compile with `--module node16` or `node18`.
- Node.js returns the module namespace object from `require(esm)`; the default export is `.default`, unless the ES module exports a value under the string name `"module.exports"`.[^8]

##### New in 5.8: `--module node18`

`--module node18` is a stable mode that freezes Node.js 18 semantics: ESM/CJS decided per file like `node16`, **import attributes allowed** (`node16` rejects them with TS2823), **import assertions allowed**, `require(esm)` **rejected**. Verified with `--showConfig` on 5.8.3: `node18` implies `--target es2022` and `--moduleResolution node16`; `nodenext` implies `--target esnext` and `--moduleResolution nodenext`. On 5.7.3 `--module node18` is error TS6046. There is **no `node20` mode in 5.8** (added in 5.9).[^1][^10]

| Behaviour (TS 5.8.3)                     | `node16` | `node18` | `nodenext` |
| ---------------------------------------- | -------- | -------- | ---------- |
| Implied `--target`                       | es2022   | es2022   | esnext     |
| `import … with { type: "json" }`         | TS2823   | ok       | ok         |
| `import … assert { type: "json" }`       | TS2821   | ok       | **TS2880** |
| JSON import without `type: "json"` in ESM | ok      | TS1543   | TS1543     |
| `require()` of ESM from CJS              | TS1471   | TS1471   | ok         |

Node.js 22 and later reject `assert` syntax at runtime (`SyntaxError: Unexpected identifier 'assert'`, verified on 22.23.4 and 24.22.1); Node.js 18.20.8 and 20.20.2 accept both `assert` and `with`. That is why `node18` keeps assertions and `nodenext` rejects them.

##### New in 5.8: Import assertions are an error under `--module nodenext`

```ts
// compiled with --module nodenext --resolveJsonModule, in a "type": "module" package
import data from "./data.json" assert { type: "json" }; // error TS2880
import data2 from "./data.json" with { type: "json" }; // ok
```

TS2880 reads "Import assertions have been replaced by import attributes. Use 'with' instead of 'assert'." Under `--module esnext` and `--module preserve`, 5.8 still accepts `assert` silently — flag it in review anyway when the output runs on Node.js 22+ or a bundler/runtime that dropped assertions. (TypeScript 6.0 deprecates `assert` on imports everywhere.)[^11]

##### New in 5.8: `--erasableSyntaxOnly`

Node.js type stripping (default-on since Node.js 23.6.0 and 22.18.0; stable since 25.2.0 and 24.12.0) only *removes* type syntax. TypeScript syntax that generates JavaScript throws `ERR_UNSUPPORTED_TYPESCRIPT_SYNTAX` at load time.[^12] `--erasableSyntaxOnly` reports those constructs at compile time with **TS1294** ("This syntax is not allowed when 'erasableSyntaxOnly' is enabled."). The flag is unknown to 5.7.3 (TS5023).[^1][^13]

```ts
// tsc --erasableSyntaxOnly --module commonjs (5.8.3)
import fs = require("fs"); // error TS1294
namespace container { // error TS1294
  export const x = 1;
}
namespace typesOnly { // ok — no runtime code
  export type T = string;
}
declare namespace Ambient { // ok — ambient
  const y: number;
}
import Bar = container.x; // error TS1294
class Point {
  constructor(public x: number, private readonly y: number) {} // error TS1294, TS1294
}
enum Direction { Up, Down } // error TS1294
const enum Flags { A } // error TS1294
declare enum Declared { A } // ok — ambient
const v = <string>(fs as unknown); // error TS1294
const w = (fs as unknown) as string; // ok — `as` is erasable
abstract class Base {
  abstract m(): void; // ok — abstract members, `declare` fields,
  declare z: number; // `private`/`readonly` modifiers on fields
  private p = 1; // and overload signatures are all erasable
}
export = Point; // error TS1294
```

Notes from testing:

- The ban on `<T>expr` angle-bracket assertions was added in **5.8.3** (PR #61244, picked into release-5.8); 5.8.2 accepts them. Pin `typescript` to `~5.8.3`, not `5.8.2`, when relying on this flag.[^14]
- `const enum` is rejected as well — it is not erasable for tools that compile one file at a time.
- The flag does not check decorators. Node.js type stripping does not transform decorators either; under Node.js 22–26 a file with a decorator fails to parse.[^12]
- Combine it with `--verbatimModuleSyntax` (type-only imports must say `type`, otherwise the stripped file still imports a name that does not exist at runtime) and, if the source imports `./x.ts`, with `--allowImportingTsExtensions` or `--rewriteRelativeImportExtensions`. The Node.js documentation recommends exactly these settings with "TypeScript 5.8 or newer" — see [Running TypeScript directly on Node.js](#running-typescript-directly-on-nodejs).[^12]

##### New in 5.8: `--libReplacement`

Since 4.5, TypeScript looks for `@typescript/lib-*` packages (e.g. `"@typescript/lib-dom": "npm:@types/web@…"`) and uses them instead of its bundled `lib.*.d.ts`. That lookup and the associated `node_modules` watching cost time even when nobody uses it. `--libReplacement false` disables it.[^1] In 5.8 the default is still `true`; TypeScript 6.0 changes the default to `false`.[^11] Recommend `"libReplacement": false` for projects that do not use lib replacement packages, and an explicit `"libReplacement": true` for projects that do. The option is unknown to 5.7.3 (TS5023).

##### New in 5.8: Preserved computed property names in declaration files

Class property names written as a computed *entity name* (`[propName]`, `[ns.key]`) are now preserved in `.d.ts` output. In 5.7, a non-literal key (`let propName: string`) was error **TS1166** ("A computed property name in a class property declaration must have a simple literal type or a 'unique symbol' type.") and the declaration file degraded the class to an index signature.[^1]

```ts
export let propName = "theAnswer";
export class MyClass {
  [propName] = 42; // 5.7.3: error TS1166; 5.8.3: ok
}
```

Declaration output (verified):

```text
// 5.7.3                                  // 5.8.3
export declare class MyClass {            export declare class MyClass {
    [x: string]: number;                      [propName]: number;
}                                         }
```

The class still effectively has a `string` index signature; the property is not statically named. For a real named member, use a `const` with a literal type or a `unique symbol`. A `.d.ts` produced by 5.8 may be read differently by 5.7 and older consumers, so library authors who support older TypeScript consumers should test their declarations with those versions (or avoid non-literal computed keys in public classes). `--isolatedDeclarations` still rejects these computed names.

##### New in 5.8: `lib.d.ts` changes

- DOM declarations were regenerated; occasional new errors in DOM-heavy code are expected on upgrade.[^1]
- `lib.esnext.float16.d.ts` (`Float16Array`, `Math.f16round()`, `DataView#getFloat16()`/`setFloat16()`) and `lib.esnext.promise.d.ts` (`Promise.try()`) are new in 5.8 (5.7.3 reports TS2693 — or TS2552 without the DOM lib — for `Float16Array`, TS2551 for `f16round`, TS2339 for `Promise.try`). They are only available with `--lib esnext` (or `esnext.float16`/`esnext.promise`). Runtime support is a separate question: Node.js 22.23.4 has neither (`typeof Float16Array === "undefined"`, `typeof Promise.try === "undefined"`); Node.js 24.22.1 has both.

```ts
// --target esnext --lib esnext,dom (5.8.3: ok; 5.7.3: TS2693, TS2551, TS2339)
const halfs = new Float16Array(2);
console.log(Math.f16round(1.337), halfs.length);
const p: Promise<number> = Promise.try(() => 1);
```

- Known incompatibility found while testing: with `@types/node` **26.x** and the DOM lib, TypeScript 5.8.3 (and 5.7.3) report error TS2430 inside `lib.dom.d.ts` ("Interface 'TextDecoder' incorrectly extends interface 'import("node:util").TextDecoder'") unless `skipLibCheck` is on. `@types/node` 22.x and 24.x work with 5.8.3, and 5.9.3+ accept 26.x. Keep `@types/node` on the major of the Node.js version you actually run, and drop `"dom"` from `lib` in Node-only projects.

##### New in 5.8: Performance

Path normalization no longer allocates intermediate arrays, and edits that do not change the project structure skip re-validation of compiler options — mainly visible in `--watch` and editors on large projects.[^1] There are no new performance-related flags beyond `--libReplacement`.

##### Patch release 5.8.3

5.8.3 contains five fixes over 5.8.2: the angle-bracket-assertion ban and an error-span fix for `--erasableSyntaxOnly`; declaration emit of accessors whose getter and setter types differ (unbound type parameters leaked into `.d.ts`); renaming shadowed type parameters in declaration emit (5.8.2 generated incorrect `.d.ts` for generic static methods); and a fix for comments from one file leaking into another file's declaration output.[^14] Projects that publish declaration files should not stay on 5.8.2.

***

#### Do Not Use or Recommend (5.9 and later)

The project compiles with TypeScript 5.8. None of the following exist in 5.8.3; each was verified to fail on 5.8.3 and to work on the version named.[^15][^11]

| Feature | Introduced in | What 5.8.3 does |
| --- | --- | --- |
| `import defer * as ns from "…"` (deferred module evaluation) | 5.9 | parse errors TS1005/TS1434 |
| `--module node20` / `--moduleResolution node20` | 5.9 | TS6046 |
| Minimal `tsc --init` template (`module: nodenext`, `types: []`, …) | 5.9 | 5.8.3 writes the long commented template with `target: es2016`, `module: commonjs`, `esModuleInterop`, `forceConsistentCasingInFileNames`, `strict`, `skipLibCheck` — outdated defaults for new code |
| `Error.isError()` (`lib.esnext.error`) | 5.9 | TS2339 |
| `Atomics.pause()` (`lib.esnext.sharedmemory`) | 5.9 | TS2339 |
| `--target es2025` / `--lib es2025` | 6.0 | TS6046 |
| `Temporal` types (`--lib esnext`) | 6.0 | TS2304 |
| `Map#getOrInsert()` / `getOrInsertComputed()` (and `WeakMap`) | 6.0 | TS2339 |
| `RegExp.escape()` | 6.0 | TS2339 |
| `#/…` subpath imports (`"imports": { "#/*": … }`) | 6.0 | TS2307 — use a named prefix such as `#lib/*` |
| `--moduleResolution bundler` with `--module commonjs` | 6.0 | TS5095 |
| `--stableTypeOrdering` | 6.0 | TS5023 |
| `"types": ["*"]` wildcard; `types` defaulting to `[]` | 6.0 | `"types": ["*"]` is TS2688; with `types` unset, 5.8 includes every `node_modules/@types` package |
| `strict: true`, `module: esnext`, `target: es2025`, `rootDir: "."`, `noUncheckedSideEffectImports: true`, `libReplacement: false` as **defaults** | 6.0 | 5.8 defaults: `strict` off, `target` ES5, `module` CommonJS (for ES5/ES3 targets), `rootDir` inferred, the other two off/on respectively |
| `dom` lib including `dom.iterable`/`dom.asynciterable` | 6.0 | `for…of` over a `NodeList` with only `"dom"` is TS2488 — list `"dom.iterable"` explicitly |
| `ignoreDeprecations: "6.0"` | 6.0 | TS5103 (only `"5.0"` is valid in 5.8) |
| Native compiler (`tsgo`, Go port), parallel checking | 7.0 | n/a — 7.0 removes every option 6.0 deprecated |

Also not in 5.8 (and not in 5.9–7.0 either): narrowing of generic conditional return types (pulled from the 5.8 beta) and runtime type checking of any kind.

**Forward-compatibility notes** — things that compile on 5.8 but will break on upgrade, worth flagging when a project plans to move on:

- 5.9 makes `ArrayBuffer` no longer a supertype of `TypedArray`s whose buffer type is `ArrayBufferLike`: `const bs: BufferSource = someUint8Array` compiles on 5.8.3 and is TS2322 on 5.9.3. Prefer `Uint8Array<ArrayBuffer>` in signatures that feed Web APIs.
- 5.9 changes type-argument inference ("leaking" type variables); add explicit type arguments where inference is fragile.
- 6.0 deprecates (and 7.0 removes, TS5102/TS5108): `--target es5`, `--downlevelIteration`, `--moduleResolution node`/`node10` and `classic`, `--module amd`/`umd`/`system`, `--baseUrl`, `--outFile`, `--esModuleInterop false`, `--allowSyntheticDefaultImports false`, `--alwaysStrict false`, `module Foo {}` namespace syntax, `assert` on imports, `/// <reference no-default-lib="true"/>`, and specifying files on the command line when a `tsconfig.json` exists. New 5.8 code should avoid all of them; existing uses are not wrong *for 5.8*, but they are migration debt.

##### Deprecated or removed before 5.8 (still relevant in review)

- **Removed in 5.5** (deprecated in 5.0; error TS5102/TS5108 on 5.8.3 even with `"ignoreDeprecations": "5.0"`): `--target ES3`, `--out`, `--charset`, `--keyofStringsOnly`, `--noImplicitUseStrict`, `--noStrictGenericChecks`, `--suppressExcessPropertyErrors`, `--suppressImplicitAnyIndexErrors`, `--preserveValueImports`, `--importsNotUsedAsValues` (any value other than the old default `remove`), and `prepend` in project references (TS5023 as a flag).[^16] Replace `importsNotUsedAsValues`/`preserveValueImports` with `--verbatimModuleSyntax`.
- **Experimental decorators** (`--experimentalDecorators`) remain supported but are the legacy proposal. New code uses standard decorators (5.0+); frameworks that need parameter decorators or `emitDecoratorMetadata` (Angular before its standard-decorator migration, NestJS, TypeORM, InversifyJS) still require the legacy flag. Never mix the two models in one project.
- **`--moduleResolution node` (`node10`)** works in 5.8 but ignores `package.json` `"exports"`/`"imports"`; it type-checks imports that fail at runtime and misses ones that succeed. Use `node16`/`node18`/`nodenext` for Node.js and `bundler` for bundled code.
- **`--target ES5`** is still the 5.8 *default* — always set `target` explicitly.

***

#### Version-Aware Review

- **Verify against 5.8.3 before claiming something is unavailable.** Many APIs are gated by `--lib`, not by the TypeScript version: `Object.groupBy` needs `es2024` (5.4+ types), `Array#toSorted` needs `es2023`, `Promise.try`/`Float16Array` need `esnext` (5.8+), `Set#union` needs `esnext` in 5.8 (`esnext.collection`), iterator helpers need `esnext` (`esnext.iterator`, 5.6+). A "does not exist" error is usually a `lib` setting, not a missing feature.
- **Say "not available in 5.8" only when 5.8 proves it** — e.g. `import defer`, `node20`, `RegExp.escape`, `Error.isError` (table above).
- **Separate type availability from runtime availability.** TypeScript does not polyfill. With `"lib": ["esnext"]` code type-checks against APIs the deployment runtime may lack. Verified on Node.js 22.23.4: `Promise.try`, `Float16Array`, `DisposableStack`, `SuppressedError` and `Error.isError` are all `undefined`; iterator helpers, `Set#union`, `Object.groupBy`, `Promise.withResolvers`, `Array.fromAsync` and `Symbol.dispose` exist. On Node.js 24.22.1, all of these plus native `using` exist. Match `lib` to the oldest supported runtime (see [tsconfig recommendations](#tsconfig-recommendations)).
- **Check the `--module` mode before judging imports.** `require()` of ESM is legal in `nodenext` but not in `node16`/`node18`; JSON imports need `with { type: "json" }` in `node18`/`nodenext` ESM; `import x = require()` is illegal in ESM files; extensionless relative imports are errors in `node16`+ ESM but fine under `bundler`.
- **Check the exact patch.** `--erasableSyntaxOnly` behaves differently on 5.8.2 and 5.8.3 (angle-bracket assertions); 5.8.2 has declaration-emit regressions. Recommend `"typescript": "~5.8.3"` (patch updates only) for projects that intend to stay on 5.8.
- **Check the toolchain around tsc.** Babel, esbuild, swc, Vite, tsx and Node.js type stripping compile one file at a time and do not type-check; their support for 5.8 syntax (none of which is new syntax) is not the issue — their lack of type checking is. CI must run `tsc --noEmit` (or `tsc -b`) separately.

***

#### tsconfig Recommendations

TypeScript 5.8's defaults are legacy (`target` ES5, `strict` off, every `@types` package loaded, `rootDir` inferred). Every project needs an explicit configuration; option semantics are in the TSConfig reference.[^22] The three configurations below were compiled with 5.8.3 and their output executed.

##### Node.js service or library compiled by `tsc` (ESM)

```json
{
  "compilerOptions": {
    "target": "es2023",
    "lib": ["es2024"],
    "module": "nodenext",
    "types": ["node"],
    "rootDir": "src",
    "outDir": "dist",

    "strict": true,
    "noUncheckedIndexedAccess": true,
    "exactOptionalPropertyTypes": true,
    "noImplicitOverride": true,
    "noImplicitReturns": true,
    "noFallthroughCasesInSwitch": true,

    "verbatimModuleSyntax": true,
    "isolatedModules": true,
    "erasableSyntaxOnly": true,
    "noUncheckedSideEffectImports": true,
    "forceConsistentCasingInFileNames": true,
    "libReplacement": false,
    "resolveJsonModule": true,

    "declaration": true,
    "declarationMap": true,
    "sourceMap": true,
    "skipLibCheck": true
  },
  "include": ["src"]
}
```

With `"type": "module"` in `package.json`, this compiles a project that uses top-level `await`, `node:` imports, `import type`, and `import config from "./config.json" with { type: "json" }`, and the output runs on Node.js 22. Notes:

- **`module`**: `nodenext` for applications on Node.js ≥ 22.12 (or ≥ 20.19) — it allows `require(esm)`. Use `node18` (5.8+) or `node16` for **libraries** that must load on older Node.js, so the compiler rejects `require(esm)` that would fail there. `moduleResolution` is implied — do not set it to anything else.
- **`target`/`lib`**: describe the *oldest runtime you deploy to*. Node.js 22 implements everything in `es2024` lib (verified: `Object.groupBy`, `Promise.withResolvers`, `Array.fromAsync`) and also iterator helpers and Set methods (`esnext.iterator`, `esnext.collection`) — add those two entries if you use them. Do not add `"esnext"` wholesale on Node.js 22: it also types `Promise.try`, `Float16Array` and `DisposableStack`, which Node.js 22 lacks. `nodenext` alone implies `target: esnext`, so always set `target`.
- **`types: ["node"]`**: without it 5.8 loads *every* package under `node_modules/@types`, which slows builds and leaks globals (e.g. `describe` from test frameworks into production code). Add test-only types in a separate `tsconfig.test.json`.
- **`verbatimModuleSyntax`** makes the emitted imports exactly what was written: type-only imports must be marked `type` (TS1484), and ESM syntax in a file that Node.js will treat as CommonJS is an error (TS1286 — verified by removing `"type": "module"`; the same project then also reports TS2856 for the JSON import attribute and TS1309 for top-level `await`).
- **`noUncheckedSideEffectImports`** (5.6+) makes `import "./polyfil"` (typo) an error (TS2307). Without it, unresolvable side-effect imports pass silently. With it, asset imports need a declaration even when the file exists: `declare module "*.css" {}`.
- **`exactOptionalPropertyTypes`** distinguishes "property absent" from "property present with value `undefined`". It is valuable for APIs that use `in`/`Object.hasOwn` or spread defaults, but it makes some third-party types awkward; enable it on new code.
- **`skipLibCheck: true`** is the pragmatic default (it also hides the `@types/node` 26 × 5.8 DOM conflict described above), but it hides genuine errors in your *own* `.d.ts` files — publish-time checks of generated declarations should run with `skipLibCheck: false` on a small consumer fixture.

##### Bundled application (Vite, esbuild, webpack, Rspack, Bun)

```json
{
  "compilerOptions": {
    "target": "es2022",
    "lib": ["es2023", "dom", "dom.iterable"],
    "module": "preserve",
    "moduleDetection": "force",
    "jsx": "react-jsx",
    "noEmit": true,
    "allowImportingTsExtensions": true,
    "types": [],

    "strict": true,
    "noUncheckedIndexedAccess": true,
    "noImplicitOverride": true,
    "noFallthroughCasesInSwitch": true,

    "verbatimModuleSyntax": true,
    "isolatedModules": true,
    "noUncheckedSideEffectImports": true,
    "libReplacement": false,
    "skipLibCheck": true
  },
  "include": ["src"]
}
```

`--module preserve` (5.4+) implies `moduleResolution: bundler`, `esModuleInterop`, `resolveJsonModule` and `resolvePackageJsonExports`/`Imports` (verified with `--showConfig`). Extensionless and `.ts` relative imports both resolve. The bundler does the emitting; `tsc --noEmit` only type-checks. List environment types explicitly (`"types": ["vite/client"]`). `allowImportingTsExtensions` requires `noEmit` or `emitDeclarationOnly`.

##### Running TypeScript directly on Node.js

Node.js 22.18+/23.6+ execute `.ts` files by stripping types; `tsconfig.json` is ignored at runtime, so `tsc` only type-checks.[^12] The Node.js documentation recommends (for "TypeScript 5.8 or newer"):

```json
{
  "compilerOptions": {
    "noEmit": true,
    "target": "esnext",
    "module": "nodenext",
    "rewriteRelativeImportExtensions": true,
    "erasableSyntaxOnly": true,
    "verbatimModuleSyntax": true
  }
}
```

Verified on 5.8.3 + Node.js 22.23.4/24.22.1: `import { mk, type User } from "./user.ts"` type-checks and runs. `rewriteRelativeImportExtensions` alone (no `allowImportingTsExtensions`) is enough for `.ts` specifiers — remove it and tsc reports TS5097. If you later drop `noEmit`, tsc rewrites `./user.ts` to `./user.js` in the output. Rules for review:

- Every relative import needs the real file extension (`./user.ts`), and every type-only import needs `type` — otherwise Node.js fails at runtime: `SyntaxError: The requested module './user.ts' does not provide an export named 'User'` (verified; `verbatimModuleSyntax` catches it as TS1484).
- No `enum`, runtime `namespace`, parameter properties, `import =`, `<T>x` assertions or decorators (`--erasableSyntaxOnly` catches all but decorators).
- `paths` aliases and down-levelling are not applied. `.tsx` is not supported. Files under `node_modules` are never stripped — packages must ship JavaScript.
- Add `"lib"` and `"types": ["node"]` to the recommended config above; `target: esnext` + no `lib` pulls in the DOM lib.

##### Strictness flags worth knowing

| Flag | In `strict`? | Since | What it catches |
| --- | --- | --- | --- |
| `noImplicitAny`, `strictNullChecks`, `strictFunctionTypes`, `strictBindCallApply`, `strictPropertyInitialization`, `noImplicitThis`, `alwaysStrict`, `useUnknownInCatchVariables` | yes | ≤ 4.4 | the basics; `catch (e)` is `unknown` |
| `strictBuiltinIteratorReturn` | yes | 5.6 | `IteratorResult` `value` of built-in iterators is `undefined` when `done` |
| `noUncheckedIndexedAccess` | no | 4.1 | `arr[i]` / `record[key]` include `undefined` |
| `exactOptionalPropertyTypes` | no | 4.4 | `{ a?: string }` rejects explicit `a: undefined` |
| `noImplicitOverride` | no | 4.3 | requires `override` on overriding members |
| `noPropertyAccessFromIndexSignature` | no | 4.2 | forces `obj["key"]` for index-signature keys |
| `noImplicitReturns`, `noFallthroughCasesInSwitch` | no | old | missing returns, switch fall-through |
| `noUnusedLocals`, `noUnusedParameters` | no | old | prefer the linter equivalents (less noisy in WIP code) |
| `allowUnreachableCode: false` | no | old | turns unreachable-code suggestions into errors |
| `noUncheckedSideEffectImports` | no | 5.6 | unresolvable `import "x"` |
| `erasableSyntaxOnly` | no | **5.8** | TS-only runtime syntax |

Never accept a PR that turns `strict` (or one of its members) off to make an error go away.

***

#### Modules and Module Resolution

##### Choosing `--module`

| Runtime / tool | `module` | Notes |
| --- | --- | --- |
| Node.js app, Node.js ≥ 20.19/22.12 | `nodenext` | per-file ESM/CJS from extension + `package.json` `"type"`; `require(esm)` allowed (5.8) |
| Node.js library supporting Node.js 18/20.<19 | `node16` or `node18` | `node18` adds import attributes; both reject `require(esm)` |
| Bundler / Bun / Vite | `preserve` (or `esnext` + `moduleResolution: bundler`) | no extension requirement |
| Legacy CommonJS-only project | `commonjs` + `moduleResolution: node10` | works in 5.8 but `node10` ignores `exports`; plan migration (deprecated in 6.0) |

`moduleResolution: bundler` requires `module` `es2015`+ or `preserve` in 5.8 (TS5095 with `commonjs`; allowed from 6.0).

##### How Node.js modes decide ESM vs CJS

- `.mts`/`.mjs` → always ESM; `.cts`/`.cjs` → always CommonJS.
- `.ts`/`.tsx`/`.js` → ESM if the nearest `package.json` has `"type": "module"`, otherwise CommonJS.
- A `.ts` file that writes `import`/`export` is still emitted as CommonJS `require` calls when it is CJS-format — unless `verbatimModuleSyntax` is on, which makes that an error (TS1286).
- In ESM files, relative imports need the **output** extension: `import "./util.js"` for `util.ts` (or `./util.ts` with `rewriteRelativeImportExtensions`/`allowImportingTsExtensions`). Directory imports (`./dir`) and extensionless imports fail at runtime and are TS2834/TS2835 errors.

```ts
// main.mts, --module nodenext
import { helper } from "./helper"; // error TS2835
import { helper as ok } from "./helper.js"; // ok — resolves helper.ts
console.log(helper, ok);
```

##### `package.json` `exports`, `imports`, and types

- Under `node16`/`node18`/`nodenext`/`bundler`, a dependency's `"exports"` map is authoritative: deep imports not listed there (`pkg/dist/internal.js`) fail to resolve (TS2307) — and fail at runtime in Node.js. Do not "fix" that with `paths`.
- Conditions are matched in object order; `"types"` must come **first** inside each condition block, and ESM/CJS entry points need separate declaration files (`index.d.mts`/`index.d.cts`, or `.d.ts` files under directories whose `package.json` sets `"type"`). A single `.d.ts` for both formats misrepresents one of them ("masquerading as ESM/CJS"); check with `@arethetypeswrong/cli` (`attw --pack`).[^17]
- `"imports"` subpath imports (`"#lib/*": "./src/lib/*"`) are the runtime-correct replacement for `paths` aliases in Node.js. In 5.8 the key must have a name after `#` — `#/*` resolves only from 6.0 (verified TS2307 on 5.8.3).
- `customConditions` (5.0+) adds conditions such as `"development"` or `"source"` for monorepos; only valid with `node16`+/`bundler` resolution.

##### `paths` and `baseUrl`

`paths` only changes how **tsc** resolves; it never rewrites emitted specifiers. Code that compiles with `"paths": { "@/*": ["src/*"] }` fails at runtime under plain `tsc` + Node.js unless a bundler, a loader, or `package.json` `"imports"` provides the same mapping. `paths` no longer needs `baseUrl` (since 4.1); prefer relative `paths` targets (`"./src/*"`) and avoid `baseUrl`, which also makes every top-level folder importable as a bare specifier (and is deprecated in 6.0).

##### Interop: `esModuleInterop`, default imports, `export =`

- `esModuleInterop` (implied by `node16`+/`preserve`) emits helpers so `import express from "express"` works for CommonJS modules. With it explicitly **off** (5.8.3 still accepts `esModuleInterop: false`, even under `nodenext`; 6.0 deprecates it), `import * as express from "express"; express()` type-checks but is a spec violation that breaks under real ESM.
- In ESM files under `nodenext`, importing CommonJS gives you `module.exports` as the default import; named imports work only for names Node.js's static analysis (cjs-module-lexer) detects. TypeScript approximates this; when in doubt, use the default import.
- `export =` / `import x = require()` are CommonJS-only constructs, rejected in ESM files and by `--erasableSyntaxOnly`. In a `.cts` file under `verbatimModuleSyntax`, `import x = require("x")` is the way to write a `require`.

##### `verbatimModuleSyntax`, `isolatedModules`, type-only imports

```ts
// --verbatimModuleSyntax
import { type User, createUser } from "./users"; // ok: User is erased, createUser kept
import type { Session } from "./session"; // ok: whole statement erased
import { Role } from "./roles"; // error TS1484 — Role is a type
export { Role }; // error TS1205 — re-exporting a type needs `export type`

const u: User = createUser();
let s: Session | undefined;
console.log(u, s);
```

- `import type`/`export type` and inline `type` modifiers are erased; everything else is kept as written. This is what single-file transpilers (esbuild, swc, Babel, Node.js) assume.
- `isolatedModules` forbids constructs a single-file transpiler cannot compile, e.g. re-exporting a type without `export type` and referencing ambient `const enum`s (TS2748; `verbatimModuleSyntax` reports the same error). Importing a non-ambient `const enum` from another file compiles under both flags in 5.8.3, but single-file transpilers cannot inline it and leave a runtime property access instead — avoid `const enum` across files.
- Side-effect-only imports (`import "./polyfill"`) are always kept.

##### Dynamic `import()` and `require` in ESM

- `import()` returns a `Promise` of the namespace; in CommonJS output under `--module commonjs`, tsc converts `import()` to `require` wrapped in a `Promise` — which cannot load ESM-only packages on Node.js < 20.19. Under `node16`+ it is preserved as real `import()`.
- In ESM, `require` is not defined. Use `import { createRequire } from "node:module"` when a CommonJS-only API is unavoidable. `__dirname`/`__filename` do not exist in ESM; use `import.meta.dirname`/`import.meta.filename` (Node.js 20.11+/21.2+, typed in current `@types/node`) or `fileURLToPath(import.meta.url)`.

##### JSON modules

- Under `node18`/`nodenext`, ESM files must import JSON with `with { type: "json" }` (TS1543 otherwise), only the default import is available, and named imports are TS1544.[^6]
- `resolveJsonModule` types JSON imports by *inferring from the file*: strings become `string`, not literal types. Validate JSON that comes from configurable locations; importing it does not validate anything.

***

#### Type System Essentials

##### `any`, `unknown`, `never`

`any` turns off checking for everything it touches and *spreads*: property accesses, calls and most operations on `any` produce `any`. `unknown` accepts every value but allows nothing until narrowed. `never` is the empty type: the result of exhaustive narrowing and the return type of functions that never return.

```ts
// BAD — `any` from JSON.parse flows everywhere unchecked
function loadPortBad(raw: string): number {
  const cfg = JSON.parse(raw); // any
  return cfg.prot; // typo compiles; returns undefined at runtime
}

// GOOD — parse to unknown, then validate
function loadPort(raw: string): number {
  const cfg: unknown = JSON.parse(raw);
  if (
    typeof cfg === "object" && cfg !== null &&
    "port" in cfg && typeof cfg.port === "number"
  ) {
    return cfg.port; // narrowed to number (4.9+ `in` narrowing)
  }
  throw new TypeError("invalid config: port must be a number");
}
console.log(loadPortBad, loadPort);
```

Review rules:

- Treat every `any` in a diff as a defect unless justified in a comment. Common silent sources: `JSON.parse`, `Response#json()` (`Promise<any>`), `catch` variables when `useUnknownInCatchVariables` is off, untyped third-party modules (`declare module "x";`), `Function`, `Map<any, any>`, `as any` casts, and `@ts-ignore`.
- `unknown` is the correct type for untrusted input; narrow it with type guards or a schema validator (Zod, Valibot, ArkType, TypeBox, `ajv`).
- `{}` means "any non-nullish value" and `object` means "any non-primitive" — neither means "empty object". Use `Record<string, never>` for an object that must have no properties, `Record<string, unknown>` for an arbitrary dictionary.

##### Narrowing

TypeScript narrows through `typeof`, `instanceof`, `in`, equality, truthiness, discriminant properties, `Array.isArray`, type predicates and assertion functions, and (5.4+) keeps narrowings inside closures created after the last assignment. Since 5.5 it also infers type predicates for simple arrow functions and narrows `obj[key]` when both `obj` and `key` are effectively constant.[^5]

```ts
function len(x: string | string[] | null): number {
  if (x === null) return 0;
  if (typeof x === "string") return x.length;
  return x.reduce((n, s) => n + s.length, 0);
}

// 5.5+: the filter callback is inferred as `(x: number | undefined) => x is number`
const nums: number[] = [1, undefined, 3].filter((x) => x !== undefined);

// 5.5+: constant indexed access narrows
function upper(obj: Record<string, unknown>, key: string): string | undefined {
  if (typeof obj[key] === "string") {
    return obj[key].toUpperCase();
  }
  return undefined;
}
console.log(len, nums, upper);
```

Pitfalls that compile but are wrong:

```ts
// BAD — truthiness narrowing drops 0 and ""
function portOrDefaultBad(port: number | undefined): number {
  return port || 8080; // port 0 → 8080
}
// GOOD
function portOrDefault(port: number | undefined): number {
  return port ?? 8080;
}

// BAD — typeof null === "object"
function isRecordBad(v: unknown): boolean {
  return typeof v === "object"; // true for null and arrays
}
// GOOD
function isRecord(v: unknown): v is Record<string, unknown> {
  return typeof v === "object" && v !== null && !Array.isArray(v);
}
console.log(portOrDefaultBad, portOrDefault, isRecordBad, isRecord);
```

Since 5.6, conditions that are *always* truthy or nullish are errors — e.g. `if (/regex/)`, `if (() => {})`, `x ?? y` where `x` can never be nullish, or `value < 0 || "default"` precedence mistakes:

```ts
declare function isValid(s: string): boolean;
function check(input: string): void {
  if (isValid) { // error TS2774 — function is always defined; did you mean isValid(input)?
    console.log("ok");
  }
  if (/^\d+$/) { // error TS2872 — this kind of expression is always truthy
    console.log(input);
  }
}
check("1");
```

##### User-defined type guards and assertion functions

A type predicate (`x is T`) is a *promise to the compiler* that is not checked. A wrong predicate is a type-safety hole that spreads to every caller. Review its body as carefully as an `as` cast.

```ts
interface User { id: string; email: string }

// BAD — claims User after checking one property
function isUserBad(v: unknown): v is User {
  return typeof v === "object" && v !== null && "id" in v;
}

// GOOD — checks every property it promises
function isUser(v: unknown): v is User {
  return (
    typeof v === "object" && v !== null &&
    "id" in v && typeof v.id === "string" &&
    "email" in v && typeof v.email === "string"
  );
}

// Assertion function: narrows after the call or throws.
function assertIsUser(v: unknown): asserts v is User {
  if (!isUser(v)) throw new TypeError("not a User");
}

function handle(body: unknown): string {
  assertIsUser(body);
  return body.email;
}
console.log(isUserBad, handle);
```

Assertion functions only narrow when called through an identifier or property whose type is **explicitly annotated** — `const assertX = (v: unknown): asserts v is X => {…}` without an annotation on the `const` produces error TS2775 at the call site.

##### Discriminated unions and exhaustiveness

```ts
type Shape =
  | { kind: "circle"; radius: number }
  | { kind: "rect"; width: number; height: number }
  | { kind: "triangle"; base: number; height: number };

function area(s: Shape): number {
  switch (s.kind) {
    case "circle":
      return Math.PI * s.radius ** 2;
    case "rect":
      return s.width * s.height;
    case "triangle":
      return (s.base * s.height) / 2;
    default:
      return assertNever(s); // compile error here when a new kind is added
  }
}

function assertNever(x: never): never {
  throw new Error(`unexpected value: ${JSON.stringify(x)}`);
}
console.log(area({ kind: "circle", radius: 1 }));
```

- Prefer a `never` check (or `satisfies never`) in `default` over a silent `default: return 0`. With `noImplicitReturns` or a declared return type, a missing case without a `default` is also caught (TS2366), but only for functions that return.
- Destructured discriminants narrow sibling variables (4.6+): with `function f({ kind, payload }: Action)`, checking `kind === "a"` narrows `payload`. A rest element does **not** narrow: after `const { kind, ...rest } = act`, `rest.payload` stays the union (verified).
- Do not use `boolean` flags (`isAdmin`, `isGuest`) where states are mutually exclusive; use a union with a discriminant.

##### `satisfies`, `as const` and `const` type parameters

```ts
type Route = { path: string; method: "GET" | "POST" };

// `satisfies` checks the shape but keeps the precise inferred type
const routes = {
  health: { path: "/health", method: "GET" },
  login: { path: "/login", method: "POST" },
} satisfies Record<string, Route>;

const healthMethod: "GET" = routes.health.method; // precise literal type, not "GET" | "POST"
routes.missing; // error TS2339 — unknown keys are still caught

// BAD — annotation widens and loses keys
const routesWide: Record<string, Route> = routes;
const r = routesWide.anything; // Route — no error for a nonexistent key

// `as const` for literal tuples / readonly config
const levels = ["debug", "info", "warn", "error"] as const;
type Level = (typeof levels)[number]; // "debug" | "info" | "warn" | "error"

// `const` type parameter (5.0+) infers literals without `as const` at the call site
function defineEnum<const T extends readonly string[]>(values: T): T {
  return values;
}
const colors = defineEnum(["red", "green"]); // readonly ["red", "green"]
console.log(healthMethod, r, colors, ((l: Level) => l)("info"));
```

`satisfies` is the right tool for configuration objects and lookup tables; an `as` assertion is not (it only checks that the types are *comparable* and hides missing properties).

##### Type assertions (`as`), non-null (`!`) and definite assignment (`!:`)

```ts
interface Config { host: string; port: number }

// BAD — `as` silences the missing property
const cfgBad = { host: "localhost" } as Config;
console.log(cfgBad.port.toFixed()); // compiles; TypeError at runtime

// GOOD — annotation (or satisfies) reports the missing property
const cfg: Config = { host: "localhost" }; // error TS2741

// Double assertion through unknown is always allowed — and always suspicious
const n = "42" as unknown as number;

// Non-null assertion hides a real possibility of undefined
const el = new Map<string, number>().get("x")!; // undefined at runtime
console.log(cfg, n, el);
```

Flag every `as`, `!`, and `!:` in a diff and ask why the compiler could not prove it. Acceptable: narrowing after a check the compiler cannot follow (documented), `as const`, test fixtures. Unacceptable: casting `JSON.parse` output to a domain type, `as any` to make an error disappear.

##### Object types: interfaces vs type aliases

- `interface` for object shapes that may be extended or merged (public library APIs, declaration merging, `implements`). Interfaces show by name in errors and are cached by the checker, which helps compile performance in large codebases.
- `type` for unions, intersections, tuples, mapped/conditional types, function types and aliases of primitives.
- **Declaration merging** happens for interfaces with the same name in the same scope — and for global interfaces such as `Window` or `ProcessEnv`. Accidental merging (two files in a script context declaring `interface User`) is a real bug source; `moduleDetection: "force"` or any `import`/`export` makes files modules and prevents it.
- An `interface` cannot extend a union; an intersection of conflicting properties silently produces `never` members.

```ts
type A = { id: string };
type B = { id: number };
type AB = A & B; // id: never — no error until you try to create one
const ab: AB = { id: "x" }; // error TS2322
console.log(ab);
```

##### Excess property checks and structural typing

Excess property checks only apply to **fresh object literals** assigned directly to a typed location. Once the object is in a variable, extra properties are allowed — including misspelled optional ones.

```ts
interface Options { timeoutMs?: number; retries?: number }
declare function connect(o: Options): void;

connect({ timeoutMS: 1000 }); // error TS2561 — did you mean 'timeoutMs'?

const typoOnly = { timeoutMS: 1000 };
connect(typoOnly); // error TS2559 — weak-type check: no property in common

const opts = { timeoutMS: 1000, retries: 2 };
connect(opts); // compiles: the typo is silently ignored, timeoutMs is undefined
```

Weak types (all properties optional) give partial protection: a non-fresh object that shares *no* property with the weak type is rejected (TS2559). As soon as one property matches, misspelled siblings pass. Prefer passing literals directly, or annotate the variable with the target type (`const opts: Options = {…}`).

##### Index signatures, `Record`, and `noUncheckedIndexedAccess`

```ts
// compiled with --noUncheckedIndexedAccess
const counts: Record<string, number> = {};
const words = ["a", "b", "a"];

for (const w of words) {
  counts[w] = (counts[w] ?? 0) + 1; // counts[w]: number | undefined
}

const first = words[0]; // string | undefined
console.log(first.toUpperCase()); // error TS18048
if (first !== undefined) console.log(first.toUpperCase());

// Destructuring and for-of are unaffected by the flag
for (const w of words) console.log(w.length); // w: string
```

Without the flag, `words[0]` is `string` even for an empty array and `counts["missing"]` is `number`. Enable it in new code; in existing code, at least flag direct index reads of arrays/records that can be empty.

Also watch:

- `Record<K, V>` with a *finite* key union means **all** keys are required; `Partial<Record<K, V>>` for sparse maps.
- Plain objects used as dictionaries inherit from `Object.prototype`: `counts["constructor"]` exists at runtime. Use `Map`, `Object.create(null)`, or `Object.hasOwn()` checks for user-controlled keys (see [Security](#security)).
- `Object.keys(obj)` returns `string[]`, not `(keyof T)[]`, because objects may have extra keys at runtime. Do not "fix" this with `as (keyof T)[]` when the object came from outside.

##### Optional properties and `exactOptionalPropertyTypes`

```ts
// compiled with --exactOptionalPropertyTypes
interface Profile { nickname?: string }

const p1: Profile = {}; // ok — absent
const p2: Profile = { nickname: undefined }; // error TS2375

// Declare `| undefined` explicitly when undefined is a valid present value
interface Patch { nickname?: string | undefined }
const patch: Patch = { nickname: undefined }; // ok — means "clear the field"
console.log(p1, p2, patch);
```

The distinction matters for PATCH semantics, `Object.assign`/spread defaults (`{ ...defaults, ...overrides }` with `overrides.x = undefined` overwrites the default), and `"x" in obj` checks.

##### Tuples, readonly arrays and variadic tuples

```ts
type Pair = [key: string, value: number]; // labeled tuple elements (4.0+)
type AtLeastOne<T> = [T, ...T[]];

function head<T>(xs: AtLeastOne<T>): T {
  return xs[0];
}
head([1, 2]);
head([]); // error TS2345

function sum(xs: readonly number[]): number {
  // xs.push(1) would be TS2339 — readonly arrays have no mutating methods
  return xs.reduce((a, b) => a + b, 0);
}
const frozen = Object.freeze([1, 2, 3]); // readonly number[]
console.log(sum(frozen), ((p: Pair) => p)(["a", 1]));
```

- Accept `readonly T[]` in parameters you do not mutate; a mutable array is assignable to a readonly one, not vice versa.
- `readonly` is shallow and compile-time only; `Object.freeze` is shallow and runtime. Neither deep-freezes.
- Array methods that mutate in place (`sort`, `reverse`, `splice`) on a parameter or on state shared with callers are a classic bug; use `toSorted`/`toReversed`/`toSpliced`/`with` (ES2023, `--lib es2023`; Node.js 20+).

##### Literal types, template literal types and branded types

```ts
type HttpMethod = "GET" | "POST" | "PUT" | "DELETE";
type EventName = `on${Capitalize<"click" | "focus">}`; // "onClick" | "onFocus"

// Brands make structurally identical primitives incompatible
type UserId = string & { readonly __brand: "UserId" };
type OrderId = string & { readonly __brand: "OrderId" };

function asUserId(s: string): UserId {
  if (!/^u_[a-z0-9]+$/.test(s)) throw new Error("bad user id");
  return s as UserId; // the only place the cast is allowed
}

declare function loadOrder(id: OrderId): void;
const uid = asUserId("u_1");
loadOrder(uid); // error TS2345
const m: HttpMethod = "GET";
const e: EventName = "onClick";
console.log(m, e);
```

Brands only work if the constructor function is the single entry point; review any other `as UserId`.

***

#### Functions

##### Signatures, overloads and `this`

```ts
// Overloads: callers see only the overload signatures, never the implementation signature
function parse(input: string): number;
function parse(input: string[]): number[];
function parse(input: string | string[]): number | number[] {
  return Array.isArray(input) ? input.map(Number) : Number(input);
}
const one: number = parse("1");
const many: number[] = parse(["1", "2"]);

declare const either: string | string[];
parse(either); // error TS2769 — no overload accepts the union

// Prefer a union parameter or a generic when the return type does not depend on the input type
function sizeOf(input: string | readonly unknown[]): number {
  return input.length;
}
console.log(one, many, sizeOf(either));
```

- The implementation signature is not checked against each overload's *return* precisely — `return Number(input)` for an array overload compiles. Keep overload implementations trivial and test each overload.
- Order overloads from most specific to least specific; the first matching overload wins.
- Use `this` parameters (`function f(this: Window)`) for functions that rely on `this`; under `noImplicitThis`, `this` in a standalone function is an error otherwise.

##### Method vs function property bivariance

Parameters of **method-syntax** members are checked bivariantly even under `strictFunctionTypes`; function-typed *properties* are checked contravariantly. Method syntax therefore lets unsafe implementations through:

```ts
interface Animal { name: string }
interface Dog extends Animal { bark(): void }

interface HandlerMethod { handle(a: Animal): void } // method syntax
interface HandlerProp { handle: (a: Animal) => void } // property syntax

const dogOnly = { handle(d: Dog) { d.bark(); } };

const m: HandlerMethod = dogOnly; // compiles (bivariant) — crashes if given a Cat
const p: HandlerProp = dogOnly; // error TS2322
console.log(m, p);
```

Use property syntax for callback members in interfaces you define (the `@typescript-eslint/method-signature-style` rule enforces it).

##### Callbacks: `void` return, optional parameters and `async`

```ts
// A callback typed `() => void` may return anything; the value is just ignored
const results: number[] = [];
[1, 2, 3].forEach((n) => results.push(n)); // ok: push returns number, ignored

// BAD — async callback in forEach: promises are dropped, errors become unhandled rejections
async function saveAll(ids: string[], save: (id: string) => Promise<void>): Promise<void> {
  ids.forEach(async (id) => {
    await save(id); // not awaited by saveAll
  });
}

// GOOD — sequential
async function saveAllSeq(ids: string[], save: (id: string) => Promise<void>): Promise<void> {
  for (const id of ids) await save(id);
}
// GOOD — concurrent
async function saveAllPar(ids: string[], save: (id: string) => Promise<void>): Promise<void> {
  await Promise.all(ids.map((id) => save(id)));
}
console.log(saveAll, saveAllSeq, saveAllPar);
```

The compiler never flags `forEach(async …)`, `arr.map(async …)` without `Promise.all`, or an `async` function passed where `() => void` is expected (event handlers, `setTimeout`, Express middleware). These need `@typescript-eslint/no-floating-promises` and `no-misused-promises`.

`parseInt` as a `map` callback is the classic optional-parameter bug — `["1", "2", "3"].map(parseInt)` type-checks and yields `[1, NaN, NaN]`. Write `.map((s) => Number.parseInt(s, 10))`.

##### Generics

```ts
// Constrain only as much as needed; infer from arguments
function pluck<T, K extends keyof T>(items: readonly T[], key: K): T[K][] {
  return items.map((it) => it[key]);
}
const names: string[] = pluck([{ name: "a", age: 1 }], "name");
pluck([{ name: "a" }], "nmae"); // error TS2345 — '"nmae"' is not assignable to '"name"'

// NoInfer (5.4+): stop a parameter from widening the inferred type
function pick<T extends string>(options: readonly T[], fallback: NoInfer<T>): T {
  return options[0] ?? fallback;
}
pick(["a", "b"], "c"); // error TS2345 — without NoInfer, T would widen to "a" | "b" | "c"
console.log(names);
```

Review rules:

- A type parameter used only once (in a parameter *or* only in the return type) is usually a disguised `any`/cast: `function get<T>(key: string): T` lets callers pick any `T` without proof. Prefer `unknown` plus validation.
- `T extends object` excludes primitives; `T extends {}` excludes only `null`/`undefined`.
- `<T,>` (trailing comma) is required for generic arrow functions in `.tsx` files.
- Default type arguments (`<T = string>`) are not inferred-from; they apply only when inference has no candidates.

##### Conditional, mapped and template literal types

```ts
// Distributive conditional types apply per union member when T is a naked type parameter
type ToArray<T> = T extends unknown ? T[] : never;
type A = ToArray<string | number>; // string[] | number[]

// Wrap in a tuple to prevent distribution
type ToArrayNonDist<T> = [T] extends [unknown] ? T[] : never;
type B = ToArrayNonDist<string | number>; // (string | number)[]

// never distributes to never — a common surprise
type C = ToArray<never>; // never, not never[]

// infer
type Awaited2<T> = T extends PromiseLike<infer U> ? Awaited2<U> : T;
type D = Awaited2<Promise<Promise<number>>>; // number

// Key remapping in mapped types (4.1+)
type Getters<T> = { [K in keyof T & string as `get${Capitalize<K>}`]: () => T[K] };
type G = Getters<{ id: number; name: string }>; // { getId: () => number; getName: () => string }

// Homomorphic mapped types preserve optional/readonly modifiers; -? and -readonly remove them
type Mutable<T> = { -readonly [K in keyof T]: T[K] };

const a: A = ["x"];
const b: B = ["x", 1];
const g: G = { getId: () => 1, getName: () => "n" };
const d: D = 1;
let cNever: C;
const mu: Mutable<{ readonly x: number }> = { x: 1 };
mu.x = 2;
console.log(a, b, g, d, mu);
```

- Deeply recursive types hit limits: TS2589 "Type instantiation is excessively deep and possibly infinite" (instantiation depth 100 for non-tail recursion; tail-recursive conditional types allow up to 1000 iterations since 4.5). Libraries that hit TS2589 in user code need simpler types, not `// @ts-ignore`.
- Prefer built-in utility types (`Partial`, `Required`, `Readonly`, `Pick`, `Omit`, `Record`, `Exclude`, `Extract`, `NonNullable`, `ReturnType`, `Parameters`, `InstanceType`, `Awaited`, `NoInfer`) over home-grown equivalents.
- `Omit<T, K>` does not check that `K` is a key of `T` (`Omit<User, "pasword">` compiles and omits nothing). Use a stricter helper when it matters: `type StrictOmit<T, K extends keyof T> = Omit<T, K>`.
- `Partial<T>` for update payloads allows `{}` and explicit `undefined`s; validate before applying to persistent state.

##### Variance annotations

`in`/`out` modifiers on type parameters (4.7+) declare variance for the checker. They are only for performance of extremely complex types and for documenting intent; a wrong annotation makes the checker accept unsound assignments it would otherwise reject. Flag `in`/`out` in application code.

***

#### Classes

##### Fields, initialization and `useDefineForClassFields`

`useDefineForClassFields` defaults to `true` for `target` ES2022 and later (including `esnext`, and therefore for `--module nodenext` without an explicit `target`). Fields are then emitted as native class fields with `[[Define]]` semantics, initialized right after `super()` returns and *before* the constructor body — including before parameter properties are assigned:

```ts
class Price {
  doubled = this.base * 2; // error TS2729 — 'base' is used before its initialization
  constructor(private readonly base: number) {}
}
console.log(new Price(21).doubled); // NaN with target >= es2022; 42 with es2020 (assignment semantics)

class Base {
  name = "base";
}
class Redeclared extends Base {
  name: string; // error TS2564, TS2612 — would reset `name` to undefined at runtime
}
class Refined extends Base {
  declare name: "base" | "other"; // type-only redeclaration: no runtime field
}
console.log(new Redeclared().name, new Refined().name); // undefined "base" (es2022)
```

Verified with 5.8.3 + Node.js 22: the code above prints `NaN` and `undefined base` when compiled with `--target es2022`, and `42` and `base base` with `--target es2020` (where TS2729/TS2612 are not reported and `useDefineForClassFields` defaults to `false`). Raising `target` across ES2022 therefore changes runtime behaviour; the compiler reports the two patterns above, so never suppress TS2729/TS2612. Rules:

- Do not call overridable methods from constructors: a subclass field initializer runs after the base constructor and overwrites whatever the overridden method assigned (in both semantics).
- Use `declare field: Type;` to only refine the type of an inherited field.
- Frameworks that rely on accessors defined on the prototype not being shadowed by instance fields (older MobX, Lit, some legacy-decorator libraries) need `useDefineForClassFields: false` or `accessor`/`declare`.

`strictPropertyInitialization` requires fields to be assigned in the constructor. `field!: T` (definite assignment) turns that check off for one field — acceptable only when a framework provably initializes it (DI, ORMs).

##### `#private` vs `private`

```ts
class Account {
  #balance = 0; // runtime-private (ES2022), invisible to other code, not in JSON.stringify keys
  private audit: string[] = []; // compile-time only — accessible via (acc as any).audit or acc["audit"]

  deposit(amount: number): void {
    if (!Number.isFinite(amount) || amount <= 0) throw new RangeError("amount must be > 0");
    this.#balance += amount;
    this.audit.push(`+${amount}`);
  }
  get balance(): number {
    return this.#balance;
  }
  static isAccount(x: unknown): x is Account {
    return typeof x === "object" && x !== null && #balance in x; // ergonomic brand check (4.5+)
  }
}
const acc = new Account();
acc.deposit(10);
console.log(acc.balance, acc["audit"], Account.isAccount(acc));
```

Use `#private` for invariants that must hold at runtime (security-sensitive state, values that must not leak into logs via spreading). Note that `#private` fields do not work through `Proxy` objects (the proxy is not the instance), which breaks some reactive frameworks.

##### `override`, `abstract`, `accessor`, parameter properties

- With `noImplicitOverride`, overriding members must say `override`; renaming a base method then breaks the build instead of silently orphaning the override.
- `accessor x = 0` (4.9+) creates an auto-accessor (private storage + getter/setter) — the hook point for standard decorators.
- Parameter properties (`constructor(private readonly repo: Repo)`) are concise but are **not erasable** syntax: rejected by `--erasableSyntaxOnly` and by Node.js type stripping. In codebases that run TypeScript via type stripping, write explicit fields.

```ts
// erasable equivalent of `constructor(private readonly repo: Repo) {}`
interface Repo { find(id: string): Promise<unknown> }
class UserService {
  readonly #repo: Repo;
  constructor(repo: Repo) {
    this.#repo = repo;
  }
  get(id: string): Promise<unknown> {
    return this.#repo.find(id);
  }
}
console.log(UserService);
```

##### Decorators

TypeScript 5.x implements **standard (TC39 stage 3) decorators** by default; `--experimentalDecorators` switches to the legacy implementation.[^18]

```ts
// standard decorators — no compiler flag needed (target es2022)
function logged<This, Args extends unknown[], R>(
  target: (this: This, ...args: Args) => R,
  context: ClassMethodDecoratorContext<This, (this: This, ...args: Args) => R>,
) {
  return function (this: This, ...args: Args): R {
    console.log(`→ ${String(context.name)}`);
    return target.call(this, ...args);
  };
}

class Calculator {
  @logged
  add(a: number, b: number): number {
    return a + b;
  }
}
console.log(new Calculator().add(1, 2)); // "→ add", then 3
```

- Standard decorators do **not** support parameter decorators and do not emit `design:type` metadata (`emitDecoratorMetadata` only works with `experimentalDecorators`). Decorator metadata (`Symbol.metadata`, 5.2+) needs a runtime polyfill for `Symbol.metadata` on Node.js — check before relying on it.
- Decorators are runtime code: they are not erasable and Node.js type stripping cannot parse them.
- A decorator that returns a replacement must preserve `this` and the signature; review wrappers for lost `this`, swallowed return values and missing `await` on async methods.

***

#### Enums and Alternatives

```ts
enum Status { Active, Disabled } // numeric enum: emits an object with reverse mapping

const s: Status = 42; // error TS2322 — since 5.0, out-of-range literals are rejected
const n: number = 7;
const t: Status = n; // compiles — any number is assignable to a numeric enum
console.log(Status[0], Status[Status.Active], s, t); // "Active" "Active" — reverse mapping

// Preferred: a const object + union type (erasable, no reverse-mapping surprises)
const Role = { Admin: "admin", User: "user" } as const;
type Role = (typeof Role)[keyof typeof Role]; // "admin" | "user"

function can(role: Role): boolean {
  return role === Role.Admin;
}
can("admin"); // plain string literals work — useful at API boundaries
console.log(can(Role.User));
```

- **Numeric enums** accept any `number` from a non-literal source and have reverse mappings (`Object.values(Status)` returns both names and numbers — a frequent bug when enumerating enum values).
- **String enums** are nominal: `"active"` is not assignable to `StatusStr.Active`, which forces casts at JSON boundaries.
- **`const enum`** is inlined by tsc but cannot be compiled by single-file transpilers across files, is rejected by `isolatedModules` when ambient, and by `--erasableSyntaxOnly`. Avoid in libraries (consumers with `isolatedModules` break).
- Enums and runtime `namespace`s are rejected by `--erasableSyntaxOnly`; projects that run under Node.js type stripping must use the `as const` pattern.

***

#### Null, Undefined and Optional Chaining

```ts
interface Order { customer?: { address?: { city: string } }; total: number | null }

function city(o: Order): string {
  return o.customer?.address?.city ?? "unknown"; // short-circuits on null/undefined only
}

// BAD — `||` replaces 0 and "" as well
function totalBad(o: Order): number {
  return o.total || -1; // total 0 becomes -1
}
// GOOD
function total(o: Order): number {
  return o.total ?? -1;
}

// `== null` matches both null and undefined (the one accepted use of loose equality)
function isMissing(v: unknown): v is null | undefined {
  return v == null;
}
console.log(city, totalBad, total, isMissing);
```

- `?.` returns `undefined` when the left side is `null`/`undefined`; it does not guard against other falsy values or against the property being a non-function (`obj.fn?.()` only checks `fn` for nullishness).
- `??=`, `||=`, `&&=` assign conditionally; `obj.count ||= 10` overwrites a legitimate `0`.
- Non-null assertions after `Map#get`, `Array#find`, `document.querySelector` and `process.env.X` are the most common source of runtime `TypeError`s in TypeScript code. Handle the `undefined` case or throw a descriptive error.

```ts
// BAD
const port = Number(process.env.PORT!); // NaN when unset, no error

// GOOD
function requireEnv(name: string): string {
  const v = process.env[name];
  if (v === undefined || v === "") throw new Error(`missing env var ${name}`);
  return v;
}
const port2 = Number.parseInt(requireEnv("PORT"), 10);
if (!Number.isInteger(port2) || port2 < 1 || port2 > 65535) throw new Error("PORT out of range");
console.log(port);
```

##### Never-initialized variables (5.7+)

Since 5.7 the compiler reports variables that are read in a nested function but never assigned anywhere:

```ts
function report(): void {
  let result: number;
  function print() {
    console.log(result); // error TS2454 — 5.7+: never assigned at all
  }
  print();
}
report();
```

Variables that are *possibly* assigned before a closure reads them are still not checked.

***

#### Errors and Exceptions

##### `catch` variables are `unknown`

Under `strict` (`useUnknownInCatchVariables`, 4.4+), `catch (e)` binds `e: unknown`. Anything can be thrown — strings, plain objects, `undefined`.

```ts
// BAD — assumes Error
async function fetchJsonBad(url: string): Promise<unknown> {
  try {
    const res = await fetch(url);
    return await res.json();
  } catch (e) {
    console.error(e.message); // error TS18046 — 'e' is of type 'unknown'
    throw e;
  }
}

// GOOD — narrow, wrap with cause, keep the original
class HttpError extends Error {
  override readonly name = "HttpError";
  constructor(readonly status: number, message: string, options?: ErrorOptions) {
    super(message, options);
  }
}

async function fetchJson(url: string, signal?: AbortSignal): Promise<unknown> {
  let res: Response;
  try {
    res = await fetch(url, signal ? { signal } : {});
  } catch (e) {
    throw new Error(`request to ${url} failed`, { cause: e }); // ErrorOptions.cause, es2022 lib
  }
  if (!res.ok) throw new HttpError(res.status, `GET ${url} → ${res.status}`);
  return res.json() as Promise<unknown>;
}
console.log(fetchJsonBad, fetchJson);
```

Rules:

- Narrow with `e instanceof Error` before reading `.message`/`.stack`. Note that `instanceof` fails for errors from other realms (iframes, `vm` contexts, some worker setups); `Error.isError()` is not available (5.9 types, Node.js 24+ runtime).
- `throw` only `Error` instances (lint: `@typescript-eslint/only-throw-error`); Promise rejections likewise (`prefer-promise-reject-errors`).
- Preserve the original with `{ cause }` (lib `es2022`); never `throw new Error(String(e))`, which loses the stack.
- `return await` inside `try` is required for the `catch`/`finally` to see a rejection — `return promise` inside `try` bypasses the local `catch`.
- Custom error classes: set `name`; with targets ≥ ES2015 `instanceof` works without `Object.setPrototypeOf(this, new.target.prototype)` (that workaround is only needed for ES5 output).
- `AggregateError` (from `Promise.any`) and `SuppressedError` (from `using`) wrap multiple errors; log `.errors` / `.error` + `.suppressed`, not only the outer message.

##### Typed results instead of exceptions

For expected failures (validation, "not found", business rule rejections), a discriminated union keeps failure handling visible in the types:

```ts
type Result<T, E = Error> = { ok: true; value: T } | { ok: false; error: E };

function parsePort(s: string): Result<number, string> {
  const n = Number.parseInt(s, 10);
  if (!Number.isInteger(n) || n < 1 || n > 65535) return { ok: false, error: `invalid port: ${s}` };
  return { ok: true, value: n };
}

const r = parsePort("8080");
if (r.ok) console.log(r.value + 1);
else console.error(r.error);
console.log(r.value); // error TS2339 — must check r.ok first
```

Keep exceptions for programmer errors and truly exceptional conditions (I/O failure, invariant violation).

***

#### Async and Concurrency

##### Promises: floating, sequential vs concurrent, error aggregation

```ts
declare function fetchUser(id: string): Promise<{ id: string }>;
declare function audit(event: string): Promise<void>;

async function handler(ids: string[]): Promise<void> {
  audit("start"); // BAD: floating promise — rejection is unhandled, ordering is lost
  void audit("start-fire-and-forget"); // explicit, but still needs .catch() somewhere
  await audit("start"); // GOOD when ordering/failure matters

  // BAD: sequential awaits in a loop when requests are independent
  for (const id of ids) await fetchUser(id);

  // GOOD: concurrent; rejects on first failure (others keep running)
  const users = await Promise.all(ids.map(fetchUser));

  // GOOD: collect all outcomes
  const settled = await Promise.allSettled(ids.map(fetchUser));
  const failed = settled.filter((s): s is PromiseRejectedResult => s.status === "rejected");
  console.log(users.length, failed.map((f) => f.reason));
}
console.log(handler);
```

- Node.js terminates the process on unhandled rejections by default (since 15). Every promise must be awaited, returned, or given a `.catch`.
- `Promise.all` over thousands of items starts all operations at once — use a concurrency limiter (`p-limit`, a worker pool, or batching) for network/database calls.
- `Promise.race` with a timeout does not cancel the losing operation. Use `AbortSignal`.

##### Cancellation and timeouts with `AbortSignal`

```ts
async function getWithTimeout(url: string, ms: number, outer?: AbortSignal): Promise<string> {
  const signal = outer
    ? AbortSignal.any([outer, AbortSignal.timeout(ms)]) // Node.js 20.3+, lib dom / @types/node
    : AbortSignal.timeout(ms);
  const res = await fetch(url, { signal });
  if (!res.ok) throw new Error(`HTTP ${res.status}`);
  return res.text();
}

async function main(): Promise<void> {
  try {
    console.log(await getWithTimeout("https://example.com", 5_000));
  } catch (e) {
    if (e instanceof DOMException && e.name === "TimeoutError") console.error("timed out");
    else throw e;
  }
}
console.log(main);
```

`AbortSignal.timeout()` rejects with a `DOMException` named `"TimeoutError"`; a manual `controller.abort()` rejects with `"AbortError"`. Long-running loops should check `signal.throwIfAborted()`.

##### `Promise.withResolvers`, `Array.fromAsync` and other ES2024 APIs

`Promise.withResolvers()`, `Object.groupBy()`, `Map.groupBy()`, `Array.fromAsync()` and resizable `ArrayBuffer`s are typed in `--lib es2024` (5.7+ for `es2024`; earlier as `esnext`). Node.js 22 has all of them (verified). Browsers: check your support matrix.

```ts
const { promise, resolve } = Promise.withResolvers<number>();
setTimeout(() => resolve(42), 10);
const byParity = Object.groupBy([1, 2, 3], (n) => (n % 2 ? "odd" : "even"));
// byParity.odd is number[] | undefined — groupBy returns Partial<Record<K, T[]>>
console.log(await promise, byParity.odd?.length);
```

##### Explicit resource management: `using` and `await using`

`using` (5.2+) calls `[Symbol.dispose]()` when the block exits, also on exceptions; `await using` awaits `[Symbol.asyncDispose]()`. With `target` below `esnext`, tsc down-levels it (helpers `__addDisposableResource`/`__disposeResources`); it still needs `Symbol.dispose`/`Symbol.asyncDispose` at runtime (Node.js 18.20.8 and 20+ have both — verified). With `target: esnext` — which `--module nodenext` implies when `target` is not set — tsc emits `using` unchanged, and Node.js 22 fails with `SyntaxError: Unexpected identifier 'tmp'` (verified). Native `using` syntax exists in Node.js 24+; `DisposableStack`/`AsyncDisposableStack`/`SuppressedError` globals exist in Node.js 24+ but not in Node.js 22 (verified).

```ts
// --lib es2022,esnext.disposable,dom
class TempFile implements Disposable {
  constructor(readonly path: string) {}
  [Symbol.dispose](): void {
    console.log(`removing ${this.path}`);
  }
}

function work(): void {
  using tmp = new TempFile("/tmp/x");
  console.log(`using ${tmp.path}`);
  throw new Error("boom"); // tmp is still disposed
}

try {
  work();
} catch (e) {
  console.log((e as Error).message);
}
// prints: using /tmp/x, removing /tmp/x, boom
```

Verified on Node.js 22.23.4 with `--target es2022` (down-levelled) and Node.js 24.22.1. Review: dispose methods must not throw for already-closed resources; resources returned from a function must not be declared with `using` inside it (they would be disposed on return).

##### Event loop and CPU work

- `async` does not make CPU-bound code concurrent. Long synchronous loops block the event loop; move them to `worker_threads` or chunk them.
- `await` inside a tight loop over in-memory data adds microtask overhead without benefit.
- Shared mutable state across `await` points is a race: two concurrent calls to `async function inc() { const v = await read(); await write(v + 1); }` lose updates. Use transactions, atomic operations, or a mutex.
- `SharedArrayBuffer` + `Atomics` are the only shared memory between workers; everything else is copied via structured clone (class instances lose their prototype).

***

#### Iterators, Generators and Collections

- Iterator helpers (`.map`, `.filter`, `.take`, `.drop`, `.flatMap`, `.reduce`, `.toArray`, `.some`, `.every`, `.find`, `.forEach`, `Iterator.from`) are typed in `lib.esnext.iterator` (5.6+) and exist in Node.js 22+. `Set` methods (`union`, `intersection`, `difference`, `symmetricDifference`, `isSubsetOf`, `isSupersetOf`, `isDisjointFrom`) are typed in `lib.esnext.collection` (5.5+) and exist in Node.js 22+. In 5.8 both are only in `esnext`, not in a year-numbered lib.
- `strictBuiltinIteratorReturn` (in `strict`, 5.6+) types the `value` of a finished built-in iterator as `undefined`:

```ts
const it = new Map([["a", 1]]).keys();
const r = it.next();
const k: string = r.value; // error TS2322 — value is string | undefined until `done` is checked
if (!r.done) {
  const ok: string = r.value;
  console.log(ok);
}
console.log(k);
```

- Generators: `Generator<T, TReturn, TNext>`; annotate `TReturn` to get the 5.8 return-branch check. Iterating a generator twice yields nothing the second time — return arrays or iterables, not iterators, from APIs that may be consumed more than once.
- `Map`/`Set` use SameValueZero: object keys compare by identity, so `map.get({ id: 1 })` never hits. Use primitive keys.
- `Array.prototype.sort()` without a comparator sorts by UTF-16 string order: `[10, 9, 1].sort()` → `[1, 10, 9]`. Always pass `(a, b) => a - b` for numbers and `localeCompare`/`Intl.Collator` for user-visible strings.
- `for…in` iterates inherited enumerable keys and yields strings for array indices; use `for…of`, `Object.entries`, or `.entries()`.

***

#### Runtime Validation at Boundaries

Types vanish at runtime. Every value that crosses a trust boundary — HTTP bodies, query strings, headers, environment variables, files, database rows, queue messages, `postMessage`, `localStorage`, third-party API responses — must be validated before it is given a domain type.

```ts
interface CreateUser { email: string; age: number; roles: ("admin" | "user")[] }

// BAD — the cast is a lie the compiler cannot check
async function createUserBad(req: Request): Promise<CreateUser> {
  return (await req.json()) as CreateUser;
}

// GOOD — validate, then the type is earned
function parseCreateUser(v: unknown): CreateUser {
  if (typeof v !== "object" || v === null) throw new TypeError("body must be an object");
  const { email, age, roles } = v as Record<string, unknown>;
  if (typeof email !== "string" || !email.includes("@")) throw new TypeError("email");
  if (typeof age !== "number" || !Number.isInteger(age) || age < 0) throw new TypeError("age");
  if (!Array.isArray(roles) || !roles.every((r) => r === "admin" || r === "user")) {
    throw new TypeError("roles");
  }
  return { email, age, roles }; // copy only known fields — drops unexpected properties
}

async function createUser(req: Request): Promise<CreateUser> {
  return parseCreateUser(await req.json());
}
console.log(createUserBad, createUser);
```

- In real projects use a schema library (Zod, Valibot, ArkType, TypeBox + Ajv, io-ts) and derive the static type from the schema (`z.infer<typeof Schema>`) so the two cannot drift. Check the library's stated TypeScript support before recommending a version (for example, Zod is tested against TypeScript 5.5 and later and requires `strict`).[^19]
- Return a **new object with known fields** after validation; passing the original through keeps unexpected properties (mass assignment, `__proto__` keys).
- Validate *outputs* of third-party APIs too, at least for fields you branch on.
- Generated clients (OpenAPI, GraphQL codegen, Prisma, tRPC) give accurate types only for data that came through them; hand-written `as` around them reintroduces the problem.

***

#### Security

##### Prototype pollution

`JSON.parse('{"__proto__": {...}}')` creates an **own** property named `__proto__`. It becomes dangerous when copied with assignment semantics:

```ts
function mergeBad(target: Record<string, any>, src: Record<string, any>): Record<string, any> {
  for (const k in src) {
    if (typeof src[k] === "object" && src[k] !== null) {
      target[k] ??= {};
      mergeBad(target[k], src[k]); // target["__proto__"] is Object.prototype!
    } else {
      target[k] = src[k];
    }
  }
  return target;
}

const payload: unknown = JSON.parse('{"__proto__": {"isAdmin": true}}');
mergeBad({}, payload as Record<string, any>);
console.log(({} as { isAdmin?: boolean }).isAdmin); // true — every object is now "admin"
```

Verified on Node.js 22: the recursive merge pollutes `Object.prototype`; `Object.assign({}, payload)` does not pollute the global prototype but **replaces the prototype of the new object** (`o.isAdmin === true`); object spread `{ ...payload }` defines a harmless own `__proto__` property. Defences:

- Skip `__proto__`, `constructor` and `prototype` keys in any recursive merge/set-by-path code; prefer `structuredClone` or a vetted library version.
- Use `Map` for user-keyed dictionaries, or objects created with `Object.create(null)`.
- Check membership with `Object.hasOwn(obj, key)` (lib `es2022`), not `key in obj` or `obj[key] !== undefined`.
- Validate input shape first (unknown keys rejected or stripped).
- Node.js: `--disable-proto=delete` removes `Object.prototype.__proto__` as defence in depth.

##### Injection

```ts
import { execFile } from "node:child_process";
import { promisify } from "node:util";
import path from "node:path";

const execFileAsync = promisify(execFile);

// BAD — shell injection: name = "x; rm -rf ~"
// exec(`convert ${name} out.png`)

// GOOD — no shell, arguments as an array
async function convert(name: string): Promise<void> {
  await execFileAsync("convert", ["--", name, "out.png"], { timeout: 30_000 });
}

// Path traversal: resolve, then check containment against the root + separator
const ROOT = path.resolve("/srv/uploads");
function safeJoin(userPath: string): string {
  const p = path.resolve(ROOT, userPath);
  if (p !== ROOT && !p.startsWith(ROOT + path.sep)) throw new Error("path escapes root");
  return p;
}
console.log(convert, safeJoin("a/b.txt"));
```

- **SQL**: always use parameterized queries or a query builder. A template literal passed to a plain `query(string)` is string concatenation; only *tagged* templates from the driver (``sql`…${v}` `` in postgres.js, Prisma `$queryRaw`, Slonik) parameterize — and `$queryRawUnsafe`/`sql.unsafe()` do not.
- **Regular expressions from user input**: escape them. `RegExp.escape()` is **not** available in 5.8 (typed in 6.0; runtime in Node.js 24+) — use a local escape function: `s.replace(/[.*+?^${}()|[\]\\]/g, "\\$&")`. Watch for catastrophic backtracking (`/(a+)+$/`) on untrusted input (ReDoS).
- **`eval`, `new Function`, `vm.runInContext`** on untrusted strings are code execution; `vm` is not a security boundary.
- **HTML**: `innerHTML`, `outerHTML`, `insertAdjacentHTML`, `document.write`, React's `dangerouslySetInnerHTML`, Vue's `v-html` with untrusted data are XSS. Use `textContent`, framework escaping, or a sanitizer (DOMPurify) plus Trusted Types.
- **URLs**: `new URL(input)` then check `protocol` (`javascript:` URLs) and host allowlists for redirects and server-side fetches (SSRF).

##### Secrets, randomness and comparison

```ts
import { randomBytes, randomUUID, timingSafeEqual, createHash } from "node:crypto";

const token = randomBytes(32).toString("base64url"); // not Math.random()
const id = randomUUID();

function safeEqual(a: string, b: string): boolean {
  // hash first so lengths are equal; timingSafeEqual throws on length mismatch
  const ha = createHash("sha256").update(a).digest();
  const hb = createHash("sha256").update(b).digest();
  return timingSafeEqual(ha, hb);
}
console.log(token.length, id.length, safeEqual("a", "a"));
```

- Never log full request bodies, headers (`authorization`, `cookie`), tokens or environment dumps. Redact in the logger configuration.
- Store passwords with `scrypt`/`argon2`/`bcrypt`, never a fast hash.
- Do not ship secrets in front-end bundles: anything referenced via `import.meta.env`/`process.env` in client code is public.

##### Dependency and supply-chain hygiene

- Commit the lockfile and install with `npm ci`/`pnpm install --frozen-lockfile`/`yarn install --immutable` in CI.
- `@types/*` packages are code from DefinitelyTyped and must match the runtime library's major version; a mismatch hides real API differences.
- Pin `typescript` with `~5.8.3` (patch only). TypeScript does not follow semver: minor versions (5.8 → 5.9) routinely introduce new errors.

***

#### Performance

##### Runtime performance

TypeScript adds no runtime cost except for the constructs that emit code and for down-levelling. Review runtime performance as JavaScript:

- **`target` too low** inflates output: async/await, generators, class fields, `using` and object spread are rewritten into helpers. Set `target` to what the oldest runtime supports (ES2022+ for Node.js 18+; ES2023 for Node.js 22). `importHelpers` + `tslib` avoids duplicated helpers per file when down-levelling is unavoidable.
- **Enums** cost an object and, for numeric enums, a reverse map; `as const` objects are cheaper and erasable.
- **Hot paths**: avoid `delete obj.prop` (changes hidden classes), megamorphic objects built from many shapes, array holes, repeated `JSON.parse(JSON.stringify(x))` (use `structuredClone`), building strings with `+=` in large loops (fine in V8 for moderate sizes, but `Array#join` for huge outputs), and repeated spreading in reducers (`acc = { ...acc, [k]: v }` is O(n²)).
- **`Array#includes` on large arrays inside loops** is O(n·m); build a `Set` first.
- **Regex** compiled inside a loop or function called per request: hoist to module scope (but beware `/g` and `/y` regexes are stateful via `lastIndex` when shared).
- **Bundle size** (front-end): `import * as _ from "lodash"` and barrel files (`index.ts` re-exporting everything) defeat tree-shaking in many setups; enums and `namespace`s are not tree-shakable.

```ts
// BAD — shared global regex keeps lastIndex between calls
const WORD = /\w+/g;
function hasWordBad(s: string): boolean {
  return WORD.test(s);
}
console.log(hasWordBad("abc"), hasWordBad("abc")); // true false

// GOOD — no `g` flag for test(), or a fresh RegExp per call
const WORD1 = /\w+/;
function hasWord(s: string): boolean {
  return WORD1.test(s);
}
console.log(hasWord("abc"), hasWord("abc")); // true true
```

##### Compile-time performance

Slow type-checking is a real cost in large monorepos (editor latency, CI time). Tools:

- `tsc --extendedDiagnostics` (time per phase, type/instantiation counts), `tsc --generateTrace <dir>` (open in `about:tracing`/Perfetto, analyze with `@typescript/analyze-trace`), `--explainFiles` (why a file is in the program), `--traceResolution` (module resolution), `--listFilesOnly`.
- `"types": [...]` explicit — 5.8 otherwise loads all of `node_modules/@types`.
- `skipLibCheck: true` skips checking `.d.ts` files.
- `incremental: true` (+ `tsBuildInfoFile`) and project references with `composite: true` + `tsc -b` for monorepos. `--stopBuildOnErrors` (5.6+) restores the old "stop at first failing project" behaviour of `tsc -b`.
- `--isolatedDeclarations` (5.5+) requires explicit types on exports so other tools can generate `.d.ts` in parallel without the checker.
- `--noCheck` (5.6+) emits without type-checking — only for a separate fast emit step when type-checking runs elsewhere in CI.
- Type-level hot spots: huge unions (> a few thousand members) in conditional types, deeply recursive mapped types, intersections of many object types (prefer `interface extends`), and generic functions with many overloads. Annotating return types of exported functions reduces work for consumers and declaration emit.
- The 5.8 program-load improvements need no configuration. Microsoft's native (Go) port, shipped as TypeScript 7.0, was announced as roughly 10× faster,[^23] but it is a different major with every 6.0-deprecated option removed; it is not a drop-in for a 5.8 project.

***

#### Testing

##### Test runners and type-checking

Most TypeScript test runners **do not type-check**: Vitest (esbuild/Oxc), Jest with `@swc/jest` or Babel, `tsx --test`, and Node.js's built-in runner with type stripping all strip types file by file. A test suite can pass while `tsc` fails. Run `tsc --noEmit -p tsconfig.test.json` (or `tsc -b`) as a separate CI step. `ts-jest` type-checks (slowly) unless `isolatedModules` is set. `ts-node` (last release 10.9.2, December 2023) is unmaintained in practice; prefer `tsx` or Node.js type stripping.

Node.js's own test runner works on `.ts` files with type stripping (Node.js 22.18+/23.6+), subject to the erasable-syntax rules:

```ts
// math.test.ts — run with: node --test (Node.js 22.18+), tsc checks with erasableSyntaxOnly
import { test } from "node:test";
import assert from "node:assert/strict";

function add(a: number, b: number): number {
  return a + b;
}

test("add", () => {
  assert.equal(add(1, 2), 3);
});
```

##### `@ts-expect-error` over `@ts-ignore`

```ts
interface User { id: string }

// @ts-expect-error — documents an intentional error; fails the build if the error disappears
const u: User = { id: 1 };

// @ts-ignore — silently suppresses anything on the next line, forever
const v: User = { id: 2 };
console.log(u, v);
```

`@ts-expect-error` itself errors (TS2578 "Unused '@ts-expect-error' directive") when the next line compiles. Ban `@ts-ignore` and `@ts-nocheck` in production code (`@typescript-eslint/ban-ts-comment`), require a description on `@ts-expect-error`.

##### Type-level tests

Public generic APIs deserve tests of their *types*. Without dependencies:

```ts
type Equal<X, Y> =
  (<T>() => T extends X ? 1 : 2) extends (<T>() => T extends Y ? 1 : 2) ? true : false;
type Expect<T extends true> = T;

declare function pick<T, K extends keyof T>(obj: T, keys: readonly K[]): Pick<T, K>;
const picked = pick({ a: 1, b: "x" }, ["a"]);

type _t1 = Expect<Equal<typeof picked, { a: number }>>;
type _t2 = Expect<Equal<typeof picked, { a: number; b: string }>>; // error TS2344
```

Libraries: Vitest's `expectTypeOf` (run with `vitest --typecheck`), `expect-type`, `tsd`. Test both the accepted and the rejected calls (`// @ts-expect-error`).

##### Testing practices that matter in TypeScript

- Fixtures typed with `as SomeType` or `Partial<T> as T` hide missing properties; build them with factory functions returning the full type, or use `satisfies`.
- Mocks created with `jest.fn()`/`vi.fn()` are typed loosely; type them (`vi.fn<(id: string) => Promise<User>>()`), or the test can call them with wrong arguments.
- Assert on error *types* (`instanceof`, `name`, `code`), not message strings.
- For time-dependent code, inject a clock or use fake timers; `Date.now()` sprinkled in code is untestable.

***

#### Tooling and Linting

##### Compiler in CI

- `tsc --noEmit` (single project) or `tsc -b` (references) must run in CI and fail the build on any error.
- Run it with the **locked** `typescript` version from `devDependencies` (`npx tsc`), never a global `tsc`.
- Editors use their bundled TypeScript unless told to use the workspace version (VS Code: "TypeScript: Select TypeScript Version → Use Workspace Version", or `"typescript.tsdk": "node_modules/typescript/lib"`). Errors that appear only in CI or only in the editor are often a version mismatch.

##### typescript-eslint

typescript-eslint 8.x supports TypeScript `>=4.8.4 <6.1.0`, so it covers 5.8 (and not 7.0).[^20] Use the type-checked configs (`recommendedTypeChecked`, `strictTypeChecked`) with `parserOptions.projectService: true`. Rules that catch bugs the compiler does not:

| Rule | Catches |
| --- | --- |
| `no-floating-promises` | unawaited promises, `forEach(async …)` results |
| `no-misused-promises` | async functions where `void`-returning callbacks or conditions are expected |
| `await-thenable`, `return-await` | awaiting non-promises; missing `return await` in `try` |
| `no-unsafe-assignment`/`-member-access`/`-call`/`-return`/`-argument` | `any` leaking out of untyped values |
| `no-explicit-any`, `ban-ts-comment` | escape hatches |
| `switch-exhaustiveness-check` | non-exhaustive `switch` over unions |
| `no-unnecessary-condition`, `strict-boolean-expressions` | impossible/unsafe truthiness checks |
| `only-throw-error`, `prefer-promise-reject-errors` | throwing/rejecting non-`Error`s |
| `consistent-type-imports` | missing `import type` (with `verbatimModuleSyntax` the compiler enforces this) |
| `restrict-template-expressions`, `restrict-plus-operands` | `${obj}` → `[object Object]`, `number + string` |
| `no-non-null-assertion`, `non-nullable-type-assertion-style` | `!` usage |
| `method-signature-style` | bivariant method-syntax callbacks |
| `require-array-sort-compare` | `sort()` without comparator |

Plain ESLint's `no-undef`, `no-unused-vars`, `no-redeclare`, `no-shadow` and `no-use-before-define` must be replaced by the `@typescript-eslint/` versions or disabled (the core versions misreport TypeScript code).

##### Formatting, transpilers, runtimes

- Prettier (or Biome/dprint) for formatting; never hand-align.
- Single-file transpilers — esbuild, swc, Babel, Oxc, Sucrase, Node.js type stripping, Bun, Deno — do not type-check and have restrictions: `isolatedModules`-style compilation (no cross-file `const enum`, type re-exports need `export type`), no `emitDecoratorMetadata` in some, no `paths` in Node.js. Enable `isolatedModules`/`verbatimModuleSyntax` so tsc reports what they cannot compile.
- `tsx` runs TypeScript with esbuild (supports enums, decorators, `paths`); it is a development/runtime convenience, not a type-checker.
- Bun and Deno bundle their own TypeScript-aware transpilers; Deno's `deno check` uses its own pinned TypeScript version, which may not be 5.8 — check what the project actually runs.
- Declaration-file quality for libraries: `@arethetypeswrong/cli` (`attw --pack .`) and `publint` catch broken `exports`/types mappings before publishing.[^17]

***

#### Observability

- **Source maps**: compile with `sourceMap: true` (or `inlineSourceMap`) and run Node.js with `--enable-source-maps` (or `process.setSourceMapsEnabled(true)`), so stack traces point at `.ts` lines. Under type stripping, Node.js keeps line/column positions (types are replaced by whitespace), so stack traces already match the source. Upload source maps to error trackers; do not serve them publicly for proprietary front-end code unless intended.
- **Errors**: log `error.stack` and `error.cause` chains (`util.inspect(err, { depth: null })` prints causes), not just `message`. `AggregateError.errors` and `SuppressedError.suppressed` carry the rest.
- **Structured logging**: use a JSON logger (pino, winston) with typed log fields; log message templates and fields separately (`logger.info({ userId }, "user created")`), never interpolated secrets.
- **Tracing/metrics**: OpenTelemetry's JS SDK provides typed APIs; wrap async work in spans and propagate context through `AsyncLocalStorage` (Node.js). Make sure spans are ended in `finally` blocks (or via `using` with a disposable span wrapper).
- **Unhandled errors**: install `process.on("unhandledRejection")`/`("uncaughtException")` handlers that log and exit — do not keep running in an unknown state.

***

#### Declaration Files, Ambient Types and Publishing

##### Writing `.d.ts` and augmentations

```ts
// globals.d.ts-style augmentation inside a module file
export {};

declare global {
  interface Window {
    __APP_VERSION__: string;
  }
  namespace NodeJS {
    interface ProcessEnv {
      NODE_ENV?: "development" | "production" | "test";
    }
  }
}

// Module augmentation: add a field to a third-party interface
declare module "node:http" {
  interface IncomingMessage {
    requestId?: string;
  }
}
console.log(window.__APP_VERSION__, process.env.NODE_ENV);
```

- Typing `ProcessEnv` as required strings (`DATABASE_URL: string`) is a lie unless startup code validates the environment; keep them optional or validate.
- `declare module "some-lib";` (shorthand ambient module) makes the whole module `any`. Prefer installing `@types/some-lib` or writing minimal real declarations.
- A `.d.ts` file with no top-level `import`/`export` is a *global script*: everything in it is global. Adding one `import` turns it into a module and silently removes the globals — use `declare global {}` explicitly.
- `/// <reference types="…" />` in published `.d.ts` files forces that types package on all consumers.

##### Publishing libraries

- Ship `.js` + `.d.ts` (+ `.d.ts.map` for go-to-source); never ship `.ts` sources as the entry point (Node.js refuses to strip types under `node_modules`).
- `declaration: true`; check the emitted declarations with the lowest TypeScript version you support — declarations emitted by 5.8 may use syntax older compilers do not understand (e.g. the preserved computed property names described above, or `NoInfer`, `const` type parameters).
- Use `"exports"` with `"types"` first in each condition; set `"type"` explicitly; prefer ESM-only packages when consumers are on Node.js ≥ 20.19/22.12 (they can `require()` them) and on bundlers.
- `typesVersions` can route older TypeScript consumers to downlevelled declarations (`downlevel-dts`).
- Mark peer dependencies for frameworks; put `@types/*` that appear in your public `.d.ts` in `dependencies`, not `devDependencies`.

##### JSX / TSX

- `"jsx": "react-jsx"` (automatic runtime, React 17+) — no `import React` needed; `"preserve"` when the bundler handles JSX; `jsxImportSource` for Preact/Solid/Emotion.
- Generic arrow components need `<T,>` in `.tsx`.
- Type props with interfaces; `React.FC` is optional and no longer implies `children`. Event handlers: `React.ChangeEvent<HTMLInputElement>` etc.
- `.tsx` cannot be executed by Node.js type stripping.

***

#### Common Bugs a Reviewer Must Catch

These compile cleanly under `strict` on 5.8.3 and are wrong at runtime (outputs verified on Node.js 22).

```ts
// 1. Months are 0-based; date-only ISO strings are parsed as UTC, date-time strings as local time
const d = new Date(2025, 1, 30); // March 2, 2025 — no error, silently rolls over
const utc = new Date("2025-03-01"); // midnight UTC
const local = new Date("2025-03-01T00:00"); // midnight local time
console.log(d.getMonth(), utc.getTime() === local.getTime()); // 2, false unless TZ=UTC

// 2. Floating point money
console.log(0.1 + 0.2 === 0.3); // false — use integer cents or a decimal library

// 3. parseInt without radix / Number on empty strings
console.log(Number(""), Number(" "), Number.parseInt("08"), Number.parseInt("1e3")); // 0 0 8 1

// 4. JSON.stringify drops/ mangles values
console.log(JSON.stringify({ m: new Map([["a", 1]]), u: undefined, d: new Date(0) }));
// {"m":{},"d":"1970-01-01T00:00:00.000Z"} — Map lost, undefined dropped, Date became a string

// 5. Sorting numbers without a comparator
console.log([10, 9, 1].sort()); // [1, 10, 9]

// 6. typeof NaN / Array checks
console.log(typeof Number.NaN, typeof [], Array.isArray([])); // number object true

// 7. Object identity in Set/Map/includes
console.log(new Set([{ id: 1 }]).has({ id: 1 })); // false
```

```ts
// 8. Comparing different literal types is caught; comparing a string to a number from `any` is not
declare const fromApi: any;
const httpStatus: number = 200;
if (fromApi.status === "200") console.log("never true if the API sends a number");
if (httpStatus === "200") console.log("x"); // error TS2367 — no overlap

// 9. BigInt cannot mix with number
const big = 10n;
const mixed = big + 1; // error TS2365
console.log(big + 1n, mixed);

// 10. String(obj) / template literals on objects
const user = { id: 1 };
console.log(`user: ${user}`); // "user: [object Object]" — lint: restrict-template-expressions
```

More patterns to flag:

- `array.length` checks after `filter` on possibly-`undefined` elements without a type guard.
- `Object.entries(record)` returning `[string, T][]` used with an enum-typed key without validation.
- Mutating function parameters (arrays/objects) that callers still hold; mutating React/Redux state.
- `await` missing on a returned promise inside `try` (`return fetchX()` — the `catch` never sees the rejection).
- Event listeners, intervals and subscriptions without cleanup (`removeEventListener`, `clearInterval`, `AbortController` signals on `addEventListener`).
- `setTimeout(fn, ms)` with `ms` > 2³¹−1 (≈ 24.8 days) fires immediately (Node.js warns `TimeoutOverflowWarning` and uses 1 ms).
- `structuredClone`/`postMessage` of class instances (prototype and methods are lost) or functions (throws `DataCloneError`).
- Locale-sensitive operations without explicit locale: `toLowerCase()` vs `toLocaleLowerCase("tr")`, `toLocaleString()` in logs, `localeCompare` without `Intl.Collator` options.
- `string.length` counting UTF-16 code units, not characters (`"👍".length === 2`); use `[...s].length` or `Intl.Segmenter` for user-visible lengths.

***

#### Removed / Changed in 5.8 — Migration Notes (5.7 → 5.8)

Upgrading from 5.7 to 5.8 is low-risk; expect a handful of new, mostly legitimate errors.

1. **New TS2322 errors in `return cond ? a : b`** where one branch is `any`. Each is a real type mismatch; fix the value or add a narrow, justified assertion on the `any` branch.
2. **`--module nodenext` projects**:
   - `import … assert { … }` is now TS2880 — switch to `with { … }` (Node.js 22+ rejects `assert` at runtime anyway).
   - `require()`/`import x = require()` of ESM from CommonJS no longer errors. If the package must support Node.js < 20.19/22.12, change to `--module node18` or `node16` so the compiler keeps rejecting it.
   - Required ES modules must not use top-level `await` anywhere in their graph; the compiler does not check this.
3. **`--module node18`** is available for projects pinned to Node.js 18 semantics. Node.js 18 reached end-of-life on April 30, 2025 and Node.js 20 on April 30, 2026; Node.js 22 (maintenance LTS until April 2027), 24 (LTS) and 26 (Current, LTS from October 28, 2026) are the supported lines as of October 2026.[^21]
4. **Declaration emit**: non-literal computed class property names (`[propName]`) are preserved; they were TS1166 errors in 5.7. Check generated `.d.ts` against older consumer TypeScript versions if you support them.
5. **DOM lib** regenerated — fix new DOM typing errors rather than pinning old libs. If `@types/node` 26.x is installed alongside the DOM lib, expect TS2430 in `lib.dom.d.ts` without `skipLibCheck` (use the `@types/node` major matching your Node.js runtime).
6. **New opt-in flags** to consider: `--erasableSyntaxOnly` (if the code runs or may run under Node.js type stripping, or simply to keep TypeScript's emit trivial), `--libReplacement false` (performance; becomes the default in 6.0).
7. **Use 5.8.3**, not 5.8.2 (declaration-emit fixes; complete `--erasableSyntaxOnly`).

Preparing for later upgrades from a 5.8 codebase: avoid everything deprecated in 6.0 (see [Do Not Use or Recommend](#do-not-use-or-recommend-59-and-later)), set `types`, `rootDir`, `strict`, `module` and `target` explicitly (their 6.0 defaults differ), replace `baseUrl`-based imports with relative `paths` or `package.json` `"imports"`, migrate `moduleResolution: node10` to `nodenext` or `bundler`, and replace `assert` import assertions.

***

#### Production / Review Checklist

##### TypeScript 5.8-specific

- [ ] `typescript` pinned to `~5.8.3` (not 5.8.2, not a floating `^5`); CI runs the locked version (`npx tsc`)
- [ ] No 5.9+ features: no `import defer`, `--module node20`, `es2025` target/lib, `Temporal`, `RegExp.escape`, `Map#getOrInsert`, `Error.isError`, `#/` subpath imports, `--stableTypeOrdering`, `bundler` + `commonjs`
- [ ] `--module nodenext` only where the runtime is Node.js ≥ 20.19/22.12; libraries supporting older Node.js use `node16`/`node18`
- [ ] No `require()` of ES modules that use top-level `await` (runtime `ERR_REQUIRE_ASYNC_MODULE`, not caught by tsc)
- [ ] Import attributes (`with`) instead of import assertions (`assert`), also under `esnext`/`preserve`
- [ ] Code run by Node.js type stripping compiles with `erasableSyntaxOnly` + `verbatimModuleSyntax` (+ `rewriteRelativeImportExtensions` or `allowImportingTsExtensions`); no decorators, `.tsx`, or `paths` there
- [ ] New `return cond ? a : b` errors fixed, not cast away; intermediate variables of type `any` in returns reviewed manually
- [ ] `target`/`lib` match the oldest deployed runtime; `esnext` lib not used on Node.js 22 for `Promise.try`, `Float16Array`, `DisposableStack`; `using` not emitted natively for Node.js 22 (`target` below `esnext`)
- [ ] `@types/node` major matches the deployed Node.js major

##### Configuration

- [ ] `strict: true` plus `noUncheckedIndexedAccess`, `noImplicitOverride`, `noFallthroughCasesInSwitch`, `noImplicitReturns` (and `exactOptionalPropertyTypes` for new code)
- [ ] Explicit `target`, `module`, `lib`, `types`, `rootDir`, `outDir`
- [ ] `verbatimModuleSyntax` + `isolatedModules` when any single-file transpiler is involved
- [ ] `noUncheckedSideEffectImports` with asset declarations (`declare module "*.css" {}`)
- [ ] `skipLibCheck` decision documented; generated `.d.ts` of libraries checked separately
- [ ] No removed (5.5) options; no 6.0-deprecated options in new code (`baseUrl`, `node10`, `es5`, `outFile`, `amd`/`umd`/`system`, `downlevelIteration`)

##### Type safety

- [ ] No unexplained `any`, `as`, `!`, `!:`, `@ts-ignore`; `@ts-expect-error` has a reason
- [ ] External data (`JSON.parse`, `fetch`, env, DB, queues) parsed from `unknown` with validation; validated objects rebuilt from known fields
- [ ] Type predicates and assertion functions check everything they claim
- [ ] Exhaustive `switch` over unions (`never` check or lint rule)
- [ ] `satisfies` for config/lookup objects instead of `as` or widening annotations
- [ ] Callback members use property syntax (no bivariant method-signature holes)
- [ ] No numeric enums used with untrusted numbers; `as const` objects preferred; no `const enum` in libraries

##### Async and errors

- [ ] No floating promises; no `forEach(async …)`; `Promise.all` bounded for large inputs
- [ ] Timeouts and cancellation via `AbortSignal`; `return await` inside `try`
- [ ] `catch (e)` narrowed before use; errors wrapped with `{ cause }`; only `Error`s thrown
- [ ] Shared state across `await` points protected; CPU-heavy work off the event loop

##### Security

- [ ] Parameterized SQL; `execFile` with argument arrays; no `eval`/`new Function` on input
- [ ] Path containment checked after `path.resolve`; URL protocol/host allowlists
- [ ] No recursive merges of untrusted objects without `__proto__`/`constructor`/`prototype` filtering; `Map` or `Object.create(null)` for user-keyed data; `Object.hasOwn`
- [ ] User input escaped before `new RegExp` (no `RegExp.escape` in 5.8); no catastrophic-backtracking patterns
- [ ] `crypto.randomBytes`/`randomUUID` for tokens; `timingSafeEqual` for secret comparison; secrets redacted from logs

##### Modules and packaging

- [ ] ESM relative imports include extensions (`./x.js` or `./x.ts` with rewriting); no directory imports
- [ ] `paths` mirrored by the runtime (bundler, `"imports"`); no `baseUrl` bare-folder imports
- [ ] Packages: `exports` with `types` first, correct per-format declarations, `attw --pack` clean
- [ ] JSON imports use `with { type: "json" }` under `node18`/`nodenext`

##### Testing, tooling and observability

- [ ] `tsc --noEmit`/`tsc -b` in CI in addition to the (non-type-checking) test runner
- [ ] typescript-eslint type-checked config with `no-floating-promises`, `no-misused-promises`, `no-unsafe-*`, `switch-exhaustiveness-check`
- [ ] Type-level tests for public generic APIs
- [ ] Source maps enabled in production stack traces (`--enable-source-maps`); error causes logged
- [ ] Editor uses the workspace TypeScript version

***

[^1]: TypeScript 5.8 Release Notes. https://www.typescriptlang.org/docs/handbook/release-notes/typescript-5-8.html
[^2]: npm registry, `typescript` package versions, publish times and dist-tags (5.8.2 2025-02-28, 5.8.3 2025-04-05, 5.9.2 2025-07-31, 5.9.3 2025-09-30, 6.0.2 2026-03-23, 6.0.3 2026-04-16, 7.0.2 2026-07-08, 7.1.0-beta 2026-10-20). https://www.npmjs.com/package/typescript?activeTab=versions
[^3]: DefinitelyTyped README — Support Window. https://github.com/DefinitelyTyped/DefinitelyTyped#support-window
[^4]: Announcing TypeScript 5.8 RC ("we have had to pull back some work on how functions with conditional return types are checked"). https://devblogs.microsoft.com/typescript/announcing-typescript-5-8-rc/
[^5]: TypeScript 5.4, 5.5 and 5.6 Release Notes (NoInfer, closure narrowing, inferred type predicates, regex checking, isolatedDeclarations, disallowed truthy/nullish checks, iterator helpers, strictBuiltinIteratorReturn, noUncheckedSideEffectImports, noCheck). https://www.typescriptlang.org/docs/handbook/release-notes/typescript-5-6.html
[^6]: TypeScript 5.7 Release Notes (never-initialized variables, rewriteRelativeImportExtensions, es2024, generic TypedArrays, validated JSON imports). https://www.typescriptlang.org/docs/handbook/release-notes/typescript-5-7.html
[^7]: microsoft/TypeScript PR #56941 — check return expressions branch by branch. https://github.com/microsoft/TypeScript/pull/56941
[^8]: Node.js documentation — Modules: CommonJS, "Loading ECMAScript modules using require()". https://nodejs.org/api/modules.html#loading-ecmascript-modules-using-require
[^9]: microsoft/TypeScript PR #60761 — require(esm) and import-assertion changes under nodenext. https://github.com/microsoft/TypeScript/pull/60761
[^10]: microsoft/TypeScript PR #60722 — `--module node18`. https://github.com/microsoft/TypeScript/pull/60722
[^11]: TypeScript 6.0 Release Notes (new defaults, deprecations, es2025, Temporal, getOrInsert, RegExp.escape, `#/` imports, stableTypeOrdering). https://www.typescriptlang.org/docs/handbook/release-notes/typescript-6-0.html
[^12]: Node.js documentation — Modules: TypeScript (type stripping history, unsupported syntax, recommended tsconfig). https://nodejs.org/api/typescript.html
[^13]: microsoft/TypeScript PR #61011 — `--erasableSyntaxOnly`. https://github.com/microsoft/TypeScript/pull/61011
[^14]: microsoft/TypeScript v5.8.2...v5.8.3 (PRs #61244, #61452, #61392, #61342, #61261). https://github.com/microsoft/TypeScript/compare/v5.8.2...v5.8.3
[^15]: TypeScript 5.9 Release Notes (import defer, node20, tsc --init, lib.d.ts ArrayBuffer change, inference changes). https://www.typescriptlang.org/docs/handbook/release-notes/typescript-5-9.html
[^16]: TypeScript 5.0 Release Notes — Deprecations and Default Changes; TypeScript 5.5 Release Notes — Disabling Features Deprecated in TypeScript 5.0. https://www.typescriptlang.org/docs/handbook/release-notes/typescript-5-5.html
[^17]: Are The Types Wrong? (`@arethetypeswrong/cli`). https://github.com/arethetypeswrong/arethetypeswrong.github.io
[^18]: TypeScript 5.0 Release Notes — Decorators. https://www.typescriptlang.org/docs/handbook/release-notes/typescript-5-0.html
[^19]: Zod documentation — Requirements. https://zod.dev/
[^20]: typescript-eslint — Dependency Versions. https://typescript-eslint.io/users/dependency-versions/
[^21]: Node.js Release Working Group schedule. https://github.com/nodejs/Release/blob/main/schedule.json
[^22]: TSConfig Reference. https://www.typescriptlang.org/tsconfig/
[^23]: A 10x Faster TypeScript (announcement of the native port). https://devblogs.microsoft.com/typescript/typescript-native-port/
