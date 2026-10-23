### TypeScript 6.0: The Complete Production Developer Guide

> **Version**: TypeScript 6.0 — beta 6.0.0-beta (2026-02-11), RC 6.0.1-rc (2026-03-06), first stable build **6.0.2 (2026-03-23)**; latest and probably final patch **6.0.3 (2026-04-16)**.[^1][^2]
> **Status (October 2026)**: 6.0 is the **last compiler written in TypeScript/JavaScript**[^10] and the bridge release to the native (Go) compiler. There is no 6.1. The newest stable release is **TypeScript 7.0** (7.0.2, 2026-07-08); 7.1.0-beta appeared on npm on 2026-10-20. TypeScript publishes no LTS or end-of-life schedule — fixes ship only in new releases of the current line — so 6.0.3 is effectively frozen. 6.0 stays relevant because TypeScript 7.0 ships **no programmatic API**: typescript-eslint, Vue/Svelte/Astro/Angular/MDX tooling and other compiler-API consumers still run on 6.0, either directly or through the `@typescript/typescript6` compatibility package (`tsc6`).[^3][^4]
> **Scope**: The full TypeScript language as usable with the 6.0 compiler, every 6.0 change versus 5.9 (new defaults, deprecations, new options, library typings), module resolution for Node.js and bundlers, tsconfig recommendations, reviewer-relevant compiler pitfalls, tooling and the 7.0 migration path. Every example was compiled with `tsc` 6.0.3 (and boundary claims against 5.9.3 and 7.0.2); emitted JavaScript was run on Node.js 20.20.2, 22.23.4, 24.22.1, 25.9.0 and 26.12.1 where runtime behaviour matters.
> **Note**: This document is self-contained. Sections marked **[New in 6.0]** cover behaviour specific to this version.

---

#### Table of Contents

1. What's New in TypeScript 6.0
2. Version-Aware Review — Do Not Use or Recommend
3. Code Style and Project Conventions
4. tsconfig.json in 6.0 — New Defaults
5. Deprecations (errors in 6.0, removed in 7.0)
6. Recommended tsconfig Profiles
7. Modules and Module Resolution
8. Types: Fundamentals
9. Narrowing and Control-Flow Analysis
10. Generics, Inference and `satisfies`
11. Advanced Types: Conditional, Mapped, Template Literal
12. Classes, Decorators and `accessor`
13. Enums, Namespaces and Erasable Syntax
14. Resource Management: `using` / `await using`
15. Library Typings: ES2025, ESNext, Temporal, DOM
16. Error Handling
17. Async Code and Promises
18. Declaration Files, `isolatedDeclarations` and Library Publishing
19. JavaScript Checking (`checkJs`, JSDoc)
20. Security-Relevant Patterns
21. Performance of the Compiler and of Emitted Code
22. Testing and Type Tests
23. Tooling: Linters, Runners, Type Stripping, Editors
24. Observability
25. Preparing for TypeScript 7.0
26. Removed / Changed in 6.0 — Migration Notes
27. Production / Review Checklist

---

#### What's New in TypeScript 6.0

TypeScript 6.0 contains few language features. It is a **configuration and ecosystem release**: it changes ten compiler defaults, turns a long list of legacy options and syntax into errors, adds the `es2025` target/lib and typings for several Stage 4 proposals, and introduces flags that help projects move to the 7.0 native compiler.[^1]

**Changed defaults** (verified by compiling the same project with 5.9.3 and 6.0.3):

| Option | 5.9 default | 6.0 default |
|---|---|---|
| `strict` (and every `strict*` family flag) | `false` | `true` |
| `module` | `commonjs` (`target` < ES2015), else `es2015` | documented as `esnext`; 6.0.3 actually derives it from `target`: `es2022` for the default `es2025` target (`esnext` only with `target: esnext`) |
| `moduleResolution` | `node10` / `classic` depending on `module` | `bundler` (unless `module` is `node16`/`node18`/`node20`/`nodenext`) |
| `target` | `es5` | `es2025` (floating: the newest supported ES year) |
| `types` | every package in `node_modules/@types` | `[]` (nothing auto-included) |
| `rootDir` | common directory of all input files | directory containing `tsconfig.json` |
| `noUncheckedSideEffectImports` | `false` | `true` |
| `libReplacement` | `true` | `false` |
| `esModuleInterop` / `allowSyntheticDefaultImports` | `false` (unless implied) | `true`, and `false` is deprecated |
| `alwaysStrict` | `false` unless `strict` | `true`, and `false` is deprecated |

**Deprecations that are now errors** (silenced with `"ignoreDeprecations": "6.0"`, removed outright in 7.0): `target: es5`, `downlevelIteration`, `moduleResolution: node`/`node10`/`classic`, `module: amd`/`umd`/`system`/`none`, `baseUrl`, `outFile`, `esModuleInterop: false`, `allowSyntheticDefaultImports: false`, `alwaysStrict: false`, and import assertions (`assert { … }`; under `module: nodenext` they are already TS2880 since 5.8, and `node20` since 5.9, which `ignoreDeprecations` does not silence). Two syntax changes are errors that `ignoreDeprecations` does **not** silence: the `module Foo { }` namespace keyword (TS1540) and, on the command line, `tsc file.ts` in a directory that contains a `tsconfig.json` (TS5112).[^1]

**New features**:

- `es2025` value for `target` and `lib`; `RegExp.escape`, `Promise.try`, Iterator helpers, `Set` methods, `Float16Array` and `Intl.DurationFormat` now live in `lib.es2025.*` instead of `esnext`.
- `Temporal` typings (`esnext.temporal`), `Map`/`WeakMap` `getOrInsert`/`getOrInsertComputed` (`esnext.collection`), `Uint8Array` base64/hex methods (`esnext.typedarrays`), `Date.prototype.toTemporalInstant` (`esnext.date`).
- `lib: ["dom"]` now includes `dom.iterable` and `dom.asynciterable`.
- Subpath imports starting with `#/` resolve under `moduleResolution: nodenext` and `bundler`.
- `moduleResolution: bundler` may be combined with `module: commonjs`.
- `--stableTypeOrdering` makes union/property ordering match 7.0.
- `--ignoreConfig` compiles command-line files while ignoring a present `tsconfig.json`.
- Inference: functions that never use `this` are no longer context-sensitive, so method-syntax callbacks infer like arrow functions. After the RC, checking of function expressions in generic calls (notably generic JSX) was also tightened, and the `assert` deprecation was extended to `import(…, { assert: … })`.
- `esnext.intl` gains the `Intl.Locale` info getters (`getWeekInfo()`, `getTextInfo()`, `getCalendars()`, `getTimeZones()` …) and `Intl.DateTimeFormat` overloads that format Temporal objects; `Intl.DurationFormat` is typed in `es2025.intl`. The old lib names `esnext.promise`, `esnext.iterator` and `esnext.float16` remain valid aliases of the new `es2025.*` files.

**API compatibility**: 6.0 is API-compatible with 5.9 — tools built on the `typescript` npm package keep working.[^1]

***

#### Version-Aware Review — Do Not Use or Recommend

##### How to judge code for a 6.0 project

- Determine the **compiler** version from `package.json` (`"typescript": "~6.0.3"`, or an alias such as `"typescript": "npm:@typescript/typescript6@^6.0.2"`), and the **runtime** separately (`engines.node`, `.nvmrc`, Dockerfile, browserslist). The compiler version decides which syntax and options type-check; the runtime decides which library APIs exist at run time. TypeScript never polyfills.
- Before claiming that an option, lib name or syntax "does not exist", check it against 6.0. Many things that look new are older than expected: `satisfies` (4.9), `const` type parameters (5.0), `using` (5.2), `NoInfer` (5.4), inferred type predicates (5.5), `erasableSyntaxOnly` (5.8), `import defer` and `module: node20` (5.9) are all available in 6.0. Say "not available in 6.0" only for things this guide or the 6.0 compiler proves missing.
- A tsconfig that still sets deprecated options together with `"ignoreDeprecations": "6.0"` compiles on 6.0, but it is **technical debt with a deadline**: 7.0 rejects every one of those options and ignores `ignoreDeprecations`. Flag it as a migration item, not as a build break.
- `"types": ["*"]` is valid in 6.0 and 7.0 but **an error in 5.9** (TS2688 "Cannot find type definition file for '*'"); do not suggest it for configs that must work with both.

##### Do not use or recommend (not available in 6.0)

The compiler is TypeScript 6.0. These belong to newer releases, other tools, or proposals that 6.0 does not type:

- **TypeScript 7.0 behaviour** — none of it applies to a 6.0 build: the native Go `tsc` binary of 7.0 (`tsgo` in the earlier `@typescript/native-preview` package) and its 8–12× speed-up, parallel checking, always-on stable type ordering (on 6.0 it costs up to 25 % check time and is opt-in via `--stableTypeOrdering`), Unicode-code-point inference for template literal types, the stricter JSDoc rules (no Closure syntax, no constructor functions, `typeof` required for values in JSDoc types), and the "new API" planned for 7.1. Do not tell a 6.0 project that a deprecated option "has been removed" — on 6.0 it is a deprecation error that `ignoreDeprecations` can silence.[^3][^5]
- **TypeScript 7.1** — a 7.1.0-beta exists on npm (2026-10-20); do not recommend anything from it.
- **Library APIs without 6.0 typings**: `Math.sumPrecise`, `Iterator.concat`, `Iterator.zip`, async iterator helpers (`AsyncIterator.prototype.map` and friends), `Error.captureStackTrace` without `@types/node`. Using them requires a local `declare global` augmentation and a runtime that has them.
- **APIs that are typed only in `esnext`** must not be suggested when the project's `lib` is `es2025` or lower: `Temporal`, `Map.prototype.getOrInsert`/`getOrInsertComputed`, `Uint8Array.fromBase64`/`toBase64`/`fromHex`/`toHex`, `Error.isError`, `Array.fromAsync`, `Atomics.pause`, `DisposableStack`/`AsyncDisposableStack`, `Symbol.dispose` (with `using`), `Intl.Locale.prototype.getWeekInfo()` and the other `Intl.Locale` info getters.
- **Runtime APIs the deployment target lacks** — verified with the installed runtimes: Node.js 20 has none of Iterator helpers, `Set` methods, `Promise.try`, `RegExp.escape`, `Float16Array`, `Object.groupBy`, `Array.fromAsync`; Node.js 22 lacks `Promise.try`, `RegExp.escape`, `Float16Array`, `Intl.DurationFormat`, `Error.isError`, `DisposableStack`; Node.js 24 lacks `Temporal`, `Map.prototype.getOrInsert` and `Uint8Array.fromBase64`; Node.js 25 lacks `Temporal` and `getOrInsert`; Node.js 26 has all of them (and also `Iterator.concat`, which 6.0 does not type). No Node.js release has `Math.sumPrecise`.
- **Decorators in Node.js type stripping**: Node's built-in TypeScript support does not transform decorators, enums, namespaces with values, parameter properties or `import =` aliases.[^6]

##### Deprecated or removed older features (do not introduce in 6.0 code)

- Removed **before** 6.0 (stopped functioning in 5.5, where `"ignoreDeprecations": "5.0"` still accepted but ignored them; hard errors with no escape hatch since 5.6): `charset`, `importsNotUsedAsValues`, `preserveValueImports`, `keyofStringsOnly`, `suppressExcessPropertyErrors`, `suppressImplicitAnyIndexErrors`, `noImplicitUseStrict`, `noStrictGenericChecks`, `out`, `target: es3`, `prepend` in project references. (6.0.3 reports TS5102 "has been removed" for all of them, including `prepend` in a project reference, and silently tolerates `importsNotUsedAsValues: "remove"`, the old default value.)[^7]
- Deprecated **in 6.0** (see the Deprecations section): everything listed in What's New.
- `experimentalDecorators` and `emitDecoratorMetadata` are **not** deprecated in 6.0 and still work, but new code should use standard decorators (5.0+).

***

#### Code Style and Project Conventions

##### Naming and formatting

- `camelCase` for variables, functions and methods; `PascalCase` for types, interfaces, classes, enums and type parameters with meaning (`TItem` or `Item`; single letters `T`, `K`, `V` for simple generics); `SCREAMING_SNAKE_CASE` only for true module-level constants.
- No `I` prefix on interfaces and no `T` suffix on types.
- File names: `kebab-case.ts` or the project's established convention; one exported concept per file where practical.
- Formatting is a tool's job (Prettier, Biome, dprint); do not review whitespace by hand.

##### Exports and imports

```ts
// Named exports (preferred): refactor-safe, tree-shakeable, consistent names at import sites
export interface UserInput { name: string }
export function createUser(input: UserInput): { id: string; name: string } {
  return { id: crypto.randomUUID(), name: input.name };
}
export const DEFAULT_PAGE_SIZE = 50;
```

- Avoid `export default` in application code: importers choose arbitrary names and renames do not propagate. Framework conventions (Next.js pages, Storybook stories, config files) are the exception.
- Barrel files (`index.ts` re-exporting everything) slow down type checking, tests and bundlers and create import cycles; use them only for a package's public entry point.
- Group imports: Node.js built-ins (`node:` prefix), external packages, internal aliases, relative imports. Use `import type` for type-only imports.
- Keep modules free of top-level side effects (connections, timers, reading env) except in entry points; import-time side effects make tests and `import defer` unpredictable.

##### Project layout

