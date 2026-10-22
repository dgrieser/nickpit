### TypeScript 5.0: The Complete Production Developer Guide

#### Overview

> **Version**: TypeScript 5.0. The stable release **5.0.2** was published to npm on March 16, 2023, the day of the announcement (5.0.0-beta: January 26, 2023; 5.0.1-rc: March 1, 2023).[^1][^2] The latest and **final** patch release is **5.0.4**, published on April 7, 2023 (5.0.3 followed on March 30, 2023).[^2][^3]
> **Status as of October 2026**: 5.0 is three and a half years old and receives no fixes; TypeScript has no LTS lines, so fixes ship only in new minor releases. The 5.x line ended with **5.9** (final patch 5.9.3, September 30, 2025). **TypeScript 6.0** (6.0.2, March 23, 2026; latest 6.0.3, April 16, 2026) is the last JavaScript-based compiler, and **TypeScript 7.0** (7.0.2, July 8, 2026), the native compiler written in Go, is the current `latest` on npm; 7.1 is in beta (October 20, 2026).[^2] DefinitelyTyped only supports TypeScript versions younger than two years, so new `@types/*` releases stopped declaring 5.0 support in spring 2025 (see [`@types` packages and TypeScript 5.0](#types-packages-and-typescript-50)).[^4]
> **Scope**: the TypeScript language and type system as usable in 5.0, compiler options and module resolution, the 5.0 delta over 4.9, the 5.0 deprecation set, idioms, pitfalls and a review checklist for production code.
> **Note**: this document is self-contained. Everything from earlier releases that 5.0 supports is carried forward. Sections that cover 5.0-specific behaviour are marked **New in 5.0**. All examples were compiled with `tsc` 5.0.4 (boundary claims also with 4.9.5 and the 5.1–5.7 compilers); the default checking configuration is `strict`, `noUncheckedIndexedAccess`, `noImplicitOverride`, `noImplicitReturns`, `verbatimModuleSyntax`, `target: es2022`, `module: esnext`, `moduleResolution: bundler`, `lib: ["es2023", "dom", "dom.iterable"]`, unless an example states otherwise with a `// @option: value` line such as `// @lib: es2022` (the convention the TypeScript playground uses; `// @filename:` starts another file of the same example). Lines marked `// error TSxxxx` are rejected by 5.0.4 with that diagnostic.

Global truths about TypeScript 5.0:

- **Types are erased.** TypeScript checks at compile time and emits JavaScript; annotations, interfaces and type aliases do not exist at runtime. A type assertion (`as`) never converts or validates a value. Data that crosses a trust boundary (HTTP, JSON, files, environment, `postMessage`) must be validated at runtime.
- **Structural typing.** Compatibility is decided by shape, not by declared name. Two unrelated interfaces with the same members are interchangeable.
- **Soundness is not a goal.** `any`, type assertions, non-null assertions (`!`), method parameter bivariance, unchecked index access (without `noUncheckedIndexedAccess`) and declaration files that lie are all holes. A reviewer must treat each one as an unverified claim.
- **Two decorator systems.** 5.0 implements the TC39 (stage 3) decorators by default and keeps the old `--experimentalDecorators` implementation behind the flag. They have different signatures, different emit and different capabilities; a decorator library is written for exactly one of them.
- **`tsc` defaults are old.** Without a tsconfig, 5.0 defaults to `target: es5`, `module: commonjs`, `moduleResolution: node10`, non-strict checking and automatic inclusion of every `@types/*` package. Real projects must set these explicitly.
- **Emit is not bundling.** `tsc` never rewrites import specifiers, never adds polyfills for library APIs, and never resolves `paths` aliases at runtime.
- **The compiler itself needs Node.js 12.20 or newer** (the 5.0 package targets ES2018; `engines.node` is `>=12.20`; 4.9 declared `>=4.2.0`).[^5] The 5.0 release notes say "at least Node.js 10"; the published `engines` field and the API Breaking Changes page (12.20) are authoritative.

***

#### What's New in TypeScript 5.0

TypeScript 5.0 is a large release: a new decorator implementation, a new module-resolution mode, a new import/export emit model, and the first large deprecation wave. It also cut the npm package size to 59% of 4.9 (the compiler was migrated from namespaces to ES modules internally) and reports 10–20% faster builds on most code bases.[^1]

| Change | Kind | Section |
|---|---|---|
| TC39 standard decorators (no flag needed); `ClassMethodDecoratorContext` & co. in `lib.decorators.d.ts`; decorators allowed after `export` | Syntax / emit | [Decorators](#new-in-50-decorators) |
| `const` modifier on type parameters | Type system | [`const` type parameters](#new-in-50-const-type-parameters) |
| `extends` in tsconfig accepts an array | Config | [Multiple `extends`](#new-in-50-multiple-configuration-files-in-extends) |
| All enums are union enums; out-of-domain literal assignments and mixed string/number indirections are errors | Type system (breaking) | [Enums](#new-in-50-all-enums-are-union-enums) |
| `--moduleResolution bundler` | Compiler option | [bundler](#new-in-50---moduleresolution-bundler) |
| `allowImportingTsExtensions`, `resolvePackageJsonExports`, `resolvePackageJsonImports`, `allowArbitraryExtensions`, `customConditions` | Compiler options | [Resolution flags](#new-in-50-resolution-customization-flags) |
| `moduleResolution: node10` (new name for `node`) | Compiler option | [Module modes](#module-and-moduleresolution-modes) |
| `--verbatimModuleSyntax` (replaces `importsNotUsedAsValues`/`preserveValueImports`) | Compiler option | [verbatimModuleSyntax](#new-in-50---verbatimmodulesyntax) |
| `export type * from "..."` and `export type * as ns from "..."` | Syntax | [`export type *`](#new-in-50-export-type-) |
| JSDoc `@satisfies` and `@overload` | JavaScript checking | [JavaScript projects](#javascript-projects-checkjs-and-jsdoc) |
| `--declaration`, `--emitDeclarationOnly`, `--declarationMap`, `--sourceMap`, `--inlineSourceMap` allowed with `tsc --build` | Tooling | [Builds](#tsc-and-builds) |
| `lib: es2023` (only `findLast`/`findLastIndex`); `lib.decorators.d.ts`, `lib.decorators.legacy.d.ts` | Lib | [Target and lib](#target-and-lib) |
| Relational operators (`<`, `>`, `<=`, `>=`) reject `number \| string` operands | Checker (breaking) | [Relational coercion](#new-in-50-forbidden-implicit-coercions-in-relational-operators) |
| Stricter checking of constructor-parameter decorators under `--experimentalDecorators` | Checker (breaking) | [Legacy decorators](#legacy---experimentaldecorators) |
| `newLine` defaults to `lf`; `forceConsistentCasingInFileNames` defaults to `true`; default `target` is `es5` (was `es3`) | Default changes | [Default changes](#new-in-50-default-changes) |
| Deprecation of `target: es3`, `out`, `noImplicitUseStrict`, `keyofStringsOnly`, `suppressExcessPropertyErrors`, `suppressImplicitAnyIndexErrors`, `noStrictGenericChecks`, `charset`, `importsNotUsedAsValues`, `preserveValueImports`, `prepend`; new `ignoreDeprecations` | Config (breaking) | [Deprecations](#new-in-50-deprecations-and-ignoredeprecations) |
| Case-insensitive import sorting auto-detection, exhaustive `switch`/`case` completions | Editor | [Editor](#editor-and-language-service) |
| Compiler requires ES2018 / Node.js 12.20; API package restructured (`typescriptServices.js` and `protocol.d.ts` removed) | Packaging (breaking) | [Tooling](#tooling) |
| DOM `lib.d.ts` regenerated (some `number` properties became literal unions, clipboard events moved) | Lib (breaking) | [Migration notes](#removed--changed-in-50--migration-notes) |

Everything earlier remains available and should be used where it fits: optional chaining and `??` (3.7); `import type` (3.8); variadic tuple types and labeled tuple elements (4.0); template literal types and key remapping in mapped types (4.1); `abstract` construct signatures (4.2); `override` and `noImplicitOverride` (4.3); `useUnknownInCatchVariables`, `exactOptionalPropertyTypes`, control-flow analysis of aliased conditions and destructured discriminated unions (4.4); `Awaited<T>` and `type` modifiers on import names (4.5); `module: node16`/`nodenext`, `in`/`out` variance annotations, instantiation expressions and `extends` constraints on `infer` (4.7); improved intersection and `unknown` narrowing, `infer` on template literal types (4.8); the `satisfies` operator, the `accessor` keyword, unlisted-property narrowing with `in` and `NaN` equality checks (4.9).[^6]

##### Do not use or recommend (newer than 5.0)

The project's compiler is 5.0. **Never suggest any of the following** — 5.0.4 rejects them, ignores them, or types them differently (each item was checked on 5.0.4 and on the release that introduced it):

- **TypeScript 5.1**: functions returning `undefined` without a `return` statement (`function f(): undefined {}` is TS2355 in 5.0), unrelated getter/setter types (`get x(): string` with `set x(v: number)` is TS2380 in 5.0), namespaced JSX attributes (`<a:b />`), decoupled JSX element/tag type checking (`JSX.ElementType`), `typeRoots` consulted during module resolution. 5.1 also raised the compiler's runtime floor to Node.js 14.17 / ES2020.[^7]
- **TypeScript 5.2**: `using` / `await using` declarations, `Symbol.dispose`, `Disposable`, `DisposableStack`, `SuppressedError`; decorator metadata (`context.metadata`, `Symbol.metadata` — `metadata` does not exist on the 5.0 context types); mixing named and anonymous tuple elements (`[first: string, number]` is TS5084 in 5.0); calling `filter`/`find`/`some`/`every`/`reduce` on a union of array types (`string[] | number[]` — TS2349 "not callable" in 5.0); copying array methods `toSorted`, `toReversed`, `toSpliced`, `with` (lib es2023 in 5.2); type-only imports with `.ts` extensions without `allowImportingTsExtensions`.[^8]
- **TypeScript 5.3**: import attributes (`import x from "./a.json" with { type: "json" }` — a syntax error in 5.0; 5.0 only knows `assert { ... }`), stable `resolution-mode` in import types for every module mode, `switch (true)` narrowing, narrowing on comparisons to `true`/`false`, `instanceof` narrowing through `Symbol.hasInstance`, interactive inlay hints and the "prefer `type` auto-imports" editor setting.[^9]
- **TypeScript 5.4**: `NoInfer<T>` (TS2304 *Cannot find name* in 5.0), preserved narrowing in closures after the last assignment (5.0 still reports TS18048 inside the closure), `Object.groupBy`/`Map.groupBy` types, `Promise.withResolvers` types, `module: preserve`, `require()` in `bundler` resolution.[^10]
- **TypeScript 5.5**: inferred type predicates (`[1, undefined].filter(x => x !== undefined)` is still `(number | undefined)[]` in 5.0), narrowing of constant indexed accesses `obj[key]`, JSDoc `@import`, regular-expression syntax checking, `isolatedDeclarations`, `${configDir}` in tsconfig, `Set` methods (`union`, `intersection`, …) types, `Array.fromAsync`, `String.prototype.isWellFormed`, `AbortSignal.any` types, and **`target: es2023`** (5.0 accepts `lib: es2023` but rejects `target: es2023` with TS6046).[^11]
- **TypeScript 5.6–5.9**: errors on always-truthy/always-nullish conditions (5.6), `--noUncheckedSideEffectImports`, `--noCheck`, `--strictBuiltinIteratorReturn`, iterator-helper types (5.6); never-initialized-variable checks, `--rewriteRelativeImportExtensions`, `target: es2024` (5.7); `--module node18`, `--erasableSyntaxOnly`, `--libReplacement`, `require()` of ESM under `nodenext` (5.8); `import defer`, `--module node20`, the minimal `tsc --init` (5.9).[^12]
- **TypeScript 6.0 / 7.0**: `strict` on by default, `types: []` by default, `es2025` target/lib, the `Temporal` types, the deprecation (6.0, error unless `"ignoreDeprecations": "6.0"`) and removal (7.0) of `baseUrl`, `moduleResolution: node10` and `target: es5`, the native Go compiler (`tsgo`) and its LSP. Do not apply their defaults to a 5.0 project — a 5.0 tsconfig must spell every setting out.[^2]

Deprecated or removed **older** features that 5.0 reports:

- **Deprecated in 5.0** (TS5101/TS5107 *"is deprecated and will stop functioning in TypeScript 5.5"*, reported as an **error** that makes `tsc` exit with code 2 although output is still emitted; silenced with `"ignoreDeprecations": "5.0"`): `target: es3`, `out`, `noImplicitUseStrict`, `keyofStringsOnly`, `suppressExcessPropertyErrors`, `suppressImplicitAnyIndexErrors`, `noStrictGenericChecks`, `charset`, `importsNotUsedAsValues` (any value except `remove`), `preserveValueImports`, `prepend` in project references. 5.5 still accepts them with `"ignoreDeprecations": "5.0"` but silently ignores them (verified: `suppressExcessPropertyErrors` no longer suppresses TS2353); 5.6 and later reject them as removed (TS5102/TS5108). Details in [Deprecations](#new-in-50-deprecations-and-ignoredeprecations).
- `typingOptions` and `enableAutoDiscovery` in tsconfig/jsconfig are no longer supported (use `typeAcquisition`).[^13]
- The `typescriptServices.js` bundle and `protocol.d.ts` are gone from the npm package; the compiler API's `ts.Map`, `ts.Set`, `ts.ESMap`, `ts.Iterator`, `ts.Collection` types and the old top-level node factory functions were removed.[^13]

##### Version-aware review

- **Check the pinned version first.** `package.json` (`"typescript": "~5.0.4"` or `"5.0.4"`), the lockfile and `npx tsc -v` decide what is available. A caret range `^5.0.0` silently resolves to **5.9.3** — not to 5.0 — so a project that wants 5.0 semantics must use `~5.0.4` or an exact pin; a bare `latest` resolves to the native 7.0 compiler.
- **Verify against 5.0 before claiming something is missing.** Many APIs exist in 5.0 only under the right `lib` (`Array.prototype.at`, `Object.hasOwn`, `Error` `cause`, RegExp `d` flag → `es2022`; `findLast`/`findLastIndex` → `es2023`; `structuredClone`, `AbortSignal.timeout` → `dom`). Say "not available in 5.0" only when the 5.0 lib or checker proves it.
- **Distinguish type availability from runtime availability.** `lib` only declares types; `findLast` type-checks with `lib: es2023` but needs Node.js 18+ at runtime. Match `lib` to the oldest runtime the code supports.
- **Do not "upgrade" to newer idioms in review comments.** No `using`, `NoInfer`, reliance on inferred type predicates, no `with { type: "json" }`, no `target: es2023`. Prefer changes that are valid in 5.0 *and* later (e.g. replacing deprecated options with `verbatimModuleSyntax`, removing `importsNotUsedAsValues`, switching to `moduleResolution: bundler` or `node16`).
- **Recommend the upgrade path when it matters.** 5.0 is unsupported; every fix since April 2023 (including 5.0.x regressions listed in [5.0.x patch releases](#50x-patch-releases-and-known-issues)) only exists in later versions. Flag new projects that pin 5.0 without a reason.

***

#### Compiler Configuration (tsconfig.json)

##### Recommended baseline

For an application compiled or bundled by a modern bundler (Vite, esbuild, webpack, Parcel, Rollup):

```jsonc
{
  "compilerOptions": {
    "target": "es2022",
    "lib": ["es2023", "dom", "dom.iterable"],
    "module": "esnext",
    "moduleResolution": "bundler",          // New in 5.0
    "verbatimModuleSyntax": true,           // New in 5.0
    "noEmit": true,                         // the bundler emits
    "allowImportingTsExtensions": true,     // optional; requires noEmit or emitDeclarationOnly
    "resolveJsonModule": true,
    "isolatedModules": true,                // implied checks are already part of verbatimModuleSyntax
    "strict": true,
    "noUncheckedIndexedAccess": true,
    "noImplicitOverride": true,
    "noImplicitReturns": true,
    "noFallthroughCasesInSwitch": true,
    "exactOptionalPropertyTypes": true,
    "useUnknownInCatchVariables": true,     // already part of strict; keep explicit for readers
    "forceConsistentCasingInFileNames": true, // default true since 5.0
    "skipLibCheck": true,
    "types": []                             // opt in to @types packages explicitly
  },
  "include": ["src"]
}
```

For a Node.js service or a library published to npm, use the Node resolution modes, which model what Node.js actually does (extensions required in ESM, `exports`, `type: "module"`):

```jsonc
{
  "compilerOptions": {
    "target": "es2022",
    "lib": ["es2023"],
    "module": "node16",            // or "nodenext"; both exist since 4.7
    "moduleResolution": "node16",  // implied by module: node16 (spell it out for readers)
    "verbatimModuleSyntax": true,
    "outDir": "dist",
    "rootDir": "src",
    "declaration": true,
    "declarationMap": true,
    "sourceMap": true,
    "strict": true,
    "noUncheckedIndexedAccess": true,
    "noImplicitOverride": true,
    "noImplicitReturns": true,
    "skipLibCheck": true,
    "types": ["node"]
  },
  "include": ["src"]
}
```

The Node.js configuration assumes `"type": "module"` in `package.json`. Without it every `.ts` file is CommonJS under `node16`, and `verbatimModuleSyntax` reports TS1287 on every top-level `export` (verified) — which is the intended early warning.

Review notes:

- `isolatedModules: true` together with `verbatimModuleSyntax: true` is harmless in 5.0.4; in 5.0.2/5.0.3 it was reported as an error (fixed in 5.0.4, #53601).[^3]
- `verbatimModuleSyntax` with `module: commonjs` (or `node16` files that are CommonJS) rejects every ESM `import`/`export` (TS1286/TS1287). Use it in ESM projects; see [verbatimModuleSyntax](#new-in-50---verbatimmodulesyntax).
- `skipLibCheck: true` is the norm, and for 5.0 it is close to mandatory: recent `@types/node` releases do not type-check under 5.0 (see [`@types` packages](#types-packages-and-typescript-50)).
- `moduleResolution: bundler` **requires** `module: es2015` or later in 5.0 (TS5095 with `commonjs`); `module: preserve` does not exist until 5.4.
- 5.0 silently accepts `moduleResolution: node16`/`nodenext` with a different `module` (e.g. `module: esnext`), which models neither Node.js nor a bundler correctly; 5.2+ reject the mismatch (TS5110 *"Option 'module' must be set to 'Node16' when option 'moduleResolution' is set to 'Node16'"*). Flag it now so the upgrade does not break.[^8]

##### New in 5.0: Multiple configuration files in `extends`

`extends` accepts an array. The entries are applied in order, later entries override earlier ones, and the file's own `compilerOptions` override all of them.[^1] 4.9 rejects an array with TS5024.

```jsonc
// tsconfig.json
{
  "extends": ["@tsconfig/strictest/tsconfig.json", "./tsconfig.base.json"],
  "compilerOptions": { "outDir": "dist" },
  "include": ["src"]
}
```

Verified behaviour: with `t1.json` = `{ "strictNullChecks": true }` and `t2.json` = `{ "noImplicitAny": true, "strictNullChecks": false }`, `"extends": ["./t1.json", "./t2.json"]` produces `strictNullChecks: false` — the **last** entry wins. Review rule: order shared bases from general to specific; run `tsc --showConfig` when in doubt. 5.0.3 fixed a regression where `extends` stopped resolving packages from `node_modules` (#53314) and added support for wildcard `exports` in tsconfig lookups (#53443).[^3]

##### Strictness flags that `strict` does not enable

`strict` enables `noImplicitAny`, `strictNullChecks`, `strictFunctionTypes`, `strictBindCallApply`, `strictPropertyInitialization`, `noImplicitThis`, `useUnknownInCatchVariables` and `alwaysStrict`. Turn these on as well in new code:

| Flag | Catches |
|---|---|
| `noUncheckedIndexedAccess` | `arr[i]` and `record[key]` typed as `T \| undefined` |
| `exactOptionalPropertyTypes` | `{ a?: string }` no longer accepts an explicit `undefined` |
| `noImplicitOverride` | subclass methods that override without `override` |
| `noImplicitReturns` | code paths that fall off the end of a value-returning function |
| `noFallthroughCasesInSwitch` | non-empty `case` falling through |
| `noPropertyAccessFromIndexSignature` | forces `obj["key"]` for index-signature properties |
| `noUnusedLocals` / `noUnusedParameters` | dead code (or leave to the linter) |

```ts
// @noUncheckedIndexedAccess: true
const ports: number[] = [80, 443];
const first: number = ports[0]; // error TS2322
const safe = ports[0] ?? 8080;  // number

const env: Record<string, string> = {};
env.HOME.toUpperCase(); // error TS18048
export { first, safe };
```

##### Optional properties and `exactOptionalPropertyTypes`

```ts
// @exactOptionalPropertyTypes: true
interface Options {
  timeoutMs?: number;
}
const a: Options = {};                     // OK: property absent
const b: Options = { timeoutMs: undefined }; // error TS2375
const c: Options = { ...(Math.random() > 0.5 ? { timeoutMs: 5 } : {}) }; // OK

interface Patch {
  name?: string | undefined; // say so explicitly when "present but undefined" is meaningful
}
const p: Patch = { name: undefined }; // OK
export { a, b, c, p };
```

Spreading user objects into option bags often produces explicit `undefined`s; this flag finds them. It is off by default even under `strict`.

##### Target and lib

`target` controls syntax downlevelling and `lib` the declared globals. In 5.0 the accepted targets are `es3` (deprecated), `es5`, `es6`/`es2015` … `es2022` and `esnext` — **`es2023` is not a valid `target` until 5.5** (TS6046), while **`lib: ["es2023"]` is new in 5.0** and contains only `Array.prototype.findLast` and `findLastIndex` (and their typed-array forms).[^14] 4.9 knows neither the `es2023` lib nor `findLast` (not even under `esnext`).

```ts
// @lib: es2022
const xs = [1, 2, 3];
xs.findLast((x: number) => x > 1); // error TS2550
xs.at(-1);                 // es2022: OK
Object.hasOwn({ a: 1 }, "a"); // es2022: OK
export {};
```

```ts
// @lib: es2023
const last = [1, 2, 3].findLast((x) => x > 1); // number | undefined
const sorted = [3, 1].toSorted(); // error TS2339
export { last, sorted };
```

Mapping to runtimes (match the *oldest* runtime you support):

| Runtime | `target` | `lib` |
|---|---|---|
| Node.js 16 | `es2021` | `["es2021"]` (Node 16.6+ has `.at()`; add `es2022` pieces only if you require them) |
| Node.js 18 | `es2022` | `["es2023"]` (`findLast` ships in Node 18) |
| Node.js 20+ | `es2022` | `["es2023"]` (newer built-ins have no 5.0 types — see *Do not use*) |
| Evergreen browsers | `es2022` | `["es2023", "dom", "dom.iterable"]` |

Review rules:

- **Never use `target: esnext` for code that uses standard decorators.** 5.0 leaves decorator syntax untouched for `esnext` (it assumes native support), and no Node.js release, including 26, can parse the output (`SyntaxError`). Use `es2022` or lower, where 5.0 emits the `__esDecorate`/`__runInitializers` helpers. Verified on Node.js 18 and 26.
- `target` ≥ `es2022` turns on `useDefineForClassFields` by default (see [Classes](#field-semantics-usedefineforclassfields)).
- `"lib": ["dom"]` in 5.0 does **not** include `dom.iterable`; `for (const el of document.querySelectorAll("a"))` needs `"dom.iterable"` (or `downlevelIteration` for ES5).
- `lib` is not a polyfill. Adding `es2023` to code that must run on Node.js 16 type-checks `findLast` but throws at runtime.

##### New in 5.0: Default changes

- `newLine` now defaults to `lf` on every platform; the old OS-dependent inference (`crlf` on Windows) is gone and cannot be selected.[^1] (Not reproducible on the Linux test machine, where both 4.9 and 5.0 emit LF.)
- `forceConsistentCasingInFileNames` now defaults to `true`, so `import "./Mod"` for `mod.ts` is an error on case-insensitive file systems too.[^1]
- The default `target` (no `target` set) is now `es5` instead of `es3`, a consequence of the ES3 deprecation that the release notes do not list. Verified: `export const o = { get x() { return 1; } }` without `--target` is TS1056 *"Accessors are only available when targeting ECMAScript 5 and higher"* in 4.9.5 and compiles in 5.0.4.

##### New in 5.0: Deprecations and `ignoreDeprecations`

5.0 deprecates the options below. They still work in 5.0–5.4, but `tsc` reports each one as an **error** (TS5101, or TS5107 for `target=ES3`), exits with code 2 and still emits output; CI pipelines that treat a non-zero exit as failure break immediately. `"ignoreDeprecations": "5.0"` silences them in 5.0–5.5. In 5.5 the options **no longer have any effect** (still accepted with `ignoreDeprecations`, otherwise TS5101), and from 5.6 on they are errors (TS5102 *"Option '…' has been removed"*, TS5108 for `target=ES3`); 6.0 keeps rejecting them (verified on 5.4, 5.5, 5.6 and 6.0).[^11] 4.9.5 also accepts `"ignoreDeprecations": "5.0"` to ease the upgrade; any other value (`"5.1"`) is TS5103 *Invalid value* in 5.0.[^1]

| Deprecated | Replacement |
|---|---|
| `target: es3` | `es5` or later |
| `out` | `outFile` (or a bundler) |
| `noImplicitUseStrict` | none; emit is strict |
| `keyofStringsOnly` | use `Extract<keyof T, string>` or `` `${...}` `` |
| `suppressExcessPropertyErrors` | fix the object literals |
| `suppressImplicitAnyIndexErrors` | add index signatures or narrow keys |
| `noStrictGenericChecks` | fix the generic signatures |
| `charset` | none (input is UTF-8) |
| `importsNotUsedAsValues` (`preserve`/`error`) | `verbatimModuleSyntax` |
| `preserveValueImports` | `verbatimModuleSyntax` |
| `prepend` in `references` | a bundler, or separate outputs |

`importsNotUsedAsValues: "remove"` (the old default) is not reported. Review rule: `ignoreDeprecations` is a **migration aid**, not a fix; every PR that adds it must also carry a plan to remove the deprecated option before moving past 5.4 (a 5.5 upgrade silently changes behaviour, a 5.6+ upgrade fails).

**5.0.4 crash:** combining `verbatimModuleSyntax` with `importsNotUsedAsValues: "error"` (with or without `ignoreDeprecations`, with or without `noEmit`) makes 5.0.4 throw `Debug Failure. False expression.` from `isReferencedAliasDeclaration` as soon as a file contains a regular import such as `import { b, type X } from "./x"` (a project with only `import type` statements gets the TS5104 diagnostic instead); 5.1–5.4 report TS5104 *"Option 'importsNotUsedAsValues' is redundant and cannot be specified with option 'verbatimModuleSyntax'"*. `preserveValueImports` plus `verbatimModuleSyntax` reports TS5104 normally. Remove the old option when introducing `verbatimModuleSyntax`.

***

#### Modules

##### Module and moduleResolution modes

| `moduleResolution` | When | Notes for 5.0 |
|---|---|---|
| `node10` (alias: `node`) | legacy CommonJS projects | **`node10` is new in 5.0**; 4.9 rejects it. Ignores `package.json` `exports`/`imports` by default |
| `node16` / `nodenext` | Node.js services, npm libraries | Implied by `module: node16`/`nodenext`; ESM files need explicit extensions (`./util.js`); respects `exports`, `imports`, `type` |
| `bundler` | code consumed by a bundler | **New in 5.0**; extensionless relative imports allowed, `exports`/`imports` honoured with the `import` condition, requires `module: es2015`+ |
| `classic` | never | pre-1.6 behaviour; still the implicit default for `module: es2015`…`esnext`, `amd`, `umd`, `system` when `moduleResolution` is unset |

The default for `moduleResolution` depends on `module`: `node10` for `commonjs`, `node16`/`nodenext` for those modes, otherwise `classic`. A tsconfig that sets `"module": "esnext"` but no `moduleResolution` silently gets `classic` and cannot find packages in `node_modules` (TS2792 *"Cannot find module … Did you mean to set the 'moduleResolution' option to 'nodenext'"*). Always set both.

##### New in 5.0: `--moduleResolution bundler`

`bundler` models the hybrid lookup of Vite, esbuild, webpack, Parcel and similar tools: extensionless and directory imports work like CommonJS, while `package.json` `exports`/`imports` are honoured with the `import` condition like ESM.[^1]

```ts
// @filename: node_modules/pkg/package.json
{"name":"pkg","version":"1.0.0","exports":{".":{"import":"./esm.js","require":"./cjs.js"},"./feature":"./feature.js"}}
// @filename: node_modules/pkg/esm.d.ts
export declare const esm: string;
// @filename: node_modules/pkg/feature.d.ts
export declare const feature: string;
// @filename: src/util.ts
export const util = 1;
// @filename: src/main.ts
import { esm } from "pkg";            // "import" condition picked
import { feature } from "pkg/feature";
import { util } from "./util";        // no extension needed
import { cjs } from "pkg/cjs";        // error TS2307
export const all = [esm, feature, util, cjs];
```

Review rules:

- **Applications behind a bundler**: `bundler` is correct and simpler than `node16`.
- **Libraries published to npm and Node.js programs run without a bundler**: use `node16`/`nodenext`. `bundler` hides missing file extensions and CJS/ESM mismatches that break consumers on plain Node.js.[^1]
- `bundler` cannot be combined with `module: commonjs` in 5.0 (TS5095). It works with `module: es2015`, `es2020`, `es2022`, `esnext`.

##### New in 5.0: Resolution customization flags

| Flag | Default | Effect |
|---|---|---|
| `allowImportingTsExtensions` | `false` | allows `import "./a.ts"` (`.mts`, `.tsx` too); only with `noEmit` or `emitDeclarationOnly` (TS5096 otherwise) |
| `resolvePackageJsonExports` | `true` under `node16`/`nodenext`/`bundler` | consult `exports` of packages in `node_modules` |
| `resolvePackageJsonImports` | same | resolve `#name` specifiers through the nearest `package.json` `imports` |
| `allowArbitraryExtensions` | `false` | allow `import styles from "./app.css"` when `app.d.css.ts` exists (TS6263 otherwise) |
| `customConditions` | `[]` | extra `exports`/`imports` conditions; only with `node16`/`nodenext`/`bundler` (TS5098 otherwise) |

```ts
// @allowImportingTsExtensions: true
// @filename: src/util.ts
export const util = 1;
// @filename: src/main.ts
import { util } from "./util.ts"; // OK only because noEmit is on
export const x = util;
```

```ts
// @filename: src/app.css
.banner { display: none; }
// @filename: src/app.d.css.ts
declare const css: { banner: string };
export default css;
// @filename: src/main.ts
import styles from "./app.css"; // error TS6263
export const b: string = styles.banner;
```

With `--allowArbitraryExtensions` the import above compiles; the declaration file must be named `app.d.css.ts` (the older `app.css.d.ts` convention only works through CommonJS-style lookup and fails in ESM under `node16`).[^1]

`customConditions` example — the condition is checked *before* `import`/`require` only if it appears first in the package's `exports` object (object key order decides):

```jsonc
{ "compilerOptions": { "moduleResolution": "bundler", "customConditions": ["development"] } }
```

Review rules:

- `allowImportingTsExtensions` is for runtimes/bundlers that load `.ts` directly. A project that emits JavaScript with `tsc` cannot use it; 5.0 has no `rewriteRelativeImportExtensions` (5.7).
- `resolvePackageJsonImports` + an `imports` entry that points at a `.ts` source file works with `noEmit`, but when `tsc` emits with `outDir` and no `rootDir`, 5.0 reports TS2210 *"The project root is ambiguous, but is required to resolve import map entry"*. Set `rootDir` explicitly.
- `customConditions` such as `"source"` that map to `.ts` files are a common monorepo trick; make sure the production build does not use them.

##### New in 5.0: `--verbatimModuleSyntax`

Under `verbatimModuleSyntax`, every import/export **without** a `type` modifier is kept in the output exactly as written, and everything **with** a `type` modifier is deleted. The compiler no longer elides imports based on usage, so what you read is what runs.[^1] It replaces `importsNotUsedAsValues` and `preserveValueImports` and includes the per-file guarantees of `isolatedModules`.

```ts
// @filename: types.ts
export interface Car { make: string }
export const VERSION = 1;
// @filename: use.ts
import { Car, VERSION } from "./types"; // error TS1484
export function drive(c: Car) { return [c.make, VERSION]; }
export { Car } from "./types";           // error TS1205
```

```ts
// @filename: types.ts
export interface Car { make: string }
export const VERSION = 1;
// @filename: use.ts
import { VERSION, type Car } from "./types";
export type { Car } from "./types";
export function drive(c: Car) { return [c.make, VERSION]; }
```

Emit (verified): `import { VERSION, type Car } from "./types"` becomes `import { VERSION } from "./types";` and `import { type Car } from "./types"` becomes `import {} from "./types";` — the module is still loaded for its side effects. Use `import type { Car }` when the module must not be loaded at all.

Under `verbatimModuleSyntax`, ESM syntax cannot be compiled to CommonJS:

```ts
// @module: commonjs
// @moduleResolution: node10
// @filename: dep.ts
export const dep = 1;              // error TS1287
// @filename: main.ts
import { dep } from "./dep";       // error TS1286
export const x = dep;              // error TS1287
```

In CommonJS output files use `import fs = require("fs")` and `export = value`. In practice: use `verbatimModuleSyntax` for ESM projects (`"type": "module"`, bundler projects), and leave it off for CommonJS projects in 5.0.

##### New in 5.0: `export type *`

```ts
// @filename: vehicles.ts
export class Spaceship { id = 1; }
// @filename: models.ts
export type * as vehicles from "./vehicles";
export type * from "./vehicles";
// @filename: main.ts
import { vehicles, Spaceship } from "./models"; // error TS1485
export function take(s: vehicles.Spaceship, t: Spaceship) { return s.id + t.id; }
export function make() { return new vehicles.Spaceship(); } // error TS1362
export function make2() { return new Spaceship(); }         // error TS1362
```

`models.ts` compiles to `export {};`. Under `verbatimModuleSyntax` the importing side must also say so: a plain `import { vehicles }` of a name that was re-exported with `export type` is TS1485 (*"resolves to a type-only declaration and must be imported using a type-only import"*); write `import type { vehicles } from "./models"`. 4.9 rejects both forms (TS1383 *"Only named exports may use 'export type'"*). Use `export type *` for barrel files that only re-export types; it guarantees no runtime import of the re-exported module.

##### Which files are ESM? (`node16`/`nodenext`)

Under `module: node16`/`nodenext` the module format of each file is decided like Node.js does: `.mts`/`.mjs` are ESM, `.cts`/`.cjs` are CommonJS, and `.ts`/`.js` follow the nearest `package.json` `"type"` field (`"module"` → ESM, missing or `"commonjs"` → CJS). Review consequences:

- A missing `"type": "module"` turns the whole project into CommonJS; with `verbatimModuleSyntax` that becomes visible as TS1286/TS1287 errors, without it the code silently compiles to `require`.
- Relative imports in ESM files need the **output** extension: `import { x } from "./util.js"` (even though the source is `util.ts`). TS2835 reports a missing extension.
- Named imports from a CommonJS dependency in an ESM file work at runtime only for exports Node.js can detect statically; TypeScript 5.0 does not check this. When in doubt, default-import the CommonJS module and destructure.
- A CommonJS file cannot `require` an ESM-only package in 5.0's model (TS1479); use `await import()`.

##### Import assertions and JSON

5.0 supports **import assertions** (`assert { type: "json" }`, since 4.5) only with `module: esnext` or `nodenext` (TS2821 with `es2022`, `node16` or `commonjs`); under `nodenext` only in files that are ESM — in a CommonJS-format file it is TS2836 *"Import assertions are not allowed on statements that transpile to commonjs 'require' calls"*. **Import attributes** (`with { type: "json" }`) do not exist before 5.3 — 5.0 reports a syntax error.

```ts
// @resolveJsonModule: true
// @filename: data.json
{ "port": 8080 }
// @filename: main.ts
import data from "./data.json" assert { type: "json" };
export const port: number = data.port;
```

Node.js 22 removed support for `assert` in favour of `with`; code that must run natively on Node.js 22+ needs a TypeScript upgrade (5.3+) for `with`, or must load JSON through `fs`/`createRequire`. Bundlers accept both forms.

##### Path aliases and `baseUrl`

`paths` only affects type checking; `tsc` does not rewrite the specifiers in emitted JavaScript. The runtime (bundler, `tsconfig-paths`, Node.js `imports`) must resolve the same aliases. Prefer `package.json` `imports` (`"#app/*": "./src/*"`) with `node16`/`bundler`, which both TypeScript and Node.js understand. `baseUrl` is not required for `paths` (since 4.1) — do not add it just for aliases; it also makes every top-level folder importable as a bare specifier.

##### CommonJS interop

- `esModuleInterop: true` (implies `allowSyntheticDefaultImports`) is required for `import express from "express"` against `export =` declarations to be correct at runtime.
- `import x = require("x")` and `export = x` are the CommonJS-native forms and the only ones allowed in CommonJS files under `verbatimModuleSyntax`.
- Default-importing a CommonJS module from ESM under `node16` gives you `module.exports`; named imports work only for names Node.js can detect.

##### `@types` packages and TypeScript 5.0

DefinitelyTyped only tests type packages against TypeScript versions younger than two years, and each published `@types` package records the oldest supported version in its `typeScriptVersion` field.[^4] Verified against the npm registry and `tsc` 5.0.4:

- The last `@types/node` releases that still declare 5.0 support are **16.18.126**, **18.19.84**, **20.17.28** and **22.13.14** (published until March 27, 2025). These type-check cleanly under 5.0.4.
- Some later releases still happen to work (22.19.0, 24.2.0, 20.19.43 compile cleanly), but `@types/node` 24.10+, 25.x and 26.x reference `Disposable`, `Symbol.dispose` and `Symbol.asyncDispose`, which 5.0 has no lib for, and use 5.2+ syntax such as mixed labeled/unlabeled tuples: 5.0.4 reports dozens of errors in the declaration files (90 for `@types/node` 26.6.5, mostly TS2304, TS2339, TS1165, TS1169, plus TS5084).
- Fixes: pin `@types/node` (`"@types/node": "~20.17.28"` / `"~22.13.14"`, matching the Node.js major you run), or enable `skipLibCheck` — which hides real errors in your own `.d.ts` files too. The same applies to other `@types/*` packages and to libraries that ship their own types written for newer compilers (declarations that use `NoInfer`, `Disposable`, `Symbol.dispose` or mixed tuple labels fail under 5.0).
- Review rule: an upgrade of an `@types/*` package or a library in a 5.0 project must pass `tsc` without `skipLibCheck`, or the PR must say why it is acceptable.

***

#### Naming and Code Style

- `camelCase` for variables, functions, methods and properties; `PascalCase` for classes, interfaces, type aliases, enums and type parameters with meaning (`TKey` or `Key`); `UPPER_SNAKE_CASE` only for true compile-time constants.
- No `I` prefix on interfaces and no `T` suffix on types; name types after what they are (`User`, `UserId`).
- One concept per module; prefer named exports over `export default` (renames stay consistent and auto-imports work).
- Use `import type`/`type` modifiers for type-only imports (mandatory under `verbatimModuleSyntax`).
- Format with Prettier (or `dprint`), lint with typescript-eslint; do not hand-format.
- Keep files ESM-first (`import`/`export`); use `require` only in CommonJS files that need it.

***

#### Types and the Type System

##### `any`, `unknown`, `never`

```ts
// BAD: any disables checking for everything it touches
function parseBad(json: string): any {
  return JSON.parse(json);
}
const port: string = parseBad('{"port":1}').port.toFixed(); // compiles, wrong at runtime

// GOOD: unknown forces a check
function parse(json: string): unknown {
  return JSON.parse(json);
}
function readPort(v: unknown): number {
  if (typeof v === "object" && v !== null && "port" in v && typeof v.port === "number") {
    return v.port; // narrowed via `in` (4.9) and typeof
  }
  throw new TypeError("invalid config");
}
export const p = readPort(parse('{"port":1}'));
export { port };
```

- `JSON.parse`, `response.json()` and `catch` variables (without `useUnknownInCatchVariables`) return `any`; assign them to `unknown` immediately.
- `never` is the type of impossible values; use it for exhaustiveness checks.
- `{}` means "any non-nullish value", `object` means "non-primitive". Neither is a replacement for a real shape.

##### Literal types, widening and `as const`

```ts
const method = "GET";          // type "GET"
let method2 = "GET";           // type string (widened)
const config = { method: "GET" };            // { method: string }
const config2 = { method: "GET" } as const;  // { readonly method: "GET" }

declare function request(m: "GET" | "POST"): void;
request(config.method);  // error TS2345
request(config2.method); // OK
export { method, method2 };
```

Prefer `as const` on literal configuration tables and `const` type parameters (5.0) on functions that need literal inference.

##### `interface` vs `type`

Use `interface` for object shapes that may be extended or merged (and for public APIs); use `type` for unions, intersections, mapped/conditional types, tuples and function types. `interface X extends A, B` reports conflicting members, while `A & B` silently produces `never` members. Declaration merging means a second `interface Window` in another file *adds* members — review global augmentations carefully.

##### Excess property checks

Excess property checks apply only to fresh object literals:

```ts
interface Point { x: number; y: number }
const p1: Point = { x: 1, y: 2, z: 3 }; // error TS2322
const tmp = { x: 1, y: 2, z: 3 };
const p2: Point = tmp;                  // OK: not a fresh literal
export { p1, p2 };
```

`suppressExcessPropertyErrors` (which turned this off) is deprecated in 5.0.

##### Index signatures, `Record` and `Object.keys`

```ts
interface Counts { [word: string]: number }
const counts: Counts = {};
const n: number = counts["missing"]; // error TS2322

// Object.keys returns string[], not (keyof T)[], because objects may have extra keys.
interface User { id: number; name: string }
function printUser(u: User) {
  for (const k of Object.keys(u)) {
    // u[k]; // would be TS7053: string cannot index User
    if (k === "id" || k === "name") console.log(u[k]);
  }
}
export { n, printUser };
```

Use `Map<K, V>` for dynamic keys from untrusted input (no prototype keys, no `__proto__` surprises). `suppressImplicitAnyIndexErrors` (deprecated in 5.0) hid the TS7053 error above — remove it and fix the access.

##### Tuples

```ts
type Range = [start: number, end: number];           // labeled (4.0)
type Args = [name: string, ...rest: number[]];       // variadic (4.0)
type Mixed = [name: string, number];                  // error TS5084
type Opt = [x: number, y?: number];
const r: Range = [0, 10];
const o: Opt = [1];
export type { Args, Mixed };
export { r, o };
```

Mixing labeled and unlabeled tuple elements is allowed only from 5.2.

##### Type assertions, non-null assertions and `satisfies`

```ts
type Color = "red" | "green";
// BAD: `as` silences the checker; the missing "green" key goes unnoticed
const palette1 = { red: "#f00" } as Record<Color, string>;

// GOOD: satisfies (4.9) checks without widening
const palette2 = { red: "#f00", gren: "#0f0" } satisfies Record<Color, string>; // error TS1360

const palette3 = { red: "#f00", green: [0, 255, 0] } satisfies Record<Color, string | number[]>;
palette3.green.map((v) => v * 2); // green keeps its precise type number[]
export { palette1, palette2 };
```

- `x as T` is acceptable only when the code proves `T` (e.g. after a validation function). `x as unknown as T` is a red flag.
- `value!` asserts non-null; review every one. Prefer narrowing or `??`.
- `satisfies` is the right tool for configuration objects and lookup tables.

##### `readonly` and immutability

```ts
interface Config {
  readonly hosts: readonly string[];
  readonly limits: Readonly<Record<string, number>>;
}
function addHost(cfg: Config, h: string): Config {
  cfg.hosts.push(h);                       // error TS2339
  return { ...cfg, hosts: [...cfg.hosts, h] };
}
const frozen = Object.freeze({ retries: 3 });
frozen.retries = 4;                        // error TS2540
const mutable: string[] = ["a"];
const ro: readonly string[] = mutable;     // OK: mutable is assignable to readonly
mutable.push("b");                         // ...and still changes `ro` — readonly is a view, not a copy
export { addHost, ro };
```

`readonly` is shallow and compile-time only; `Object.freeze` is shallow and runtime-only. Accept `readonly T[]` in parameters you do not mutate so callers can pass both mutable and readonly arrays.

##### Optional chaining and nullish coalescing

```ts
interface Cfg { retries?: number; server?: { host?: string } }
function build(cfg: Cfg) {
  const retries = cfg.retries || 3;  // BAD: 0 becomes 3
  const retries2 = cfg.retries ?? 3; // GOOD
  const host = cfg.server?.host ?? "localhost";
  return { retries, retries2, host };
}
export { build };
```

##### Module augmentation, `declare global` and namespaces

```ts
// @filename: express-like.ts
export interface Request { url: string }
// @filename: augment.ts
import type { Request as _R } from "./express-like";
declare module "./express-like" {
  interface Request { userId?: string } // merges into the original interface
}
declare global {
  interface Window { __APP_VERSION__: string }
}
export {};
// @filename: use.ts
import type { Request } from "./express-like";
export function who(r: Request) { return r.userId ?? window.__APP_VERSION__; }
```

- Augmentations must live in a module (a file with `import`/`export`), and the augmented module specifier must resolve exactly as the original does.
- Every `declare global` widens the types of the whole program; review that the runtime really provides what is declared (a global declared but never assigned is a crash waiting to happen).
- `namespace` (and the older `module Foo {}` keyword form) is legacy for application code; use ES modules. `declare namespace` in `.d.ts` files describing script globals is still fine. Runtime namespaces are not erasable syntax: type-stripping runtimes reject them.

***

#### Narrowing and Control-Flow Analysis

##### Built-in narrowing

`typeof`, `instanceof`, `in`, equality, truthiness, `Array.isArray`, discriminant properties and aliased conditions (4.4) all narrow:

```ts
function describe(x: string | number | Date | null | undefined) {
  if (x == null) return "nothing";          // null and undefined
  const isString = typeof x === "string";   // aliased condition (4.4)
  if (isString) return x.toUpperCase();
  if (x instanceof Date) return x.toISOString();
  return x.toFixed(2);
}

function hasId(v: unknown) {
  if (typeof v === "object" && v !== null && "id" in v) {
    return v.id; // unknown, but accessible: `in` adds the property (4.9)
  }
  return undefined;
}
export { describe, hasId };
```

Not available in 5.0: `switch (true) { case typeof x === "string": ... }` does not narrow (5.3), comparisons to `true`/`false` (`if (isString(x) === true)`) do not narrow (5.3), and `instanceof` does not use `Symbol.hasInstance` (5.3).

```ts
function s(x: string | number) {
  switch (true) {
    case typeof x === "string":
      return x.toUpperCase(); // error TS2339
    default:
      return 0;
  }
}
export { s };
```

##### Discriminated unions and exhaustiveness

```ts
type Shape =
  | { kind: "circle"; r: number }
  | { kind: "rect"; w: number; h: number };

function assertNever(x: never): never {
  throw new Error(`unexpected: ${JSON.stringify(x)}`);
}

function area(s: Shape): number {
  switch (s.kind) {
    case "circle":
      return Math.PI * s.r ** 2;
    case "rect":
      return s.w * s.h;
    default:
      return assertNever(s); // compile error here when a new kind is added
  }
}

// Destructured discriminants narrow too (4.6)
function area2({ kind, ...rest }: Shape) {
  return kind;
}
export { area, area2 };
```

5.0's editor offers *exhaustive `switch`/`case` completions* that scaffold all missing `case` labels for a literal union.[^1]

##### Type predicates and assertion functions

5.0 does **not** infer type predicates (5.5). A filter callback must be an explicit predicate:

```ts
const raw = [1, undefined, 2];
const bad: number[] = raw.filter((x) => x !== undefined); // error TS2322
const good: number[] = raw.filter((x): x is number => x !== undefined);

function assertIsString(v: unknown, name: string): asserts v is string {
  if (typeof v !== "string") throw new TypeError(`${name} must be a string`);
}
declare const input: unknown;
assertIsString(input, "input");
input.toUpperCase();
export { bad, good };
```

A type predicate is an unchecked promise: `(x): x is number => true` compiles. Review predicate bodies like assertions. Assertion functions must be called through an explicitly typed reference (a `const` holding an arrow function needs an annotation) or TS2775 is reported.

##### Narrowing and closures

In 5.0 narrowing of a `let`/parameter is **not** preserved inside closures created after the last assignment (5.4 adds that):

```ts
function bad(v: string | undefined) {
  if (v === undefined) v = "default";
  return () => v.length; // error TS18048
}

function good(v: string | undefined) {
  const value = v ?? "default"; // const keeps its narrowed type in closures
  return () => value.length;
}

function idx(o: Record<string, unknown>, k: string) {
  if (typeof o[k] === "string") {
    return (o[k] as string).toUpperCase(); // 5.0 does not narrow o[k] (5.5 does)
  }
  const v = o[k];
  return typeof v === "string" ? v.toUpperCase() : "";
}
export { bad, good, idx };
```

Copy into a `const` instead of reaching for `!`.

***

#### Functions

##### Parameters and return types

- Annotate parameters and the return type of exported functions; inference is fine for locals.
- Prefer an options object over more than three positional parameters or boolean flags.
- `void` return in a *function type* means "the result is ignored", not "returns nothing": `const f: () => void = () => 42` is legal. Do not rely on the return value of a `void` callback.
- In 5.0 a function declared to return `undefined` must contain `return` (5.1 relaxed this):

```ts
function f1(): undefined {} // error TS2355
function f2(): undefined { return; }
function f3(): void {}
export { f1, f2, f3 };
```

##### Overloads

Overload signatures are what callers see; the implementation signature is hidden and must be compatible with all of them. Order from most specific to least specific. Prefer a union parameter or a generic when the return type does not depend on the argument type.

```ts
function parse(input: string): object;
function parse(input: Uint8Array, encoding: "utf8" | "latin1"): object;
function parse(input: string | Uint8Array, encoding?: "utf8" | "latin1"): object {
  const text = typeof input === "string" ? input : new TextDecoder(encoding === "latin1" ? "latin1" : "utf-8").decode(input);
  return JSON.parse(text) as object;
}
parse("{}");
parse(new Uint8Array(), "utf8");
parse("{}", "utf8"); // error TS2345
```

##### Method parameter bivariance

`strictFunctionTypes` checks *function-typed properties* contravariantly but still checks *methods* bivariantly:

```ts
interface Animal { name: string }
interface Dog extends Animal { bark(): void }

interface HandlerMethod { handle(a: Animal): void }      // method syntax: bivariant
interface HandlerProp { handle: (a: Animal) => void }    // property syntax: checked

const dogOnly = (d: Dog) => d.bark();
const m: HandlerMethod = { handle: dogOnly };  // compiles — unsound
const p: HandlerProp = { handle: dogOnly };    // error TS2322
export { m, p };
```

Declare callbacks in interfaces with property syntax.

##### `this` parameters

```ts
function onClick(this: HTMLButtonElement, ev: MouseEvent) {
  this.disabled = true;
  return ev.button;
}
const btn = document.createElement("button");
btn.addEventListener("click", onClick);
onClick(new MouseEvent("click")); // error TS2684
```

##### New in 5.0: Forbidden implicit coercions in relational operators

5.0 reports `<`, `>`, `<=`, `>=` on operands that may be strings when compared to numbers (4.9 accepted them):[^1]

```ts
function isBig(ns: number | string) {
  return ns > 4; // error TS2365
}
function isBigFixed(ns: number | string) {
  return Number(ns) > 4; // explicit conversion; check for NaN where input is untrusted
}
export { isBig, isBigFixed };
```

Typical fallout: `string | number` IDs, values from loosely typed parsers or form inputs. Fix the types, do not cast to `any`.

***

#### Generics

##### Constraints, defaults and inference

```ts
function pluck<T, K extends keyof T>(items: readonly T[], key: K): T[K][] {
  return items.map((i) => i[key]);
}
interface Page<T = unknown> { items: T[]; next?: string }

const names = pluck([{ id: 1, name: "a" }], "name"); // string[]
pluck([{ id: 1 }], "nope"); // error TS2345
const page: Page = { items: [] };
export { names, page };
```

- Every type parameter should appear at least twice (in a parameter and the return type, or two parameters); otherwise use the constraint type directly.
- Do not use generics to avoid writing a union.

##### New in 5.0: `const` type parameters

A `const` modifier on a type parameter infers arguments as if they were written with `as const`:[^1]

```ts
type HasNames = { names: readonly string[] };
function getNames<const T extends HasNames>(arg: T): T["names"] {
  return arg.names;
}
const names = getNames({ names: ["Alice", "Bob"] }); // readonly ["Alice", "Bob"]
const exact: readonly ["Alice", "Bob"] = names;

// Pitfall 1: a mutable constraint defeats const inference (falls back to string[])
declare function fnBad<const T extends string[]>(args: T): T;
const r1 = fnBad(["a", "b"]);
const r1check: ["a", "b"] = r1; // error TS2322

// Pitfall 2: only literals written in the call are affected
declare function fnGood<const T extends readonly string[]>(args: T): T;
const arr = ["a", "b"];
const r2 = fnGood(arr);           // string[]
const r3 = fnGood(["a", "b"]);    // readonly ["a", "b"]
export { exact, r2, r3 };
```

Review rule: when you see `const T`, the constraint should be `readonly` (arrays/tuples) — otherwise the modifier silently does nothing in 5.0. 5.3 changed this (microsoft/TypeScript#55229): newer compilers infer a mutable tuple `["a", "b"]` for a mutable constraint, so `r1check` above compiles on 5.3+ but not on 5.0–5.2 (verified). Do not rely on the 5.3 behaviour. 4.9 rejects the syntax entirely (TS1139).

##### Controlling inference without `NoInfer`

`NoInfer<T>` is 5.4. In 5.0 block inference from a parameter with the classic idiom:

```ts
type NoInferCompat<T> = [T][T extends any ? 0 : never];

function createFSM<S extends string>(states: S[], initial: NoInferCompat<S>) {
  return { states, initial };
}
createFSM(["idle", "busy"], "idle");
createFSM(["idle", "busy"], "done"); // error TS2345
```

##### Variance annotations

`in`/`out` modifiers (4.7) state variance explicitly (`interface Producer<out T>`). They are checked against the structure, so a wrong annotation is reported; use them only for performance in very large generic types or to document intent.

***

#### Advanced Types

##### Conditional types, `infer` and distribution

```ts
type ElementOf<T> = T extends readonly (infer E)[] ? E : never;
type A = ElementOf<string[] | number[]>; // string | number (distributes over the union)

type NonDistributive<T> = [T] extends [string] ? "str" : "other";
type B = NonDistributive<string | number>; // "other"

type FirstNum<T> = T extends [infer H extends number, ...unknown[]] ? H : never; // infer ... extends (4.7)
type C = FirstNum<[1, "a"]>; // 1

type Num<S> = S extends `${infer N extends number}` ? N : never; // 4.8
type D = Num<"42">; // 42
const check: [A, B, C, D] = ["x", "other", 1, 42];
export { check };
```

##### Mapped types, key remapping and template literal types

```ts
type Getters<T> = {
  [K in keyof T as `get${Capitalize<string & K>}`]: () => T[K];
};
interface Person { name: string; age: number }
type PersonGetters = Getters<Person>; // { getName(): string; getAge(): number }

type Mutable<T> = { -readonly [K in keyof T]: T[K] };
type EventName = `on${Capitalize<"click" | "focus">}`; // "onClick" | "onFocus"
const g: PersonGetters = { getName: () => "a", getAge: () => 1 };
const e: EventName = "onClick";
export type { Mutable };
export { g, e };
```

##### Built-in utility types

`Partial`, `Required`, `Readonly`, `Pick`, `Omit`, `Record`, `Exclude`, `Extract`, `NonNullable`, `Parameters`, `ConstructorParameters`, `ReturnType`, `InstanceType`, `ThisParameterType`, `OmitThisParameter`, `ThisType`, `Awaited` (4.5), `Uppercase`/`Lowercase`/`Capitalize`/`Uncapitalize`. `NoInfer` does not exist in 5.0. `Omit<T, K>` does not check that `K` is a key of `T` — a typo in `K` is silently accepted.

##### Branded (nominal) types

```ts
type UserId = string & { readonly __brand: "UserId" };
type OrderId = string & { readonly __brand: "OrderId" };

function asUserId(s: string): UserId {
  if (!/^u_[a-z0-9]+$/.test(s)) throw new Error("bad user id");
  return s as UserId; // the only place a cast is allowed
}
declare function loadUser(id: UserId): void;
const order = "o_1" as OrderId;
loadUser(asUserId("u_1"));
loadUser(order); // error TS2345
```

##### Recursive types and instantiation limits

Deeply recursive conditional types hit TS2589 (*"Type instantiation is excessively deep and possibly infinite"*); tail-recursive conditional types (4.5) allow about 1000 levels. Keep recursion bounded (e.g. a depth counter tuple) and avoid recursive types over user-controlled string lengths.

***

#### New in 5.0: All Enums Are Union Enums

5.0 gives every enum member its own literal type, including computed members, so every enum can be narrowed and its members used as types. Two new errors come with it:[^1]

```ts
enum SomeEvenDigit { Zero = 0, Two = 2, Four = 4 }
let m: SomeEvenDigit = 1; // error TS2322

enum Letters { A = "a" }
enum Numbers { one = 1, two = Letters.A }
const t: number = Numbers.two; // error TS2322

enum Rand { X = Math.random(), Y = Math.random() }
type OnlyX = Rand.X; // OK in 5.0; TS2535 in 4.9
function pick(r: Rand): OnlyX | undefined {
  return r === Rand.X ? r : undefined; // narrowing works
}
export { m, t, pick };
```

Remaining enum pitfalls a reviewer must catch:

- **Numeric enums accept any number** only from non-literal expressions: `let e: SomeEvenDigit = someNumber` still compiles. Validate numbers from input before converting.
- **Reverse mappings** exist only for numeric members; `Object.values(NumericEnum)` contains both names and numbers.
- **`const enum`** is inlined and erased. Under `isolatedModules`/`verbatimModuleSyntax` (and with Babel, esbuild, swc), `declare const enum` from another package cannot be inlined; do not export `const enum` from libraries (TS2748 for ambient const enums under `isolatedModules`).
- Prefer a `const` object + union type when values come from JSON, when interop with plain strings matters, or when the code must be erasable:

```ts
export const Status = { Pending: "pending", Active: "active" } as const;
export type Status = (typeof Status)[keyof typeof Status];
function isStatus(v: string): v is Status {
  return (Object.values(Status) as string[]).includes(v);
}
export { isStatus };
```

***

#### Classes

##### Field semantics (`useDefineForClassFields`)

With `target: es2022` or `esnext`, `useDefineForClassFields` defaults to `true`: fields are emitted as native class fields (`[[Define]]` semantics). Consequences:

- A field redeclared in a subclass **overwrites** an accessor or a value set by the base constructor. Use `declare field: Type;` to only narrow the type without emitting a field.
- Field initializers run **after** `super()` returns but **before** the rest of the subclass constructor body — and, with `[[Define]]` semantics, **before parameter properties are assigned**. 5.0 does not report this:

```ts
export class P {
  full = this.first + "!";           // reads the parameter property too early
  constructor(public first: string) {}
}
console.log(new P("a").full);        // "undefined!" with target es2022; "a!" with es2021 or useDefineForClassFields: false
```
- Libraries that rely on `[[Set]]` (MobX `observable` legacy decorators, some ORMs) break when `target` moves from `es2021` to `es2022` unless `useDefineForClassFields: false` is set.

```ts
class Base {
  greeting = "hello";
  constructor() { this.init(); }
  init() {}
}
class Derived extends Base {
  declare greeting: "hello";  // GOOD: type-only, no emitted field
  value = 1;                  // initialised after Base's constructor ran
  override init() { console.log(this.value); } // logs undefined at construction time
}
export { Derived };
```

##### Parameter properties, visibility and `#private`

`private`/`protected` are compile-time only; `#private` fields are enforced by the runtime. Use `#` for secrets, invariants and anything a plain JavaScript caller must not reach. `private` members are still visible to `obj["secret"]` and `JSON.stringify`. Parameter properties (`constructor(private readonly repo: Repo)`) are not erasable syntax and are not supported by type-stripping runtimes; they are fine for a `tsc`/bundler build.

##### `override`, abstract classes and `implements`

```ts
// @noImplicitOverride: true
abstract class Repo<T> {
  abstract find(id: string): Promise<T | undefined>;
  describe(): string { return "repo"; }
}
class UserRepo extends Repo<{ id: string }> {
  async find(id: string) { return { id }; }
  describe(): string { return "users"; } // error TS4114
}
export { UserRepo };
```

`implements` checks the class but does not contextually type its members: method parameters still need annotations.

##### Accessors

In 5.0 the getter type must be assignable to the setter type (TS2380 otherwise); fully unrelated getter/setter types arrive in 5.1. A setter that accepts a wider type than the getter returns has been allowed since 4.3:

```ts
class Temperature {
  #c = 0;
  get celsius(): number { return this.#c; }
  set celsius(v: number | string) { this.#c = Number(v); } // OK: number assignable to number | string
}
class Bad {
  #v = "";
  get v(): string { return this.#v; } // error TS2380
  set v(x: number) { this.#v = String(x); }
}
export { Temperature, Bad };
```

The `accessor` keyword (4.9) declares an auto-accessor backed by a private slot; it is the target of `ClassAccessorDecoratorContext` decorators.

##### `strictPropertyInitialization` and definite assignment

Fields must be initialised in the declaration or the constructor. `field!: Type` (definite assignment) is acceptable only when a framework guarantees initialisation (e.g. dependency injection); every `!` is a review point.

***

#### New in 5.0: Decorators

5.0 implements the TC39 decorators proposal (stage 3), which becomes the default whenever `--experimentalDecorators` is **not** set.[^1][^15] Decorators can be applied to classes, methods, getters, setters, fields and `accessor` fields — **not** to parameters — and receive `(value, context)`:

| Decorated | `value` | Context type (from `lib.decorators.d.ts`) | May return |
|---|---|---|---|
| class | the class | `ClassDecoratorContext<Class>` | a replacement class |
| method | the method | `ClassMethodDecoratorContext<This, Fn>` | a replacement method |
| getter / setter | the function | `ClassGetterDecoratorContext` / `ClassSetterDecoratorContext` | a replacement |
| field | `undefined` | `ClassFieldDecoratorContext<This, Value>` | an initializer transform `(initial) => value` |
| `accessor` field | `{ get, set }` | `ClassAccessorDecoratorContext<This, Value>` | `{ get?, set?, init? }` |

Every context has `kind`, `name` and `addInitializer()`. Member contexts add `static`, `private` and `access`: `access.has` everywhere, `access.get` for methods, getters, fields and accessors, `access.set` for setters, fields and accessors (`ClassDecoratorContext` has none of these). No context has `metadata` in 5.0 (5.2).

```ts
function logged<This, Args extends unknown[], R>(
  target: (this: This, ...args: Args) => R,
  context: ClassMethodDecoratorContext<This, (this: This, ...args: Args) => R>,
) {
  const name = String(context.name);
  return function (this: This, ...args: Args): R {
    console.log(`enter ${name}`);
    try {
      return target.call(this, ...args);
    } finally {
      console.log(`exit ${name}`);
    }
  };
}

function bound<This, F extends (this: This, ...a: any[]) => any>(_m: F, ctx: ClassMethodDecoratorContext<This, F>) {
  if (ctx.private) throw new Error("cannot bind private methods");
  ctx.addInitializer(function (this: This) {
    const self = this as any;
    self[ctx.name] = self[ctx.name].bind(this);
  });
}

function atLeast(min: number) {
  return function <This>(_t: undefined, _ctx: ClassFieldDecoratorContext<This, number>) {
    return (initial: number) => Math.max(initial, min);
  };
}

const registry: string[] = [];
function register<C extends abstract new (...a: any) => any>(_cls: C, ctx: ClassDecoratorContext<C>) {
  ctx.addInitializer(() => registry.push(String(ctx.name)));
}

export @register class Service {
  @atLeast(1024) port = 80;
  constructor(public name: string) {}
  @bound @logged hello() { return `hello ${this.name}:${this.port}`; }
}

const hello = new Service("api").hello;
console.log(hello(), registry);
// enter hello
// exit hello
// hello api:1024 [ 'Service' ]
```

Verified by compiling with `target: es2022` and running on Node.js 18: the method stays bound when detached, the field initializer is transformed, and the class initializer runs once.

Rules and pitfalls:

- **Evaluation order**: decorators are *evaluated* top to bottom and *applied* bottom to top (`@bound @logged m()` → `logged` wraps first).
- **Placement**: decorators may appear before *or* after `export`/`export default` — not both (`@a export @b class X {}` is TS8038). Parameter decorators are TS1206 without `--experimentalDecorators`.
- **`emitDecoratorMetadata` requires `experimentalDecorators`** (TS5052); standard decorators have no `design:type` reflection. Frameworks built on legacy decorators (NestJS, TypeORM, InversifyJS, class-validator, type-graphql; most of them also read `reflect-metadata`) **require** `experimentalDecorators: true`. Do not "modernise" those projects by removing the flag.
- **A legacy decorator applied without the flag** fails type-checking (TS1241/TS1270/TS1240) — the error message is about signatures, not about the missing flag.
- **Emit**: for `target` ≤ `es2022` 5.0 emits `__esDecorate`/`__runInitializers` helpers (use `importHelpers` + `tslib` ≥ 2.5 to share them). For `target: esnext` 5.0 emits the `@decorator` syntax verbatim, which Node.js cannot parse (verified on Node.js 18 and 26: `SyntaxError`).
- 5.0.3 fixed a double emit of constructors when getters are decorated (#53448).[^3]
- Type decorators generically as above; `any`-typed decorators hide mistakes such as decorating a field with a method decorator.

##### Legacy `--experimentalDecorators`

The flag keeps the old (TypeScript 1.5-era) semantics: `(target, key, descriptor)` signatures, parameter decorators, `emitDecoratorMetadata`. 5.0 checks constructor-parameter decorators more accurately — the `key` argument is `undefined` for constructor parameters:[^1]

```ts
// @experimentalDecorators: true
declare const inject: (token: unknown) => (target: object, key: string | symbol, index?: number) => void;
declare const injectOk: (token: unknown) => (target: object, key: string | symbol | undefined, index: number) => void;
class Foo {}
export class C {
  constructor(
    @inject(Foo) private a: Foo,   // error TS1239
    @injectOk(Foo) private b: Foo,
  ) {}
}
```

Fix the decorator's `key` type (`string | symbol | undefined`) or wrap a third-party decorator. 4.9 accepted the code above.

***

#### Error Handling

##### `unknown` in `catch`

```ts
class HttpError extends Error {
  constructor(message: string, readonly status: number, options?: { cause?: unknown }) {
    super(message, options); // Error cause needs lib es2022
    this.name = "HttpError";
  }
}

async function fetchJson(url: string): Promise<unknown> {
  let res: Response;
  try {
    res = await fetch(url);
  } catch (err) {
    // err: unknown (useUnknownInCatchVariables, part of strict)
    throw new HttpError(`network error for ${url}`, 0, { cause: err });
  }
  if (!res.ok) throw new HttpError(`HTTP ${res.status}`, res.status);
  return res.json() as Promise<unknown>;
}

function message(err: unknown): string {
  return err instanceof Error ? err.message : String(err);
}
export { fetchJson, message };
```

- Never `catch (e: any)`; never assume `e` is an `Error` (any value can be thrown, and `instanceof Error` fails across realms).
- Keep the original error as `cause` when wrapping.
- Do not swallow errors in empty `catch {}` blocks without a comment explaining why.
- A `Result` union (`{ ok: true; value } | { ok: false; error }`) is a good fit for expected failures (validation, not-found); exceptions for bugs and infrastructure failures.

***

#### Async Code

##### Floating and misused promises

```ts
declare function save(): Promise<void>;
declare const items: string[];

async function bad() {
  save();                                      // BAD: floating promise, rejection unhandled
  items.forEach(async (i) => { await save(); }); // BAD: forEach ignores the promises
  if (save()) { /* always true */ }            // error TS2801
}

async function good() {
  await save();
  await Promise.all(items.map(async () => save()));
  for (const _ of items) await save();         // sequential when order/backpressure matters
}
export { bad, good };
```

The compiler reports only the un-awaited promise in a condition (TS2801, "This condition will always return true since this 'Promise<void>' is always defined"); the floating call and the `async` `forEach` callback compile silently. Enable typescript-eslint's `no-floating-promises`, `no-misused-promises` and `await-thenable`.

##### Concurrency, cancellation and timeouts

```ts
async function fetchWithTimeout(url: string, ms: number, outer?: AbortSignal): Promise<Response> {
  const ctrl = new AbortController();
  const timer = setTimeout(() => ctrl.abort(new Error(`timeout after ${ms}ms`)), ms);
  const onAbort = () => ctrl.abort(outer?.reason);
  outer?.addEventListener("abort", onAbort, { once: true });
  try {
    return await fetch(url, { signal: ctrl.signal });
  } finally {
    clearTimeout(timer);
    outer?.removeEventListener("abort", onAbort);
  }
}

async function settle(urls: string[]) {
  const results = await Promise.allSettled(urls.map((u) => fetchWithTimeout(u, 5_000)));
  return results.filter((r): r is PromiseFulfilledResult<Response> => r.status === "fulfilled").map((r) => r.value);
}
export { settle };
```

`AbortSignal.timeout(ms)` is typed in the 5.0 DOM lib; `AbortSignal.any([...])` is not (5.5) — combine signals manually as above. `Promise.withResolvers` is not typed in 5.0 (5.4). Limit concurrency for large fan-outs (a small worker pool or `p-limit`); unbounded `Promise.all` over thousands of items exhausts sockets and memory.

##### Async iteration and `Awaited`

`Awaited<T>` (4.5) models recursive unwrapping; `Promise.all` uses it. Use `for await (const chunk of stream)` for async iterables; below `target: es2018` TypeScript downlevels it with helpers (no flag needed — 5.0 compiles it even for `es5`), which require `Symbol.asyncIterator` at runtime.

***

#### Resource Cleanup Without `using`

`using`/`await using` are 5.2 (5.0 reports TS1434 *Unexpected keyword or identifier* on `using r = ...`). Use `try`/`finally`:

```ts
interface Conn { query(sql: string): Promise<unknown>; release(): void }
declare function acquire(): Promise<Conn>;

async function withConn<T>(fn: (c: Conn) => Promise<T>): Promise<T> {
  const c = await acquire();
  try {
    return await fn(c); // `return await` so that finally runs after the work completes
  } finally {
    c.release();
  }
}
export { withConn };
```

Review rule: `return fn(c)` (without `await`) inside `try` releases the connection before the promise settles.

***

#### Iterators and Generators

- `for...of` over `Map`, `Set`, strings and DOM collections needs `target` ≥ `es2015` (or `downlevelIteration` for ES5) and, for DOM collections, `lib: dom.iterable`.
- Generators are typed `Generator<Yield, Return, Next>`; async generators `AsyncGenerator<...>`. 5.0 has no iterator-helper types (`iterator.map()`, 5.6) and no `BuiltinIteratorReturn`.
- Spreading a large iterable into an argument list (`Math.max(...bigArray)`) throws `RangeError` for very large arrays; use a loop.

***

#### Declaration Files and Publishing Libraries

##### Declaration hygiene

- Ship `.d.ts` from `tsc --declaration` (or `emitDeclarationOnly` alongside a bundler); 5.0 has no `isolatedDeclarations` (5.5), so declaration emit needs the full type checker.
- Exported functions need explicit return types: inferred types may reference non-exported or deep-import types (TS2742 *"The inferred type of X cannot be named without a reference to ..."*).
- Point `package.json` `exports` at both code and types, with `types` first in each condition object:

```jsonc
{
  "name": "my-lib",
  "type": "module",
  "exports": {
    ".": {
      "types": "./dist/index.d.ts",
      "import": "./dist/index.js",
      "require": "./dist/index.cjs"
    }
  },
  "types": "./dist/index.d.ts"
}
```

- Dual CJS/ESM packages need separate declaration files for each format under `node16` (`index.d.ts` for ESM and `index.d.cts` for CJS); a single `.d.ts` misdescribes one of them.
- Do not publish types that require 5.1+ features if consumers may be on 5.0 (and vice versa: if *you* are on 5.0, the dependencies you add must not require newer TypeScript). `typesVersions` can serve older declarations: `"typesVersions": { "<5.2": { "*": ["ts5.0/*"] } }`.
- Test the published shape with `@arethetypeswrong/cli` and `publint`.

##### `types` and `typeRoots`

Without `types`, every package in `node_modules/@types` (walking up the directory tree) is included globally — test-runner globals leak into production code and conflicting versions break builds. Set `"types": ["node"]` (or `[]`) explicitly. In 5.0, an explicit `typeRoots` is **not** consulted during module resolution (5.1 changed that).

***

#### JavaScript Projects (`checkJs` and JSDoc)

`allowJs` + `checkJs` (or `// @ts-check`) type-check JavaScript through JSDoc. 5.0 adds two tags:[^1]

```js
// @ts-check
/**
 * @typedef CompilerOptions
 * @prop {boolean} [strict]
 * @prop {string} [outDir]
 */

/** @satisfies {CompilerOptions} */
export const opts = { outdir: "../lib" }; // error TS1360

export const routes = /** @satisfies {Record<string, string[]>} */ ({ home: ["/"], about: ["/about"] });
// @ts-expect-error -- satisfies keeps the literal keys, so unknown keys are errors
routes.missing;

/**
 * @overload
 * @param {string} value
 * @return {string}
 */
/**
 * @overload
 * @param {number} value
 * @param {number} [digits]
 * @return {string}
 */
/**
 * @param {string | number} value
 * @param {number} [digits]
 */
export function fmt(value, digits) {
  return typeof value === "number" ? value.toFixed(digits) : value;
}
fmt("a");
fmt(1, 2);
fmt("a", 2); // error TS2345
```

- `@satisfies` works on declarations and inline on parenthesised expressions: `/** @satisfies {T} */ ({ ... })`.
- Each JSDoc comment with `@overload` declares one overload of the following function; the last plain comment describes the implementation.
- Not available in 5.0: JSDoc `@import` (5.5) — use `/** @typedef {import("./types").User} User */`.
- 4.9 silently ignores both tags (no errors at all for the code above).

***

#### Security

TypeScript's types do not exist at runtime; every security property must be enforced by runtime code.

##### Validate untrusted input

```ts
interface CreateUser { email: string; age: number }

// BAD: a cast is not validation
async function handlerBad(req: Request) {
  const body = (await req.json()) as CreateUser;
  return body.email.toLowerCase(); // throws or misbehaves on malicious input
}

// GOOD: validate at the boundary (hand-written guard, or a schema library such as zod/valibot)
function isCreateUser(v: unknown): v is CreateUser {
  if (typeof v !== "object" || v === null) return false;
  const o = v as Record<string, unknown>;
  return typeof o.email === "string" && o.email.length <= 254 &&
    typeof o.age === "number" && Number.isInteger(o.age) && o.age >= 0;
}
async function handler(req: Request) {
  const body: unknown = await req.json();
  if (!isCreateUser(body)) return new Response("bad request", { status: 400 });
  return new Response(body.email.toLowerCase());
}
export { handlerBad, handler };
```

Check that the schema library's declaration files compile under 5.0 before upgrading it — current releases of many libraries are only tested against newer compilers; see [`@types` packages](#types-packages-and-typescript-50).

##### Prototype pollution and dynamic keys

```ts
// BAD: merging untrusted JSON into an object literal
function mergeBad(target: Record<string, unknown>, src: Record<string, unknown>) {
  for (const k in src) target[k] = src[k]; // "__proto__" / "constructor" pollute prototypes
  return target;
}

// GOOD: reject dangerous keys, use null-prototype objects or Map
const FORBIDDEN = new Set(["__proto__", "constructor", "prototype"]);
function merge(target: Record<string, unknown>, src: Record<string, unknown>) {
  for (const [k, v] of Object.entries(src)) {
    if (FORBIDDEN.has(k)) continue;
    target[k] = v;
  }
  return target;
}
const safeDict: Record<string, string> = Object.create(null);
export { mergeBad, merge, safeDict };
```

##### Other review points

- No `eval`, `new Function`, or `setTimeout(string)`; no dynamic `import(userInput)`.
- Build SQL with parameterised queries, shell commands with `execFile`/`spawn` argument arrays (never `exec` with string interpolation), HTML through an escaping template or DOM APIs (`textContent`, not `innerHTML`).
- Validate file paths derived from input (`path.resolve` + prefix check) to prevent traversal.
- Secrets come from the environment or a secret store, never from source; `process.env.X` is `string | undefined` — check it at startup.
- `skipLibCheck` and `any`-typed third-party code can hide security-relevant type errors; review `.d.ts` changes in dependencies.
- Keep the compiler and type packages pinned (lockfile, `npm ci`) — the build toolchain is part of the supply chain.

***

#### Performance

##### Runtime performance (what TypeScript affects)

- `target` decides how much helper code is emitted: ES5 output for `async`/generators/spread/classes is much larger and slower than ES2022 output. Set `target` to the oldest runtime you actually support.
- Standard decorators add per-class helper calls at definition time and initializer arrays per instance; keep them off hot allocation paths.
- `enum` compiles to an object plus reverse-mapping code; `const enum` and `as const` objects avoid lookups.
- `importHelpers: true` + `tslib` avoids duplicating helpers in every file.

##### Compiler performance

- 5.0 itself is faster than 4.9 (the release reports build times at 80–89% of 4.9 for large projects and a package 59% of the size).[^1]
- Use project references (`composite`, `tsc --build`) and `incremental` for monorepos; 5.0 allows `--declaration`, `--emitDeclarationOnly`, `--declarationMap`, `--sourceMap`, `--inlineSourceMap` on the `tsc -b` command line (4.9: TS5094).
- `skipLibCheck: true` and an explicit `types` list cut work.
- Diagnose with `tsc --extendedDiagnostics`, `--generateTrace <dir>` (open in `@typescript/analyze-trace` or a Chromium trace viewer), `--explainFiles` and `--traceResolution`.
- Large unions (thousands of members), deeply recursive conditional types and huge intersections dominate check time; prefer interfaces with `extends` over intersections of many object types.

***

#### Testing

- Run `tsc --noEmit` (or `tsc -b`) in CI as its own step; test runners that strip types (Vitest, esbuild-based Jest transforms, `tsx`) do **not** type-check.
- `// @ts-expect-error` is preferred over `// @ts-ignore`: it fails when the error disappears. Add a reason after it.
- Type-level tests: `expectTypeOf` (Vitest, `expect-type`) or `tsd`/`tstyche`; keep them in files included by `tsc`.

```ts
type Equal<A, B> = (<T>() => T extends A ? 1 : 2) extends (<T>() => T extends B ? 1 : 2) ? true : false;
type Expect<T extends true> = T;

declare function getNames<const T extends readonly string[]>(x: T): T;
const r = getNames(["a", "b"]);
type _t1 = Expect<Equal<typeof r, readonly ["a", "b"]>>;
// @ts-expect-error -- mutable tuple is not the inferred type
type _t2 = Expect<Equal<typeof r, ["a", "b"]>>;
export {};
```

- Test-runner compatibility for 5.0: `ts-jest` 29.x declares `typescript >=4.3 <6`/`<7`; `ts-node` 10.9.2 accepts `>=2.7`. Current Vitest (5.x) needs Node.js 22.12+ and does not type-check; check the toolchain's own Node.js requirements, not just TypeScript.
- Do not share mutable module state across tests; with ESM, mocking requires the runner's module mocking API (`vi.mock`, `jest.unstable_mockModule`), not reassignment of imports.

***

#### Tooling

##### `tsc` and builds

- `tsc` 5.0 runs on Node.js ≥ 12.20 (5.1 requires 14.17).[^5]
- `tsc -b` (build mode) builds project references in dependency order; **new in 5.0** it accepts `--declaration`, `--emitDeclarationOnly`, `--declarationMap`, `--sourceMap` and `--inlineSourceMap` on the command line, e.g. `tsc -b --declaration` for a release build while the tsconfig keeps declarations off.[^1] Other emit flags such as `--outDir` are still rejected with TS5094.
- `tsc --showConfig` prints the effective configuration after `extends` (now including arrays).
- `noEmitOnError: true` is recommended for `tsc`-emitted production builds; note that 5.0 deprecation *errors* still allow emit without it.
- The `typescript` npm package 5.0 dropped `typescriptServices.js` and `protocol.d.ts`; tools that import those paths (old ts-loader/fork-ts-checker versions, custom scripts) break.[^13]

##### Running TypeScript directly

- `ts-node` type-checks (slow) or transpiles only (`--transpileOnly`/`swc`); `tsx` and Node.js loaders use esbuild and never type-check.
- Node.js type stripping (`--experimental-strip-types` in 22.6+, on by default since 23.6/22.18) only removes erasable syntax: enums, runtime namespaces and parameter properties need `--experimental-transform-types` (Node.js 22–25; Node.js 26 no longer accepts that flag and rejects such files with `ERR_UNSUPPORTED_TYPESCRIPT_SYNTAX`), and decorators are not supported at all. A 5.0 code base using those must keep a compile step. `allowImportingTsExtensions` is required to write `./x.ts` specifiers that such runtimes load.

##### Linting

typescript-eslint 8.x (8.72.0, October 2026) supports TypeScript `>=4.8.4 <6.1.0`, so it covers 5.0; typescript-eslint 6.x supported `>=4.3.5 <5.4.0` and 7.x `>=4.7.4 <5.6.0`.[^16] Recommended rules for production: `no-floating-promises`, `no-misused-promises`, `await-thenable`, `no-unnecessary-type-assertion`, `consistent-type-imports` (or rely on `verbatimModuleSyntax`), `switch-exhaustiveness-check`, `no-explicit-any`, `strict-boolean-expressions` (for strict teams), `prefer-nullish-coalescing`, `no-unsafe-*`.

##### Editor and language service

- 5.0 auto-detects whether import lists are sorted case-sensitively or case-insensitively when organizing imports, so it no longer fights ESLint's `sort-imports`; finer control is experimental (`typescript.unstable.organizeImportsIgnoreCase`, `organizeImportsCollation`, …).[^1]
- Exhaustive `switch`/`case` completions scaffold missing cases for literal unions.
- Pin the workspace TypeScript in VS Code (`"typescript.tsdk": "node_modules/typescript/lib"`) so the editor reports the same 5.0 errors as CI — the bundled editor version is much newer and accepts 5.1+ syntax.
- 5.0.4 fixed *Go To Source Definition* under `moduleResolution: bundler` and two crashes (JSX recovery in unary expressions, private identifiers in array assignments).[^3]

***

#### Observability and Debugging

- Emit source maps (`sourceMap` or `inlineSourceMap`, plus `inlineSources` when the sources are not deployed) and run Node.js with `--enable-source-maps` so stack traces point to `.ts` lines.
- `declarationMap` makes "Go to definition" jump into library sources in monorepos.
- Keep error classes with stable `name`/`code` properties for structured logging; log `cause` chains.
- Standard method decorators are a clean way to add tracing/metrics around methods (see `logged` above); keep them generic-typed so they do not erase signatures.
- Compiler diagnostics: `--extendedDiagnostics`, `--generateTrace`, `--traceResolution`, `--explainFiles`, `--listFilesOnly`.

***

#### 5.0.x Patch Releases and Known Issues

| Release | Date | Notable fixes |
|---|---|---|
| 5.0.2 | March 16, 2023 | first stable release |
| 5.0.3 | March 30, 2023 | `extends` resolving packages from `node_modules` again (#53314); wildcard `exports` in tsconfig lookup (#53443); crash with `verbatimModuleSyntax` (#53302) and its check on `.d.ts` files (#53227); missing ambient check in the `verbatimModuleSyntax` `export =` error (#53385); regression where an unused type-guard result changed an inferred type (#53311); assertion functions/type guards on `Record` types (#53379); double constructor emit with decorated getters (#53448); subtype reduction fix (#53388) |
| 5.0.4 | April 7, 2023 | `isolatedModules` + `verbatimModuleSyntax` no longer an error (#53601); `allowImportingTsExtensions` ignored by `transpileModule` (#53599); Go To Source Definition under `bundler` (#53613); crashes with JSX recovery in unary expressions (#53114/#53666) and private identifiers in array assignments (#53665) |

Known in 5.0.4 (verified): the `Debug Failure` crash when `verbatimModuleSyntax` and `importsNotUsedAsValues: "error"` are combined (see [Deprecations](#new-in-50-deprecations-and-ignoredeprecations)). Always use 5.0.4 if 5.0 is required.[^3]

***

#### Removed / Changed in 5.0 — Migration Notes

1. **Upgrade to 5.0.4**, not 5.0.2/5.0.3, and pin with `~5.0.4`.
2. **Deprecated options**: remove or replace each option in the deprecation table; add `"ignoreDeprecations": "5.0"` only as a temporary bridge. Replace `importsNotUsedAsValues`/`preserveValueImports` with `verbatimModuleSyntax` (ESM) or plain `isolatedModules` (CJS).
3. **Decorators**: if the project uses decorators written for the old proposal (NestJS, TypeORM, InversifyJS, MobX 6 legacy decorators, class-validator…), make sure `experimentalDecorators: true` is set — without it, 5.0 type-checks them as standard decorators and fails. Projects that want standard decorators must remove `experimentalDecorators` *and* `emitDecoratorMetadata` and rewrite every decorator.
4. **Enums**: fix new TS2322 errors from out-of-domain numeric literals and mixed string/number enum indirections.
5. **Relational operators**: convert `string | number` operands explicitly (`Number(x) > 4`).
6. **Constructor-parameter decorators** under `experimentalDecorators`: widen the `key` parameter type to include `undefined`.
7. **Runtime**: the compiler needs Node.js 12.20+; CI images with Node.js 10/12.0–12.19 must be upgraded.
8. **DOM lib**: some numeric DOM properties are now literal unions and clipboard-event members moved between interfaces; fix the type errors rather than casting.[^1]
9. **`newLine`/`forceConsistentCasingInFileNames`**: generated files may change line endings on Windows builds; imports with wrong casing now fail everywhere.
10. **Compiler API consumers**: the package is restructured (no `typescriptServices.js`, no `protocol.d.ts`, `ts.Map`/`ts.Set` removed, node factory functions only via `ts.factory`). Update ts-loader, fork-ts-checker-webpack-plugin, ts-morph, ts-patch and custom transformers to versions that support 5.0.[^13]
11. **Type packages**: pin `@types/node` and other `@types` to versions whose `typeScriptVersion` is ≤ 5.0 (see [`@types` packages](#types-packages-and-typescript-50)).

***

#### Idioms and Things to Avoid

| Avoid | Prefer |
|---|---|
| `as T` on parsed JSON | runtime validation (`unknown` → guard/schema) |
| `any` | `unknown` + narrowing, generics |
| `!` non-null assertions | narrowing, `??`, early return |
| `enum` for string sets from JSON | `as const` object + union |
| `x \|\| default` for numbers/strings | `x ?? default` |
| `catch (e: any)` | `catch (e)` with `unknown` |
| `importsNotUsedAsValues`, `preserveValueImports` | `verbatimModuleSyntax` + `import type` |
| `moduleResolution: node`/`node10` for new code | `bundler` (apps) or `node16`/`nodenext` (Node, libraries) |
| `module: esnext` without `moduleResolution` | set both (otherwise `classic`) |
| standard decorators with `target: esnext` | `target: es2022` |
| `const T extends string[]` | `const T extends readonly string[]` |
| `function f(): undefined {}` | `: void`, or `return undefined;` (5.0) |
| `NoInfer`, `using`, `with { type: "json" }` and other 5.1+ features | the 5.0 workarounds in this guide |
| `@ts-ignore` | `@ts-expect-error` with a reason |
| `Function`, `Object`, `{}` as types | precise function/object types |
| method-syntax callbacks in interfaces | property syntax (`cb: (x: T) => void`) |
| floating promises | `await`, `void` with a comment, or explicit `.catch()` |

***

#### Production / Review Checklist

##### Configuration

- [ ] `typescript` pinned to `~5.0.4` (or a deliberate newer version); lockfile committed.
- [ ] `strict: true` plus `noUncheckedIndexedAccess`, `noImplicitOverride`, `noImplicitReturns`, `noFallthroughCasesInSwitch`.
- [ ] `module` **and** `moduleResolution` set explicitly; `bundler` only for bundled apps, `node16`/`nodenext` for Node.js and libraries.
- [ ] No deprecated options (table above); `ignoreDeprecations: "5.0"` only with a removal plan.
- [ ] No `verbatimModuleSyntax` together with `importsNotUsedAsValues` (5.0.4 crash).
- [ ] `target` matches the oldest runtime; not `esnext` when standard decorators are used; `lib` not newer than the runtime.
- [ ] `types` lists the needed `@types` packages; `@types/*` versions support TypeScript 5.0.
- [ ] `extends` arrays ordered general → specific; `tsc --showConfig` reviewed for surprises.

##### Types and code

- [ ] No `any` at boundaries; external data is `unknown` and validated.
- [ ] Every `as`, `!`, `@ts-expect-error` and type predicate is justified.
- [ ] `satisfies` instead of `as` for config tables.
- [ ] `const` type parameters have `readonly` constraints.
- [ ] Exhaustive `switch` over unions with `never` checks.
- [ ] Enums: no out-of-domain numeric values; no exported `const enum` in libraries.
- [ ] No 5.1+ features (`using`, `NoInfer`, import attributes, mixed tuple labels, union-array `filter`, `toSorted` types, `target: es2023`).

##### Decorators and classes

- [ ] One decorator system per project; `experimentalDecorators` present iff the framework needs it; `emitDecoratorMetadata` only with it.
- [ ] Standard decorators are generically typed and do not depend on `context.metadata`.
- [ ] `useDefineForClassFields` behaviour understood when `target` ≥ ES2022 (`declare` for type-only field redeclarations).
- [ ] `#private` for runtime privacy.

##### Async and errors

- [ ] No floating promises; no `async` callbacks in `forEach`; lint rules enabled.
- [ ] Timeouts and cancellation via `AbortController`; `finally` clears timers and listeners.
- [ ] `return await` inside `try` blocks that clean up.
- [ ] Errors wrapped with `cause`; `catch` variables treated as `unknown`.

##### Security

- [ ] Input validation at every trust boundary; no casts as validation.
- [ ] No prototype pollution via dynamic keys; `Map` or null-prototype objects for dictionaries.
- [ ] No `eval`/`new Function`/shell string interpolation; parameterised SQL.
- [ ] Secrets from environment/secret store; `process.env` checked at startup.

##### Build, tooling and observability

- [ ] `tsc --noEmit`/`tsc -b` runs in CI (type-stripping test runners do not type-check).
- [ ] Source maps enabled for production debugging.
- [ ] typescript-eslint version supports 5.0 (8.x does); editor uses the workspace TypeScript.
- [ ] Compiler API consumers (loaders, plugins, ts-morph) are on versions built for 5.0.
- [ ] Node.js in CI ≥ 12.20 for `tsc` and ≥ the runtime targeted by `lib`.

***

[^1]: Announcing TypeScript 5.0 / TypeScript 5.0 release notes. Microsoft. https://devblogs.microsoft.com/typescript/announcing-typescript-5-0/ , https://www.typescriptlang.org/docs/handbook/release-notes/typescript-5-0.html
[^2]: npm registry: typescript package versions and publish times (5.0.0-beta … 7.1.0-beta). https://registry.npmjs.org/typescript
[^3]: GitHub releases v5.0.3 and v5.0.4 and the closed issues/PRs of the "TypeScript 5.0.3" and "TypeScript 5.0.4" milestones. https://github.com/microsoft/TypeScript/releases/tag/v5.0.4 , https://github.com/microsoft/TypeScript/issues?q=milestone%3A%22TypeScript+5.0.4%22+is%3Aclosed
[^4]: DefinitelyTyped README — "Support window" (TypeScript versions less than two years old) and the `typeScriptVersion` field in published `@types` packages. https://github.com/DefinitelyTyped/DefinitelyTyped#support-window , https://registry.npmjs.org/@types/node
[^5]: typescript 5.0.4 package.json (`"engines": { "node": ">=12.20" }`) and the API Breaking Changes wiki (ES2018 / Node 12.20). https://github.com/microsoft/TypeScript/wiki/API-Breaking-Changes#typescript-50
[^6]: TypeScript release notes 3.7–4.9. https://www.typescriptlang.org/docs/handbook/release-notes/overview.html
[^7]: TypeScript 5.1 release notes. https://www.typescriptlang.org/docs/handbook/release-notes/typescript-5-1.html
[^8]: TypeScript 5.2 release notes. https://www.typescriptlang.org/docs/handbook/release-notes/typescript-5-2.html
[^9]: TypeScript 5.3 release notes. https://www.typescriptlang.org/docs/handbook/release-notes/typescript-5-3.html
[^10]: TypeScript 5.4 release notes. https://www.typescriptlang.org/docs/handbook/release-notes/typescript-5-4.html
[^11]: TypeScript 5.5 release notes. https://www.typescriptlang.org/docs/handbook/release-notes/typescript-5-5.html
[^12]: TypeScript 5.6, 5.7, 5.8 and 5.9 release notes. https://www.typescriptlang.org/docs/handbook/release-notes/typescript-5-6.html , https://www.typescriptlang.org/docs/handbook/release-notes/typescript-5-7.html , https://www.typescriptlang.org/docs/handbook/release-notes/typescript-5-8.html , https://www.typescriptlang.org/docs/handbook/release-notes/typescript-5-9.html
[^13]: TypeScript wiki — API Breaking Changes, TypeScript 5.0 section. https://github.com/microsoft/TypeScript/wiki/API-Breaking-Changes
[^14]: TypeScript 5.0.4 `lib/lib.es2023.array.d.ts` and the TSConfig reference for `target`/`lib`. https://www.typescriptlang.org/tsconfig#target
[^15]: TC39 proposal-decorators (stage 3 at the time of 5.0) and the implementing pull request microsoft/TypeScript#50820. https://github.com/tc39/proposal-decorators , https://github.com/microsoft/TypeScript/pull/50820
[^16]: typescript-eslint — supported TypeScript versions (`SUPPORTED_TYPESCRIPT_VERSIONS` in `@typescript-eslint/typescript-estree` 6.21.0, 7.18.0 and 8.72.0). https://typescript-eslint.io/users/dependency-versions/
