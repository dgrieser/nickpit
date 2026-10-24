### TypeScript 5.9: The Complete Production Developer Guide

#### Overview

> **Version**: TypeScript 5.9. The stable release **5.9.2** was published to npm on July 31, 2025 and announced on August 1, 2025 (5.9.0-beta: July 8, 2025; 5.9.1-rc: July 24, 2025). The latest and **final** patch release is **5.9.3**, published to npm on September 30, 2025 (the GitHub release is dated October 1, 2025).
> **Status as of October 2026**: 5.9 is the **last 5.x release** and is no longer updated. TypeScript has no LTS lines; fixes ship only in new releases. **TypeScript 6.0** (6.0.2, March 23, 2026; latest 6.0.3, April 16, 2026) is the last release of the JavaScript-based compiler and the bridge to **TypeScript 7.0**, the native (Go) compiler, released July 8, 2026 (latest 7.0.2; 7.1 is in beta). DefinitelyTyped tests `@types` packages only against TypeScript versions less than two years old, so 5.9 leaves that window around August 2027.
> **Scope**: the TypeScript language and type system as usable in 5.9, the compiler options and module system, the 5.9 delta over 5.8, the deprecations that become errors in 6.0, idioms, pitfalls and a review checklist for production code.
> **Note**: this document is self-contained. Everything from earlier releases that 5.9 supports is carried forward. Sections that cover 5.9-specific behaviour are marked **New in 5.9**. All examples were compiled with `tsc` 5.9.3 (and boundary claims with 5.8.3 and 6.0.3); the default checking configuration is `strict`, `noUncheckedIndexedAccess`, `noImplicitOverride`, `noImplicitReturns`, `verbatimModuleSyntax`, `module: nodenext`, `target: esnext`, unless an example states otherwise with a `// @option: value` line (the same convention the TypeScript playground uses).

Global truths about TypeScript 5.9:

- **Types are erased.** TypeScript checks at compile time and emits JavaScript; annotations, interfaces and type aliases do not exist at runtime. A type assertion (`as`) never converts or validates a value. Data that crosses a trust boundary (HTTP, JSON, files, environment, `postMessage`) must be validated at runtime.
- **Structural typing.** Compatibility is decided by shape, not by declared name. Two unrelated interfaces with the same members are interchangeable.
- **Soundness is not a goal.** `any`, type assertions, non-null assertions (`!`), method parameter bivariance, unchecked index access (without `noUncheckedIndexedAccess`) and declaration files that lie are all holes. A reviewer must treat each one as an unverified claim.
- **The compiler is the 5.9 `tsc`, written in TypeScript and running on Node.js.** The native Go compiler (previewed as `tsgo`, shipped as TypeScript 7.0) is a different, newer product with a reduced option set and no programmatic API in 7.0.
- **`tsc` defaults are old.** Without a tsconfig, 5.9 defaults to `target: es5`, `module: commonjs`, non-strict checking and automatic inclusion of every `@types/*` package. Real projects must set these explicitly (6.0 changes several defaults; see *Do not use or recommend*).
- **Emit is not bundling.** `tsc` never rewrites import specifiers (except with `rewriteRelativeImportExtensions`), never adds polyfills, and never resolves `paths` aliases at runtime.

***

#### What's New in TypeScript 5.9

TypeScript 5.9 is a small release. There were no changes between the release candidate and the stable release.