```text
src/
├── domain/        # types and pure logic, no I/O
├── services/      # business logic
├── adapters/      # HTTP, DB, queues — validation at these boundaries
├── config.ts      # env parsing and validation, once at start-up
└── index.ts       # entry point / public API
```

***

#### tsconfig.json in 6.0 — New Defaults

##### `strict` is on by default **[New in 6.0]**

An empty `tsconfig.json` (`{}`) now type-checks in strict mode. A project that relied on the old default suddenly gets `TS7006 Parameter 'x' implicitly has an 'any' type`, `TS18048 'x' is possibly 'undefined'`, `TS2564 Property has no initializer` and similar errors. Opting out with `"strict": false` is still allowed and is **not** deprecated — but review it as a red flag in new code.

```ts
// tsconfig: {}  — 5.9 compiled this silently, 6.0 reports TS7006
export function legacy(x) { // error TS7006: Parameter 'x' implicitly has an 'any' type.
  return x;
}
```

`alwaysStrict` no longer follows `strict`: it is on even with `"strict": false` (only `"alwaysStrict": false` plus `ignoreDeprecations` turns it off). Emitted CommonJS starts with `"use strict"`, and sloppy-mode-only code (`with`, octal literals, reserved words such as `static`, `public`, `await` used as identifiers) is rejected.

##### `module` → ES modules, `moduleResolution` → `bundler` **[New in 6.0]**

When `module` is not set, 6.0 emits **ECMAScript modules** and resolves imports with **bundler** rules (extensionless relative imports are accepted). This silently changes the output of any project that relied on the old CommonJS default:

```ts
// tsconfig: {"compilerOptions":{"rootDir":"src","outDir":"dist"}}, package.json without "type"
// src/main.ts
import { two } from "./lib";   // accepted under moduleResolution: bundler
console.log(two);
```

Compiled with 5.9, `dist/main.js` was CommonJS (`require("./lib")`) and ran on every Node.js version. Compiled with 6.0, `dist/main.js` contains `import { two } from "./lib";` — **no compile error**, but Node.js 20, 22 and 24 all fail at run time with `ERR_MODULE_NOT_FOUND: Cannot find module '…/dist/lib'` (Node.js ESM requires file extensions; Node 24 additionally warns `MODULE_TYPELESS_PACKAGE_JSON`). The effective default is narrower than the release notes say: 6.0.3 computes `module` from `target`, so with the default `target: es2025` it is `es2022`, not `esnext` (7.0.2 behaves the same). Verified consequence: a config without `module` rejects import attributes (`import data from "./data.json" with { type: "json" }` is TS2823) and `import defer` (TS18060); setting `"module": "esnext"` (or `preserve`) fixes both. Review rule: **every tsconfig must set `module` explicitly**; for code that Node.js loads it must be `nodenext`, `node20` or `commonjs`. Never rely on the default.

##### `target` → `es2025` **[New in 6.0]**

The default target is now the newest ECMAScript year TypeScript supports — `es2025` in 6.0 — and it **floats**: a future compiler with a newer default would emit newer syntax for the same config. The implied default `lib` is `es2025.full`, which includes the DOM. Consequences:

- Code is no longer down-levelled: class fields, `#private`, `??=`, `static {}` blocks are emitted as written. Only syntax newer than ES2025 (for example `using` declarations) is still transformed.
- Node.js projects without an explicit `lib` see `document`, `window` and the ES2025 library — APIs the runtime may lack (Node 22 has no `Promise.try`/`RegExp.escape`).
- Set `target` and `lib` explicitly for anything that runs outside an evergreen browser.

##### `types` → `[]` **[New in 6.0]**

Global declarations from `@types/*` packages are no longer included automatically. A Node.js project with `{}` as config fails with:

```text
error TS2591: Cannot find name 'process'. Do you need to install type definitions for node? Try `npm i --save-dev @types/node` and then add 'node' to the types field in your tsconfig.
```

Fix with an explicit list, which is also faster (the TypeScript team reports 20–50 % shorter builds on projects that used to load hundreds of `@types` packages):[^1]

```json
{
  "compilerOptions": {
    "types": ["node"]
  }
}
```

- Test globals need their own entries: `"types": ["node", "vitest/globals"]` or `["node", "jest"]` — usually in a separate `tsconfig.test.json`.
- `"types": ["*"]` restores 5.9 behaviour (6.0+ only; 5.9 rejects it).
- `types` only controls **globals**. Packages you `import` still load their own types; `import { readFile } from "node:fs"` without `@types/node` in `types` fails because `node:fs` is declared by the global `@types/node` ambient modules.

##### `rootDir` → directory of `tsconfig.json` **[New in 6.0]**

In 5.9, a config with `"outDir": "dist"` and sources only under `src/` emitted `dist/index.js`, because `rootDir` was inferred as `src`. In 6.0 the inferred root is the config directory. `tsc` reports the following error — and, unless `noEmitOnError` is set, still writes `dist/src/index.js` (verified on 6.0.3 and 7.0.2):

```text
error TS5011: The common source directory of 'tsconfig.json' is './src'. The 'rootDir' setting must be explicitly set to this or another path to adjust your output's file layout.
```

Fix: `"rootDir": "./src"`. (`tsc --help` in 6.0.3 still prints "default: Computed from the list of input files" — that text describes only the no-tsconfig command-line case.) Check also that `package.json` `main`/`exports`/`types` point at the right `dist/` paths after the change.

##### `noUncheckedSideEffectImports` → `true` **[New in 6.0]**

Side-effect imports (`import "./x"`) are now resolved and checked. Typos are caught — and so are asset imports that a bundler handles:

```ts
// 6.0 defaults (moduleResolution bundler)
import "./styles.css";   // error TS2882 — needs a declaration such as `declare module "*.css" {}`
import "./polyfill";     // error TS2882 — a real typo / missing file
```

The fix for assets is an ambient declaration (`declare module "*.css" {}`) or the bundler's client types (`"types": ["vite/client"]`), **not** turning the check off.

##### `libReplacement` → `false` **[New in 6.0]**

`@typescript/lib-*` replacement packages are no longer looked up unless `"libReplacement": true` is set. Projects that install `@typescript/lib-dom` (for example to pin DOM typings) must set it explicitly.

##### `ignoreDeprecations: "6.0"` and `--ignoreConfig` **[New in 6.0]**

- `"ignoreDeprecations": "6.0"` turns all 6.0 deprecation errors (TS5101, TS5107, TS2880) back into silent acceptance. It does **not** silence TS1540 (`module` namespace keyword). `"5.0"` is still a valid value but does not silence 6.0 deprecations (TS5101 still reports `baseUrl`); any other value, such as `"7.0"`, is TS5103. 7.0 accepts the option but the deprecated settings are errors there regardless (TS5102/TS5108 "has been removed").
- `--ignoreConfig` is new in 6.0 (5.9: TS5023 "Unknown compiler option").
- Running `tsc file.ts` in a directory with a `tsconfig.json` used to ignore the config silently; 6.0 reports `TS5112: tsconfig.json is present but will not be loaded if files are specified on commandline. Use '--ignoreConfig' to skip this error.` Scripts and lint-staged hooks that call `tsc --noEmit $FILES` break on upgrade — use `tsc -p .` or add `--ignoreConfig` deliberately (and accept that the project's options are not applied).

***

#### Deprecations (errors in 6.0, removed in 7.0)

Every row except `amd-module` was verified: 5.9.3 accepts the setting, 6.0.3 reports the listed error unless `"ignoreDeprecations": "6.0"` is set, and 7.0.2 rejects it even with `ignoreDeprecations`.

| Setting / syntax | 6.0 diagnostic | 7.0 diagnostic | Replacement |
|---|---|---|---|
| `"target": "es5"` | TS5107 | TS5108 removed | `es2015`+; use an external transpiler for ES5 |
| `"downlevelIteration"` (any value, even `false`) | TS5101 | TS5102 removed | delete it |
| `"moduleResolution": "node"` / `"node10"` | TS5107 | TS5108 removed | `nodenext` (Node.js) or `bundler` |
| `"moduleResolution": "classic"` | TS5107 | TS5108 removed | `nodenext` or `bundler` |
| `"module": "amd"` / `"umd"` / `"system"` / `"none"` | TS5107 | TS5108 / TS6046 | `esnext`/`preserve` + bundler, or `nodenext` |
| `"baseUrl"` | TS5101 | TS5102 removed | prefix `paths` entries with `./src/…` |
| `"outFile"` | TS5101 | TS5102 removed | a bundler |
| `"esModuleInterop": false` / `"allowSyntheticDefaultImports": false` | TS5107 | TS5108 removed | delete; fix `import * as x` calls |
| `"alwaysStrict": false` | TS5107 | TS5108 removed | delete; fix sloppy-mode code |
| `import x from "./a.json" assert { type: "json" }` (also `import("./a.json", { assert: … })`) | TS2880 | TS2880 (7.0.2 flags only the static form) | `with { type: "json" }` |
| `module Foo { }` (namespace) | TS1540, **not** silenced by `ignoreDeprecations` | TS1540 | `namespace Foo { }` |
| `/// <reference no-default-lib="true"/>` | silently ignored | ignored | `noLib` or `libReplacement` |
| `/// <amd-module name="…"/>` | no effect (per release notes) | no effect | — |
| `tsc a.ts` next to a `tsconfig.json` | TS5112 | TS5112 | `tsc -p .` or `--ignoreConfig` |

Diagnostics read, for example: `error TS5101: Option 'baseUrl' is deprecated and will stop functioning in TypeScript 7.0. Specify compilerOption '"ignoreDeprecations": "6.0"' to silence this error.`

##### `baseUrl` — migrate before 7.0

`baseUrl` was both a prefix for `paths` and a **lookup root** for bare specifiers (`import "utils/x"` resolved to `<baseUrl>/utils/x`). 6.0 deprecates it. With `"ignoreDeprecations": "6.0"` the old behaviour is kept — verified: `import { v } from "utils/x"` with `"baseUrl": "./src"` still resolves to `src/utils/x.ts` on 6.0.3 — but 7.0 rejects the option, so bare lookups break there. The release notes describe the lookup root as going away; do not rely on it. Migrate:

```json
{
  "compilerOptions": {
    "paths": {
      "@app/*": ["./src/app/*"],
      "@lib/*": ["./src/lib/*"]
    }
  }
}
```

Projects that really relied on bare lookups add `"*": ["./src/*"]` to `paths` (7.0's error message suggests exactly that). Remember that `paths` is **type-checking only**: the runtime (Node.js, the bundler, the test runner) needs its own alias configuration — prefer package.json `imports` (`#/…`, below), which Node.js, bundlers and TypeScript all understand.

##### `esModuleInterop: false` is a type/emit mismatch trap in 6.0

With `"esModuleInterop": false` plus `"ignoreDeprecations": "6.0"`, 6.0 **type-checks as if interop were on but emits as if it were off**. Verified with a CommonJS package whose typings use `export =`:

```ts
// tsconfig: module commonjs, esModuleInterop false, ignoreDeprecations "6.0"
import legacy from "legacy";   // 5.9: TS1259 error. 6.0: no error.
console.log(legacy());
```

5.9 rejected the default import. 6.0 accepts it, but the emitted code is `const legacy_1 = require("legacy"); (0, legacy_1.default)()` without the `__importDefault` helper, and Node.js throws `TypeError: (0 , legacy_1.default) is not a function`. Removing `esModuleInterop: false` makes 6.0 emit `__importDefault` and the program prints `42`. **Flag every `esModuleInterop: false` in a 6.0 config as a runtime bug risk**, not just a deprecation.

The opposite pattern — `import * as legacy from "legacy"; legacy()` — is an error in 6.0 (TS2497: "This module can only be referenced with ECMAScript imports/exports by turning on the 'esModuleInterop' flag and referencing its default export"). Use a default import (or `import legacy = require("legacy")` in CommonJS output without `erasableSyntaxOnly`).

***

#### Recommended tsconfig Profiles

Write the options that matter explicitly even when 6.0's defaults match: the defaults float with the compiler version, and explicit settings make the intent reviewable. Every profile below compiles a sample project on 6.0.3 without deprecation errors and is accepted unchanged by 7.0.2.

##### Node.js application or library (ESM or CommonJS, emitted by `tsc`)

```json
{
  "compilerOptions": {
    "module": "nodenext",
    "target": "es2023",
    "lib": ["es2023"],
    "types": ["node"],
    "rootDir": "./src",
    "outDir": "./dist",
    "strict": true,
    "noUncheckedIndexedAccess": true,
    "exactOptionalPropertyTypes": true,
    "noImplicitOverride": true,
    "noFallthroughCasesInSwitch": true,
    "verbatimModuleSyntax": true,
    "isolatedModules": true,
    "noUncheckedSideEffectImports": true,
    "skipLibCheck": true,
    "sourceMap": true,
    "declaration": true,
    "declarationMap": true
  },
  "include": ["src"]
}
```

- `module: nodenext` implies `moduleResolution: nodenext`, makes the output format follow `package.json` `"type"` and the `.mts`/`.cts` extensions, and enforces ESM rules (explicit `.js` extensions, JSON import attributes). Use `node20` instead when the build must not change behaviour on a compiler upgrade; `nodenext` floats with the newest Node.js semantics.
- Pick `target`/`lib` from the **oldest supported runtime**: `es2023` for Node.js 20, `es2024` for Node.js 22, `es2025` for Node.js 24 (Node 24 still lacks some `esnext` library APIs, see Library Typings).
- **[New in 6.0]** In 5.9, `module: nodenext` implied `target: esnext` and `module: node20` implied `target: es2023`. In 6.0.3 neither implication applies any more — both fall back to the `es2025` default (verified: `/a/v` regex is accepted with `module: node20`, `using` is down-levelled with `module: nodenext`, unlike 5.9). Set `target` explicitly.

##### Bundled application (Vite, esbuild, webpack, Rspack, Bun)

```json
{
  "compilerOptions": {
    "module": "preserve",
    "moduleResolution": "bundler",
    "target": "es2022",
    "lib": ["es2023", "dom"],
    "types": [],
    "noEmit": true,
    "strict": true,
    "noUncheckedIndexedAccess": true,
    "verbatimModuleSyntax": true,
    "isolatedModules": true,
    "allowImportingTsExtensions": true,
    "jsx": "react-jsx",
    "skipLibCheck": true
  },
  "include": ["src"]
}
```

- `module: preserve` (5.4+) keeps `import`/`export` and `require` exactly as written and implies `moduleResolution: bundler`, `esModuleInterop` and `resolveJsonModule`. `allowImportingTsExtensions` requires `noEmit` or `emitDeclarationOnly`.
- In 6.0 `lib: ["dom"]` already contains `dom.iterable` and `dom.asynciterable`; listing them is harmless (they are empty files now) but redundant.
- Add the bundler's ambient types (`"types": ["vite/client"]`) so asset and `import.meta.env` imports type-check under the new `noUncheckedSideEffectImports` default.

##### Gradual migration of an old CommonJS project **[New in 6.0]**

`moduleResolution: bundler` can now be combined with `module: commonjs` (5.9: TS5095 "Option 'bundler' can only be used when 'module' is set to 'preserve' or to 'es2015' or later"). It is the closest replacement for the deprecated `moduleResolution: node` when the output is CommonJS that a bundler or a CommonJS-aware runtime consumes:

```json
{
  "compilerOptions": {
    "module": "commonjs",
    "moduleResolution": "bundler",
    "target": "es2022",
    "types": ["node"],
    "rootDir": "./src",
    "outDir": "./dist"
  },
  "include": ["src"]
}
```

Treat it as a stepping stone: bundler resolution accepts extensionless imports and `exports` conditions that plain Node.js may resolve differently. For code that Node.js loads directly, move to `nodenext`.

##### `tsc --init` in 6.0

`tsc --init` (minimal since 5.9) writes `module: nodenext`, `target: esnext`, `types: []`, `strict`, `noUncheckedIndexedAccess`, `exactOptionalPropertyTypes`, `verbatimModuleSyntax`, `isolatedModules`, `noUncheckedSideEffectImports`, `moduleDetection: force`, `jsx: react-jsx`, `skipLibCheck`, source maps and declarations. `target: esnext` is fine for type-checking only, but for emitted code choose a concrete year.

##### Options worth enabling beyond `strict`

| Option | Since | What it catches |
|---|---|---|
| `noUncheckedIndexedAccess` | 4.1 | `arr[i]` and `record[key]` are `T \| undefined` |
| `exactOptionalPropertyTypes` | 4.4 | `{ a?: string }` no longer accepts an explicit `a: undefined` |
| `noImplicitOverride` | 4.3 | overriding methods must say `override` |
| `noPropertyAccessFromIndexSignature` | 4.2 | forces `obj["key"]` for index-signature access |
| `noImplicitReturns`, `noFallthroughCasesInSwitch` | — | missing returns and fall-through `case`s |
| `useUnknownInCatchVariables` (part of `strict`, 4.4) | 4.4 | `catch (e)` is `unknown`, not `any` |
| `strictBuiltinIteratorReturn` (part of `strict`, 5.6) | 5.6 | built-in iterators return `undefined`, not `any` |
| `verbatimModuleSyntax` | 5.0 | type-only imports must say `type`; ESM syntax rejected in CommonJS output |
| `isolatedModules` | — | constructs single-file transpilers cannot compile |
| `erasableSyntaxOnly` | 5.8 | enums, runtime namespaces, parameter properties, `import =` |
| `isolatedDeclarations` | 5.5 | exported API without explicit types (libraries) |
| `noUncheckedSideEffectImports` | 5.6 (default in 6.0) | unresolvable `import "x"` |
| `allowUnreachableCode: false`, `allowUnusedLabels: false` | — | dead code and stray labels become errors |

***

#### Modules and Module Resolution

##### Choosing `module` / `moduleResolution`

| Situation | `module` | `moduleResolution` |
|---|---|---|
| Code run directly by Node.js (ESM or CJS) | `nodenext` (or pinned `node20`) | implied (`nodenext` / `node16`) |
| Code consumed by a bundler, Bun, or a framework build | `preserve` (or `esnext`) | `bundler` |
| Library published to npm | `nodenext` | implied |
| Legacy CommonJS output consumed by a bundler | `commonjs` | `bundler` (6.0+) |

Values that no longer exist in 6.0 without `ignoreDeprecations`: `module: amd/umd/system/none`, `moduleResolution: node/node10/classic`.

##### ESM rules under `nodenext`

```ts
// Project: package.json {"type":"module"}, module nodenext, verbatimModuleSyntax
import { one } from "./util";            // error TS2835: Relative import paths need explicit file extensions … Did you mean './util.js'?
import { Id } from "./util.js";           // error TS1484: 'Id' is a type and must be imported using a type-only import when 'verbatimModuleSyntax' is enabled.
import data from "./data.json";           // error TS1543: Importing a JSON file into an ECMAScript module requires a 'type: "json"' import attribute …
```

Correct:

```ts
import { one } from "./util.js";                       // .js even though the source is util.ts
import type { Id } from "./util.js";
import data from "./data.json" with { type: "json" };  // import attributes (5.3+), not `assert`
```

- The `.js` extension refers to the **output** file. With `allowImportingTsExtensions` + `rewriteRelativeImportExtensions` (5.7+), `./util.ts` may be written and `tsc` rewrites it to `./util.js` on emit — useful for code that also runs under Node.js type stripping, which requires the real `.ts` extension. `rewriteRelativeImportExtensions` rewrites only relative specifiers that are string literals.
- In a CommonJS-format file (`.cts`, or `.ts` without `"type": "module"`), `verbatimModuleSyntax` rejects `import x from` (TS1295) and `export const` (TS1287) because they would have to be rewritten; use `import x = require("x")` and `export =`, or switch the file to ESM.

##### `require()` of ES modules (5.8+)

Under `module: nodenext` and `node20`, a CommonJS file may `import`/`require()` an ES module (Node.js 20.19+ and 22.12+ support synchronous `require(esm)` without a flag; verified on 20.20.2 and 22.23.4). Under `node16`/`node18` the same import is TS1479. The ES module must not use top-level `await`.

##### Subpath imports and `#/` **[New in 6.0]**

`package.json` `imports` are the runtime-standard alternative to `paths`:

```json
{
  "name": "my-app",
  "type": "module",
  "imports": { "#/*": "./dist/*" }
}
```

```ts
// src/main.ts — resolves to src/util/greet.ts for type checking, ./dist/util/greet.js at run time
import { greet } from "#/util/greet.js";
```

- TypeScript 6.0 resolves `#/…` specifiers under `moduleResolution: nodenext` and `bundler`; 5.9 reports TS2307. Prefixes such as `#app/*` worked before 6.0.
- **Runtime support differs from the release notes' wording** ("newer Node.js 20 releases"): verified, Node.js 20.20.2 and 22.23.4 reject `#/util/greet.js` with `ERR_INVALID_MODULE_SPECIFIER`; Node.js 24.22.1, 25.9.0 and 26.12.1 accept it. The feature shipped in Node.js 25.4.0 and was back-ported to 24.14.0 (2026-02-24).[^8] A package whose `engines.node` allows 20 or 22 must use a named prefix (`#app/*`), not `#/`.
- When `imports` point at `./dist/*`, TypeScript maps them back to the sources through `rootDir`/`outDir`; keep both set.

##### `paths`

`paths` affects **type checking only**. A review must confirm the runtime has a matching alias (bundler config, `imports` field, test-runner `moduleNameMapper`/`resolve.alias`) — otherwise code type-checks and fails at run time. Since 6.0 `paths` entries must not rely on `baseUrl`; write them relative to the tsconfig (`"@lib/*": ["./src/lib/*"]`).

##### `import defer` (5.9+)

```ts
// module: esnext or preserve only (TS18060 under nodenext)
import defer * as heavy from "./heavy.js";

export function rarelyUsed(): number {
  return heavy.compute(); // the module body runs on first property access
}
```

Only the namespace form is allowed (5.8 and earlier: parse error); TypeScript does not down-level it,[^11] and **no Node.js version up to 26.12.1 parses it** (`SyntaxError: Unexpected token '*'`, verified). It is only useful when a bundler transforms it. Flag `import defer` in code that Node.js runs directly.

##### `verbatimModuleSyntax` and `isolatedModules`

`verbatimModuleSyntax` (5.0+) means "what you write is what is emitted": imports without `type` stay, `import type` disappears. It replaces the removed `importsNotUsedAsValues`/`preserveValueImports`. Under it:

```ts
import type { Server } from "node:http";             // erased
import { createServer, type IncomingMessage } from "node:http"; // createServer kept, IncomingMessage erased
import "./register-side-effects.js";                   // kept

export function handler(req: IncomingMessage): Server {
  return createServer(() => req.socket.destroy());
}
```

Re-exporting a type needs `export type { T } from "./x.js"` under `isolatedModules`. Prefer `verbatimModuleSyntax` everywhere a single-file transpiler (esbuild, SWC, Babel, Node.js type stripping) is in the pipeline.

##### Module detection and `.mts`/`.cts`

- `moduleDetection: "force"` treats every non-declaration file as a module (no accidental global scripts). Default `auto` treats files without `import`/`export` as scripts whose top-level declarations are global.
- `.mts` always emits `.mjs` (ESM); `.cts` always emits `.cjs` (CommonJS), regardless of `package.json`.
- `__dirname`, `__filename` and `require` do not exist in ESM. Use `import.meta.dirname`/`import.meta.filename` (Node.js 20.11+, typed by `@types/node`) or `fileURLToPath(import.meta.url)`.

***

#### Types: Fundamentals

Global truths that hold in 6.0:

- **Types are erased.** Annotations, interfaces, type aliases and generics produce no JavaScript. Nothing is validated at run time unless code does it (`JSON.parse`, `fetch().json()`, `process.env`, `req.body` are all untrusted).
- **Typing is structural.** Two types with the same shape are interchangeable regardless of name. Nominal behaviour needs brands or `#private` fields.
- **`strict` is the baseline** — and the 6.0 default. Code review assumes `strictNullChecks`, `noImplicitAny`, `strictFunctionTypes`, `strictPropertyInitialization`, `useUnknownInCatchVariables` and `strictBuiltinIteratorReturn`.
- **The compiler does not polyfill.** `lib` describes what the runtime has; `target` only controls syntax down-levelling.

##### `any` vs `unknown`

```ts
// BAD: any disables checking and spreads silently
function parseBad(json: string): any {
  return JSON.parse(json);
}
const cfg = parseBad("{}");
cfg.port.toFixed(); // compiles, throws at run time

// GOOD: unknown forces a check before use
function parse(json: string): unknown {
  return JSON.parse(json);
}
interface Config { port: number }
function isConfig(v: unknown): v is Config {
  return typeof v === "object" && v !== null && "port" in v && typeof v.port === "number";
}
const raw = parse('{"port":8080}');
if (isConfig(raw)) {
  console.log(raw.port.toFixed());
}
raw.port; // error TS18046: 'raw' is of type 'unknown'.
```

At trust boundaries prefer a schema validator (Zod, Valibot, ArkType, TypeBox) whose output type is derived from the schema; a hand-written type guard that checks fewer fields than the type declares is a lie the compiler cannot detect.

##### Type assertions and non-null assertions

`x as T` and `x!` are unchecked promises to the compiler. Review each one:

```ts
interface User { id: string; name: string }

const u1 = {} as User;               // compiles; u1.name is undefined at run time
const u2 = "x" as unknown as User;   // double assertion: always a smell

declare const map: Map<string, User>;
const a = map.get("id")!.name;       // throws if absent — prefer an explicit check
const b = map.get("id")?.name ?? "anonymous";

const n = "42" as number; // error TS2352: Conversion of type 'string' to type 'number' may be a mistake …
export { u1, u2, a, b, n };
```

Prefer `satisfies` (checks without widening or lying), annotations, or narrowing. Never use angle-bracket assertions (`<User>x`): they are a syntax error in `.tsx`, and `erasableSyntaxOnly` rejects them (TS1294, verified). Use `as`.

##### `interface` vs `type`

Both describe object shapes. House rules:

| Use case | Prefer |
|---|---|
| Object shapes, class contracts, public library APIs | `interface` (extendable, mergeable, better error messages and caching) |
| Unions, intersections of non-objects, tuples, function types, mapped and conditional types | `type` |
| Augmenting a third-party or global type | `interface` (declaration merging) |

`interface A extends B` reports incompatible members; `type A = B & { … }` silently produces `never` members for conflicts:

```ts
type Broken = { id: string } & { id: number };
const x: Broken = { id: "a" }; // error TS2322: Type 'string' is not assignable to type 'never'.

interface Base { id: string }
interface Child extends Base { id: number } // error TS2430: Interface 'Child' incorrectly extends interface 'Base'.
export { x };
```

##### Optional properties, `undefined` and `exactOptionalPropertyTypes`

```ts
// with exactOptionalPropertyTypes: true
interface Options { timeoutMs?: number }

const o1: Options = {};                       // OK: property absent
const o2: Options = { timeoutMs: undefined }; // error TS2375: Type '{ timeoutMs: undefined; }' is not assignable to type 'Options' with 'exactOptionalPropertyTypes: true'. …

interface Patch { timeoutMs?: number | undefined } // explicitly allow undefined when "unset" must be expressible
const p: Patch = { timeoutMs: undefined };
export { o1, o2, p };
```

`"key" in obj` and `obj.key !== undefined` mean different things under this flag; spreading `{ ...defaults, ...overrides }` with an explicit `undefined` in `overrides` overwrites the default — a common configuration bug.

##### Index access and `noUncheckedIndexedAccess`

```ts
// with noUncheckedIndexedAccess: true
const ports: number[] = [80, 443];
const first = ports[0];          // number | undefined
first.toFixed();                 // error TS18048: 'first' is possibly 'undefined'.

const env: Record<string, string> = { HOME: "/root" };
const home = env["HOME"] ?? "/";  // string

for (const p of ports) p.toFixed();               // iteration is not affected
const [a = 0, b = 0] = ports;                      // destructuring defaults
export { home, a, b };
```

Without the flag, `ports[5]` is typed `number` and `record[key]` is never `undefined` — reviewers should treat every unchecked index as potentially missing.

##### Readonly data

```ts
interface Settings {
  readonly name: string;
  readonly tags: readonly string[];
}
const s: Settings = { name: "a", tags: ["x"] };
s.name = "b";        // error TS2540: Cannot assign to 'name' because it is a read-only property.
s.tags.push("y");    // error TS2339: Property 'push' does not exist on type 'readonly string[]'.

const ROLES = ["admin", "user"] as const;   // readonly ["admin", "user"]
type Role = (typeof ROLES)[number];         // "admin" | "user"
const r: Role = "admin";
export { r };
```

`readonly` is shallow and compile-time only; `Object.freeze` is shallow and run-time only. A `readonly T[]` parameter documents that the function does not mutate its input — prefer it for every array parameter that is not mutated. Note that a mutable array is assignable to a `readonly` array type, but not the other way round.

##### Excess property checks and structural typing

```ts
interface Point { x: number; y: number }

const p1: Point = { x: 1, y: 2, z: 3 }; // error TS2353: Object literal may only specify known properties, and 'z' does not exist in type 'Point'.

const tmp = { x: 1, y: 2, z: 3 };
const p2: Point = tmp;                   // OK: excess checks apply only to fresh object literals

function save(p: Point): string { return JSON.stringify(p); }
save(tmp);                               // serialises z too — a data-leak vector for DTOs
export { p1, p2 };
```

When an object is serialised or sent to a client, copy the allowed fields explicitly (`{ x: p.x, y: p.y }`) or validate with a strict schema — a structural type does not strip extra properties.

##### Branded (nominal) types

```ts
type Brand<T, B extends string> = T & { readonly __brand: B };
type UserId = Brand<string, "UserId">;
type OrderId = Brand<string, "OrderId">;

function asUserId(raw: string): UserId {
  if (!/^u_[a-z0-9]+$/.test(raw)) throw new Error("invalid user id");
  return raw as UserId; // the only place an assertion is allowed
}

declare function loadUser(id: UserId): void;
const oid = "o_1" as OrderId;
loadUser(asUserId("u_1"));
loadUser(oid); // error TS2345: Argument of type 'OrderId' is not assignable to parameter of type 'UserId'.
```

##### `object`, `{}`, `Object`

- `object` — any non-primitive. Use for "some object".
- `{}` — any value except `null`/`undefined` (including strings and numbers). Rarely what is meant.
- `Object` — the wrapper interface; never use as a type.
- `Record<string, unknown>` — a plain dictionary whose values must be checked.

##### Functions

```ts
// Overloads: implementation signature is not callable from outside
function pad(value: string, width: number): string;
function pad(value: number, width: number): string;
function pad(value: string | number, width: number): string {
  return String(value).padStart(width, "0");
}
pad(7, 3);

// `this` parameter (erased) documents and checks the receiver
interface Counter { count: number }
function inc(this: Counter, by = 1): void {
  this.count += by;
}
const c: Counter = { count: 0 };
inc.call(c, 2);
inc(1); // error TS2684: The 'this' context of type 'void' is not assignable to method's 'this' of type 'Counter'.

// Callback returning a value where void is expected is allowed — a source of ignored promises
const handlers: Array<() => void> = [];
handlers.push(async () => { await Promise.resolve(); }); // OK for tsc; lint with no-misused-promises
```

- Prefer union parameters to overloads when the return type does not depend on the argument type.
- Optional parameters must come last; prefer an options object over long positional parameter lists, especially booleans.
- Method-shorthand members (`m(x: T): void`) are checked **bivariantly** even under `strictFunctionTypes`; property-style members (`m: (x: T) => void`) are checked contravariantly. Use property style in interfaces where the variance matters (callbacks, event handlers).

***

#### Narrowing and Control-Flow Analysis

##### Discriminated unions and exhaustiveness

```ts
type Shape =
  | { kind: "circle"; radius: number }
  | { kind: "square"; side: number }
  | { kind: "rect"; w: number; h: number };

function area(s: Shape): number {
  switch (s.kind) {
    case "circle":
      return Math.PI * s.radius ** 2;
    case "square":
      return s.side ** 2;
    case "rect":
      return s.w * s.h;
    default: {
      const unreachable: never = s; // compile error here when a new variant is added
      throw new Error(`unhandled shape: ${JSON.stringify(unreachable)}`);
    }
  }
}
export { area };
```

Alternatives: `s satisfies never` in the `default` branch, or `noImplicitReturns` with a return type. A `default:` that silently returns a fallback hides new variants — flag it.

##### Built-in narrowing

```ts
function describe(v: string | number | Date | null | undefined | string[]): string {
  if (v == null) return "missing";                 // null and undefined
  if (typeof v === "string") return v.toUpperCase();
  if (typeof v === "number") return v.toFixed(2);
  if (v instanceof Date) return v.toISOString();
  return v.join(",");                               // string[]
}

function hasId(x: object): x is { id: unknown } {
  return "id" in x;
}
const thing: object = { id: 1 };
if (hasId(thing)) console.log(thing.id);
if ("id" in thing) console.log(thing.id); // `in` narrows to object & Record<"id", unknown> (4.9+)
export { describe };
```

Truthiness narrowing excludes `0`, `""`, `NaN` and `0n` — `if (count)` and `if (name)` are bugs when zero or empty are valid values. Use `!== undefined` / `!= null`.

##### Inferred type predicates (5.5+)[^18]

```ts
const raw: Array<string | undefined> = ["a", undefined, "b"];
const names = raw.filter((s) => s !== undefined); // string[] — predicate inferred
const lens = names.map((s) => s.length);

// No predicate is inferred when the function does not narrow exactly:
const nonEmpty = raw.filter((s) => !!s);           // (string | undefined)[] — "" would be dropped too
const x: string[] = nonEmpty; // error TS2322: Type '(string | undefined)[]' is not assignable to type 'string[]'.
export { lens, x };
```

##### User-defined type guards and assertion functions

```ts
function isNonEmptyString(v: unknown): v is string {
  return typeof v === "string" && v.length > 0;
}

function assert(cond: unknown, msg: string): asserts cond {
  if (!cond) throw new Error(msg);
}

function assertIsDefined<T>(v: T, name: string): asserts v is NonNullable<T> {
  if (v === undefined || v === null) throw new Error(`${name} is required`);
}

const port: string | undefined = process.env["PORT"];
assertIsDefined(port, "PORT");
console.log(port.length, isNonEmptyString(port));
assert(port.length < 6, "port too long");
```

Assertion functions must be called through an explicitly typed reference: `const check = assertIsDefined;` without an annotation produces TS2775. A type guard whose body does not prove its claim (for example `return true`) silently corrupts every caller.

##### Narrowing in closures and after `const` indexed access

Boundaries verified: closure narrowing compiles on 5.4 (TS2339 on 5.3), constant indexed access on 5.5 (TS2571 on 5.4), `switch (true)` on 5.3 (TS18046 on 5.2), `NoInfer` on 5.4, inferred predicates on 5.5, `const` type parameters on 5.0.

```ts
function late(url: string | URL, names: Map<string, string>) {
  if (typeof url === "string") {
    url = new URL(url);
  }
  // 5.4+: narrowing is preserved in closures created after the last assignment
  return [...names.keys()].map((k) => url.searchParams.get(k));
}

function byKey(obj: Record<string, unknown>, key: string) {
  if (typeof obj[key] === "string") {
    return obj[key].toUpperCase(); // 5.5+: obj[key] narrows when obj and key are not reassigned
  }
  return undefined;
}
export { late, byKey };
```

Narrowing is **not** preserved when the variable is reassigned after the closure is created, inside the closure itself, or for `let` variables captured by callbacks that may run later. A narrowed property is also not invalidated by function calls that might mutate it:

```ts
interface Job { state: "idle" | "running" }
function start(job: Job, run: () => void) {
  if (job.state === "idle") {
    run();                       // may set job.state = "running"
    job.state;                   // still typed "idle" — TypeScript assumes calls do not mutate
  }
}
export { start };
```

##### `switch (true)` narrowing (5.3+)

```ts
function classify(v: unknown): string {
  switch (true) {
    case typeof v === "string":
      return v.toUpperCase();
    case Array.isArray(v):
      return `array(${v.length})`;
    default:
      return "other";
  }
}
export { classify };
```

##### Checks the compiler adds (5.6–5.8)

```ts
declare const input: string;
declare const limit: number;

if (/^\d+$/) { }                         // error TS2872: This kind of expression is always truthy.
const tooShort = input.length < limit ?? true; // error TS2869: Right operand of ?? is unreachable because the left operand is never nullish.
if (input.trim) { }                      // error TS2774: This condition will always return true since this function is always defined. …

function f() {
  let total: number;
  function show() {
    return total.toFixed(); // error TS2454: Variable 'total' is used before being assigned.
  }
  return show;
}

declare const untyped: any;
const cache = new Map<string, URL>();
function lookup(key: string): URL {
  return cache.has(key) ? cache.get(key) : untyped; // error TS2322: Type 'URL | undefined' is not assignable to type 'URL'. (5.8+: branches are checked separately; 5.7 accepted it because the `any` branch absorbed the union)
}
export { tooShort, f, lookup };
```

Boundaries (verified): TS2872/TS2869 appear from 5.6, the never-initialized check inside nested functions (TS2454) from 5.7, the per-branch return check from 5.8.[^14][^13][^12] The TS2872/TS2869 checks are **syntactic**: `Number(input) ?? 80` is *not* flagged although `Number()` never returns a nullish value — `Number("")` is `0` and `Number("x")` is `NaN`, so the default never applies. Reviewers must catch that pattern themselves; validate numeric input explicitly (`Number.isFinite(n) ? n : 80`).

***

#### Generics, Inference and `satisfies`

##### Constraints, defaults and when not to be generic

```ts
// GOOD: the type parameter relates input and output
function firstOf<T>(items: readonly T[]): T | undefined {
  return items[0];
}

function byId<T extends { id: string }>(items: readonly T[], id: string): T | undefined {
  return items.find((i) => i.id === id);
}

interface Page<T, Cursor = string> {
  items: T[];
  next?: Cursor;
}

// BAD: T appears once — it is just a disguised `any`-like cast point
function parseBad<T>(json: string): T {
  return JSON.parse(json); // caller picks T, nothing checks it
}
const user = parseBad<{ admin: boolean }>('{"admin":"no"}'); // "no" is truthy at run time

export { firstOf, byId, user };
export type { Page };
```

A type parameter used only in the return position is an unchecked assertion. Return `unknown` and validate instead.

##### `satisfies` (4.9+)

`satisfies` checks that an expression matches a type **without** widening the expression's own type:

```ts
type Route = { path: string; method: "GET" | "POST" };

const routes = {
  list: { path: "/users", method: "GET" },
  create: { path: "/users", method: "POST" },
} satisfies Record<string, Route>;

routes.list.method;      // "GET" (literal kept)
routes.missing;          // error TS2339: Property 'missing' does not exist on type '{ list: …; create: …; }'.

const annotated: Record<string, Route> = routes;
annotated.missing;       // compiles: Record<string, Route> allows any key — the annotation widened it

const bad = { path: "/x", method: "PUT" } satisfies Route; // error TS2322: Type '"PUT"' is not assignable to type '"GET" | "POST"'.
export { bad };
```

Combine with `as const` for immutable configuration: `{ … } as const satisfies Config`.

##### `const` type parameters (5.0+)[^16]

```ts
function tuple<const T extends readonly unknown[]>(...xs: T): T {
  return xs;
}
const t = tuple("a", 1, true); // readonly ["a", 1, true] — no `as const` needed at the call site

function routesOf<const T extends readonly string[]>(paths: T): T[number][] {
  return [...paths];
}
const r = routesOf(["/a", "/b"]); // ("/a" | "/b")[]
export { t, r };
```

With a `readonly` constraint the inferred tuple is `readonly`; with a mutable constraint (`T extends unknown[]`) 6.0 infers a mutable tuple (`mut(["a", 1])` is `["a", 1]`, verified). `const` only affects literals written directly in the call — an array stored in a variable first is already widened.

##### `NoInfer<T>` (5.4+)[^15]

```ts
function createFsm<S extends string>(states: S[], initial: NoInfer<S>): { states: S[]; current: S } {
  return { states, current: initial };
}
createFsm(["idle", "running"], "idle");
createFsm(["idle", "running"], "stopped"); // error TS2345: Argument of type '"stopped"' is not assignable to parameter of type '"idle" | "running"'.
```

Without `NoInfer`, `"stopped"` would become an inference candidate and `S` would silently widen to include it.

##### Context-sensitive callbacks and inference order **[New in 6.0]**

TypeScript infers type arguments first from arguments that are not "context sensitive" (functions without parameter annotations). In 5.9, every method-syntax function counted as context sensitive because of its implicit `this`, so the order of properties changed the result. 6.0 treats a function that never uses `this` like an arrow function:

```ts
declare function callIt<T>(obj: { produce: (x: number) => T; consume: (y: T) => void }): void;

callIt({
  consume(y) { return y.toFixed(); }, // 5.9: TS2339 Property 'toFixed' does not exist on type 'unknown'; 6.0: y is number
  produce(x: number) { return x * 2; },
});
```

Verified: 5.9.3 reports TS2339 (`y` is `unknown`), 6.0.3 and 7.0.2 compile it. The change can also *alter* inferred types in existing code. If an upgrade produces new errors in generic calls (the 6.0 announcement also mentions an adjustment for function expressions in generic calls, including generic JSX), add an explicit type argument or annotate the callback parameter. 5.9 had already changed inference to fix type-variable "leaks"; both changes show up as new errors far from the edited code.

##### Variance annotations (`in`/`out`, 4.7+)

`interface Producer<out T>` and `interface Consumer<in T>` document and speed up variance checks for complex generic types. Never add them to "fix" an error — a wrong annotation makes unsound assignments compile.

***

#### Advanced Types: Conditional, Mapped, Template Literal

##### Utility types

```ts
interface User {
  id: string;
  name: string;
  email: string;
  password: string;
  createdAt: Date;
}

type UserPatch = Partial<Pick<User, "name" | "email">>;
type PublicUser = Omit<User, "password">;
type UserIndex = Record<User["id"], PublicUser>;
type Ctor = ConstructorParameters<typeof URL>;      // [url: string | URL, base?: string | URL]
type Loaded = Awaited<Promise<Promise<User>>>;     // User
type Fetch = ReturnType<typeof fetch>;              // Promise<Response>
type Upper = Uppercase<"get">;                      // "GET"
export type { UserPatch, PublicUser, UserIndex, Ctor, Loaded, Fetch, Upper };
```

##### `Omit`/`Pick` on unions are not distributive

```ts
type Event =
  | { type: "click"; x: number; secret: string }
  | { type: "key"; key: string; secret: string };

type Bad = Omit<Event, "secret">;           // { type: "click" | "key" } — x and key are gone
type DistributiveOmit<T, K extends PropertyKey> = T extends unknown ? Omit<T, K> : never;
type Good = DistributiveOmit<Event, "secret">;

const e: Good = { type: "key", key: "Enter" };
const b: Bad = { type: "key", key: "Enter" }; // error TS2353: Object literal may only specify known properties, and 'key' does not exist in type 'Bad'.
export { e, b };
```

##### Conditional types and `infer`

```ts
type ElementOf<T> = T extends readonly (infer E)[] ? E : never;
type A = ElementOf<string[]>;                 // string

type IsString<T> = T extends string ? true : false;
type D = IsString<string | number>;           // boolean — distributes over the union
type NoDistribute<T> = [T] extends [string] ? true : false;
type N = NoDistribute<string | number>;       // false

type FirstArg<F> = F extends (first: infer P extends string, ...rest: never[]) => unknown ? P : never;
type F1 = FirstArg<(id: "x") => void>;        // "x"
export type { A, D, N, F1 };
```

Deeply recursive conditional types slow the checker and can hit TS2589 ("Type instantiation is excessively deep"). In review, question type-level cleverness that a simpler explicit type would cover.

##### Mapped types and key remapping

```ts
type Getters<T> = {
  [K in keyof T & string as `get${Capitalize<K>}`]: () => T[K];
};
type G = Getters<{ name: string; age: number }>; // { getName: () => string; getAge: () => number }

type Mutable<T> = { -readonly [K in keyof T]: T[K] };
type DeepReadonly<T> = T extends (...args: never[]) => unknown
  ? T
  : { readonly [K in keyof T]: DeepReadonly<T[K]> };

type PartialBy<T, K extends keyof T> = Omit<T, K> & Partial<Pick<T, K>>;
export type { G, Mutable, DeepReadonly, PartialBy };
```

##### Template literal types

```ts
type Method = "GET" | "POST";
type Path = `/api/${string}`;
type Key = `${Method} ${Path}`;

const k: Key = "GET /api/users";
const bad: Key = "PATCH /api/users"; // error TS2322: Type '"PATCH /api/users"' is not assignable to type '`GET /api/${string}` | `POST /api/${string}`'.

type Parts<S extends string> = S extends `${infer Head}.${infer Rest}` ? [Head, ...Parts<Rest>] : [S];
type P = Parts<"a.b.c">; // ["a", "b", "c"]
export { k, bad };
export type { P };
```

Inference of a template literal type from a string containing astral characters (emoji) splits UTF-16 surrogate pairs in 6.0; 7.0 consumes whole code points. Verified: for `` S extends `${infer C}${infer R}` `` with `"😀a"`, 6.0.3 infers `C` as a lone surrogate (assigning `"😀"` is TS2322), 7.0.2 infers `"😀"`. Do not rely on either behaviour.

##### `keyof`, `typeof`, indexed access

```ts
const LEVELS = { debug: 10, info: 20, warn: 30, error: 40 } as const;
type Level = keyof typeof LEVELS;                  // "debug" | "info" | "warn" | "error"
type LevelValue = (typeof LEVELS)[Level];          // 10 | 20 | 30 | 40

function threshold(l: Level): LevelValue {
  return LEVELS[l];
}

// keyof on a type with an index signature includes number for string keys
type Dict = { [k: string]: unknown };
type DK = keyof Dict;                               // string | number
export { threshold };
export type { DK };
```

`Object.keys(obj)` returns `string[]`, not `(keyof T)[]`, because objects may have extra properties at run time. Casting it to `(keyof T)[]` is only safe for objects the code created itself.

***

#### Classes, Decorators and `accessor`

##### Class basics

```ts
class Account {
  static #instances = 0;            // ECMAScript private: enforced at run time
  #balance = 0;
  private legacyFlag = false;       // TypeScript private: erased, visible at run time
  readonly id: string;

  constructor(id: string) {
    this.id = id;
    Account.#instances++;
  }

  get balance(): number {
    return this.#balance;
  }

  deposit(amount: number): void {
    if (!Number.isFinite(amount) || amount <= 0) throw new RangeError("amount must be positive");
    this.#balance += amount;
  }

  static count(): number {
    return Account.#instances;
  }
}

class Savings extends Account {
  override deposit(amount: number): void { // `override` required under noImplicitOverride
    super.deposit(amount);
  }
}
export { Savings };
```

- Prefer `#private` for real encapsulation; `private` is only a compile-time check (`(obj as any).legacyFlag` works).
- Under `strictPropertyInitialization`, every field needs an initializer, a constructor assignment, or `!` (definite assignment) — review each `!`.
- With `target` ≥ ES2022 (the 6.0 default) and `useDefineForClassFields` (default true for ES2022+), class fields use `[[Define]]` semantics and subclass field initializers run **after** the base constructor. Verified on Node.js 24: a subclass field `name = "child"` overwrites the value the base constructor assigned (silently — no diagnostic); a redeclared field without initializer (`kind?: "f"`) resets the base value to `undefined` (TS2612); a field that overrides a base accessor is TS2610. Use `declare kind: "e";` to re-type an inherited field without emitting it, and never call overridable methods from a base constructor.
- Parameter properties (`constructor(private readonly repo: Repo)`) are TypeScript-only runtime syntax; they are rejected by `erasableSyntaxOnly` and by Node.js type stripping.

##### Standard decorators (5.0+)[^16]

```ts
function logged<This, Args extends unknown[], R>(
  target: (this: This, ...args: Args) => R,
  context: ClassMethodDecoratorContext<This, (this: This, ...args: Args) => R>,
) {
  const name = String(context.name);
  return function (this: This, ...args: Args): R {
    console.debug(`-> ${name}`);
    return target.call(this, ...args);
  };
}

class Service {
  @logged
  greet(name: string): string {
    return `hi ${name}`;
  }

  @((_value, ctx) => { ctx.addInitializer(function () { /* runs per instance */ }); })
  accessor retries = 3; // auto-accessor: a private backing field plus get/set
}
console.log(new Service().greet("x"));
```

- Standard decorators need no flag, use `ClassMethodDecoratorContext` and friends, and receive no parameter information: **parameter decorators** (`constructor(@Inject() repo: Repo)`) are an error without `experimentalDecorators` (TS1206 "Decorators are not valid here").
- Node.js (V8) up to 26.12.1 does not parse decorator syntax: with `target: esnext`, TypeScript leaves `@dec` in the output and Node.js 26.12.1 throws a `SyntaxError` (verified). With `target` ≤ `es2025` (the default), decorators are down-levelled and run on every supported Node.js version. Do not set `target: esnext` for code that uses decorators and is executed without a bundler transform.
- Decorator metadata: `Symbol.metadata` is typed only with `lib: ["esnext.decorators"]` or `esnext`, and `Symbol.metadata` is `undefined` in Node.js 22, 24 and 26 (verified) — the down-levelled code needs a polyfill (`Symbol.metadata ??= Symbol("Symbol.metadata")`) loaded first.
- `experimentalDecorators` + `emitDecoratorMetadata` (Angular, NestJS, TypeORM, InversifyJS) still work unchanged in 6.0 and 7.0 — they are not deprecated. Do not mix the two decorator systems in one project; the flag switches all decorators in the program.

***

#### Enums, Namespaces and Erasable Syntax

##### Prefer union types and `as const` objects

```ts
// GOOD: erasable, works with type stripping and isolatedModules
const Status = {
  Pending: "pending",
  Active: "active",
  Closed: "closed",
} as const;
type Status = (typeof Status)[keyof typeof Status];

function isOpen(s: Status): boolean {
  return s !== Status.Closed;
}
export { isOpen };
```

##### If enums are used

```ts
enum Color { Red, Green }
declare const fromApi: number;
const c: Color = fromApi;           // compiles: any `number` is assignable to a numeric enum
const lit: Color = 42;              // error TS2322: Type '42' is not assignable to type 'Color'.
const name = Color[Color.Red];      // reverse mapping exists only for numeric members

enum Mode { Read = "r", Write = "w" }
const m: Mode = "r";                // error TS2322: Type '"r"' is not assignable to type 'Mode'.

enum Flags { None = 0, A = 1 << 0, B = 1 << 1, AB = A | B } // bit flags: the remaining good use of numeric enums
export { c, lit, name, m, Flags };
```

- Numeric enums are not closed: since 5.0 an out-of-range *literal* is rejected, but any computed `number` (from JSON, a database, arithmetic) is accepted, so a `switch` over an enum still needs a `default` that throws. String enums are nominal: plain strings are not assignable, so parse external input explicitly.
- `const enum` is inlined by `tsc` but cannot be compiled by single-file transpilers; under `isolatedModules`, using a `const enum` declared in a `.d.ts` file is TS2748 ("Cannot access ambient const enums when 'isolatedModules' is enabled", verified). Do not publish `const enum`s in libraries.
- Enums are TypeScript-only runtime syntax: rejected by `erasableSyntaxOnly` and Node.js type stripping.

##### `erasableSyntaxOnly` (5.8+)[^12]

```ts
// with erasableSyntaxOnly: true — every line below is TS1294
enum Dir { Up, Down }                                      // error TS1294
namespace Runtime { export const v = 1; }                  // error TS1294
class Repo { constructor(private readonly url: string) {} } // error TS1294
import fs = require("node:fs");                            // error TS1294
```

Erasable alternatives: `as const` objects, ES modules, explicit fields assigned in the constructor, `import`/`createRequire`. Type-only namespaces (`namespace NS { export type T = string }`) and `declare` forms remain allowed. Enable `erasableSyntaxOnly` whenever code runs through Node.js type stripping, `ts-blank-space`, or Bun/Deno with strip-only transforms.

##### Namespaces

ES modules replace namespaces for organising code. The only legitimate uses left are type-only namespaces, `declare namespace` in `.d.ts` files, and merging with functions/classes in declarations. The legacy keyword form is a 6.0 error:

```ts
module Legacy { export const a = 1; } // error TS1540: A 'namespace' declaration should not be declared using the 'module' keyword. Please use the 'namespace' keyword instead.
```

`declare module "pkg" { … }` (ambient module declarations and module augmentation) is unaffected.

***

#### Resource Management: `using` / `await using`

`using` (5.2+)[^17] calls `[Symbol.dispose]()` when the scope ends, in reverse order, even on exceptions; `await using` awaits `[Symbol.asyncDispose]()`:

```ts
class TempDir implements AsyncDisposable {
  constructor(readonly path: string) {}
  async [Symbol.asyncDispose](): Promise<void> {
    const { rm } = await import("node:fs/promises");
    await rm(this.path, { recursive: true, force: true });
  }
}

class Lock implements Disposable {
  #held = true;
  [Symbol.dispose](): void {
    this.#held = false;
  }
}

export async function job(): Promise<void> {
  await using dir = new TempDir("/tmp/job-1");
  using lock = new Lock();
  using stack = new DisposableStack();
  stack.defer(() => console.debug("cleanup", dir.path));
  // … work; disposal runs lock → stack → dir on every exit path
}
```

- Typing requires `lib` `esnext.disposable` (or `esnext`): with `lib: ["es2025"]` alone `Disposable` is missing (TS2318). 6.0 still down-levels `using` for every target except `esnext`; the helper works on Node.js 20+.
- `DisposableStack`/`AsyncDisposableStack` are runtime globals only in Node.js 24+ (verified absent in 20.20.2 and 22.23.4) — guard or polyfill them.
- A `using` variable must be initialised with an object with `[Symbol.dispose]`, `null` or `undefined`; disposal errors are aggregated into a `SuppressedError`.
- Review rule: resources acquired in a function (file handles, locks, DB connections, timers, subscriptions) should be scoped with `using`/`try … finally`, never left to garbage collection.

***

#### Library Typings: ES2025, ESNext, Temporal, DOM

`lib` decides which globals type-check; it does not make them exist. Match `lib` to the oldest runtime the code must support and verify every newer API against that runtime.

##### `es2025` target and lib **[New in 6.0]**

TypeScript has no ES2025-specific syntax transform, so `target: es2025` mostly matters for the implied `lib`. The lib moves several declarations from `esnext` into `lib.es2025.*` and adds `RegExp.escape` and `Intl.DurationFormat`:

```ts
// lib: ["es2025"] — every line type-checks on 6.0; 5.9 has no "es2025" lib (TS6046)
const word = "a+b";
const re = new RegExp(`\\b${RegExp.escape(word)}\\b`, "g");             // es2025.regexp
const p = Promise.try(() => JSON.parse('{"a":1}') as unknown);           // es2025.promise
const evens = Iterator.from([1, 2, 3, 4]).filter((n) => n % 2 === 0).toArray(); // es2025.iterator
const both = new Set([1, 2]).intersection(new Set([2, 3]));              // es2025.collection
const half = new Float16Array([1.5]);                                    // es2025.float16
const d = new Intl.DurationFormat("en", { style: "short" }).format({ hours: 1, minutes: 5 }); // es2025.intl
export { re, p, evens, both, half, d };
```

Runtime availability (verified with `typeof` checks): Node.js 22 has Iterator helpers and `Set` methods but **not** `Promise.try`, `RegExp.escape`, `Float16Array` or `Intl.DurationFormat`; Node.js 24 has all of them. Node.js 20 has none. With the 6.0 default `target` (and therefore default `lib: es2025.full`), code compiles even when the deployment runtime is Node.js 20 or 22 — set `lib` to the runtime's year.

`RegExp.escape` replaces hand-written escaping helpers, which typically cover only the classic metacharacters and break when the escaped text is placed inside a character class, after a back-reference, or in a `v`-flag pattern:

```ts
// FRAGILE: misses `-` (character classes), leading digits after `\1`, and `v`-flag syntax characters
const naive = (s: string) => s.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
// GOOD (lib es2025, Node.js 24+)
const safe = (s: string) => RegExp.escape(s);
export { naive, safe };
```

##### `esnext` additions in 6.0 **[New in 6.0]**

```ts
// lib: ["esnext"] required — none of these exist in lib es2025
const counts = new Map<string, number>();
counts.set("a", counts.getOrInsert("a", 0) + 1);                      // esnext.collection
const cache = new Map<string, string[]>();
cache.getOrInsertComputed("k", (key) => [key]).push("v");

const bytes = Uint8Array.fromBase64("aGVsbG8=");                        // esnext.typedarrays
const hex = bytes.toHex();

const now = Temporal.Now.instant();                                       // esnext.temporal
const tomorrow = Temporal.Now.plainDateISO().add({ days: 1 });
const inst = new Date().toTemporalInstant();                              // esnext.date
const isErr = Error.isError(new TypeError("x"));                          // esnext.error (5.9 already)
export { hex, now, tomorrow, inst, isErr };
```

With `lib: ["es2025"]` the same calls fail:

```ts
// lib: ["es2025"]
new Map<string, number>().getOrInsert("a", 0); // error TS2550: Property 'getOrInsert' does not exist on type 'Map<string, number>'. Do you need to change your target library? …
Temporal.Now.instant();                         // error TS2304: Cannot find name 'Temporal'.
```

Runtime availability (verified): `Temporal` and `Map.prototype.getOrInsert` exist only in Node.js 26 (absent in 24.22.1 and 25.9.0); `Uint8Array.fromBase64` in Node.js 25+; `Error.isError` in Node.js 24+. In browsers, check current compatibility data before using them without a polyfill. Typing `Temporal` with `lib: ["esnext"]` while running on Node.js 24 compiles and then throws `ReferenceError: Temporal is not defined`.

Prefer granular libs when only one proposal is needed: `"lib": ["es2024", "esnext.temporal", "dom"]`.

##### `dom` now includes `dom.iterable` and `dom.asynciterable` **[New in 6.0]**

```ts
// lib: ["es2025", "dom"] — 5.9 needed "dom.iterable" as well (TS2488 without it)
for (const el of document.querySelectorAll("a[href]")) {
  console.log(el.getAttribute("href"));
}
```

##### `TypedArray`s are generic over their buffer (5.7+, tightened in 5.9)

`Uint8Array` is `Uint8Array<ArrayBufferLike>`; since 5.9 (verified: 5.8.3 accepts, 5.9.3 rejects) an `ArrayBufferLike`-backed array is no longer assignable where `ArrayBuffer`/`BufferSource` is required. Node.js `Buffer` values often hit this:

```ts
declare function digest(data: BufferSource): Promise<ArrayBuffer>;
declare const viaShared: Uint8Array<ArrayBufferLike>;
await digest(viaShared); // error TS2345: Argument of type 'Uint8Array<ArrayBufferLike>' is not assignable to parameter of type 'BufferSource'. …

const owned: Uint8Array<ArrayBuffer> = new Uint8Array(32); // constructor results are ArrayBuffer-backed
await digest(owned);
```

Fix by typing producers as `Uint8Array<ArrayBuffer>`, copying (`new Uint8Array(viaShared)`), and keeping `@types/node` current. Do not "fix" it with `as any`.

##### Iterator return types (`strictBuiltinIteratorReturn`, 5.6+)

Built-in iterators are typed `IteratorObject<T, BuiltinIteratorReturn>`; under `strict`, `BuiltinIteratorReturn` is `undefined`, so `it.next().value` is `T | undefined` until `done` is checked:

```ts
const it = [1, 2, 3][Symbol.iterator]();
const r = it.next();
const n: number = r.value; // error TS2322: Type 'number | undefined' is not assignable to type 'number'.
if (!r.done) {
  const m: number = r.value;
  console.log(m);
}
export { n };
```

***

#### Error Handling

##### `catch` variables are `unknown`

```ts
async function load(url: string): Promise<string> {
  try {
    const res = await fetch(url);
    if (!res.ok) throw new HttpError(res.status, `GET ${url} failed`);
    return await res.text();
  } catch (err) {
    // err: unknown under strict (useUnknownInCatchVariables)
    if (err instanceof HttpError && err.status === 404) return "";
    throw new Error(`load(${url}) failed`, { cause: err }); // keep the original error
  }
}

class HttpError extends Error {
  override name = "HttpError";
  constructor(
    readonly status: number,
    message: string,
    options?: ErrorOptions,
  ) {
    super(message, options);
  }
}
export { load };
```

- `return await` inside `try` is required for the `catch` to see the rejection; a bare `return promise` escapes the `try`.
- Use `{ cause }` (ES2022) instead of string concatenation; loggers and Node.js print the cause chain.
- `err.message` on `unknown` is an error (TS18046). `String(err)` or `err instanceof Error ? err.message : String(err)` are safe. `Error.isError(err)` (esnext lib, Node.js 24+) also recognises errors from other realms.
- `throw "text"` and rejecting with non-`Error` values lose stack traces — flag them.

##### Errors as values

For expected failures (validation, not-found, conflicts), a discriminated result type keeps the error channel visible in signatures:

```ts
type Result<T, E = Error> = { ok: true; value: T } | { ok: false; error: E };

function parsePort(raw: string): Result<number, "not-a-number" | "out-of-range"> {
  const n = Number(raw);
  if (!Number.isInteger(n)) return { ok: false, error: "not-a-number" };
  if (n < 1 || n > 65535) return { ok: false, error: "out-of-range" };
  return { ok: true, value: n };
}

const r = parsePort("8080");
if (r.ok) console.log(r.value);
else console.error(r.error);
```

##### Custom error classes

- Set `name` (or `override name = "…"`) so logs show the class.
- With `target` ≥ ES2015 (always in 6.0 without `ignoreDeprecations`), `instanceof` works for subclasses of `Error`; the old `Object.setPrototypeOf(this, new.target.prototype)` workaround is only needed for ES5 output.
- `Error.captureStackTrace` is a V8/Node.js API typed by `@types/node`, not by `lib`.

***

#### Async Code and Promises

##### Floating promises

The compiler does not report un-awaited promises. These compile cleanly and lose errors:

```ts
declare function save(): Promise<void>;

function handler(): void {
  save();                        // BAD: rejection becomes an unhandledRejection (process exit in Node.js 15+)
  void save().catch(reportError); // GOOD when intentionally fire-and-forget
}

declare const items: string[];
items.forEach(async (i) => {     // BAD: forEach ignores the returned promises
  await Promise.resolve(i);
});
await Promise.all(items.map(async (i) => Promise.resolve(i))); // GOOD

if (save()) { }                  // error TS2801: This condition will always return true since this 'Promise<void>' is always defined. …

declare function reportError(e: unknown): void;
export { handler };
```

Enable typescript-eslint's `no-floating-promises`, `no-misused-promises`, `await-thenable` and `return-await` (type-aware rules) in every TypeScript service.

##### Concurrency patterns

```ts
declare function fetchUser(id: string): Promise<{ id: string }>;

// Sequential awaits in a loop serialise independent work
async function slow(ids: string[]) {
  const out = [];
  for (const id of ids) out.push(await fetchUser(id));
  return out;
}

// Parallel with fail-fast
async function fast(ids: string[]) {
  return Promise.all(ids.map(fetchUser));
}

// Parallel, collect failures
async function tolerant(ids: string[]) {
  const settled = await Promise.allSettled(ids.map(fetchUser));
  return settled.flatMap((s) => (s.status === "fulfilled" ? [s.value] : []));
}

// Timeouts and cancellation
async function withTimeout(url: string) {
  const res = await fetch(url, { signal: AbortSignal.timeout(5_000) });
  return res.json() as Promise<unknown>;
}

// Promise.withResolvers (es2024)
function deferred<T>() {
  const { promise, resolve, reject } = Promise.withResolvers<T>();
  return { promise, resolve, reject };
}
export { slow, fast, tolerant, withTimeout, deferred };
```

- Unbounded `Promise.all(hugeArray.map(…))` opens thousands of sockets/DB connections; use a concurrency limit (p-limit, a worker pool) for large inputs.
- `res.json()` returns `Promise<any>`; cast to `unknown` and validate.
- Pass `AbortSignal`s through every layer that does I/O; check `signal.throwIfAborted()` in long loops.
- `async` functions always return a promise; an `async` callback passed to an API expecting a sync callback (event emitters, `Array.prototype.forEach`, `setInterval`) needs explicit error handling.

##### Async iteration

```ts
async function* lines(stream: AsyncIterable<string>): AsyncGenerator<string, void, undefined> {
  let buf = "";
  for await (const chunk of stream) {
    buf += chunk;
    let i: number;
    while ((i = buf.indexOf("\n")) >= 0) {
      yield buf.slice(0, i);
      buf = buf.slice(i + 1);
    }
  }
  if (buf) yield buf;
}
export { lines };
```

Breaking out of `for await` calls the generator's `return()`, which runs its `finally` blocks — put cleanup there (or `await using`).

***

#### Declaration Files, `isolatedDeclarations` and Library Publishing

##### `isolatedDeclarations` (5.5+)

Requires exported declarations to carry enough annotations that a `.d.ts` file can be produced per file without type checking (fast, parallel declaration emit in other tools):

```ts
// with isolatedDeclarations: true, declaration: true
export function add(a: number, b: number) { return a + b; } // error TS9007: Function must have an explicit return type annotation with --isolatedDeclarations.
export function sub(a: number, b: number): number { return a - b; }
export const config = { retries: 3 };                        // OK: literal initialisers are inferable
export const list = [1, 2];                                  // error TS9017: Only const arrays can be inferred with --isolatedDeclarations.
export const fixed = [1, 2] as const;                        // OK
```

Enable it for published libraries and large monorepos; it also makes public API changes visible in diffs. Diagnostic codes are not stable across compilers: 7.0.2 reports the missing return type above as TS9013 ("Expression type can't be inferred") instead of TS9007 — do not key CI filters on these codes.

##### `package.json` for a TypeScript library

```json
{
  "name": "@acme/lib",
  "version": "1.0.0",
  "type": "module",
  "exports": {
    ".": {
      "types": "./dist/index.d.ts",
      "default": "./dist/index.js"
    },
    "./package.json": "./package.json"
  },
  "files": ["dist"],
  "engines": { "node": ">=20.19" },
  "peerDependencies": { "typescript": ">=5.0" },
  "peerDependenciesMeta": { "typescript": { "optional": true } }
}
```

- The `types` condition must come **first** in each `exports` entry; conditions are matched in order.
- Once `exports` exists, deep imports not listed are blocked at run time and in TypeScript (`nodenext`/`bundler`).
- Build libraries with `module: nodenext` so the emitted `.d.ts` files are valid for both Node.js and bundler consumers; check the result with `@arethetypeswrong/cli` and `publint`.
- Declaration output produced by 6.0 must stay consumable by the oldest TypeScript version the library supports. Newer syntax in emitted `.d.ts` (for example `const` type parameters or `NoInfer`) breaks consumers on older compilers; use `typesVersions` or raise the documented minimum.
- With `--stableTypeOrdering` off (default), union member order in generated `.d.ts` files can differ between 6.0 and 7.0 builds; snapshot tests of declaration output should not be the only API-compatibility check.

##### Writing `.d.ts` and augmentations

```ts
// globals.d.ts — make it a module so `declare global` is an augmentation
export {};

declare global {
  interface Window {
    __APP_CONFIG__?: { apiUrl: string };
  }
  namespace NodeJS {
    interface ProcessEnv {
      readonly DATABASE_URL?: string;
    }
  }
}
```

- Typing `process.env` keys does not make them exist — they remain `string | undefined` at run time unless validated at start-up.
- `skipLibCheck: true` (recommended for speed) also skips errors in your **own** `.d.ts` files; keep hand-written declarations small or check them in a separate CI step.
- A `declare module "x"` without a body (`declare module "x";`) types the whole module as `any`; prefer real types or a narrow declaration.

***

#### JavaScript Checking (`checkJs`, JSDoc)

`allowJs` + `checkJs` (or `// @ts-check` per file) type-check JavaScript using JSDoc annotations:

```js
// @ts-check
/** @import { Server } from "node:http" */

/**
 * @param {number} port
 * @returns {Promise<Server>}
 */
export async function listen(port) {
  const { createServer } = await import("node:http");
  const server = createServer();
  return new Promise((resolve) => server.listen(port, () => resolve(server)));
}
```

- `@import` (5.5+) imports types in JSDoc without a runtime import.
- 6.0 keeps the TypeScript 5.x JSDoc semantics. 7.0 drops Closure-style syntax, JSDoc constructor functions (`@constructor` on a `function`), values used as types without `typeof`, and postfix `!` in JSDoc types — JavaScript that relies on those passes on 6.0 but not on 7.0. Treat them as migration debt in checked JS.[^5]

***

#### Security-Relevant Patterns

Types disappear at run time; security depends on runtime checks.

##### Validate every trust boundary

```ts
interface CreateUser { email: string; role: "user" | "admin" }

// BAD: the type annotation is a claim, not a check — a client can send role: "admin"
async function createBad(req: { json(): Promise<any> }): Promise<CreateUser> {
  const body: CreateUser = await req.json();
  return body;
}

// GOOD: validate shape and values, and decide privileged fields server-side
function parseCreateUser(input: unknown): CreateUser {
  if (typeof input !== "object" || input === null) throw new TypeError("body must be an object");
  const { email } = input as Record<string, unknown>;
  if (typeof email !== "string" || !email.includes("@") || email.length > 254) {
    throw new TypeError("invalid email");
  }
  return { email, role: "user" };
}
export { createBad, parseCreateUser };
```

- `JSON.parse`, `Response.json()`, `req.body`, `process.env`, `localStorage`, `postMessage` data and message-queue payloads are untrusted, whatever their declared type.
- Object spread/merge of untrusted input into config objects (`{ ...defaults, ...input }`, recursive "deep merge") enables prototype pollution when keys like `__proto__` and `constructor` are not rejected. Use `Object.create(null)` maps or `Map`, and validate keys.
- Never build SQL, shell commands, HTML or regular expressions by string concatenation of untrusted input: use parameterised queries, `execFile`/`spawn` with argument arrays (no `shell: true`), DOM APIs/`textContent` instead of `innerHTML`, and `RegExp.escape` (es2025 lib, Node.js 24+).
- `eval`, `new Function`, `vm` and `setTimeout(string)` with any externally influenced string are injection points.
- `as` and `!` at a trust boundary bypass all checks — reject them in review.
- Compare secrets with `crypto.timingSafeEqual`, not `===`.

##### Supply chain and configuration

- Pin `typescript` with `~6.0.3` (or an exact version) and commit the lockfile. TypeScript does not follow semantic versioning: every minor release may add errors or change inference, and 6.0 changed defaults under unchanged configs.
- Do not run `tsc` against untrusted repositories expecting it to be side-effect free in editors: language-service plugins in `tsconfig.json` `plugins` load code from `node_modules`.
- Values injected through bundler `define` replacements or `import.meta.env` end up verbatim in client bundles; typing them in a `.d.ts` does not make them secret. Never route credentials through them.

***

#### Performance of the Compiler and of Emitted Code

##### Compiler performance

- Set `types` explicitly (the 6.0 default `[]` already avoids loading every `@types` package).
- `skipLibCheck: true`, `incremental: true` (or `composite` with project references and `tsc -b`) for large repositories.
- `isolatedModules` + a fast transpiler (esbuild/SWC) for builds, `tsc --noEmit` for checking in CI; `--noCheck` (5.6+) emits without checking when a separate check job exists.
- Avoid huge unions (thousands of members), deeply recursive conditional types and wide `Record<TemplateLiteral, …>` mapped types. `tsc --extendedDiagnostics` and `--generateTrace <dir>` locate slow types.
- `--stableTypeOrdering` costs up to 25 % check time on 6.0; enable it only in a dedicated job comparing 6.0 and 7.0 output, not in the regular build.
- The largest available speed-up is TypeScript 7.0 itself (reported 8–12× on full builds) — but only once the project is free of 6.0 deprecations and no longer depends on the compiler API.[^3]

##### Emitted code

- `target` below ES2022 down-levels class fields, private names, `??=` and `static {}` into helper-based code that is slower and larger; with 6.0 the minimum target is effectively ES2015 and the default ES2025 avoids this. Pick the highest target the runtime supports.
- `importHelpers` + `tslib` avoids duplicated helpers per file (only relevant when down-levelling, e.g. `using` or decorators).
- `enum` and `namespace` emit IIFEs; `const` objects are smaller and tree-shakeable.
- Spreading large arrays into arguments (`Math.max(...arr)`, `arr.push(...other)`) throws `RangeError` once the argument count exceeds the engine limit (Node.js 24: 100 000 elements work, 130 000 throw); use a loop or `reduce` for unbounded input.
- `JSON.parse(JSON.stringify(x))` turns `Date` into strings, `Map`/`Set` into `{}`, drops `undefined` and functions, and throws on `BigInt`; use `structuredClone` for deep copies.

***

#### Testing and Type Tests

##### Type-level tests

```ts
type Equals<A, B> = (<T>() => T extends A ? 1 : 2) extends <T>() => T extends B ? 1 : 2 ? true : false;
type Expect<T extends true> = T;

interface User { id: string; name: string }
type _publicHasNoPassword = Expect<Equals<keyof Omit<User & { password: string }, "password">, keyof User>>;

// @ts-expect-error — id is required
const invalid: User = { name: "x" };
export { invalid };
```

- Prefer `// @ts-expect-error <reason>` to `// @ts-ignore`: it fails when the error disappears, so stale suppressions are noticed. Both suppress *all* errors on the next line — keep that line minimal.
- Vitest's `expectTypeOf`/`assertType` (run with `vitest --typecheck`) and `tsd` provide readable type assertions.
- Under the 6.0 `types: []` default, test files need their test-framework globals listed (`"types": ["node", "vitest/globals"]`) or explicit imports (`import { describe, it, expect } from "vitest"`). Prefer explicit imports.

##### Runtime tests

- Test runners that strip types (Vitest, Jest with SWC/esbuild, `node --test` with type stripping, tsx) **do not type-check**. Run `tsc --noEmit -p tsconfig.json` (or `tsc -b`) as a separate CI step; a green test run says nothing about type errors.
- `node --test` runs `.ts` test files directly on Node.js 22.18+/23.6+ (type stripping on by default); see the tooling section for the syntax restrictions.

***

#### Tooling: Linters, Runners, Type Stripping, Editors

##### typescript-eslint

typescript-eslint supports TypeScript `>=4.8.4 <6.1.0` (ESLint 8.57+, 9 and 10), so 6.0 is the newest compiler it can use; it cannot use 7.0's native compiler, which has no API.[^9] Projects that build with 7.0 keep 6.0 for linting through the compatibility package:

```json
{
  "devDependencies": {
    "typescript": "npm:@typescript/typescript6@^6.0.2",
    "@typescript/native": "npm:typescript@^7.0.2"
  }
}
```

`@typescript/typescript6` re-exports the 6.0 API and installs a `tsc6` binary, so it can sit next to 7.0's `tsc`.[^3][^4]

Recommended type-aware rules for review-relevant bugs: `no-floating-promises`, `no-misused-promises`, `await-thenable`, `return-await`, `no-unnecessary-condition`, `switch-exhaustiveness-check`, `no-unsafe-*` (assignment, member-access, call, return, argument), `restrict-template-expressions`, `no-explicit-any`, `consistent-type-imports`, `only-throw-error`, `prefer-promise-reject-errors`, `no-deprecated`.

##### Node.js type stripping

Node.js runs `.ts`/`.mts`/`.cts` files by erasing types: unflagged since 23.6.0 and 22.18.0, warning-free since 24.3.0, stable since 25.2.0/24.12.0; `--experimental-transform-types` was removed in 26.0.0.[^6] Verified on the installed runtimes: Node.js 22.23.4, 24.22.1 and 26.12.1 run a `.ts` file importing `./b.ts`; Node.js 20.20.2 rejects the `.ts` extension; `enum` fails with `ERR_UNSUPPORTED_TYPESCRIPT_SYNTAX … not supported in strip-only mode`; an extensionless `./b` import fails with `ERR_MODULE_NOT_FOUND`; Node.js 26 rejects `--experimental-transform-types` as a bad option.

Rules for code that Node.js strips:

- Set `erasableSyntaxOnly`, `verbatimModuleSyntax`, `allowImportingTsExtensions` + `rewriteRelativeImportExtensions` (or `noEmit`), and `module: nodenext`.
- Write imports with `.ts` extensions; type-only imports must use `import type` (otherwise Node.js tries to import a non-existent runtime binding).
- No `paths` aliases (Node.js ignores `tsconfig.json`); use `package.json` `imports`.
- No decorators, enums, value namespaces, parameter properties, `import =`; `.tsx` is not supported. TypeScript files inside `node_modules` are refused.
- Node.js never type-checks — keep `tsc --noEmit` in CI.

##### Other tools

- **tsx, esbuild, SWC, Babel, Bun, Deno, Vite** transpile per file without type checking. They need `isolatedModules`-compatible code (`verbatimModuleSyntax` makes it explicit) and ignore most tsconfig semantics (e.g. `paths` support varies).
- **Editors**: VS Code bundles its own TypeScript version; select the workspace version ("TypeScript: Select TypeScript Version") so the editor matches `node_modules/typescript` 6.0.3. Vue, Svelte, Astro, Angular and MDX language tooling continues to need 6.0 because 7.0 offers no API yet.[^3]
- **ts5to6** (experimental, by a TypeScript team member) rewrites `baseUrl`-based `paths` and adds `rootDir` across a repository.[^1]

***

#### Observability

- **Source maps**: emit `sourceMap` (or `inlineSourceMap`) and run Node.js with `--enable-source-maps` (or `NODE_OPTIONS=--enable-source-maps`) so stack traces point at `.ts` lines. Do not ship source maps with embedded sources (`inlineSources`) to public CDNs unless intended.
- **Error serialisation**: `JSON.stringify(err)` yields `{}` for `Error` objects (`message`, `stack` and `cause` are not enumerable). Log errors through a logger that serialises them (pino, winston) or map them explicitly: `{ name: err.name, message: err.message, stack: err.stack, cause: … }`.
- **Structured logs**: pass context objects (`logger.info({ userId, orderId }, "order created")`) instead of interpolated strings; type the context with an interface so field names stay consistent.
- **Unknown errors**: in `catch (err: unknown)` handlers, log the value even when it is not an `Error` — `String(err)` for primitives — instead of discarding it.
- **Process-level handlers**: register `process.on("unhandledRejection")`/`("uncaughtException")` to log and exit; never use them to keep a corrupted process running.
- **Tracing**: OpenTelemetry instrumentation for Node.js must be loaded before the instrumented modules (`node --import ./instrumentation.js`); with ESM output this ordering is not guaranteed by a plain `import` in the entry file.

```ts
function toLog(err: unknown): Record<string, unknown> {
  if (err instanceof Error) {
    return { name: err.name, message: err.message, stack: err.stack, cause: err.cause === undefined ? undefined : toLog(err.cause) };
  }
  return { value: String(err) };
}
console.log(JSON.stringify(new Error("x")));                  // {}
console.log(JSON.stringify(toLog(new Error("x", { cause: 1 }))));
export { toLog };
```

***

#### Preparing for TypeScript 7.0

6.0 exists so that a project which compiles cleanly on 6.0 **without** `ignoreDeprecations` compiles the same way on 7.0. A 6.0 codebase is "7.0-ready" when:

1. `tsc -p .` reports no TS5101/TS5107/TS2880/TS1540/TS5112 diagnostics with `ignoreDeprecations` removed.
2. `rootDir`, `types`, `module`, `target` and `lib` are explicit (7.0 has the same new defaults, and its default `target` also floats).
3. Nothing imports `typescript` as a library at build time (custom transformers, `ts-patch`, `ttypescript`, `ts-morph`-based codegen, language-service plugins, `typescript.transpileModule`). 7.0's npm package exposes only `typescript/unstable/*` entry points and no stable API; those tools stay on 6.0 via `@typescript/typescript6`.[^3]
4. Checked JavaScript does not depend on Closure-style JSDoc or constructor functions.[^5]
5. Declaration-output comparisons between 6.0 and 7.0 are run with `--stableTypeOrdering` on the 6.0 side.

##### `--stableTypeOrdering` **[New in 6.0]**

6.0 orders union members by internal type IDs, which depend on the order in which the checker first saw each type; 7.0 (parallel checker) sorts deterministically. Verified example:

```ts
const pre = "zeta";
const obj = { kind: "beta" as const };
export function pick(n: number) {
  if (n === 0) return "alpha";
  if (n === 1) return obj.kind;
  return pre;
}
export const mixed = (n: number) => (n ? { b: 1 } : n > 2 ? "s" : 3);
```

| Compiler | Emitted `pick` return type | `mixed` return type |
|---|---|---|
| 5.9.3 and 6.0.3 (default) | `"beta" \| "zeta" \| "alpha"` | `3 \| "s" \| { b: number; }` |
| 6.0.3 `--stableTypeOrdering`, 7.0.2 | `"alpha" \| "beta" \| "zeta"` | `"s" \| 3 \| { b: number; }` |

(The release notes' own `100 | 500` example shows the instability only on 5.9.3, which emits `500 | 100`; 6.0.3 prints `100 | 500` with and without the flag.) The flag is a **diagnostic aid**, not a production setting: it can slow checking by up to 25 %, and new errors that appear with it point to inference that only worked because of declaration order — fix those with explicit type arguments or annotations.[^1]

##### Differences that appear only on 7.0 (not applicable to 6.0 builds)

- Deprecated options are hard errors (TS5102/TS5108 "has been removed"); `module: none` and `target: es3` are TS6046 (invalid value).
- Stable type ordering is always on; template-literal inference consumes whole Unicode code points.
- JSDoc/JS checking: no Closure types, no constructor functions, `typeof` required for values in type positions, `@typedef` in class bodies hoisted, mixing `module.exports =` with `module.exports.x =` rejected.
- With `strict: false`, arguments can no longer be omitted for parameters of type `undefined`/`unknown`/`any`; with `skipLibCheck`, conflicts between `.d.ts` and source declarations are reported at every non-`.d.ts` site.[^5]
- The native `tsc` binary (Go) replaces `tsc.js`; a new, different compiler API is planned for 7.1.

***

#### Removed / Changed in 6.0 — Migration Notes

Upgrade order that keeps builds green:

1. **Pin** `typescript` to `~6.0.3` and run `tsc -p . --noEmit`. Expect three groups of errors: deprecation errors on the tsconfig, missing globals (`types`), and new strict-mode errors (if the project never set `strict`).
2. **Make defaults explicit** in every tsconfig (including `tsconfig.*.json` used by tests, ESLint and tools): `strict`, `module`, `moduleResolution` (if not implied), `target`, `lib`, `types`, `rootDir`.
3. **Fix deprecations** one by one; use `"ignoreDeprecations": "6.0"` only as a temporary, ticketed bridge.

| 5.9 config | 6.0 replacement |
|---|---|
| no `strict` | `"strict": true` and fix errors (or `"strict": false` explicitly, as debt) |
| no `module` (CommonJS output) | `"module": "commonjs"` or `"nodenext"`; otherwise output silently becomes ESM |
| `"moduleResolution": "node"` | `"nodenext"` (Node.js) or `"bundler"` (+ `"module": "commonjs"` allowed now) |
| `"baseUrl": "./src"` + `paths` | `paths` with `./src/…` prefixes; runtime: `imports` field |
| `"target": "es5"` | `es2015`+; run Babel/SWC afterwards if ES5 output is truly required |
| `"downlevelIteration": true/false` | delete |
| `"module": "amd"/"umd"/"system"`, `"outFile"` | a bundler (esbuild, Rollup, Vite) |
| `"esModuleInterop": false` | delete; change `import * as x from "cjs"` + `x()` to `import x from "cjs"` |
| `"alwaysStrict": false` | delete; remove sloppy-mode code |
| implicit `@types` loading | `"types": ["node", …]` |
| inferred `rootDir` | `"rootDir": "./src"`; verify `dist/` layout and `package.json` paths |
| `"dom", "dom.iterable"` | `"dom"` (both still accepted) |
| `"lib": ["esnext"]` for `Promise.try`, Iterator helpers, Set methods | `"es2025"` |
| `import … assert { type: "json" }` | `with { type: "json" }` |
| `module Foo {}` | `namespace Foo {}` |
| `/// <reference no-default-lib="true"/>` | `"noLib": true` or `libReplacement` |
| `tsc --noEmit $FILES` in hooks | `tsc -p . --noEmit` (or `--ignoreConfig` deliberately) |
| `@typescript/lib-*` replacement packages | `"libReplacement": true` |

4. **Run the emitted code**, not just the checker: the `module` and `target` defaults change the output format without any type error (the `rootDir` change at least stops with TS5011 when `outDir` is set).
5. **Check tool compatibility**: typescript-eslint must be a version supporting `<6.1.0`; ts-node/ts-jest/ts-loader/fork-ts-checker and framework CLIs must declare 6.0 support.
6. Optionally run a 7.0 build in CI (`npm:typescript@^7.0.2` aliased as `@typescript/native`) to catch 7.0-only differences early.

***

#### Production / Review Checklist

##### Configuration

- [ ] `typescript` pinned to `~6.0.3` (or aliased via `@typescript/typescript6`); lockfile committed
- [ ] `strict`, `module`, `target`, `lib`, `types`, `rootDir` set explicitly in every tsconfig
- [ ] No 6.0-deprecated options; any `"ignoreDeprecations": "6.0"` has a removal ticket
- [ ] No `esModuleInterop: false` (type-checks as on, emits as off → runtime `default is not a function`)
- [ ] `module` matches the runtime: `nodenext`/`node20` for Node.js, `preserve`/`esnext` + `bundler` for bundlers
- [ ] `target`/`lib` match the oldest supported runtime (Node.js 20 → `es2023`, 22 → `es2024`, 24 → `es2025`); `esnext` APIs guarded or polyfilled
- [ ] `noUncheckedIndexedAccess`, `exactOptionalPropertyTypes`, `noImplicitOverride`, `verbatimModuleSyntax` enabled for new code
- [ ] `erasableSyntaxOnly` when Node.js type stripping or strip-only tools run the code
- [ ] Side-effect/asset imports covered by declarations, not by disabling `noUncheckedSideEffectImports`
- [ ] `#/` subpath imports only when every supported Node.js version is ≥ 24.14 / 25.4

##### Code correctness

- [ ] No `any` at boundaries; `unknown` + validation for parsed/external data
- [ ] Every `as`, `!`, `@ts-expect-error` justified; no `@ts-ignore`, no double assertions
- [ ] Discriminated unions handled exhaustively (`never` check), no silent `default`
- [ ] Truthiness checks do not drop `0`/`""`; `??` used only where the left side can be nullish
- [ ] Type guards and assertion functions actually prove what they claim
- [ ] No reliance on narrowing surviving calls that may mutate the narrowed property
- [ ] `Omit`/`Pick` on unions use a distributive helper when variant fields matter
- [ ] Class field redeclarations use `declare`; no overridable calls in base constructors
- [ ] Numeric enums validated at boundaries; no `const enum` in published APIs

##### Async and resources

- [ ] No floating promises; no `async` callbacks passed to `forEach`/event APIs without error handling
- [ ] `return await` inside `try`; errors wrapped with `{ cause }`
- [ ] Bounded concurrency for large fan-outs; `AbortSignal` passed through I/O layers
- [ ] Resources scoped with `using`/`await using` or `try … finally`; `DisposableStack` only on Node.js 24+

##### Modules and packaging

- [ ] Relative ESM imports carry `.js` (or `.ts` with `rewriteRelativeImportExtensions`)
- [ ] `import type` for type-only imports; JSON imports use `with { type: "json" }`
- [ ] `paths` aliases mirrored in the runtime/bundler/test-runner config (or replaced by `imports`)
- [ ] Libraries: `exports` with `types` first, `isolatedDeclarations`, checked with attw/publint
- [ ] No `import defer` in code Node.js executes directly; no decorators with `target: esnext` without a transform

##### Security

- [ ] Request bodies, env vars, JSON and message payloads validated at run time
- [ ] No prototype-pollution-prone merges of untrusted objects
- [ ] Parameterised SQL, argument-array process spawning, no `innerHTML`/`eval` with untrusted data
- [ ] `RegExp.escape` (Node.js 24+) or a vetted helper for dynamic regular expressions

##### Tooling and CI

- [ ] `tsc --noEmit` (or `tsc -b`) runs in CI separately from tests and bundling
- [ ] typescript-eslint type-aware rules enabled (`no-floating-promises`, `no-misused-promises`, `switch-exhaustiveness-check`, `no-unsafe-*`)
- [ ] Editor uses the workspace TypeScript version
- [ ] Optional: CI job with `--stableTypeOrdering` and/or a 7.0 build to track 7.0 readiness

***

[^1]: TypeScript 6.0 release notes and "Announcing TypeScript 6.0" (2026-03-23). https://www.typescriptlang.org/docs/handbook/release-notes/typescript-6-0.html , https://devblogs.microsoft.com/typescript/announcing-typescript-6-0/
[^2]: npm registry — typescript versions and publish dates (6.0.0-beta 2026-02-11, 6.0.1-rc 2026-03-06, 6.0.2 2026-03-23, 6.0.3 2026-04-16, 7.0.2 2026-07-08, 7.1.0-beta 2026-10-20); GitHub release v6.0.3. https://www.npmjs.com/package/typescript?activeTab=versions , https://github.com/microsoft/TypeScript/releases/tag/v6.0.3
[^3]: Announcing TypeScript 7.0 (2026-07-08) — new defaults, removed options, no API, `@typescript/typescript6`. https://devblogs.microsoft.com/typescript/announcing-typescript-7-0/
[^4]: @typescript/typescript6 — TypeScript 6.0 compatibility package (`tsc6`). https://www.npmjs.com/package/@typescript/typescript6
[^5]: typescript-go CHANGES.md — behavioural differences of the native compiler. https://github.com/microsoft/typescript-go/blob/main/CHANGES.md
[^6]: Node.js documentation — Modules: TypeScript (type stripping history and limitations). https://nodejs.org/api/typescript.html
[^7]: TypeScript 5.5 release notes — Disabling Features Deprecated in TypeScript 5.0. https://www.typescriptlang.org/docs/handbook/release-notes/typescript-5-5.html
[^8]: Node.js v25.4.0 and v24.14.0 release notes — "module: allow subpath imports that start with #/" (nodejs/node#60864). https://nodejs.org/blog/release/v25.4.0 , https://nodejs.org/blog/release/v24.14.0
[^9]: typescript-eslint — Dependency Versions (TypeScript `>=4.8.4 <6.1.0`). https://typescript-eslint.io/users/dependency-versions/
[^10]: Announcing TypeScript 6.0 Beta (2026-02-11) — "the last release based on the current JavaScript codebase". https://devblogs.microsoft.com/typescript/announcing-typescript-6-0-beta/
[^11]: TypeScript 5.9 release notes — `import defer`, `--module node20`, `lib.d.ts` ArrayBuffer changes, inference changes. https://www.typescriptlang.org/docs/handbook/release-notes/typescript-5-9.html
[^12]: TypeScript 5.8 release notes — `require()` of ESM, `--module node18`, `--erasableSyntaxOnly`, `--libReplacement`, conditional return checks. https://www.typescriptlang.org/docs/handbook/release-notes/typescript-5-8.html
[^13]: TypeScript 5.7 release notes — never-initialized variables, `--rewriteRelativeImportExtensions`, generic TypedArrays, JSON import validation. https://www.typescriptlang.org/docs/handbook/release-notes/typescript-5-7.html
[^14]: TypeScript 5.6 release notes — truthy/nullish checks, iterator helpers, `--strictBuiltinIteratorReturn`, `--noUncheckedSideEffectImports`, `--noCheck`. https://www.typescriptlang.org/docs/handbook/release-notes/typescript-5-6.html
[^15]: TypeScript 5.4 release notes — preserved narrowing in closures, `NoInfer`, `--module preserve`. https://www.typescriptlang.org/docs/handbook/release-notes/typescript-5-4.html
[^16]: TypeScript 5.0 release notes — standard decorators, `const` type parameters, `--moduleResolution bundler`, `--verbatimModuleSyntax`. https://www.typescriptlang.org/docs/handbook/release-notes/typescript-5-0.html
[^17]: TypeScript 5.2 release notes — `using` declarations and explicit resource management. https://www.typescriptlang.org/docs/handbook/release-notes/typescript-5-2.html
[^18]: TypeScript 5.5 release notes — inferred type predicates, constant indexed access narrowing, `@import`, regex checking, `isolatedDeclarations`. https://www.typescriptlang.org/docs/handbook/release-notes/typescript-5-5.html