| Change | Kind | Section |
|---|---|---|
| `import defer * as ns from "..."` (TC39 deferred module evaluation) | Syntax | [import defer](#new-in-59-import-defer) |
| `import.defer("...")` dynamic form (not in the release notes; accepted and type-checked by 5.9.3) | Syntax | [import defer](#new-in-59-import-defer) |
| `--module node20` (stable Node.js 20 module mode; implies `target: es2023`) | Compiler option | [Module modes](#module-and-moduleresolution-modes) |
| `nodenext` and `node20` now imply `resolveJsonModule` (5.8 `nodenext` did not) | Compiler option | [Module modes](#module-and-moduleresolution-modes) |
| Minimal, prescriptive `tsc --init` output | Tooling | [tsc --init](#new-in-59-tsc---init-output) |
| `lib.d.ts`: `ArrayBuffer` is no longer a structural supertype of typed arrays, `SharedArrayBuffer` is no longer assignable to `ArrayBuffer` (even with `lib` below `es2024`), and `Uint8Array<ArrayBufferLike>` is no longer a `BufferSource` | Breaking lib change | [ArrayBuffer](#new-in-59-arraybuffer-typed-arrays-and-buffer) |
| `lib.esnext`: `Error.isError()` (`esnext.error`) and `Atomics.pause()` (`esnext.sharedmemory`) | Lib | [Target and lib](#target-and-lib) |
| `lib.es2022.intl`: `Intl.Segments.prototype.containing()` returns `SegmentData \| undefined` (5.8: `SegmentData`); not in the release notes | Breaking lib change | [Target and lib](#target-and-lib) |
| Type-argument inference no longer "leaks" type variables | Breaking checker change | [Generics](#new-in-59-type-argument-inference-changes) |
| DOM APIs carry MDN summary descriptions in hovers | Editor | [Tooling](#editor-and-language-service) |
| Expandable hovers (preview) and configurable maximum hover length | Editor | [Tooling](#editor-and-language-service) |
| Caching of instantiations on type mappers; fewer closures in file-existence checks | Performance | [Compiler performance](#compiler-performance) |

Everything earlier remains available and should be used where it fits: `satisfies` (4.9); standard decorators, `const` type parameters, `--moduleResolution bundler`, `--verbatimModuleSyntax` and union enums (5.0); unrelated getter/setter types (5.1); `using`/`await using` and decorator metadata (5.2); import attributes and `switch (true)` narrowing (5.3); `NoInfer<T>` and narrowing preserved in closures after the last assignment (5.4); inferred type predicates, `isolatedDeclarations`, regular-expression syntax checking, JSDoc `@import` and `Set` method types (5.5); disallowed always-truthy/always-nullish checks, `strictBuiltinIteratorReturn`, `--noUncheckedSideEffectImports`, `--noCheck` and iterator-helper types (5.6); never-initialized-variable checks, `--rewriteRelativeImportExtensions`, `target: es2024` and generic typed arrays (5.7); `require()` of ESM under `nodenext`, `--module node18`, `--erasableSyntaxOnly`, `--libReplacement` and per-branch checking of conditional `return` expressions (5.8).

##### Do not use or recommend (newer than 5.9)

The project's compiler is 5.9. **Never suggest any of the following** — they do not exist in 5.9 or behave differently:

- **TypeScript 6.0 options and values**: `target`/`lib` value `es2025` (5.9 rejects it with TS6046), `lib: ["esnext.temporal"]` and the `Temporal` global types, `RegExp.escape` types, `Map`/`WeakMap` `getOrInsert()`/`getOrInsertComputed()` types, `Uint8Array` `toBase64()`/`fromBase64()`/`toHex()` types (`esnext.typedarrays` in 6.0), `--stableTypeOrdering`, `--ignoreConfig`, `"ignoreDeprecations": "6.0"` (5.9 reports TS5103 *Invalid value*), `"types": ["*"]` (5.9 treats `*` as a package name and fails with TS2688), `#/` subpath-import patterns (5.9 cannot resolve `"#/*"` keys; `#name/*` works), and `moduleResolution: bundler` together with `module: commonjs` (TS5095 in 5.9).
- **TypeScript 6.0 default changes**: in 5.9 `strict` is **off** by default, `module` defaults to `commonjs`, `target` to `es5`, `types` to "all visible `@types` packages", `noUncheckedSideEffectImports` to `false`, `libReplacement` to `true`, and `rootDir` is inferred from the input files. Do not assume 6.0's defaults (`strict: true`, `module: esnext`, `target: es2025`, `types: []`, `rootDir` = tsconfig directory) — a 5.9 tsconfig must spell them out.
- **TypeScript 6.0 lib merges**: in 5.9 `"dom"` does **not** include `dom.iterable`/`dom.asynciterable`; `for (const el of document.querySelectorAll("div"))` fails with TS2488 unless `"dom.iterable"` is listed. Iterator helpers, `Set` methods, `Promise.try`, `Float16Array`/`Math.f16round` are only in `lib: esnext` in 5.9 (6.0 moves them to `es2025`).
- **TypeScript 6.0 checker changes**: the "functions that do not use `this` are not context-sensitive" inference improvement and the 6.0 deprecation errors (see [Deprecations that become errors in 6.0](#deprecations-that-become-errors-in-60)) do not apply to 5.9.
- **TypeScript 7.0**: the native Go compiler (the preview binary was `tsgo`; 7.0's npm package ships it as `tsc`), its LSP-based editor extension, and its removal of every option deprecated in 6.0. TypeScript 7.0 ships no compiler API; tools built on the `typescript` package API (typescript-eslint, ts-morph, many bundler plugins) need the 5.x/6.0 JavaScript compiler.
- **Runtime features 5.9 has no types for** (even with `lib: esnext`): `Temporal`, `RegExp.escape`, `Uint8Array.prototype.toBase64`, `Map.prototype.getOrInsert`. Code that uses them needs local `declare global` augmentations — flag such augmentations for review.

Deprecated or removed **older** features that 5.9 still rejects or warns about:

- The options deprecated in 5.0 are **removed** and fail with TS5102/TS5108 in 5.9 (since 5.6); `"ignoreDeprecations": "5.0"` is still accepted as a value but no longer silences them: `importsNotUsedAsValues` (except the no-op value `"remove"`), `preserveValueImports`, `target: es3`, `out`, `noImplicitUseStrict`, `keyofStringsOnly`, `suppressExcessPropertyErrors`, `suppressImplicitAnyIndexErrors`, `noStrictGenericChecks`, `charset`, and `prepend` in project references. Replace the first two with `verbatimModuleSyntax`.
- Import **assertions** (`assert { type: "json" }`) are an error under `--module node20` and `nodenext` (TS2880); 5.9 still accepts them silently under `esnext`, `preserve` and `node18`, but 6.0 rejects them everywhere. Use import **attributes** (`with { type: "json" }`).
- Options and syntax that still work in 5.9 but **are errors in 6.0 and removed in 7.0**: `target: es5`, `downlevelIteration`, `moduleResolution: node`/`node10` and `classic`, `module: amd`/`umd`/`system`/`none`, `outFile`, `baseUrl`, `esModuleInterop: false`, `allowSyntheticDefaultImports: false`, `alwaysStrict: false`, the `module Foo {}` namespace keyword, `/// <reference no-default-lib="true"/>`. New code must not introduce them (details in [Deprecations that become errors in 6.0](#deprecations-that-become-errors-in-60)).

##### Version-aware review

- **Check the pinned version first.** `package.json` (`"typescript": "~5.9.3"`), the lockfile and `npx tsc -v` decide what is available. A caret range `^5.9.0` cannot resolve to 6.0, but a bare `latest` or `*` can resolve to 7.0, whose `tsc` is the native compiler.
- **Verify against 5.9 before claiming something is missing.** Many "new" APIs exist in 5.9 only under `lib: ["esnext"]` (iterator helpers, `Set` methods, `Promise.try`, `Error.isError`, `Float16Array`, `DisposableStack`, `Array.fromAsync`). Say "not available in 5.9" only when the 5.9 lib or checker proves it.
- **Distinguish type availability from runtime availability.** `lib` only declares types. `Error.isError` type-checks with `lib: esnext` in 5.9 but exists only on Node.js 24+; `using` declarations type-check everywhere but run natively only on Node.js 24+ (older targets need TypeScript's downlevel emit). Match `lib` to the oldest runtime the code supports.
- **Do not "upgrade" to 6.0 idioms in review comments** for a 5.9 project (e.g., do not delete an explicit `"strict": true` because "it is the default", do not suggest `types: ["*"]`, do not suggest `es2025`). Suggest migration-friendly changes that are valid in 5.9 and 6.0, such as replacing `baseUrl`, `moduleResolution: node` and `assert` imports.

***

#### Compiler Configuration (tsconfig.json)

##### New in 5.9: `tsc --init` output

`tsc --init` in 5.9 writes a 44-line `tsconfig.json` (5.8 wrote 113 lines, mostly comments) with prescriptive settings:

```jsonc
{
  // Visit https://aka.ms/tsconfig to read more about this file
  "compilerOptions": {
    // File Layout
    // "rootDir": "./src",
    // "outDir": "./dist",

    // Environment Settings
    // See also https://aka.ms/tsconfig/module
    "module": "nodenext",
    "target": "esnext",
    "types": [],
    // For nodejs:
    // "lib": ["esnext"],
    // "types": ["node"],
    // and npm install -D @types/node

    // Other Outputs
    "sourceMap": true,
    "declaration": true,
    "declarationMap": true,

    // Stricter Typechecking Options
    "noUncheckedIndexedAccess": true,
    "exactOptionalPropertyTypes": true,

    // Style Options
    // "noImplicitReturns": true,
    // "noImplicitOverride": true,
    // "noUnusedLocals": true,
    // "noUnusedParameters": true,
    // "noFallthroughCasesInSwitch": true,
    // "noPropertyAccessFromIndexSignature": true,

    // Recommended Options
    "strict": true,
    "jsx": "react-jsx",
    "verbatimModuleSyntax": true,
    "isolatedModules": true,
    "noUncheckedSideEffectImports": true,
    "moduleDetection": "force",
    "skipLibCheck": true,
  }
}
```

Review the generated file before committing it — the defaults surprise people (all verified on 5.9.3):

- **`"types": []` hides `@types/node`.** With it, `import { readFileSync } from "node:fs"` fails with TS2307 *Cannot find module 'node:fs'* and `process` fails with TS2591. Node.js projects must set `"types": ["node"]` (and install `@types/node`).
- **`"module": "nodenext"` follows `package.json` `"type"`.** Without `"type": "module"`, `.ts` files are CommonJS, and with `verbatimModuleSyntax` every `import`/`export` becomes an error (TS1295, TS1287). Add `"type": "module"` or use `.mts`.
- **`"target": "esnext"` floats.** Each TypeScript upgrade can emit newer syntax. Pin a concrete target (`es2023` for Node.js 20+, `es2024` for Node.js 22+) for code that ships to a fixed runtime.
- **`"jsx": "react-jsx"`** is irrelevant for non-React projects but harmless; it requires `react/jsx-runtime` types when `.tsx` files exist.
- **`"exactOptionalPropertyTypes": true`** changes the meaning of `?:` (see [Optional properties](#optional-properties-and-exactoptionalpropertytypes)) and breaks code that assigns `undefined` explicitly; enabling it in an existing codebase is a migration, not a formality.
- The file uses JSONC with trailing commas, which `tsc` accepts but strict JSON parsers in other tools may not.

##### Recommended baseline

```jsonc
{
  "compilerOptions": {
    // Type checking
    "strict": true,                         // must be explicit in 5.9 (default off)
    "noUncheckedIndexedAccess": true,
    "noImplicitOverride": true,
    "noImplicitReturns": true,
    "noFallthroughCasesInSwitch": true,
    "noPropertyAccessFromIndexSignature": true,
    "exactOptionalPropertyTypes": true,     // new projects; migration for old ones
    "useUnknownInCatchVariables": true,     // already implied by strict
    "noUncheckedSideEffectImports": true,   // 5.6+, default false in 5.9

    // Modules
    "module": "nodenext",                   // or "node20", or "preserve" for bundlers
    "verbatimModuleSyntax": true,
    "isolatedModules": true,
    "moduleDetection": "force",

    // Emit / environment
    "target": "es2023",                     // match the oldest runtime
    "lib": ["es2023"],                      // add "dom", "dom.iterable" for browsers
    "types": ["node"],                      // explicit: avoid loading every @types package
    "rootDir": "src",
    "outDir": "dist",
    "sourceMap": true,
    "declaration": true,
    "declarationMap": true,

    // Hygiene
    "forceConsistentCasingInFileNames": true, // default true since 5.0
    "skipLibCheck": true,
    "libReplacement": false                 // 5.8+, avoids @typescript/lib-* lookups
  },
  "include": ["src"]
}
```

`strict` enables `noImplicitAny`, `strictNullChecks`, `strictFunctionTypes`, `strictBindCallApply`, `strictPropertyInitialization`, `noImplicitThis`, `useUnknownInCatchVariables`, `alwaysStrict` and `strictBuiltinIteratorReturn` (5.6). Individual flags can be turned off after `strict`; a review should ask why whenever one is.

##### Strictness flags that `strict` does not enable

| Flag | Effect | Review note |
|---|---|---|
| `noUncheckedIndexedAccess` | `arr[i]`, `record[key]` become `T \| undefined` | The single most valuable extra flag; catches off-by-one and missing-key bugs |
| `exactOptionalPropertyTypes` | `p?: T` no longer accepts an explicit `undefined` | Distinguishes "absent" from "present but undefined" (`in`, `Object.keys`, spreads) |
| `noImplicitOverride` | Overriding members need `override` | Catches renamed base methods |
| `noPropertyAccessFromIndexSignature` | Index-signature keys need `obj["key"]` | Makes dynamic keys visible |
| `noImplicitReturns` | Every code path must return | |
| `noFallthroughCasesInSwitch` | Non-empty `case` must `break`/`return` | |
| `noUnusedLocals` / `noUnusedParameters` | Unused declarations are errors | Often delegated to the linter |
| `noUncheckedSideEffectImports` (5.6) | `import "./x.js"` must resolve | Catches typos in side-effect imports; needs `declare module "*.css"` for assets |
| `allowUnreachableCode: false` | Unreachable code is an error (default: suggestion) | |
| `erasableSyntaxOnly` (5.8) | Rejects enums, runtime namespaces, parameter properties, `import =`/`export =` | Required for Node.js type stripping |
| `isolatedDeclarations` (5.5) | Exported API needs explicit types | For libraries with parallel declaration emit |

##### Optional properties and `exactOptionalPropertyTypes`

```ts
// @exactOptionalPropertyTypes: true
interface Options {
  timeoutMs?: number;
  label?: string | undefined; // explicitly allows undefined
}

const a: Options = {};                       // OK: property absent
const b: Options = { timeoutMs: undefined }; // error TS2375 under exactOptionalPropertyTypes
const c: Options = { label: undefined };     // OK: undefined is declared

function merge(base: Options, patch: Options): Options {
  return { ...base, ...patch };              // a present-but-undefined key would overwrite base
}
export { a, c, merge };
```

Without the flag, `{ ...defaults, ...userOptions }` silently replaces a default with `undefined` when the caller passes `{ timeoutMs: undefined }`. With the flag, such objects cannot be built without saying so in the type.

##### Target and lib

`target` controls syntax downlevelling *and* the default `lib`. 5.9 accepts `es5`, `es2015` … `es2024`, `esnext` (no `es2025`). `lib` only declares types; it never polyfills.

| Runtime | Suggested `target` / `lib` in 5.9 |
|---|---|
| Node.js 20 (EOL April 30, 2026) | `es2023` (what `--module node20` implies) |
| Node.js 22 (maintenance LTS until April 2027) | `es2024` |
| Node.js 24 (maintenance LTS since October 20, 2026) / 26 (Current; LTS from October 28, 2026) | `es2024` plus selected `esnext.*` libs for APIs the runtime has |
| Evergreen browsers via bundler | `es2022`–`es2024`, `lib: ["es2024", "dom", "dom.iterable"]` |

Node.js release dates are from the Node.js release schedule.

API availability in 5.9's lib files (verified by compiling each call):

| API | `lib` needed in 5.9 | Runtime (Node.js) |
|---|---|---|
| `Object.groupBy`, `Map.groupBy`, `Promise.withResolvers`, `ArrayBuffer.prototype.transfer`/`resize`, `String.prototype.isWellFormed`, `Atomics.waitAsync`, regex `v` flag | `es2024` | 22 |
| `Array.prototype.toSorted`/`toReversed`/`with`/`findLast` | `es2023` | 20 |
| Iterator helpers (`Iterator.from`, `.map()`, `.toArray()` on iterators), `Set.prototype.union` etc. | `esnext` (6.0: `es2025`) | 22 |
| `Promise.try`, `Float16Array`, `Math.f16round` | `esnext` (6.0: `es2025`) | 24 |
| `Disposable`, `DisposableStack`, `Symbol.dispose` types, `Array.fromAsync` | `esnext` | `Symbol.dispose`: 18+; `Array.fromAsync`: 22; `DisposableStack`: 24 |
| **New in 5.9**: `Error.isError()` (`esnext.error`), `Atomics.pause()` (`esnext.sharedmemory`) | `esnext` | 24 |
| `Temporal`, `RegExp.escape`, `Uint8Array.prototype.toBase64`, `Map.prototype.getOrInsert` | **not in 5.9** (6.0) | `RegExp.escape`: 24; the others: 26 |

```ts
// @lib: ["es2024"]
const s = new Set([1, 2]);
s.union(new Set([3]));                            // error TS2550: needs lib esnext in 5.9
[1, 2].values().map((x) => x * 2);                // error TS2339: iterator helpers need esnext
const groups = Object.groupBy([1, 2, 3], (n) => (n % 2 ? "odd" : "even")); // OK in es2024
export { groups };
```

With `lib: ["esnext"]` the first two lines compile — but `lib: esnext` also admits `Promise.try` and `Float16Array`, which Node.js 22 lacks (Node.js 20 lacks all of them). When in doubt, check `typeof Promise.try` on the deployed runtime; the runtime column above was checked on Node.js 20.20.2, 22.23.4, 24.22.1 and 26.12.1.

```ts
// @lib: ["es2022"]
// New in 5.9 (lib): containing() returns undefined for an index outside the string
const words = new Intl.Segmenter("en", { granularity: "word" }).segment("hello world");
const w: string = words.containing(100).segment;   // error TS2532 in 5.9 (5.8 typed it as SegmentData)
const w2: string = words.containing(0)?.segment ?? "";
export { w, w2 };
```

`Intl.Segments.prototype.containing(i)` really returns `undefined` when `i` is out of range (verified on Node.js 22); 5.9 fixes the type, so upgrades surface new TS2532/TS18048 errors at call sites that dereferenced the result unchecked — real bugs, not noise.

***

#### Modules

##### Module and moduleResolution modes

| `module` | Implied `target` (5.9) | `moduleResolution` | Use for |
|---|---|---|---|
| `nodenext` | `esnext` | `nodenext` | Node.js apps on the newest Node.js (floating) |
| `node20` (**New in 5.9**) | `es2023` | `node16` (must be `node16`/`nodenext`) | Libraries and apps that must stay stable across TS upgrades and run on Node.js 20+ |
| `node18` (5.8) | `es2022` | `node16` | Node.js 18 semantics (no `require(esm)`) |
| `node16` | `es2022` | `node16` | Legacy Node.js 16 semantics |
| `preserve` (5.4) | — | `bundler` | Code consumed by a bundler/runtime that understands TS (Vite, esbuild, Bun) |
| `esnext`/`es2022` + `moduleResolution: bundler` | — | `bundler` (must be set: without it 5.9 falls back to `classic`, which does not look in `node_modules`) | Same as above when `import x = require()` is not needed |
| `commonjs` | — | `node10` (**deprecated in 6.0**) | Legacy; prefer `node20`/`nodenext` |

Facts verified with `tsc --showConfig` and compilation on 5.9.3 and 5.8.3:

- `--module node20` is rejected by 5.8 (TS6046). It implies `target: es2023`, `moduleResolution: node16`, `resolveJsonModule: true`, `esModuleInterop`, `resolvePackageJsonExports`/`Imports` and `moduleDetection: force`.
- **New in 5.9**: `--module nodenext` now also implies `resolveJsonModule`; in 5.8, `import data from "./data.json" with { type: "json" }` under `nodenext` failed with TS2732 unless `resolveJsonModule` was set.
- `require()` of an ES module (`import m = require("./esm.mjs")` in a `.cts` file) is allowed under `nodenext` (since 5.8) and `node20`; it fails with TS1471 under `node16`/`node18`. Node.js unflagged `require(esm)` in 22.12 and 20.19; modules with top-level `await` still cannot be `require`d.
- Import assertions (`assert { type: "json" }`) are an error under `node20` and `nodenext` (TS2880) and allowed under `node18` (6.0 rejects them under `node18` too).
- `moduleResolution: bundler` requires `module: preserve` or `es2015`+ in 5.9 (TS5095).
- `moduleResolution: node20` does not exist; with `module: node20`, setting `moduleResolution: bundler` is TS5109.

##### Which files are ESM? (`nodenext`/`node20`)

Under the Node modes, each file's format follows Node.js rules: `.mts` → ESM, `.cts` → CommonJS, `.ts`/`.tsx` → the nearest `package.json` `"type"` (`"module"` → ESM, absent or `"commonjs"` → CommonJS). The emitted syntax follows the format: an `import` in a CommonJS-format `.ts` file is emitted as `require()`.

```ts
// @filename: lib.ts
export function add(a: number, b: number): number {
  return a + b;
}
// @filename: main.ts
import { add } from "./lib";      // error TS2835: relative ESM imports need an extension
import { add as add2 } from "./lib.js"; // OK: write the *output* extension
console.log(add2(1, 2));
```

Write relative imports with the runtime extension (`.js` for `.ts`, `.mjs` for `.mts`, `.cjs` for `.cts`). Writing `./lib.ts` requires `allowImportingTsExtensions` (only with `noEmit`/`emitDeclarationOnly`) or `rewriteRelativeImportExtensions` (5.7), which rewrites `./lib.ts` to `./lib.js` in emitted JS for relative paths only.

##### `verbatimModuleSyntax` and type-only imports

With `verbatimModuleSyntax` (5.0), what you write is what is emitted: imports without `type` are kept, imports marked `type` are dropped. This is what Node.js type stripping, esbuild, swc and Babel assume (single-file transforms cannot know whether a name is a type).

```ts
// @filename: model.ts
export interface User { id: string; name: string }
export const ANON: User = { id: "0", name: "anon" };
// @filename: main.ts
import { User } from "./model.js";              // error TS1484: must be a type-only import
import { ANON, type User as U } from "./model.js"; // OK: inline type modifier
import type { User as U2 } from "./model.js";     // OK: whole import erased
const u: U = ANON;
const v: U2 = u;
console.log(v);
```

`import { type A, type B } from "x"` keeps an empty `import {} from "x"` (and therefore the module's side effects) in the output; `import type { A, B } from "x"` removes the statement entirely. Pick the latter when the module must not be loaded at runtime.

Under `nodenext`, `verbatimModuleSyntax` forbids ESM syntax in CommonJS-format files (TS1295 for `import`, TS1287 for `export`); use `import x = require("x")` and `export =` there, or switch the file to ESM.

##### Import attributes and JSON

```ts
// @filename: data.json
{ "port": 8080 }
// @filename: main.ts
import config from "./data.json" with { type: "json" };       // OK under node20/nodenext (5.9 implies resolveJsonModule)
import legacy from "./data.json" assert { type: "json" };      // error TS2880: assertions replaced by attributes
const port: number = config.port;
console.log(port, legacy);
```

Under `nodenext`/`node20`, a JSON import from an ES module **must** carry `with { type: "json" }` (TS1543 otherwise), and only the default import is available — named imports of JSON keys are an error (TS1544) because Node.js does not provide them.

##### New in 5.9: `import defer`

`import defer` (TC39 "Deferring Module Evaluation", Stage 3) loads a module and its dependencies but delays *executing* it until an export of the namespace is first accessed.

```ts
// @module: esnext
// @moduleResolution: bundler
// @filename: feature.ts
console.log("feature module evaluated");
export const answer = 42;
// @filename: main.ts
import defer * as feature from "./feature.js";
// feature.ts has been loaded and linked, but not executed yet
export function useFeature(): number {
  return feature.answer;                     // module body runs here, synchronously, on first access
}
```

Rules verified on 5.9.3:

- Only the namespace form is allowed. `import defer { answer } from "./feature.js"` is TS18059 and `import defer answer from ...` is TS18058.
- It compiles **only** with `--module esnext` or `--module preserve`. Under `nodenext`, `node20`, `es2022` or `commonjs` it is TS18060 *Deferred imports are only supported when the '--module' flag is set to 'esnext' or 'preserve'*. 5.8 does not parse it (TS1005).
- TypeScript does **not** downlevel it; the statement is emitted unchanged. **No Node.js release (checked through 26.12.1) parses `import defer`**: running the output is a `SyntaxError`. Use it only with a bundler or runtime that implements the proposal (for example Babel's `@babel/plugin-proposal-import-defer`), and verify that the toolchain actually defers evaluation rather than silently importing eagerly.
- Not in the release notes: 5.9.3 also accepts and type-checks the dynamic form `const ns = await import.defer("./feature.js")` under the same `--module` restriction (5.8 rejects it with TS17012).
- Errors in a deferred module are thrown at the first property access — possibly far from the import, and inside code that does not expect it (a getter, a hot path). Reviewers should check that evaluation errors and latency at first access are acceptable.
- `defer` remains a valid identifier: `import defer from "./x.js"` is an ordinary default import named `defer`.

##### Side-effect imports

`import "./polyfills.js"` is not checked by default: a typo silently compiles. With `noUncheckedSideEffectImports` (5.6), unresolvable side-effect imports are TS2307. Declare non-code assets explicitly:

```ts
// @noUncheckedSideEffectImports: true
// @module: esnext
// @moduleResolution: bundler
// @filename: globals.d.ts
declare module "*.css" {}
// @filename: main.ts
import "./styles.css";             // OK: matched by the ambient declaration
import "./polyfils.js";            // error TS2307: typo is caught
export {};
```

##### Path aliases and `baseUrl`

`paths` only tells the *type checker* how to resolve; it does not change emitted specifiers. The runtime (Node.js, bundler, test runner) must resolve the same alias. For Node.js, prefer `package.json` `"imports"` (`"#lib/*": "./dist/lib/*"`), which both Node.js and `tsc` (`node16`+/`bundler`) understand. Avoid adding `baseUrl`: it is deprecated in 6.0; `paths` works without it (entries are resolved relative to the tsconfig) since 4.1. In 5.9, subpath imports must start with `#` followed by a non-`/` character (`#lib/*`); the `#/*` form is a 6.0 addition.

```ts
// @nocheck
// BAD — compiles, crashes at runtime with ERR_MODULE_NOT_FOUND unless a bundler rewrites it
// tsconfig: "paths": { "@app/*": ["./src/*"] }
import { db } from "@app/db.js";

// GOOD — resolved by tsc and by Node.js
// package.json: "imports": { "#app/*": { "types": "./src/*", "default": "./dist/*" } }
import { db } from "#app/db.js";
```

##### CommonJS interop

- `esModuleInterop` (implied by Node modes) makes `import fs from "fs"`-style default imports of CommonJS modules work by emitting `__importDefault`. Do not turn it off: `esModuleInterop: false` is deprecated in 6.0.
- In a CommonJS-format file under `nodenext`/`node20`, `import("./esm.mjs")` is emitted as a real dynamic `import()`, so it can load ESM. Under `module: commonjs` the same call is rewritten to `Promise.resolve().then(() => require("./esm.mjs"))`, which cannot load ES modules that use top-level `await` and fails on runtimes without `require(esm)` — one more reason to move CommonJS projects to `node20`/`nodenext`.
- `export =` and `import x = require()` are TypeScript-only CommonJS syntax; they are rejected by `erasableSyntaxOnly` and by Node.js type stripping.
- Dual packages: declare `"exports"` with `"types"` conditions first in each branch, and ship separate `.d.mts`/`.d.cts` when ESM and CJS builds differ. Check published packages with `@arethetypeswrong/cli` (`attw --pack`).


##### Module augmentation and ambient declarations

```ts
// @filename: express-like.ts
export interface Request { url: string }
// @filename: augment.ts
import type {} from "./express-like.js";
declare module "./express-like.js" {
  interface Request { userId?: string }       // merges into the module's interface
}
// @filename: env.d.ts
declare global {
  namespace NodeJS {
    interface ProcessEnv { DATABASE_URL?: string }
  }
}
export {};
// @filename: main.ts
import type { Request } from "./express-like.js";
function handler(req: Request): string {
  return req.userId ?? "anonymous";
}
const db: string = process.env.DATABASE_URL;     // error TS2322: string | undefined
export { handler, db };
```

- An augmentation must be inside a module (a file with `import`/`export`) and must target an existing module specifier; otherwise it declares a new ambient module instead of merging.
- Typing `ProcessEnv` keys as `string` (non-optional) is a lie unless startup code validates them. Declare them optional and validate once at startup, exporting a typed config object.
- `declare module "*.svg" { const url: string; export default url; }` wildcard declarations are a promise that the bundler handles that extension.

***

#### Project Layouts

##### Node.js application or library (ESM, `tsc` emit)

```jsonc
// package.json
{
  "type": "module",
  "exports": {
    ".": { "types": "./dist/index.d.ts", "default": "./dist/index.js" }
  },
  "imports": { "#internal/*": "./dist/internal/*" },
  "engines": { "node": ">=20.19" },
  "devDependencies": { "typescript": "~5.9.3", "@types/node": "^20.19.0" }
}
```

```jsonc
// tsconfig.json
{
  "compilerOptions": {
    "module": "node20",              // stable semantics; implies target es2023
    "target": "es2023",
    "lib": ["es2023"],
    "types": ["node"],
    "strict": true,
    "noUncheckedIndexedAccess": true,
    "verbatimModuleSyntax": true,
    "isolatedDeclarations": true,    // libraries
    "declaration": true,
    "declarationMap": true,
    "sourceMap": true,
    "rootDir": "src",
    "outDir": "dist",
    "skipLibCheck": true
  },
  "include": ["src"]
}
```

Keep `@types/node` at the major of the **oldest** supported Node.js (here 20), otherwise the code type-checks against APIs the runtime lacks.

##### Bundled application (Vite, esbuild, webpack, Bun)

```jsonc
{
  "compilerOptions": {
    "module": "preserve",            // implies moduleResolution: bundler
    "target": "es2022",
    "lib": ["es2023", "dom", "dom.iterable"],
    "types": ["vite/client"],
    "jsx": "react-jsx",
    "noEmit": true,                  // the bundler emits; tsc only checks
    "strict": true,
    "noUncheckedIndexedAccess": true,
    "verbatimModuleSyntax": true,
    "isolatedModules": true,
    "allowImportingTsExtensions": true,
    "skipLibCheck": true
  },
  "include": ["src"]
}
```

`moduleResolution: bundler` allows extensionless relative imports and honours `package.json` `exports`; it must not be used for code that Node.js runs directly (Node.js requires extensions).

##### Monorepo with project references

```jsonc
// packages/core/tsconfig.json
{ "compilerOptions": { "composite": true, "rootDir": "src", "outDir": "dist" }, "include": ["src"] }

// packages/api/tsconfig.json
{
  "compilerOptions": { "composite": true, "rootDir": "src", "outDir": "dist" },
  "references": [{ "path": "../core" }],
  "include": ["src"]
}

// tsconfig.json (root, solution file)
{ "files": [], "references": [{ "path": "packages/core" }, { "path": "packages/api" }] }
```

Build with `tsc -b` (add `--verbose` to see what is rebuilt). `composite` forces `declaration: true` and requires all inputs to be matched by `include`/`files`. Packages consume each other through `package.json` names and `exports`, not relative paths across package boundaries.

***

#### Naming and Code Style

- `camelCase` for variables, functions, methods and properties; `PascalCase` for types, interfaces, classes, enums and type parameters with meaning (`TKey`, `TValue`) — single letters (`T`, `K`, `V`) are fine for short generic helpers.
- `UPPER_SNAKE_CASE` only for true compile-time constants and environment-derived configuration.
- No `I` prefix on interfaces and no `T` prefix on type aliases.
- File names: `kebab-case.ts` or `camelCase.ts` consistently; one public concept per module.
- Prefer named exports. Default exports make renames and re-exports inconsistent and interact badly with CommonJS interop (`require("./x").default`).
- Let Prettier (or dprint/Biome) own formatting; do not review whitespace.
- Annotate the return types of exported functions and public methods. It documents the contract, stops accidental API changes, speeds up checking, and is required by `isolatedDeclarations`. Let local variables infer.

***

#### Types and the Type System

##### `any`, `unknown`, `never`

`any` disables checking in both directions and spreads: anything derived from an `any` is `any`. `unknown` accepts everything but allows nothing until narrowed. `never` is the empty type (unreachable code, exhaustive checks, impossible branches).

```ts
// BAD: any leaks into every caller
function parseBad(json: string): any {
  return JSON.parse(json);
}
const port1: string = parseBad('{"port":1}').port; // compiles; wrong at runtime

// GOOD: unknown forces validation at the boundary
function parse(json: string): unknown {
  return JSON.parse(json);
}
const raw = parse('{"port":1}');
const port2: string = raw.port;                     // error TS18046: 'raw' is of type 'unknown'
if (typeof raw === "object" && raw !== null && "port" in raw && typeof raw.port === "number") {
  const port3: number = raw.port;                   // OK after narrowing
  console.log(port3);
}
console.log(port1, port2);
```

Sources of hidden `any` a reviewer must spot: `JSON.parse`, `response.json()` (`Promise<any>`), `catch (e)` without `useUnknownInCatchVariables`, untyped third-party modules (`declare module "x";`), `Function`, `arguments`, `Object.values` on `any`, `as any`, and `// @ts-ignore`. Prefer `// @ts-expect-error` with an explanation: it fails when the error disappears.

##### Literal types, widening and `as const`

```ts
let a = "GET";                    // string  (mutable → widened)
const b = "GET";                  // "GET"   (const → literal)
const methods = ["GET", "POST"];              // string[]
const frozen = ["GET", "POST"] as const;      // readonly ["GET", "POST"]
type Method = (typeof frozen)[number];        // "GET" | "POST"

function send(m: Method) { return m; }
send(a);                          // error TS2345: string is not assignable to Method
send(b);
send(methods[0]);                 // error TS2345
send(frozen[0]);
a = "PUT";
```

`as const` makes everything `readonly` and literal; it does not freeze the object at runtime (use `Object.freeze` for that, which TypeScript also types as `Readonly<T>`).

##### `interface` vs `type`

Both describe object shapes. Use `interface` for object types that others extend or implement (better error messages, cached relationships, declaration merging); use `type` for unions, intersections, mapped/conditional types, tuples and function types. Be aware that interfaces merge:

```ts
interface Config { port: number }
interface Config { host: string }      // merges: Config has port and host
const c: Config = { port: 1 };        // error TS2741: property 'host' is missing

type Settings = { port: number };     // error TS2300: duplicate identifier
type Settings = { host: string };     // error TS2300
export { c };
```

Accidental merging with a global (`interface Request`, `interface Event`, `interface Window` in a script file) silently changes library types. With `moduleDetection: force` every file is a module and this risk is limited to `declare global` blocks.

##### Excess property checks

Excess property checks apply only to fresh object literals assigned directly to a typed target. They are a typo detector, not a guarantee of exact types.

```ts
interface Opts { retries: number; verbose?: boolean }
const o1: Opts = { retries: 3, verbos: true };  // error TS2561: did you mean 'verbose'?
const literal = { retries: 3, verbos: true };
const o2: Opts = literal;                      // OK: not a fresh literal, extra key allowed

interface Weak { retries?: number; verbose?: boolean }
const w: Weak = literal;                       // OK: shares 'retries'
const w3: Weak = new Date();                   // error TS2559: weak type, no properties in common
export { o1, o2, w, w3 };
```

A "weak" type (all properties optional) additionally rejects any value that shares none of its properties (TS2559). TypeScript has no exact object types; values may carry extra properties at runtime. Never `Object.keys()`/spread a typed object into a database row, log line or HTTP response assuming only the declared keys are present — pick the fields explicitly.

##### Index signatures, `Record` and `Object.keys`

```ts
const counts: Record<string, number> = {};
const n: number = counts["missing"];      // error TS2322 with noUncheckedIndexedAccess: number | undefined
counts["a"] = (counts["a"] ?? 0) + 1;

interface Point { x: number; y: number }
function sum(p: Point): number {
  let total = 0;
  for (const k of Object.keys(p)) {       // k: string, not keyof Point
    total += p[k as keyof Point];         // assertion: p may have extra keys at runtime
  }
  return total;
}
export { n, sum };
```

`Object.keys`, `Object.entries` and `for...in` return `string` keys by design (structural typing allows extra properties). Prefer `Map<K, V>` for dynamic keys: no prototype keys (`"__proto__"`, `"constructor"`), any key type, and `get()` returns `V | undefined` without extra flags.

##### Tuples

```ts
type Range = [start: number, end: number];            // labelled
type Args = [string, number?];                        // optional element
type Rest = [first: string, ...rest: number[]];       // variadic
function concat<T extends unknown[], U extends unknown[]>(a: [...T], b: [...U]): [...T, ...U] {
  return [...a, ...b];
}
const r = concat([1, "a"], [true]);                   // [number, string, boolean]
const range: Range = [1, 2, 3];                       // error TS2322: source has 3 elements
export type { Args, Rest };
export { r, range };
```

##### Type assertions, non-null assertions and `satisfies`

`x as T` is an unchecked claim (allowed when either type is assignable to the other); `x as unknown as T` is an unconditional cast; `x!` removes `null | undefined`. None emit code. Review every one: is there a guard, a schema, or an invariant that proves it?

`satisfies` (4.9) checks a value against a type **without** changing the inferred type:

```ts
type Route = { path: string; auth: boolean };
const routes = {
  home: { path: "/", auth: false },
  admin: { path: "/admin", auth: true },
} satisfies Record<string, Route>;

routes.admin.path;          // OK, keys are known: "home" | "admin"
routes.missing;             // error TS2339: property 'missing' does not exist

const widened: Record<string, Route> = routes;
widened.missing.path;       // error TS18048 (noUncheckedIndexedAccess): possibly undefined

const bad = { home: { path: "/", auth: "no" } } satisfies Record<string, Route>; // error TS2322
export { bad };
```

Use `satisfies` for configuration tables, lookup maps and `as const` objects whose literal types you want to keep. Use `x satisfies never` in the `default` of a switch for exhaustiveness (see below).

##### Optional chaining and nullish coalescing

```ts
interface User { profile?: { email?: string; age?: number } }
declare const u: User;
const email = u.profile?.email ?? "unknown";
const age = u.profile?.age || 18;    // BUG: age 0 becomes 18
const age2 = u.profile?.age ?? 18;   // GOOD: only null/undefined fall back
export { email, age, age2 };
```

`??` and `?.` treat only `null`/`undefined` as missing; `||` and `&&` treat `0`, `""`, `NaN` and `false` as missing too. Since 5.6, TypeScript reports `??` whose left side is syntactically never nullish (TS2869) and conditions that are always truthy, such as `if (/regex/)` or `if (x => 0)` (TS2872).

***

#### Narrowing and Control-Flow Analysis

##### Built-in narrowing

```ts
function describe(x: string | number | Date | null | string[]): string {
  if (x === null) return "null";
  if (typeof x === "string") return x.toUpperCase();
  if (typeof x === "number") return x.toFixed(2);
  if (x instanceof Date) return x.toISOString();
  return x.join(",");                       // string[]
}
export { describe };
```

Pitfalls a reviewer must know:

- `typeof null === "object"`: `typeof x === "object"` does not exclude `null` (TypeScript narrows to `T | null`).
- Truthiness narrowing (`if (x)`) also drops `0`, `""` and `NaN` — wrong for numbers and strings that may legitimately be empty.
- `"key" in obj` narrows unions by property presence; it also narrows `unknown`/`object` to `object & Record<"key", unknown>` (4.9+).
- `Array.isArray(x)` narrows to `any[]` when `x` is `unknown`, which re-introduces `any`.
- Property narrowing survives function calls: TypeScript does **not** invalidate the narrowing of `obj.prop` when a call might mutate `obj`, which is unsound:

```ts
interface State { user: { name: string } | null }
function logout(s: State) { s.user = null; }

function greet(s: State): string {
  if (s.user !== null) {
    logout(s);
    return s.user.name;      // compiles; throws TypeError at runtime
  }
  return "anonymous";
}
export { greet };
```

Copy narrowed values into a `const` before calling out, or re-check after the call.

##### Discriminated unions and exhaustiveness

```ts
type Shape =
  | { kind: "circle"; radius: number }
  | { kind: "square"; size: number }
  | { kind: "rect"; w: number; h: number };

function area(s: Shape): number {
  switch (s.kind) {
    case "circle":
      return Math.PI * s.radius ** 2;
    case "square":
      return s.size ** 2;
    default:
      s satisfies never;     // error TS1360: "rect" is not handled
      throw new Error(`unknown shape: ${JSON.stringify(s)}`);
  }
}
export { area };
```

Use a literal discriminant (`kind`, `type`, `status`) on every member. Destructured discriminants narrow too (`const { kind, ...rest } = s; if (kind === "circle")` narrows `s`, not `rest`). Prefer `satisfies never` or an `assertNever(x: never): never` helper over a silent `default` branch.

##### Type predicates and inferred type predicates (5.5)

```ts
interface Cat { meow(): void }
interface Dog { bark(): void }
function isCat(a: Cat | Dog): a is Cat {
  return "meow" in a;
}

// 5.5+: the predicate `x is number` is inferred for simple arrow functions
const nums = [1, null, 2, undefined].filter((x) => x != null); // number[]
const first: number = nums[0] ?? 0;
export { isCat, first };
```

A user-defined predicate is trusted blindly: `function isCat(a): a is Cat { return true }` compiles. Check that the body actually proves the claim. Inferred predicates appear only when the function returns a boolean expression that narrows its parameter *and* the false branch proves the opposite; `x => !!x` is **not** inferred for `number | undefined` because `0` is falsy.

##### Assertion functions

```ts
function assert(cond: unknown, msg: string): asserts cond {
  if (!cond) throw new Error(msg);
}
function assertIsString(v: unknown): asserts v is string {
  if (typeof v !== "string") throw new TypeError("not a string");
}
declare const input: unknown;
assertIsString(input);
input.toUpperCase();                    // narrowed to string

const check = (c: unknown): asserts c => { if (!c) throw new Error(); };
check(input);                           // error TS2775: assertions require explicit type annotations on the call target
```

Assertion signatures only work through identifiers whose declared type is explicitly annotated (`const check: (c: unknown) => asserts c = ...`).

##### Narrowing in closures (5.4) and constant indexed access (5.5)

```ts
function build(url: string | URL, names: Map<string, string>, key: string) {
  if (typeof url === "string") url = new URL(url);
  // 5.4+: narrowing survives into closures created after the last assignment
  const hosts = [1, 2].map(() => url.host);

  const obj: Record<string, string | undefined> = Object.fromEntries(names);
  if (typeof obj[key] === "string") {
    obj[key].toUpperCase();             // 5.5+: obj[key] narrows when obj and key are const/unchanged
  }
  return hosts;
}
export { build };
```

Narrowing is still lost in closures if the variable is assigned *after* the closure is created anywhere in the function, and for `let` variables captured by callbacks that run later.

##### `switch (true)` (5.3)

```ts
function kind(x: unknown): string {
  switch (true) {
    case typeof x === "string":
      return x.trim();                 // narrowed to string
    case Array.isArray(x):
      return `array of ${x.length}`;
    default:
      return "other";
  }
}
export { kind };
```

***

#### Functions

##### Parameters and return types

- Prefer optional parameters (`x?: T`) and defaults (`x = 0`) over overloads when the return type does not depend on the argument.
- Use `readonly T[]` / `ReadonlyArray<T>` for array parameters the function does not mutate; it documents intent and lets callers pass `as const` tuples.
- Use object parameters (`{ timeoutMs, retries }`) once a function has more than two or three optional arguments.
- `void` as a *return type of a callback type* means "the return value is ignored", so `() => number` is assignable to `() => void`. A function *declared* to return `void` must not return a value.

```ts
const out: number[] = [];
[1, 2].forEach((n) => out.push(n));       // OK: push returns number, callback type returns void

function f(): void {
  return 1;                               // error TS2322
}
export { f };
```

##### Overloads

Overload signatures are visible to callers; the implementation signature is not. Order from most specific to least specific; the implementation must be compatible with every overload, but TypeScript does not check that each overload's return type is honoured by the body.

```ts
function parse(input: string): number;
function parse(input: string[]): number[];
function parse(input: string | string[]): number | number[] {
  return Array.isArray(input) ? input.map(Number) : Number(input);
}
const one = parse("1");                  // number
const many = parse(["1", "2"]);          // number[]
declare const either: string | string[];
parse(either);                           // error TS2769: no overload matches a union argument
export { one, many };
```

When callers can pass a union, add a third overload `(input: string | string[]): number | number[]` or use a conditional return type.

##### Method parameter bivariance

`strictFunctionTypes` checks parameters contravariantly only for **function-type properties**. Parameters of **method syntax** members stay bivariant, which lets unsound assignments through:

```ts
interface Animal { name: string }
interface Dog extends Animal { bark(): void }

interface HandlerMethod { handle(a: Animal): void }           // method syntax: bivariant
interface HandlerProp { handle: (a: Animal) => void }         // property syntax: contravariant

const dogOnly = { handle(d: Dog) { d.bark(); } };
const m: HandlerMethod = dogOnly;          // compiles; m.handle({ name: "cat" }) throws
const p: HandlerProp = dogOnly;            // error TS2322
export { m, p };
```

Declare callbacks and handler interfaces with property syntax (`handle: (a: Animal) => void`) when soundness matters.

##### Granular checks of conditional `return` expressions (5.8)

Since 5.8, each branch of `return cond ? a : b` is checked against the declared return type, so an `any` in one branch no longer hides a wrong type in the other:

```ts
declare const cache: Map<any, any>;
function getUrl(s: string): URL {
  return cache.has(s) ? cache.get(s) : s;  // error TS2322: 'string' is not assignable to 'URL'
}
export { getUrl };
```

##### `this` parameters

```ts
function onClick(this: HTMLButtonElement, ev: MouseEvent) {
  this.disabled = true;
  console.log(ev.button);
}
document.querySelector("button")?.addEventListener("click", onClick);
const detached = onClick;
detached(new MouseEvent("click"));       // error TS2684: 'this' context of type 'void'
```

Class methods passed as callbacks lose `this` at runtime; TypeScript does not catch `setTimeout(this.tick, 10)` for ordinary methods. Use arrow-function properties or `.bind(this)` and review every unbound method reference (`@typescript-eslint/unbound-method`).

***

#### Generics

##### Constraints, defaults and inference

```ts
function pluck<T, K extends keyof T>(items: readonly T[], key: K): T[K][] {
  return items.map((i) => i[key]);
}
const ids = pluck([{ id: 1, n: "a" }], "id");     // number[]
pluck([{ id: 1 }], "nope");                      // error TS2345

interface Page<T = unknown> { items: T[]; next?: string }
const p: Page = { items: [1, "a"] };
export { ids, p };
```

Guidelines:

- Every type parameter should relate two or more positions (input to output, or two inputs). A type parameter used once (`function f<T>(x: T): void`) is just `unknown` with extra steps — and `function f<T>(): T` lets callers pick any return type, which is an unchecked cast in disguise (`parse<User>(json)`).
- Constrain with the weakest constraint that the body needs.
- Avoid explicit type arguments at call sites unless inference is impossible; they hide mismatches.

##### `const` type parameters (5.0)

```ts
function routes<const T extends readonly string[]>(paths: T): T {
  return paths;
}
const r = routes(["/a", "/b"]);       // readonly ["/a", "/b"] without `as const` at the call site
type R = (typeof r)[number];          // "/a" | "/b"
export type { R };
```

The `const` modifier only affects inference from literals written at the call site; passing a variable of type `string[]` still infers `string[]`. With a `readonly` constraint the result is a readonly tuple; with a mutable constraint (`T extends string[]`) 5.9 infers a mutable tuple (`["/a"]`).

##### `NoInfer<T>` (5.4)

```ts
function createFsm<S extends string>(states: S[], initial: NoInfer<S>): S {
  return states.includes(initial) ? initial : states[0]!;
}
createFsm(["idle", "busy"], "idle");
createFsm(["idle", "busy"], "done");  // error TS2345: "done" is not "idle" | "busy"
```

Without `NoInfer`, `"done"` would be inferred into `S` and the call would compile.

##### Variance annotations

`in`/`out` modifiers on type parameters (`interface Producer<out T>`) declare variance explicitly. They speed up checking of deeply recursive types and catch accidental variance changes, but the compiler trusts them only partially; use them in hot library types, not by default.

##### New in 5.9: type-argument inference changes

5.9 fixes "leaks" of type variables during inference (microsoft/TypeScript#61668). Some calls now infer different types or report new errors; the documented fix is to add explicit type arguments.

```ts
declare function call<A, R>(obj: { x: (a: A) => R }, arg: A): R;
declare function wrap<X>(x: X): { x: X };
declare function id<T>(x: T): T;

// 5.8: error TS2345 "Argument of type 'number' is not assignable to parameter of type 'A'",
//      and x is declared as the leaked type parameter `A`.
// 5.9: compiles; x is `unknown`.
export const x = call(wrap(id), 1);
```

Known fallout (from the PR discussion): `fp-ts` exact-type tests saw different (no longer tuple-preserving) types, and a user reported new errors in MobX `observable.map(...)` calls that were fixed in MobX. When a 5.8 → 5.9 upgrade produces new inference errors in generic-heavy code (state libraries, functional pipelines, ORMs), add explicit type arguments at the failing call rather than loosening types with `any`.

***

#### Advanced Types

##### Conditional types, `infer` and distribution

```ts
type ElementOf<T> = T extends readonly (infer E)[] ? E : never;
type A = ElementOf<string[] | number[]>;    // string | number (distributes over the union)

type IsString<T> = T extends string ? true : false;
type B = IsString<string | number>;         // boolean (distributed: true | false)
type IsStringStrict<T> = [T] extends [string] ? true : false;
type C = IsStringStrict<string | number>;   // false
type D = IsString<never>;                   // never (distribution over an empty union)

type First<T extends unknown[]> = T extends [infer H extends string, ...unknown[]] ? H : never;
type E = First<["a", 1]>;                   // "a"
export type { A, B, C, D, E };
```

Wrap the checked type in a tuple (`[T] extends [U]`) to stop distribution — a frequent source of bugs in "is X" helper types.

##### Mapped types, key remapping and template literal types

```ts
type Getters<T> = {
  [K in keyof T & string as `get${Capitalize<K>}`]: () => T[K];
};
type G = Getters<{ name: string; age: number }>;  // { getName(): string; getAge(): number }

type Mutable<T> = { -readonly [K in keyof T]: T[K] };
type Strict<T> = { [K in keyof T]-?: T[K] };

type EventName = `on${"Click" | "Hover"}`;        // "onClick" | "onHover"
type Route = `/users/${number}`;
const ok: Route = "/users/42";
const bad: Route = "/users/abc";                  // error TS2322
export type { G, Mutable, Strict, EventName };
export { ok, bad };
```

##### Built-in utility types

`Partial`, `Required`, `Readonly`, `Pick`, `Omit`, `Record`, `Exclude`, `Extract`, `NonNullable`, `ReturnType`, `Parameters`, `ConstructorParameters`, `InstanceType`, `Awaited` (4.5), `ThisParameterType`, `OmitThisParameter`, `ThisType`, `NoInfer` (5.4), and the intrinsic string types `Uppercase`, `Lowercase`, `Capitalize`, `Uncapitalize`.

- `Omit<T, K>` does not check that `K` is a key of `T` (`Omit<User, "pasword">` compiles and omits nothing). Use a stricter helper `type StrictOmit<T, K extends keyof T> = Omit<T, K>` for public APIs.
- `Omit` and `Pick` over a union are not distributive: `Omit<A | B, "x">` keeps only common keys. Use `T extends unknown ? Omit<T, K> : never` to distribute.
- `Partial<T>` is shallow.

##### Branded (nominal) types

```ts
type UserId = string & { readonly __brand: "UserId" };
type OrderId = string & { readonly __brand: "OrderId" };

function toUserId(s: string): UserId {
  if (!/^u_[a-z0-9]+$/.test(s)) throw new Error("invalid user id");
  return s as UserId;                 // the only cast, behind validation
}
function loadUser(id: UserId) { return id; }

const uid = toUserId("u_1");
loadUser(uid);
loadUser("u_1");                      // error TS2345
loadUser(uid as string as OrderId);   // error TS2345: 'OrderId' is not assignable to 'UserId'
```

##### Recursive types and type-instantiation limits

Deeply recursive conditional types hit TS2589 *Type instantiation is excessively deep and possibly infinite*. 5.9 caches more intermediate instantiations, which reduces these errors in schema libraries (Zod, tRPC-style routers) but does not remove the limit. A new TS2589 after an innocent change usually means a type that recurses over a union or an unbounded string; restructure with tail-recursive conditional types (4.5+, up to 1000 levels) or bound the recursion depth.

***

#### Objects, Collections and Data

##### `object`, `{}` and `Object`

`object` means "any non-primitive". `{}` (and `Object`) means "any value except `null` and `undefined`" — strings and numbers included. `{}` as a parameter type accepts almost everything; it is not "an empty object".

```ts
function takesObject(o: object) { return o; }
function takesBraces(o: {}) { return o; }
takesObject("str");                  // error TS2345
takesBraces("str");                  // OK — probably not what the author meant
type EmptyRecord = Record<string, never>; // "an object with no properties"
const e: EmptyRecord = { a: 1 };     // error TS2322
export { e };
```

##### Readonly data

```ts
interface Order { readonly id: string; items: readonly string[] }
function addItem(o: Order, item: string): Order {
  o.items.push(item);                // error TS2339: 'push' does not exist on readonly string[]
  o.id = "x";                        // error TS2540: read-only property
  return { ...o, items: [...o.items, item] };
}
const cfg: Readonly<Record<string, number>> = { a: 1 };
const shallow: Readonly<{ nested: { n: number } }> = { nested: { n: 1 } };
shallow.nested.n = 2;                // OK: Readonly is shallow
export { addItem, cfg };
```

`readonly` is a compile-time view: an aliasing mutable reference can still change the value, and `ReadonlyArray<T>` is assignable *from* `T[]` but not *to* it. Use `ReadonlyMap`/`ReadonlySet` in public signatures that should not mutate.

##### Exhaustive lookup tables

```ts
type Status = "active" | "disabled" | "pending";
const labels: Record<Status, string> = {   // error TS2741: property 'pending' is missing
  active: "Active",
  disabled: "Disabled",
};                                          // every key required: adding a Status breaks compilation
export { labels };
```

Prefer `Record<Union, T>` (or `{ [K in Union]: T }`) over `Record<string, T>` for closed sets: the compiler then enforces completeness.

##### Arrays and `noUncheckedIndexedAccess`

```ts
const xs: number[] = [];
const [head] = xs;                 // number | undefined with noUncheckedIndexedAccess
const last = xs[xs.length - 1];    // number | undefined
const found = xs.find((x) => x > 1); // number | undefined, always
const total: number = head + 1;    // error TS18048: 'head' is possibly 'undefined'
for (const x of xs) console.log(x.toFixed());  // for...of yields number (no undefined)
xs.at(-1)?.toFixed();
export { last, found, total };
```

Without the flag, `xs[0]` on an empty array is typed `number` and fails at runtime — the most common source of `Cannot read properties of undefined` in strict-mode code.

##### JSON is not your interface

```ts
interface Event { id: number; at: Date; tags: Set<string>; amount: bigint }

const wire = JSON.stringify({ id: 1, at: new Date(0), tags: new Set(["a"]), amount: 1n }); // throws TypeError: BigInt
const parsed = JSON.parse('{"id":1,"at":"1970-01-01T00:00:00.000Z","tags":{}}') as Event;
parsed.at.getTime();             // compiles; TypeError at runtime: at is a string
export { wire };
```

- `JSON.stringify` turns `Date` into a string, `Set`/`Map` into `{}`, drops `undefined` and functions, and **throws** on `bigint` and cycles. `JSON.parse` returns `any`.
- Model wire formats with separate DTO types (`at: string`) and convert explicitly. Never cast `JSON.parse` output to a domain type that contains `Date`, `Map`, `Set`, `bigint` or class instances.

##### Numbers, `bigint` and money

- `number` is an IEEE-754 double: integers are exact only up to `Number.MAX_SAFE_INTEGER` (2^53 − 1). Database IDs from `BIGINT` columns and Twitter-style snowflake IDs must be `string` or `bigint`.
- `bigint` needs `target`/`lib` ES2020+; `1n + 1` is a TypeScript error (TS2365) and a runtime `TypeError`.
- Money: integer minor units (`bigint` or `number` cents within safe range) or a decimal library; never floating-point arithmetic on currency.
- `Number("")` is `0` and `parseInt("12px")` is `12`; validate numeric input with a regex or `Number.isFinite` after conversion.

```ts
const id = 9007199254740993;           // silently 9007199254740992 at runtime
const big = 9007199254740993n;
const mixed = big + 1;                 // error TS2365: cannot mix bigint and number
export { id, mixed };
```

##### Environment variables

`process.env.NAME` is `string | undefined` (`@types/node`). With `noPropertyAccessFromIndexSignature`, dotted access is TS4111 — use `process.env["NAME"]`. Parse and validate configuration once at startup into a typed, frozen object; do not sprinkle `process.env.X!` through the code.

***

#### Concurrency: Workers and Shared Memory

JavaScript is single-threaded per realm; parallelism comes from Worker threads (`node:worker_threads`, Web Workers) that exchange messages (structured clone) or share memory through `SharedArrayBuffer`.

- Messages are structured-cloned: class instances lose their prototype and methods, functions throw `DataCloneError`. Type the message protocol as a discriminated union of plain data and validate it on receipt like any other untrusted input.
- **New in 5.9 (lib)**: `SharedArrayBuffer` is no longer assignable to `ArrayBuffer` regardless of `lib` (5.8 already rejected it with `lib: es2024`+ because of the resizable-buffer members, but accepted it with older libs), so code that passes shared memory to APIs that require a non-shared buffer now fails to compile — that is the intended fix, not a regression to silence.
- `Atomics.pause()` is declared in `lib.esnext.sharedmemory.d.ts` (**new in 5.9**); it exists at runtime from Node.js 24. Use it only in spin-wait loops; prefer `Atomics.wait`/`Atomics.waitAsync` (`es2024`) for blocking.
- `Atomics` operations require integer typed arrays (`Int32Array`, `BigInt64Array`); plain reads/writes of a shared typed array are not synchronised.

```ts
const sab = new SharedArrayBuffer(4);
const counter = new Int32Array(sab);
Atomics.add(counter, 0, 1);
const ab: ArrayBuffer = sab;            // error TS2739: SharedArrayBuffer is not an ArrayBuffer
export { ab };
```

***

#### Classes

##### Field semantics (`useDefineForClassFields`)

With `target` ES2022 or later (or `esnext`), `useDefineForClassFields` defaults to `true`: class fields are emitted as native fields with `[[Define]]` semantics, initialised after `super()` returns and before the rest of the constructor. Two consequences catch people:

```ts
// @run
class Base {
  greeting = "base";
  constructor() {
    this.init();
  }
  init(): void {}
}
class Derived extends Base {
  value = "default";                  // runs AFTER Base's constructor called init()
  override init(): void {
    this.value = "from init";
  }
}
console.log(new Derived().value);     // "default" — the field initialiser overwrote init()'s write

class Model {
  declare id: string;                 // `declare`: type only, no runtime field emitted
  name?: string;                      // emitted as `name;` → own property with value undefined
}
console.log(Object.keys(new Model())); // [ 'name' ]
```

- A field redeclared in a subclass without `declare` (`override name: string;`) shadows an accessor or property set by the base class (it is reset to `undefined`). Use `declare` to only narrow the type.
- Libraries that rely on `[[Set]]` semantics (legacy decorators that define accessors on the prototype, MobX 5-style observables, some ORMs) break; set `useDefineForClassFields: false` only if such a library requires it, and document why.

##### Parameter properties, visibility and `#private`

```ts
class Account {
  readonly #balance: number;          // ECMAScript private: enforced at runtime
  private audit: string[] = [];       // TypeScript private: compile time only
  constructor(public readonly id: string, balance: number) {
    this.#balance = balance;
  }
  get balance(): number {
    return this.#balance;
  }
}
const acc = new Account("a1", 10);
acc.audit;                             // error TS2341: property 'audit' is private
(acc as any).audit;                    // compiles and works: `private` is erased
console.log(acc.id, acc.balance);
```

- `private`/`protected` are erased; anything can read them via `as any`, `JSON.stringify`, `Object.keys` or the debugger. Use `#private` for secrets and invariants (but `#private` fields are not copied by spread/`structuredClone`, and `#x in obj` brand checks are available since 4.5).
- Parameter properties (`constructor(public id: string)`) generate code and are rejected by `erasableSyntaxOnly` and Node.js type stripping. In code that must be erasable, declare the field and assign it in the constructor.

##### `override`, abstract classes and `implements`

```ts
abstract class Repo<T> {
  abstract find(id: string): Promise<T | undefined>;
  async get(id: string): Promise<T> {
    const v = await this.find(id);
    if (v === undefined) throw new Error(`not found: ${id}`);
    return v;
  }
}
class UserRepo extends Repo<{ id: string }> {
  override async find(id: string) {
    return id === "1" ? { id } : undefined;
  }
  async fnid(id: string) { return id; }       // typo for 'find' without `override`: silently a new method
  override async remove(id: string) { return id; } // error TS4113: not in base class
}

interface Clock { now(): number }
class FakeClock implements Clock {
  now(x?: number) { return x ?? 0; }  // `implements` checks compatibility only; it adds no types
}
export { UserRepo, FakeClock };
```

`implements` does not contextually type the class: parameters of implemented methods without annotations are `any` (an error under `noImplicitAny`), and an extra optional parameter is allowed.

##### `strictPropertyInitialization` and definite assignment

```ts
class Conn {
  socket: WebSocket;                   // error TS2564: not definitely assigned in the constructor
  url!: string;                        // `!` silences the check — a promise you must keep
  retries = 0;
}
export { Conn };
```

Every `!:` is a review point: is the field really set before first use (by a framework, `init()` method, dependency injection)?

##### Accessors (5.1) and the `accessor` keyword

Getters and setters may have unrelated types since 5.1 (the setter type need not be a subtype of the getter type). The `accessor` keyword (5.0, standard decorators) declares an auto-accessor backed by a private slot; it is intended for decorators to intercept.

***

#### Decorators

TypeScript 5.9 supports two incompatible decorator systems:

| | Standard (TC39, default since 5.0) | Legacy (`experimentalDecorators: true`) |
|---|---|---|
| Signature | `(value, context: ClassMethodDecoratorContext<...>) => ...` | `(target, key, descriptor) => ...` |
| Parameter decorators | No | Yes |
| `emitDecoratorMetadata` | Not supported | Supported (`design:type` via `reflect-metadata`) |
| Metadata | `context.metadata` / `Symbol.metadata` (5.2, needs `lib: esnext.decorators` or `esnext` and a runtime/polyfill for `Symbol.metadata`) | — |
| Emit | Downlevelled to `__esDecorate`/`__runInitializers` helpers for `target` ≤ `es2024`; **emitted as-is for `target: esnext`** | Emitted as `__decorate` calls for every target |

```ts
// @target: es2022
// @run
function logged<This, Args extends unknown[], R>(
  target: (this: This, ...args: Args) => R,
  context: ClassMethodDecoratorContext<This, (this: This, ...args: Args) => R>,
) {
  const name = String(context.name);
  return function (this: This, ...args: Args): R {
    console.log(`→ ${name}`);
    return target.call(this, ...args);
  };
}

class Service {
  @logged
  ping(): string {
    return "pong";
  }
}
console.log(new Service().ping());
```

Review notes:

- **`target: esnext` + standard decorators produces code no Node.js release can run.** 5.9.3 emits `@logged` unchanged for `esnext` (the target `tsc --init` now writes), and Node.js 24.22 and 26.12 fail with `SyntaxError: Invalid or unexpected token`. Projects that use standard decorators and run `tsc` output directly must pin `target` to `es2024` or lower. The same applies to every other syntax `esnext` leaves untransformed (`using` on Node.js < 24, `import defer`).
- Frameworks that need parameter decorators or `emitDecoratorMetadata` (Angular before its standard-decorator migration, NestJS, TypeORM, InversifyJS) require `experimentalDecorators: true`. Do not mix: with that flag on, all decorators in the project use the legacy semantics.
- Decorators are runtime syntax: they are rejected by Node.js type stripping, and `erasableSyntaxOnly` does not flag them (it checks enums, namespaces, parameter properties and `import =`/`export =` only).
- A decorator that replaces a method must preserve `this` and the return type; check generic signatures like the one above rather than `(...args: any[]) => any`.

***

#### Enums and Their Alternatives

Since 5.0 all enums are union enums: each member is its own type, and assigning an out-of-range numeric literal is an error.

```ts
// @run
enum Status { Active, Disabled }
let s: Status = Status.Active;
s = 5;                                   // error TS2322: 5 is not assignable to Status (5.0+)
const n: number = 7;
s = n;                                   // compiles: a non-literal number is still assignable
console.log(Status[0], Status["Active"]); // "Active" 0 — numeric enums have a reverse mapping

enum Level { Debug = "debug", Info = "info" }
const fromConfig: string = "debug";
const lvl1 = fromConfig as Level;        // unchecked cast: any string passes
const isLevel = (v: string): v is Level => (Object.values(Level) as string[]).includes(v);
console.log(lvl1, isLevel("trace"));     // "debug" false
```

- `const enum` is inlined at compile time. With `isolatedModules` (or `verbatimModuleSyntax`), using an ambient `const enum` from a `.d.ts` is an error and exported const enums are preserved like ordinary enums; never publish `const enum` in a library's public `.d.ts`.
- Enums are rejected by `erasableSyntaxOnly` and Node.js type stripping. In new code that may run under type stripping, prefer an `as const` object plus a derived union type:

```ts
export const Color = { Red: "red", Green: "green" } as const;
export type Color = (typeof Color)[keyof typeof Color];   // "red" | "green"

function paint(c: Color) { return c; }
paint(Color.Red);
paint("green");                         // plain strings work too, unlike string enums
paint("blue");                          // error TS2345
```

***

#### Explicit Resource Management: `using` and `await using` (5.2)

```ts
// @run
class TempFile implements Disposable {
  constructor(public readonly path: string) {
    console.log(`open ${path}`);
  }
  [Symbol.dispose](): void {
    console.log(`close ${this.path}`);
  }
}

function work(): void {
  using a = new TempFile("a");
  using b = new TempFile("b");
  console.log("working");
}                                      // disposes b, then a — even if work() throws
work();

async function withStack(): Promise<void> {
  await using stack = new AsyncDisposableStack();
  stack.defer(async () => console.log("async cleanup"));
  console.log("in scope");
}
await withStack();
```

Facts for 5.9:

- Types need `lib: ["esnext"]` or `"esnext.disposable"` (`Disposable`, `AsyncDisposable`, `DisposableStack`, `AsyncDisposableStack`, `SuppressedError`). `target` below `esnext` downlevels the syntax with helpers (`__addDisposableResource`, `__disposeResources`).
- Runtime: `Symbol.dispose`/`Symbol.asyncDispose` exist in Node.js 18+; `using` syntax and `DisposableStack`/`AsyncDisposableStack`/`SuppressedError` are native from Node.js 24. On Node.js 20/22, compile with `target` ≤ `es2024` and polyfill `DisposableStack` if you use it.
- `using x = null` / `undefined` is allowed (nothing is disposed). A value without `[Symbol.dispose]` throws `TypeError` at the declaration.
- If both the block and a disposer throw, the error is a `SuppressedError` (`error` = disposal error, `suppressed` = original). Catch blocks that check `instanceof MyError` may need to unwrap it.
- `using` is block scoped and disposes on block exit — not when the value is returned. Returning a `using` resource from a function returns an already-disposed object (a real bug pattern); use `DisposableStack#move()` to transfer ownership.

***

#### Error Handling

##### `unknown` in `catch`

With `strict` (`useUnknownInCatchVariables`), the catch variable is `unknown`. Narrow before use; anything can be thrown.

```ts
class HttpError extends Error {
  override name = "HttpError";
  constructor(message: string, readonly status: number, options?: ErrorOptions) {
    super(message, options);
  }
}

async function load(url: string): Promise<string> {
  try {
    const res = await fetch(url);
    if (!res.ok) throw new HttpError(`GET ${url} failed`, res.status);
    return await res.text();            // `return await` inside try so rejections are caught here
  } catch (err) {
    if (err instanceof HttpError && err.status === 404) return "";
    throw new Error(`load(${url}) failed`, { cause: err }); // preserve the chain (lib es2022)
  }
}

function message(err: unknown): string {
  return err instanceof Error ? err.message : String(err);
}
export { load, message };
```

- **New in 5.9 (lib)**: `Error.isError(value)` is declared in `lib.esnext.error.d.ts` (needs `lib: esnext`). It is true for genuine errors across realms (iframes, `vm` contexts), where `instanceof Error` fails. It exists at runtime only from Node.js 24; do not use it in code that runs on Node.js 22.
- Do not set `this.name = new.target.name` blindly in minified bundles (class names get mangled); use a literal.
- With `target` ES2015+, `class MyError extends Error` works without `Object.setPrototypeOf(this, MyError.prototype)`; that workaround is only needed for `target: es5` (deprecated in 6.0).
- Prefer `Result`-style return values (`{ ok: true, value } | { ok: false, error }`) for expected failures in domain code, and exceptions for programmer errors and I/O.

***

#### Async Code

##### Floating and misused promises

TypeScript does not report an unawaited promise. These are the most common production async bugs; enable `@typescript-eslint/no-floating-promises`, `no-misused-promises`, `await-thenable` and `require-await`/`return-await` (type-aware linting).

```ts
declare function save(id: string): Promise<void>;

function bad(ids: string[]): void {
  ids.forEach(async (id) => {          // forEach ignores the returned promises
    await save(id);                    // rejections are unhandled; caller cannot await completion
  });
  save("x");                           // floating promise: errors crash the process (unhandled rejection)
}

async function good(ids: string[]): Promise<void> {
  await Promise.all(ids.map((id) => save(id)));          // parallel, fail-fast
  const results = await Promise.allSettled(ids.map(save)); // parallel, collect all outcomes
  for (const r of results) if (r.status === "rejected") console.error(r.reason);
  for (const id of ids) await save(id);                  // sequential when order/rate matters
}
export { bad, good };
```

- An `async` function passed where `() => void` is expected (event handlers, `forEach`, `setTimeout`, Express handlers without a wrapper) compiles because `void` callbacks accept any return value. Its rejection becomes an unhandled rejection; on Node.js 15+ the default `--unhandled-rejections=throw` terminates the process.
- `Promise.all` over thousands of items starts everything at once; use a concurrency limiter.
- `if (promise)` is always true; TypeScript 5.6 reports some always-truthy checks, and `no-misused-promises` catches the rest (for example `if (user.isAdmin())` where `isAdmin` is async).

##### Cancellation and timeouts

```ts
async function fetchJson(url: string, signal?: AbortSignal): Promise<unknown> {
  const timeout = AbortSignal.timeout(5_000);
  const combined = signal ? AbortSignal.any([signal, timeout]) : timeout;
  const res = await fetch(url, { signal: combined });
  if (!res.ok) throw new Error(`HTTP ${res.status}`);
  return (await res.json()) as unknown; // json() is Promise<any>: re-type as unknown and validate
}
export { fetchJson };
```

`AbortSignal.any` needs Node.js 20.3+ and is typed by the DOM lib and `@types/node`.

##### `Promise.withResolvers` (lib es2024)

```ts
function deferred<T>() {
  const { promise, resolve, reject } = Promise.withResolvers<T>();
  return { promise, resolve, reject };
}
export { deferred };
```

Available in Node.js 22+; requires `lib: es2024` or later.

***

#### Iterators and Generators

- **`strictBuiltinIteratorReturn` (5.6, part of `strict`)**: built-in iterators (`Array.prototype.values()`, `Map.prototype.entries()`, …) return `BuiltinIteratorReturn` = `undefined` in their done result instead of `any`, so `it.next().value` is `T | undefined`.
- Iterator helpers (`map`, `filter`, `take`, `drop`, `flatMap`, `reduce`, `toArray`, `forEach`, `some`, `every`, `find`, `Iterator.from`) are declared only with `lib: esnext` in 5.9 (`esnext.iterator`); 6.0 moves them to `es2025`. Runtime: Node.js 22+.
- Generators are typed `Generator<Yield, Return, Next>`; annotate public generator functions. `for...of` over a `Generator` ignores the return value.

```ts
// @lib: ["esnext"]
function* ids(): Generator<number, void, undefined> {
  let i = 0;
  while (true) yield i++;
}
const firstFive = ids().take(5).toArray();      // lazy: takes only what it needs
const m = new Map([["a", 1]]);
const v: number = m.values().next().value;      // error TS2322: number | undefined
export { firstFive, v };
```

***

#### New in 5.9: ArrayBuffer, Typed Arrays and `Buffer`

5.7 made typed arrays generic over their buffer (`Uint8Array<TArrayBuffer extends ArrayBufferLike = ArrayBufferLike>`). 5.9 completes the change: `ArrayBuffer` is no longer a structural supertype of typed arrays, and `SharedArrayBuffer` is no longer structurally assignable to `ArrayBuffer` (in 5.9's lib, `ArrayBuffer[Symbol.toStringTag]` is the literal type `"ArrayBuffer"` instead of `string`, so a `Uint8Array` or `SharedArrayBuffer` — tagged `"Uint8Array"`/`"SharedArrayBuffer"` — no longer matches it structurally; the `SharedArrayBuffer` `[Symbol.species]` member also moved from the instance type to the constructor), so a view typed with the default `ArrayBufferLike` is no longer a `BufferSource`.

The exact boundary, measured with 5.8.3 and 5.9.3 (`Uint8Array → ArrayBuffer`, `SharedArrayBuffer → ArrayBuffer`, `Uint8Array<ArrayBufferLike> → Uint8Array<ArrayBuffer>`, `Uint8Array<ArrayBufferLike> → BufferSource`):

| | `lib` ≤ `es2023` | `lib` `es2024`/`esnext` |
|---|---|---|
| 5.8 | all four compile | first three are errors; `BufferSource` compiles |
| 5.9 | all four are errors | all four are errors |

So projects on `lib: es2022`/`es2023` see the most new errors; projects already on `es2024`+ mainly see the `BufferSource` errors (Web Crypto, `fetch`, `Blob`, `TextDecoder`):

```ts
// @lib: ["es2022", "dom"]
const u8 = new Uint8Array([1, 2, 3]);            // Uint8Array<ArrayBuffer>
const ab: ArrayBuffer = u8;                      // error TS2322 (5.9; 5.8 accepted it with lib es2022)
const ok: ArrayBuffer = u8.buffer;               // pass the buffer, not the view

declare const view: Uint8Array;                  // = Uint8Array<ArrayBufferLike>
await crypto.subtle.digest("SHA-256", view);     // error TS2345: ArrayBufferLike not assignable to BufferSource
const exact: Uint8Array<ArrayBuffer> = view;     // error TS2322: may be backed by a SharedArrayBuffer

function hash(data: Uint8Array<ArrayBuffer>) {   // GOOD: say what Web APIs need
  return crypto.subtle.digest("SHA-256", data);
}
await hash(u8);
export { ab, ok, exact };
```

Typical 5.9 upgrade errors and fixes:

- `Type 'Buffer' is not assignable to type 'Uint8Array<ArrayBufferLike>'` / `'ArrayBufferLike' is not assignable to 'BufferSource'` — update `@types/node` first (current versions type `Buffer.from()` as `Buffer<ArrayBuffer>` and `readFileSync` as a non-shared buffer).
- Annotate parameters that flow into `fetch` bodies, `Blob`, `TextDecoder`, `crypto.subtle` or `Response` as `Uint8Array<ArrayBuffer>` rather than bare `Uint8Array`.
- Where an API truly expects an `ArrayBuffer`, pass `view.buffer` — but note that a view may cover only part of its buffer (`byteOffset`/`byteLength`, Node.js `Buffer` pooling): `buf.buffer` can contain unrelated bytes. Use `buf.buffer.slice(buf.byteOffset, buf.byteOffset + buf.byteLength)` or copy with `new Uint8Array(buf)`.
- Do not "fix" these errors with `as ArrayBuffer` or `as any`: the old assignability let a `SharedArrayBuffer`-backed view or a whole pooled buffer reach APIs that need an exact, non-shared `ArrayBuffer`.

***

#### Declaration Files and Publishing Libraries

##### `isolatedDeclarations` (5.5)

`isolatedDeclarations` requires every exported declaration to carry enough annotations that a `.d.ts` file can be produced from the single file, without type checking — so other tools (and parallel builds) can emit declarations.

```ts
// @isolatedDeclarations: true
// @declaration: true
export function add(a: number, b: number) {        // error TS9007: explicit return type required
  return a + b;
}
export const limit = 10;                            // OK: literal is trivially inferable
export const defaults = { retries: 3 };             // OK: simple object literal
export const handler = (x: string): string => x;    // OK: annotated
export const computed = Math.max(1, 2);             // error TS9010: explicit type annotation required
```

Use it in libraries and monorepos; it pairs with `--noCheck` (5.6) for "emit declarations fast, type-check separately" pipelines.

##### Declaration hygiene

- Ship `.d.ts` files generated by `tsc` (`declaration: true`, `declarationMap: true` for go-to-source). Hand-written declarations drift from the implementation.
- Check exports with `@arethetypeswrong/cli` before publishing: wrong `"types"` condition order, CJS-only types for an ESM package, or missing `.d.mts`/`.d.cts` are common.
- A `.d.ts` compiled by 5.8+ may contain computed property names (`[propName]: number`) that TypeScript 5.7 and older read differently; libraries that support older consumers must test with the oldest supported compiler (`typesVersions` can redirect old compilers).
- `skipLibCheck: true` skips checking all `.d.ts` files, including your own hand-written ones. Keep it on for speed but check hand-written declaration files in a separate CI step or move them into `.ts` files.
- `declare global { ... }` and `declare module "x" { ... }` augmentations change types for the whole program. Review them like global mutable state.
- `/// <reference types="..." />` in published `.d.ts` files forces a dependency on the consumer; prefer imports.

##### `types` and `typeRoots`

Without `types`, 5.9 includes **every** package under `node_modules/@types` (from all ancestor directories) as globals. Mixed `@types/jest` + `@types/mocha` or `@types/node` in a browser project create conflicting or misleading globals (`setTimeout` returning `NodeJS.Timeout` in a browser app). Set `"types": ["node"]` (or `[]`) explicitly; 6.0 makes `[]` the default.

***

#### JavaScript Projects (`checkJs` and JSDoc)

TypeScript type-checks JavaScript with `allowJs` + `checkJs` (or `// @ts-check` per file). JSDoc carries the types:

```ts
// @nocheck
// file: math.js  (checked with "allowJs": true, "checkJs": true)
/** @import { Config } from "./config.js" */      // 5.5: type-only import in JSDoc

/**
 * @param {Config} cfg
 * @param {number} [scale]
 * @returns {number}
 */
export function size(cfg, scale = 1) {
  return cfg.width * scale;
}

/** @satisfies {Record<string, number>} */
export const limits = { a: 1, b: 2 };

/**
 * @overload
 * @param {string} x
 * @returns {string}
 */
/**
 * @overload
 * @param {number} x
 * @returns {number}
 */
/** @param {string | number} x */
export function echo(x) { return x; }
```

JSDoc `@import` (5.5), `@satisfies` (4.9) and `@overload` (5.0) are supported. `@type {any}` and missing annotations silently produce `any` unless `noImplicitAny` applies (it does for `checkJs` with `strict`).

***

#### Security

TypeScript types provide **no** runtime protection. Review every boundary:

##### Validate untrusted input

```ts
interface Transfer { to: string; amount: number }

// BAD: compiles, validates nothing
async function handleBad(req: Request): Promise<Transfer> {
  return (await req.json()) as Transfer;      // amount may be "1e309", a string, or missing
}

// GOOD: parse unknown, check every field (or use a schema library such as Zod/Valibot/ArkType)
function parseTransfer(v: unknown): Transfer {
  if (typeof v !== "object" || v === null) throw new TypeError("body must be an object");
  const { to, amount } = v as Record<string, unknown>;
  if (typeof to !== "string" || !/^[A-Z]{2}\d{2}[A-Z0-9]{10,30}$/.test(to)) throw new TypeError("bad 'to'");
  if (typeof amount !== "number" || !Number.isFinite(amount) || amount <= 0) throw new TypeError("bad 'amount'");
  return { to, amount };
}
async function handle(req: Request): Promise<Transfer> {
  return parseTransfer(await req.json());
}
export { handleBad, handle };
```

Derive types from schemas (`z.infer<typeof Schema>`) instead of writing an interface and a schema separately; two sources of truth drift.

##### Prototype pollution and dynamic keys

```ts
// BAD: attacker controls key "__proto__" or "constructor"
function setPathBad(target: Record<string, any>, path: string[], value: unknown): void {
  let obj = target;
  for (const key of path.slice(0, -1)) obj = obj[key] ??= {};
  obj[path.at(-1)!] = value;                 // ["__proto__", "isAdmin"] pollutes Object.prototype
}

// GOOD: reject dangerous keys, use null-prototype objects or Map
const FORBIDDEN = new Set(["__proto__", "prototype", "constructor"]);
function setPath(target: Record<string, unknown>, path: readonly string[], value: unknown): void {
  if (path.length === 0 || path.some((k) => FORBIDDEN.has(k))) throw new Error("invalid path");
  let obj: Record<string, unknown> = target;
  for (const key of path.slice(0, -1)) {
    const next = Object.hasOwn(obj, key) ? obj[key] : undefined;
    obj = (typeof next === "object" && next !== null ? next : (obj[key] = Object.create(null))) as Record<string, unknown>;
  }
  obj[path[path.length - 1]!] = value;
}
export { setPathBad, setPath };
```

`Record<string, T>` and index signatures type `obj["__proto__"]` as `T`, which is false for ordinary objects. Merging user JSON into objects (`Object.assign`, deep-merge utilities, `{ ...user }` is safe but `deepMerge(config, user)` often is not) needs key filtering.

##### Other review points

- `eval`, `new Function`, `setTimeout(string)`, `innerHTML`, `document.write`, `dangerouslySetInnerHTML` with untrusted data — types do not help. Use Trusted Types / sanitisers.
- Regular expressions from user input: escape them (5.9 has no `RegExp.escape` types; write a helper or a local declaration) and bound their complexity (ReDoS). TypeScript 5.5+ checks the *syntax* of regex literals (`/(?<a>x)\k<b>/` is TS1532), not their performance.
- `child_process.exec` with interpolated strings → use `execFile`/`spawn` with argument arrays.
- Path traversal: `path.join(root, userPath)` must be followed by a containment check on the resolved path.
- Secrets: compare tokens with `crypto.timingSafeEqual` on equal-length buffers; never log request bodies or headers wholesale.
- `JSON.parse` revivers and `structuredClone` do not validate shapes.
- Dependency types: a `@types/*` package can be wrong about nullability or exceptions; read the implementation for security-critical calls.

***

#### Performance

##### Runtime performance (what TypeScript affects)

- `target` decides how much helper code is emitted: `target: es5` (deprecated) turns `for...of`, spread, `async`/generators and classes into slower helpers; ES2017+ keeps `async`/`await` native; ES2022+ keeps class fields and `#private` native.
- `importHelpers: true` + `tslib` deduplicates helpers across files in CommonJS/older-target builds.
- Enums compile to objects with lookups; `const enum` and `as const` objects compile to literals or plain objects. Namespaces compile to IIFEs.
- Type-only constructs cost nothing at runtime; `verbatimModuleSyntax` guarantees that `import type` imports do not load modules.
- Avoid `delete obj.key` in hot paths and objects that change shape; TypeScript types do not prevent V8 de-optimisations.

##### Compiler performance

- **New in 5.9**: the checker caches instantiations on type mappers (fewer repeated instantiations in libraries such as Zod and tRPC), and file-existence checks avoid closure allocation (about 11% faster in some large projects, per the release notes).
- Measure before tuning: `tsc --extendedDiagnostics` (time per phase, instantiation counts), `tsc --generateTrace <dir>` (open in Perfetto / `@typescript/analyze-trace`), `tsc --explainFiles` (why each file is in the program), `tsc --listFilesOnly`.
- Use project references (`composite: true`, `tsc -b`) and `incremental: true` (`.tsbuildinfo`) in large repositories. Keep `.tsbuildinfo` out of published packages.
- `skipLibCheck: true` removes the cost of checking `node_modules` declarations.
- Explicit return types on exported functions, `interface` extension instead of large intersections, and named types instead of huge inline unions reduce checker work.
- `--noCheck` (5.6) emits without type checking (for fast emit when a separate `tsc --noEmit` job checks types); never use it as the only build step.
- Scope inputs with `include: ["src"]`. Without `include`, `tsc` excludes `outDir` only while `exclude` is unset: a tsconfig with `"exclude": ["node_modules"]` and `outDir: "dist"` reads `dist/*.d.ts` from the previous build and fails with TS5055 *Cannot write file ... because it would overwrite input file* (verified on 5.9.3).

***

#### Testing

- Type-check in CI: `tsc --noEmit` (or `tsc -b` for project references) as a required job. Bundlers, `tsx`, esbuild, swc, Vite and Node.js type stripping **do not type-check**.
- Test runners that strip types (Vitest, Jest with `ts-jest` `isolatedModules`, `node --test` with type stripping) also run code that does not type-check. Do not rely on them for type safety.
- Type-level tests: use `@ts-expect-error` for negative cases and `expectTypeOf` (Vitest) or `tsd` for exact type assertions.

```ts
type Equal<A, B> = (<T>() => T extends A ? 1 : 2) extends <T>() => T extends B ? 1 : 2 ? true : false;
type Expect<T extends true> = T;

declare function pick<T, K extends keyof T>(o: T, k: K): T[K];
type _t1 = Expect<Equal<ReturnType<typeof pick<{ a: number }, "a">>, number>>;

// @ts-expect-error — "b" is not a key
pick({ a: 1 }, "b");
```

- `@ts-expect-error` comments must name the expected problem; an unexplained one hides unrelated errors on the same line.
- Mocks typed as `any` (`jest.fn() as any`) defeat type checking of the code under test; prefer `vi.fn<typeof realFn>()` / `jest.fn<typeof realFn>()` or `satisfies Partial<Service>`.

***

#### Tooling

##### `tsc` and builds

- Pin the compiler exactly in `devDependencies` (`"typescript": "~5.9.3"`); type-checking results can change in minor releases (as 5.9's inference and lib changes show).
- `tsc -p tsconfig.build.json` for emit and `tsc --noEmit` for checking are common; ensure both include the same files.
- `tsc --showConfig` prints the effective configuration including implied options (useful to see what `module: node20` implies).
- `extends` accepts arrays (5.0) and package names (`"extends": ["@tsconfig/strictest", "@tsconfig/node22"]`); `${configDir}` (5.5) in `extends`-ed configs refers to the directory of the *extending* config.

##### Running TypeScript directly

| Runner | Type checks? | Notes |
|---|---|---|
| Node.js type stripping (`node file.ts`) | No | On by default since Node.js 22.18/23.6, no warning since 22.18/24.3, stable since 24.12/25.2. Erasable syntax only: no enums, runtime namespaces, parameter properties, `import =`, decorators; no `.tsx`; does not read `tsconfig.json` (no `paths`); refuses `.ts` under `node_modules`; type-only imports must use `import type`. |
| `tsx` / esbuild / swc / Bun / Deno | No | Single-file transforms; same `isolatedModules` constraints |
| `tsc` then `node` | Yes | Slowest, but the only option that type-checks |

Recommended tsconfig for code executed by Node.js type stripping (5.9):

```jsonc
{
  "compilerOptions": {
    "module": "nodenext",              // or "node20"
    "target": "esnext",
    "noEmit": true,                    // or emit with rewriteRelativeImportExtensions
    "allowImportingTsExtensions": true,
    "rewriteRelativeImportExtensions": true,
    "erasableSyntaxOnly": true,
    "verbatimModuleSyntax": true,
    "strict": true,
    "types": ["node"]
  }
}
```

With this configuration `enum`, parameter properties and runtime `namespace`s fail with TS1294, and a value import of a type fails with TS1484 — the same code Node.js rejects at runtime with `ERR_UNSUPPORTED_TYPESCRIPT_SYNTAX` or a missing-export `SyntaxError`.

##### Linting

- typescript-eslint (8.x supports TypeScript `>=4.8.4 <6.1.0`, so 5.9 is supported) with type-aware rules (`recommendedTypeChecked` / `strictTypeChecked`). Key rules: `no-floating-promises`, `no-misused-promises`, `await-thenable`, `no-unsafe-*` (any leaks), `no-unnecessary-condition`, `switch-exhaustiveness-check`, `consistent-type-imports`, `no-non-null-assertion`, `prefer-nullish-coalescing`, `restrict-template-expressions`, `unbound-method`, `only-throw-error`.
- Linting with type information needs the TypeScript compiler API; it cannot run on TypeScript 7.0's native compiler (no API in 7.0).

##### Editor and language service

- **New in 5.9**: expandable hovers (preview) let you expand a type in a hover (`+`/`-` in VS Code), and the maximum hover length is configurable (`js/ts.hover.maximumLength` in VS Code) with a much larger default. DOM APIs show MDN-derived summary descriptions.
- Make sure the editor uses the workspace TypeScript (VS Code: "TypeScript: Select TypeScript Version" → "Use Workspace Version"), otherwise the editor and CI disagree.

***

#### Observability and Debugging

- Emit source maps (`sourceMap: true`, or `inlineSourceMap`) and run Node.js with `--enable-source-maps` so stack traces point to `.ts` lines. `declarationMap: true` lets "go to definition" land in sources of referenced projects.
- Node.js type stripping replaces type annotations with whitespace, so line and column numbers match the source without source maps.
- Keep error classes with stable `name`s and `cause` chains (see Error Handling); structured loggers should serialise `cause`.
- `tsc --generateTrace` and `--extendedDiagnostics` are the observability tools for slow builds.

***

#### Deprecations That Become Errors in 6.0

TypeScript 6.0 turns the following into errors (TS5101/TS5107 for options, specific errors for syntax) unless `"ignoreDeprecations": "6.0"` is set, and TypeScript 7.0 removes them entirely (TS5102/TS5108). **5.9 accepts all of them without any warning** (verified on 5.9.3), so a 5.9 codebase can accumulate them unnoticed. Flag new uses in review and prefer the replacements, which all work in 5.9.

| Deprecated in 6.0 | 5.9 behaviour | Replacement valid in 5.9 |
|---|---|---|
| `target: "es5"` | OK (and the 5.9 default) | `es2015` or later (ideally `es2022`+) |
| `downlevelIteration` | OK | Raise `target` to `es2015`+ |
| `moduleResolution: "node"` / `"node10"` | OK (default for `module: commonjs`) | `nodenext`/`node16` (Node.js) or `bundler` |
| `moduleResolution: "classic"` | OK (and the 5.9 default when `module` is `es2015`…`esnext` and `moduleResolution` is unset) | `bundler` or `nodenext` |
| `module: "amd"`, `"umd"`, `"system"`, `"none"` | OK (`none` rejects imports) | `esnext`/`preserve` + bundler, or `nodenext` |
| `outFile` | OK (with `amd`/`system`) | A bundler |
| `baseUrl` | OK | `paths` without `baseUrl`, or `package.json` `"imports"` |
| `esModuleInterop: false`, `allowSyntheticDefaultImports: false` | OK | Remove (both effectively always on) |
| `alwaysStrict: false` | OK | Remove; all code is strict-mode |
| `module Foo { }` (namespace declared with `module`) | OK | `namespace Foo { }` (`declare module "pkg"` stays valid) |
| `import x from "./a.json" assert { type: "json" }` and `import("./a.json", { assert: {...} })` | OK under `esnext`/`node18`/`preserve`; TS2880 under `node20`/`nodenext` | `with { type: "json" }` |
| `/// <reference no-default-lib="true"/>` | OK | `noLib` or `libReplacement` |
| Passing files on the command line while a `tsconfig.json` exists (`tsc file.ts`) | Silently ignores the tsconfig | `tsc -p`, or (in 6.0) `--ignoreConfig` |

6.0 also changes defaults (`strict`, `module`, `target`, `types`, `rootDir`, `noUncheckedSideEffectImports`, `libReplacement`). A 5.9 tsconfig that sets all of these explicitly behaves the same on 6.0, which is the cheapest way to prepare for the upgrade.

***

#### Removed / Changed in 5.9 — Migration Notes

Upgrading from 5.8 to 5.9 (all verified by compiling the same code with 5.8.3 and 5.9.3):

1. **ArrayBuffer / typed-array assignability** (most common break). Expect TS2322/TS2345 around `Buffer`, `Uint8Array`, `BufferSource`, `BlobPart`, `crypto.subtle`, `TextDecoder`. Update `@types/node`, annotate `Uint8Array<ArrayBuffer>`, or pass `.buffer` correctly sliced (see [ArrayBuffer section](#new-in-59-arraybuffer-typed-arrays-and-buffer)).
2. **Type-argument inference** changes in generic-heavy code. Add explicit type arguments at the failing call sites.
3. **`Intl.Segments.containing()`** now returns `SegmentData | undefined`; dereferencing the result without a check is TS2532 (see [Target and lib](#target-and-lib)).
4. **DOM lib regeneration**: some DOM types changed shape (`lib.dom.d.ts` is regenerated every release). Update code rather than pinning an old `@types/web` unless necessary.
5. **`tsc --init`** writes different defaults; existing tsconfigs are unaffected.
6. **`module: nodenext`** now implies `resolveJsonModule`; JSON imports that used to fail now compile (and must use `with { type: "json" }` in ESM files).
7. **New syntax** (`import defer`) and **new option value** (`module: node20`) are rejected by 5.8 and earlier; do not use them in libraries whose `.ts` sources or tsconfig are consumed by older compilers.
8. Nothing was removed from the compiler options in 5.9. The options deprecated in 5.0 were announced for removal in 5.5, but 5.5.4 still accepts them with `"ignoreDeprecations": "5.0"`; they fail unconditionally from 5.6 (verified on 5.5.4 and 5.6.3).

Upgrading from 5.9 to 6.0 is covered in [Deprecations that become errors in 6.0](#deprecations-that-become-errors-in-60). Upgrading to 7.0 additionally needs every tool that imports the `typescript` API (linters, `ts-morph`, `ts-patch`, Angular compiler, custom transformers) to keep a 6.0 installation side by side, since 7.0 has no API.

***

#### Idioms and Things to Avoid

| Avoid | Prefer | Why |
|---|---|---|
| `any`, `as any`, `Function`, `Object`, `{}` as "anything" | `unknown`, specific function types, `object`, `Record<string, unknown>` | `any` disables checking and spreads |
| `obj!.x` / `x as T` without proof | Narrowing, assertion functions, validated parsing | Unchecked claims |
| `@ts-ignore` | `@ts-expect-error <reason>` | Fails when no longer needed |
| `enum` in erasable / type-stripped code | `as const` object + union | Enums are runtime syntax |
| `namespace` for code organisation | ES modules | Namespaces are legacy, not erasable |
| Default exports | Named exports | Refactoring and interop |
| `import { SomeType }` | `import type { SomeType }` / inline `type` | Required by `verbatimModuleSyntax` and single-file transpilers |
| Extensionless relative imports under `nodenext` | `./x.js` | Required by Node.js ESM |
| `paths` aliases without runtime support | `package.json` `"imports"` | `paths` does not rewrite output |
| `\|\|` for defaults | `??` | `0`, `""`, `false` are valid values |
| `arr.forEach(async ...)` | `for...of` + `await`, or `Promise.all(arr.map(...))` | Lost rejections, no completion |
| `catch (e: any)` | `catch (e)` (unknown) + narrowing | Anything can be thrown |
| Method-syntax callbacks in interfaces | Property-syntax function types | Parameter bivariance |
| `Object.keys(obj) as (keyof T)[]` | Explicit key lists or `Map` | Objects may carry extra keys |
| Bare `Uint8Array` parameters passed to Web APIs (5.9) | `Uint8Array<ArrayBuffer>` | 5.9 `BufferSource` checks |
| `target: "esnext"` for code run by a fixed Node.js version | A concrete `esNNNN` target | `esnext` emits syntax (decorators, `using`) the runtime may not parse |
| Relying on 5.9 defaults | Explicit `strict`, `module`, `target`, `types` | Defaults change in 6.0 |

***

#### Production / Review Checklist

##### Configuration

- [ ] `"typescript"` pinned to `~5.9.3` (or exact) in `devDependencies`; lockfile committed; the editor uses the workspace version
- [ ] `strict: true` explicit; extra flags considered: `noUncheckedIndexedAccess`, `noImplicitOverride`, `exactOptionalPropertyTypes` (new code), `noFallthroughCasesInSwitch`, `noImplicitReturns`
- [ ] `module` is `nodenext`/`node20` (Node.js) or `preserve`/`esnext` + `bundler` (bundled code); not `commonjs` + `node10` in new projects
- [ ] `target`/`lib` match the oldest supported runtime; `lib: esnext` APIs checked against the runtime (`Promise.try`, `Error.isError`, `Float16Array` need Node.js 24)
- [ ] Browser projects list `"dom.iterable"` next to `"dom"` (5.9 does not merge them)
- [ ] `types` explicit (`["node"]`, test-runner types only in test configs)
- [ ] `verbatimModuleSyntax` (and `erasableSyntaxOnly` when Node.js type stripping or another strip-only runtime executes the code)
- [ ] No options deprecated in 6.0 (`baseUrl`, `moduleResolution: node`, `target: es5`, `downlevelIteration`, `outFile`, `esModuleInterop: false`, `alwaysStrict: false`) added; no 6.0-only options (`es2025`, `ignoreDeprecations: "6.0"`, `types: ["*"]`, `stableTypeOrdering`)
- [ ] `tsc --noEmit` (or `tsc -b`) runs in CI; tools that only strip types are not the only gate

##### Types and code

- [ ] No unexplained `any`, `as`, `!`, `@ts-ignore`; `@ts-expect-error` has a reason
- [ ] External data (`JSON.parse`, `fetch().json()`, env vars, message events) parsed from `unknown` with validation
- [ ] Discriminated unions with exhaustive `switch` (`satisfies never` / `assertNever`)
- [ ] Narrowed property values not used after calls that may mutate the object
- [ ] Public functions have explicit return types; `isolatedDeclarations` in libraries
- [ ] Generic type parameters are used at least twice; no "return-type-only" generics as hidden casts
- [ ] Callback-typed interface members use property syntax
- [ ] `exactOptionalPropertyTypes` semantics respected (no `undefined` written into optional properties unless declared)
- [ ] Class fields: no subclass field shadowing base-class initialisation; `declare` for type-only redeclarations; `#private` for real privacy
- [ ] Standard vs legacy decorators chosen deliberately; no `target: esnext` with standard decorators when `tsc` output runs on Node.js
- [ ] `using`/`await using` resources not returned from their scope; `SuppressedError` handled where relevant
- [ ] Typed arrays passed to Web APIs typed as `Uint8Array<ArrayBuffer>`; `.buffer` uses respect `byteOffset`/`byteLength`
- [ ] No `import defer` unless the bundler/runtime implements it and `module` is `esnext`/`preserve`

##### Async and errors

- [ ] No floating promises; no `async` callbacks where `void` is expected (`forEach`, event emitters) without error handling
- [ ] `return await` inside `try` blocks; `cause` preserved when wrapping errors
- [ ] Timeouts/cancellation with `AbortSignal.timeout`/`AbortSignal.any`; bounded concurrency for large `Promise.all`
- [ ] `catch` variables treated as `unknown`

##### Security

- [ ] No type assertions on untrusted input; schemas define both validation and types
- [ ] Dynamic keys from users rejected/filtered (`__proto__`, `constructor`, `prototype`) or stored in `Map`/null-prototype objects
- [ ] No `eval`/`new Function`/`innerHTML` with untrusted data; user-supplied regexes escaped and bounded
- [ ] Secrets compared in constant time; not logged

##### Build, tooling and observability

- [ ] Source maps emitted and `--enable-source-maps` set in production Node.js processes (unless running type-stripped `.ts`)
- [ ] Published packages checked with `attw`; `exports` map has `types` conditions; `.d.ts` generated, not hand-written
- [ ] typescript-eslint type-aware rules enabled (`no-floating-promises`, `no-misused-promises`, `no-unsafe-*`, `switch-exhaustiveness-check`)
- [ ] Slow builds investigated with `--extendedDiagnostics` / `--generateTrace`; `incremental`/project references for large repos
