### TypeScript 7.0: The Complete Production Developer Guide

> **Version**: TypeScript 7.0 — the native compiler written in Go. The release candidate `7.0.1-rc` shipped on June 18, 2026; the stable release `7.0.2` shipped on **July 8, 2026** (there is no `7.0.0` or `7.0.1` stable package — `7.0.2` *is* the first and, as of October 22, 2026, the latest 7.0 release).[^1][^2]
> **Status (October 2026)**: 7.0 is the current stable line (`npm install -D typescript` installs 7.0.2). TypeScript 6.0 (6.0.2, March 23, 2026; latest patch 6.0.3, April 16, 2026) is the last compiler built on the JavaScript codebase and gets no new features; its API lives on in the `@typescript/typescript6` compatibility package. TypeScript **7.1 is only at beta** (`7.1.0-beta`, October 20, 2026) — nothing from 7.1 is available in a 7.0 project.[^2][^3][^4]
> **Scope**: The TypeScript language as usable with the 7.0 compiler, everything that changed versus 6.0 and 5.x, `tsconfig.json` choices for Node.js and bundlers, the tooling ecosystem as of October 2026, and the bugs a reviewer must catch.
> **Note**: This document is self-contained. Sections that cover 7.0-specific behaviour are marked **New in 7.0**; behaviour inherited from 6.0 is marked **(6.0+)** because most projects reach 7.0 directly from 5.x.
> **Code conventions**: every TypeScript/JavaScript example was compiled with `tsc` 7.0.2. Lines ending in `// error TSxxxx` fail with exactly that diagnostic and nothing else in the block fails. `// file: name.ts` starts a new file of a multi-file example; `// tsconfig: {…}` lists options added to the base configuration (strict, `module: nodenext`, `target`/`lib` `es2025`, `noUncheckedIndexedAccess`, `exactOptionalPropertyTypes`, `noImplicitOverride`, `noImplicitReturns`, `verbatimModuleSyntax`, `types: ["node"]`).

TypeScript 7.0 is a *port*, not a redesign: the Go compiler (code name "Corsa") reproduces the type system of TypeScript 6.0 (code name "Strada") almost exactly. Its headline is speed — Microsoft reports full builds 7.7x–11.9x faster than 6.0 with the default four type-checker workers, and up to 16.7x with `--checkers 8` — plus multi-threaded parsing, checking and emitting, a rebuilt `--watch` mode and a new LSP-based language server.[^1] What breaks code is everything around the type system:

- **6.0's new defaults are in force and its deprecations are hard errors.** `strict` defaults to `true`, `types` defaults to `[]`, `rootDir` defaults to the `tsconfig.json` directory, `noUncheckedSideEffectImports` defaults to `true`; `target: es5`, `downlevelIteration`, `moduleResolution: node`/`node10`/`classic`, `module: amd`/`umd`/`system`/`none`, `baseUrl`, `outFile`, `esModuleInterop: false` and `alwaysStrict: false` are rejected — and `"ignoreDeprecations": "6.0"` no longer silences anything.[^1][^3]
- **There is no compiler API.** `require("typescript")` exports only `version` and `versionMajorMinor`. Tools that load the compiler as a library (typescript-eslint, ts-jest, ts-node, ts-morph, Angular's compiler, Vue/Svelte/Astro/MDX language tools, webpack's ts-loader) keep running on 6.0 side by side; the new API is planned for 7.1.[^1][^5]
- **JavaScript checking (`allowJs`/`checkJs`) follows TypeScript rules.** Constructor functions, `@enum`, `@class`, Closure-style JSDoc types and values-used-as-types are gone.[^6]
- **A few type-system results differ**: template-literal inference consumes whole Unicode code points, type ordering is always the "stable" one, and declaration conflicts are reported at every site even under `skipLibCheck`.[^1][^6]

Global truths about TypeScript 7.0:

- **Types are erased.** Nothing in a type annotation exists at runtime. A value typed `User` that came from `JSON.parse` is exactly as unvalidated as before.
- **`tsc` is a native binary.** The `typescript` npm package is a thin launcher that picks a platform package such as `@typescript/typescript-linux-x64` (optional dependencies for 20 platforms). Installs with `--no-optional`/`--omit=optional` break `tsc`.
- **Checking is structural.** Two types with the same shape are interchangeable; nominal behaviour requires brands or `#private` fields.
- **Strict by default.** A `tsconfig.json` with no `strict` key is strict in 7.0 — and loose in 5.x. Read configs with that in mind.
- **Emit is a syntax transform, not a polyfill.** `target` controls which syntax is down-levelled; `lib` controls which APIs type-check. Neither adds runtime APIs. A program can type-check against `RegExp.escape` and crash on a runtime that lacks it.

***

#### What's New in TypeScript 7.0

##### The native compiler and its command line (New in 7.0)

- **Parallelism.** Parsing and emitting are parallel per file. Type checking runs in a fixed number of checker workers (default **4**), each with its own view of the program; files are always partitioned identically, so results are deterministic for a given worker count.[^1]
  - `--checkers <n>` — number of type-checker workers per project. More workers: faster on big codebases, more memory. `--checkers 1` checks single-threaded. Microsoft warns that in rare cases *different* checker counts can surface order-dependent results, so pin the value if local and CI results must match.[^1]
  - `--builders <n>` — number of projects built concurrently under `tsc --build`. It multiplies with `--checkers` (`--checkers 4 --builders 4` allows 16 checkers). Changing it must not change results.[^1]
  - `--singleThreaded` — parse, check and emit on one thread (debugging, constrained CI, or when an outer tool already parallelises).[^1]
  - `--pprofDir <dir>` — writes Go pprof CPU/memory profiles. `--quiet`/`-q` and `--watchInterval` are also new on the command line. (`--generateTrace`, `--extendedDiagnostics`, `--generateCpuProfile` still exist.)
- **`--watch` rebuilt** on a Go port of the Parcel file watcher, replacing polling-heavy watching.[^1]
- **No `tsserver`.** The package ships only `bin/tsc`. Editors talk to a new multi-threaded LSP server; VS Code uses a dedicated TypeScript 7 extension (the command "Disable TypeScript 7 Language Server" reverts to 6.0), and Visual Studio enables TypeScript 7 per workspace. `compilerOptions.plugins` (language-service plugins) is still accepted by `tsc` but nothing loads the plugins.[^1]
- **Positions are UTF-8 byte offsets** internally (relevant only to tools consuming the unstable API).[^6]
- **Unstable IPC API.** 7.0.2 already exports `typescript/unstable/sync`, `typescript/unstable/async`, `typescript/unstable/ast` and related subpaths: a client that spawns the Go binary and talks to it over a pipe. It is explicitly unstable and not a replacement for the 6.0 API; 7.1 renames the entry points to `typescript/sync`, `typescript/async`, `typescript/ast`, `typescript/fs`.[^4] Do not build production tooling on `typescript/unstable/*`.

##### Default changes (6.0+, enforced in 7.0)

Verified by compiling the same `tsconfig.json` with 5.9.3, 6.0.3 and 7.0.2:

| Option | 5.9 default | 6.0 / 7.0 default | Consequence in review |
| --- | --- | --- | --- |
| `strict` | `false` | `true` | A config without `strict` is strict. `"strict": false` is still allowed. |
| `target` | `es5` | `es2025` (7.0 help text: "default: es2025"; "the stable ES version immediately preceding esnext") | Output is untransformed modern JS: classes, `?.`, `??`, `**` and BigInt stay as-is; class fields use `[[Define]]` semantics. |
| `module` | `commonjs` (for `target: es5`) | `esnext` | A config with no `module` now emits ESM. |
| `moduleResolution` | `node10` / `classic` | `bundler` (unless `module` is `node16`/`node18` → `node16`, `nodenext` → `nodenext`) | Bare `tsc` resolves like a bundler — wrong for Node.js-run code. |
| `types` | all `@types/*` packages | `[]` | `process`, `describe`, `Buffer` are "Cannot find name" (TS2591/TS2593) until listed: `"types": ["node"]`. `["*"]` restores the old behaviour. |
| `rootDir` | common directory of the inputs | directory containing `tsconfig.json` | With `include: ["src"]` and `outDir: "dist"`, output moves from `dist/x.js` to `dist/src/x.js`; 6.0/7.0 report **TS5011** and ask for an explicit `rootDir`. |
| `noUncheckedSideEffectImports` | `false` | `true` | `import "./polyfill"` that does not resolve is **TS2882**; `import "./x.css"` needs a `declare module "*.css";`. |
| `esModuleInterop`, `allowSyntheticDefaultImports` | `false` | `true`, cannot be `false` | Default imports of CommonJS modules always go through the interop helpers. |
| `alwaysStrict` | `false` | `true`, cannot be `false` | Every file is strict-mode code. |
| `libReplacement` | `true` | `false` | `@typescript/lib-*` packages are no longer picked up unless enabled. |
| `stableTypeOrdering` (6.0 flag) | — | `true` in 7.0, cannot be disabled | Union member order in hovers/errors/`.d.ts` is deterministic. 7.0 accepts `"stableTypeOrdering": false` silently but ignores it. |

`tsc --init` in 7.0 writes `module: nodenext`, `target: esnext`, `types: []`, `strict`, `verbatimModuleSyntax`, `isolatedModules`, `noUncheckedIndexedAccess`, `exactOptionalPropertyTypes`, `noUncheckedSideEffectImports`, `moduleDetection: force`, `skipLibCheck`, `jsx: react-jsx`, `sourceMap`, `declaration` and `declarationMap`. **`target: esnext` from `tsc --init` is a trap for Node.js**: it emits `using` declarations and decorators untransformed, and Node.js 24.22 and 26.12 reject `@decorator` syntax while Node.js 22 rejects `using` (see Classes and Async).

##### Removed options and syntax (hard errors in 7.0)

Each row was compiled with 5.9.3 (accepted), 6.0.3 (deprecation error, silenced by `"ignoreDeprecations": "6.0"`) and 7.0.2 (error that `ignoreDeprecations` cannot silence):[^1][^3]

| Removed in 7.0 | 6.0 diagnostic | 7.0 diagnostic | Replacement |
| --- | --- | --- | --- |
| `"target": "es5"` | TS5107 deprecated | TS5108 "has been removed" | `es2015` is the minimum; use a transpiler (Babel/SWC) for ES5 output. |
| `"downlevelIteration": true` | TS5101 | TS5102 | Drop it — with an ES2015+ target iteration is native. |
| `"moduleResolution": "node"` / `"node10"` | TS5107 | TS5108 | `nodenext` (Node.js) or `bundler`. |
| `"moduleResolution": "classic"` | TS5107 | TS5108 | `bundler` or `nodenext`. |
| `"module": "amd"` / `"umd"` / `"system"` | TS5107 | TS5108 (+ TS5095 because the default `bundler` resolution needs ES modules) | `esnext` or `preserve` with a bundler. |
| `"module": "none"` | TS5107 | TS6046 (not a valid value) | `esnext`/`preserve`. |
| `"baseUrl"` | TS5101 | TS5102, with the hint `Use '"paths": {"*": ["./*"]}' instead.` | `paths` entries relative to the `tsconfig.json`. |
| `"outFile"` | TS5101 | TS5102 | A bundler. |
| `"esModuleInterop": false`, `"allowSyntheticDefaultImports": false` | TS5107 | TS5108 | Remove the setting. |
| `"alwaysStrict": false` | TS5107 | TS5108 | Remove the setting. |
| `module Foo { }` (namespace via `module` keyword) | TS1540 | TS1540 | `namespace Foo { }`; `declare module "pkg" { }` is still fine. |
| `import x from "./a.json" assert { type: "json" }` | TS2880 | TS2880 | `with { type: "json" }`. |
| `tsc file.ts` while a `tsconfig.json` exists in the directory | TS5112 | TS5112 | `tsc -p .`, or `tsc --ignoreConfig file.ts`. |
| `/// <reference no-default-lib="true"/>` under `skipDefaultLibCheck` | — | no longer respected | Use `noLib`/`lib`. |

Options removed back in 5.5 (`importsNotUsedAsValues`, `preserveValueImports`, `keyofStringsOnly`, `suppressImplicitAnyIndexErrors`, `noImplicitUseStrict`, `noStrictGenericChecks`, `charset`, `out`) changed from "TS5102 has been removed" in 5.9/6.0 to **TS5023 "Unknown compiler option"** in 7.0.

##### Type-checking differences versus 6.0 (New in 7.0)

7.0 is designed so that code compiling cleanly under 6.0 *with `stableTypeOrdering` on and no `ignoreDeprecations`* compiles identically.[^1] Known intentional differences:

- **Template-literal inference takes whole code points.** `` S extends `${infer H}${string}` `` applied to `"😀abc"` infers `"😀"` in 7.0 and a lone surrogate `"\uD83D"` in 6.0. Type-level `Length`/`Split` utilities that counted UTF-16 units change results (a `Len<"😀">` utility gives `1` in 7.0, `2` in 6.0).[^1][^6]
- **Declaration conflicts are reported at every contributing site**, so some errors that `skipLibCheck` hid (because the only reported site was in a `.d.ts`) now appear in `.ts` files.[^6]
- **Type ordering is always stable**, which can change which overload or union member an error message mentions and the order of union members in emitted `.d.ts` files. In rare cases an inference that depended on declaration order changes. Measured example — for `declare const flag: boolean; const x = flag ? "zebra" : flag ? 42 : flag ? "apple" : null; export function f() { return flag ? x : [x]; }` the declaration emit is:
  - 6.0.3: `export declare function f(): "zebra" | 42 | "apple" | (string | number | null)[] | null;`
  - 7.0.2 (and 6.0.3 with `--stableTypeOrdering`): `export declare function f(): "apple" | "zebra" | 42 | (string | number | null)[] | null;`

  Generated `.d.ts` files and snapshot tests of type output therefore change on upgrade even when nothing else does; the types are equivalent.
- **Diagnostics for `isolatedDeclarations` differ.** For `export function id(a: number) { return a; }` 6.0 reports TS9007 "Function must have an explicit return type annotation" at the name; 7.0.2 reports TS9039 "Type containing private name 'a' can't be used with --isolatedDeclarations" at the `return` expression. For an untyped arrow (`export const sum = (a: number, b: number) => a + b`) 6.0 reports TS9007, 7.0.2 TS9013. The fix (annotate the return type) is the same; CI greps for TS9007 need updating.
- **JavaScript emit for `.ts` input is unchanged.** A file mixing an `import` default from a CommonJS module, an `enum`, parameter properties, `#private` fields, static fields, an async generator, object spread and a `using` declaration produced byte-identical JavaScript with 6.0.3 and 7.0.2 at both `target: es2025` and `target: es2017`. Differences in emit are limited to `.d.ts` union ordering (above) and declaration emit from `.js` input.
- **After a grammar error in a file, 7.0 may report fewer follow-up semantic errors** in that file than 6.0. Fix the first error and recompile before concluding that code is clean.

##### JavaScript and JSDoc checking (New in 7.0)

`checkJs` now applies TypeScript's rules to `.js` files instead of a separate set of JS heuristics. Verified on 7.0.2 vs 6.0.3 (details and fixes in the JavaScript section):[^6]

- Constructor functions (`function C() { this.x = 1 }` + `C.prototype.m = …`) are no longer understood: `new C()` is TS7009 under `strict`. Use `class`.
- `@enum` is a plain tag: using the object as a type is TS2749. Values in type positions (`@typedef {FORWARD | BACKWARD}`) are TS2749 — write `typeof FORWARD`.
- `arguments` no longer implies a rest parameter: `f("x")` on a function with no parameters is TS2554.
- Closure syntax `{function(string): void}` and the standalone `{?}` type are parse errors (TS1005/TS1110).
- `@class`/`@constructor`, postfix `!`, identifier-named typedefs, `this` aliases, `module.exports = {}` placeholders, whole-prototype assignment and mixing `module.exports = …` with `module.exports.x = …` are gone or changed.
- Declaration emit from `.js` input was rewritten and does not aim to match 6.0 byte for byte.

##### The compiler API is gone (New in 7.0)

`import ts from "typescript"` (or `require`) gives `{ version: "7.0.2", versionMajorMinor: "7.0" }` — no `createProgram`, `transpileModule`, `LanguageService`, transformers or `tsserver` plugins. The official workaround is to install 6.0 under the `typescript` name and 7.0 under another name:[^1]

```json
{
  "devDependencies": {
    "@typescript/native": "npm:typescript@^7.0.2",
    "typescript": "npm:@typescript/typescript6@^6.0.2"
  }
}
```

`@typescript/typescript6` (latest version 6.0.2) re-exports the 6.0 API and installs a `tsc6` binary; the aliased 7.0 package provides `tsc`. (`@typescript/native` is just an alias name chosen in the blog post — there is no such package on npm.) Ecosystem status as of October 2026 (npm metadata checked on 2026-10-22):

| Tool | Status with 7.0 |
| --- | --- |
| typescript-eslint 8.72.0 | Peer range `typescript >=4.8.4 <6.1.0`; type-aware rules need the 6.0 API. Run it on 6.0 via the alias above. Issue #12518 ("TypeScript 7.0.2 Support") was closed as a duplicate of #10940; 8.65.0 added a warning when TS 7 is detected.[^5] |
| ts-jest 29.4.15 | Peer range `typescript >=4.3 <7`. Uses `LanguageService`; keep it on 6.0 or switch to a transpile-only transformer (SWC/esbuild) plus a separate `tsc --noEmit`. |
| ts-node 10.9.2 | Last release 2023; needs the compiler API. Use Node.js type stripping, `tsx`, or compile first. |
| ts-morph, ts-loader, fork-ts-checker, `@angular/compiler-cli`, Vue (vue-tsc/Volar), Svelte, Astro, MDX | Depend on the 6.0 API or tsserver. Angular: type-check with 7.0 `tsc`, keep 6.0 for the Angular compiler/editor; Vue/Svelte/Astro/MDX: stay on 6.0 for now.[^1] |
| oxlint with `oxlint-tsgolint` 7.x | Type-aware linting built on typescript-go (7.0.2) — works with 7.0 without the 6.0 alias.[^9] |
| esbuild, SWC, Vite, Vitest, tsx, Bun, Node.js type stripping | Do not use the TypeScript compiler (transpile only), so they are unaffected — and they never type-check. |

##### Inherited from 6.0 — new to anyone coming from 5.x

- **`target`/`lib` `es2025`** (5.9 rejects `--target es2025` with TS6046): `RegExp.escape`, `Promise.try`, iterator helpers (`Iterator.prototype.map/filter/take/drop/flatMap/reduce/toArray/…`), `Set` methods (`union`, `intersection`, `difference`, `symmetricDifference`, `isSubsetOf`, …), `Float16Array`/`Math.f16round`.[^3]
- **`esnext` lib**: 6.0 added Temporal (`Temporal.*`, also via `esnext.temporal`), `Map`/`WeakMap` `getOrInsert`/`getOrInsertComputed` and the `Uint8Array` base64/hex methods (`toBase64`, `fromBase64`, … — 5.9's `esnext` lacks them). `esnext` also holds older proposals such as `Error.isError`, `Array.fromAsync` and `Disposable`/`AsyncDisposable`/`Symbol.dispose` (the disposable declarations are **not** in `es2025`).[^3]
- **`dom` includes `dom.iterable` and `dom.asynciterable`**; those lib names still exist but are empty.[^3]
- **`#/` subpath imports** (`"imports": { "#/*": "./src/*" }`) resolve under `nodenext` and `bundler` (5.9: TS2307). At runtime Node.js 24.22 accepts them; **Node.js 20.20 and 22.23 throw `ERR_INVALID_MODULE_SPECIFIER`**, so do not use `#/` in code that must run on Node.js 22.[^3]
- **`moduleResolution: bundler` with `module: commonjs`** is allowed (5.9: TS5095).[^3]
- **Less context-sensitivity for methods without `this`.** In a generic call, a method-syntax property that never uses `this` no longer blocks inference, so `{ consume(y) { y.toFixed() }, produce(x: number) { return x * 2 } }` infers `T = number` in 6.0/7.0 (5.9: TS2339 on `unknown`).[^3]
- **Command-line files with a present `tsconfig.json`** are an error (TS5112) — use `--ignoreConfig`.

***

#### Do Not Use or Recommend

##### Features from 7.1 and later (not in 7.0)

TypeScript 7.1 is at beta (October 20, 2026). None of the following works with 7.0.2 — never suggest them for a project pinned to 7.0:[^4]

- **`--target es2026` / `--lib es2026`** — 7.0.2's `--target` accepts `es2015`…`es2025` and `esnext` only.
- **Source phase imports**: `import source mod from "./x.wasm"` and `import.source(…)` returning `AbstractModuleSource`/`WebAssembly.Module`.
- **Import attributes on ambient module declarations**: `declare module "*.css" with { type: "css" } { … }`, and attribute-typed `"text"`/`"bytes"` imports.
- **`contentMappers`** in `tsconfig.json` and the **`--runExternalCode`** flag; the VS Code setting `js/ts.contentMappers.enabled`.
- **The stable API entry points** `typescript/sync`, `typescript/async`, `typescript/fs` (`createFileSystem`, `createFileSystemWithLib`, `createFileSystemLayer`), `typescript/ast`, `typescript/ast/is`, `typescript/ast/factory`, `api.createSnapshot`, `program.emitToString()`, `Disposable` API objects. 7.0 only has `typescript/unstable/*`.
- **7.1 `esnext` lib additions**: `Promise.allKeyed`, `Promise.allSettledKeyed`, `Iterator.zip`, `Iterator.zipKeyed`, `Iterator.prototype.chunks`, `Iterator.prototype.join`, `Iterator.prototype.includes`. On 7.0.2 these are type errors.
- 7.1's stricter checking of distributive conditional type parameters — do not "fix" 7.0 code for an error only 7.1 reports.

Also do not recommend **`@typescript/native-preview`** / the `tsgo` command for a 7.0 project: they were the pre-release channel; nightlies are now `typescript@next` (currently 7.1 dev builds).[^1]

##### Deprecated or removed — flag in review

- Anything in the "Removed options" table: `target: es5`, `downlevelIteration`, `moduleResolution: node|node10|classic`, `module: amd|umd|system|none`, `baseUrl`, `outFile`, `esModuleInterop: false`, `allowSyntheticDefaultImports: false`, `alwaysStrict: false`, `ignoreDeprecations: "6.0"` (useless in 7.0), `module` namespaces, `assert {}` import attributes.
- `importsNotUsedAsValues`, `preserveValueImports` → `verbatimModuleSyntax`.
- `experimentalDecorators` / `emitDecoratorMetadata` — still supported in 7.0, but legacy. Use them only for frameworks that require them (NestJS, TypeORM, Angular before standard-decorator support, InversifyJS); new code uses standard decorators.
- `/// <reference path>` between modules, `namespace` for code organisation, `import x = require()` in ESM code, `<T>value` angle-bracket assertions — all incompatible with `erasableSyntaxOnly`/type-stripping runtimes.
- `ts-node`, `ts-jest` with `isolatedModules: false`, `fork-ts-checker-webpack-plugin` — tied to the 6.0 API.

##### Version-aware review

- Determine the compiler from the lockfile: `typescript` resolving to `7.0.x` means the Go compiler. If `typescript` is aliased (`npm:@typescript/typescript6@…`) the editor/lint toolchain is 6.0 while `tsc` may be 7.0 — both must accept the code.
- Determine the runtime separately (`engines.node`, `.nvmrc`, Dockerfile `FROM node:…`, browserslist). `target`/`lib` must not exceed it. TypeScript does not check this.
- Before saying "X is not available in 7.0", check the 7.0.2 `--target`/`--lib` lists and the lib declarations; before saying "X is new in 7.0", check whether it already existed in 6.0 or 5.x (most "new" behaviour arrived in 6.0).
- Say "not available in TypeScript 7.0" only for the items in the 7.1 list above or when a 7.0.2 compile proves it. Runtime availability (Node.js, browsers) is a separate claim — name the runtime version.
- A diff that only adds `"ignoreDeprecations": "6.0"` to make 7.0 build is wrong: 7.0 ignores it and still fails.

***

#### tsconfig.json for 7.0

Write every option that matters explicitly. Defaults changed twice in one year (6.0 and 7.0 share them, 5.x differs), and many tools still read `tsconfig.json` with 5.x/6.0 assumptions.

##### Node.js application or library (ESM, compiled with tsc)

```json
{
  "compilerOptions": {
    "rootDir": "src",
    "outDir": "dist",
    "module": "nodenext",
    "target": "es2024",
    "lib": ["es2024"],
    "types": ["node"],

    "strict": true,
    "noUncheckedIndexedAccess": true,
    "exactOptionalPropertyTypes": true,
    "noImplicitOverride": true,
    "noImplicitReturns": true,
    "noFallthroughCasesInSwitch": true,
    "noPropertyAccessFromIndexSignature": true,

    "verbatimModuleSyntax": true,
    "isolatedModules": true,
    "erasableSyntaxOnly": true,
    "moduleDetection": "force",
    "skipLibCheck": true,

    "declaration": true,
    "declarationMap": true,
    "sourceMap": true
  },
  "include": ["src"]
}
```

- `module: nodenext` implies `moduleResolution: nodenext` and reads `package.json` `"type"` to decide whether each file is ESM or CommonJS. Do not combine Node.js-run output with `module: esnext`/`moduleResolution: bundler`: the compiler then accepts extensionless relative imports and `paths` aliases that Node.js cannot resolve.
- Pick `target`/`lib` from the **oldest runtime** you deploy to. On Node.js 22, `es2024` is the safe choice: Node.js 22.23 lacks `RegExp.escape`, `Promise.try` and `Float16Array`, all of which type-check under the 7.0 default `es2025`. Node.js 24.22 and 26.12 have all `es2025` APIs, so `es2025` is fine there. Only `esnext` exposes Temporal, `Map.prototype.getOrInsert` and `Uint8Array.prototype.toBase64`, which exist at runtime on Node.js 26 but **not** on Node.js 24.
- `types: ["node"]` is required now that `types` defaults to `[]`; add test-runner globals (`"vitest/globals"`, `"jest"`, `"mocha"`) only in the test project's config.
- `erasableSyntaxOnly` keeps sources runnable by Node.js type stripping and other strip-only tools (see Modules).

##### Application bundled by Vite/esbuild/webpack/Rspack

```json
{
  "compilerOptions": {
    "module": "preserve",
    "moduleResolution": "bundler",
    "target": "es2022",
    "lib": ["es2022", "dom"],
    "types": ["vite/client"],
    "jsx": "react-jsx",
    "noEmit": true,
    "allowImportingTsExtensions": true,
    "strict": true,
    "noUncheckedIndexedAccess": true,
    "exactOptionalPropertyTypes": true,
    "verbatimModuleSyntax": true,
    "isolatedModules": true,
    "moduleDetection": "force",
    "skipLibCheck": true
  },
  "include": ["src"]
}
```

- The bundler emits, `tsc` only checks (`noEmit`). `module: preserve` keeps `import`/`require` as written and implies `moduleResolution: bundler`.
- `lib` must include `dom` explicitly when you set `lib` (the default lib for a target already includes it). Since 6.0 `dom` contains the iterable declarations.
- `allowImportingTsExtensions` requires `noEmit`, `emitDeclarationOnly` or `rewriteRelativeImportExtensions`.

##### Library published to npm

- `module: nodenext` (not `bundler`): code that resolves under Node.js rules also resolves in bundlers; the reverse is not true.
- `declaration: true`, `declarationMap: true`, and preferably `isolatedDeclarations: true` (every export has an explicit type, so declaration emit is a per-file syntax transform that other tools can do and `--build` can parallelise).
- `package.json` `"exports"` with a `"types"` condition first; do not ship `paths` aliases in emitted code (see Modules).
- Do **not** set `target: esnext` for published JavaScript: consumers on older runtimes get untransformed syntax.

##### Strictness flags

`strict: true` (default in 7.0) enables `noImplicitAny`, `strictNullChecks`, `strictFunctionTypes`, `strictBindCallApply`, `strictPropertyInitialization`, `strictBuiltinIteratorReturn`, `noImplicitThis`, `useUnknownInCatchVariables` and `alwaysStrict`. Not part of `strict`, but recommended for new code:

| Flag | What it catches |
| --- | --- |
| `noUncheckedIndexedAccess` | `arr[i]`, `record[key]` are `T \| undefined`. Catches off-by-one and missing-key bugs. |
| `exactOptionalPropertyTypes` | `{ a?: string }` does not accept `{ a: undefined }`; distinguishes "absent" from "present but undefined" (matters for `in`, `Object.keys`, spreads, PATCH payloads). |
| `noImplicitOverride` | Overriding members need `override`; a renamed base method no longer silently leaves a dead "override". |
| `noImplicitReturns` | A code path in a value-returning function falls off the end. |
| `noFallthroughCasesInSwitch` | Non-empty `case` without `break`/`return`. |
| `noPropertyAccessFromIndexSignature` | Forces `obj["key"]` for index-signature access, so `obj.typo` on a declared shape stays an error. |
| `noUnusedLocals` / `noUnusedParameters` | Dead code; often left to the linter instead. |

A diff that sets `"strict": false` or turns off a strict sub-flag in a 7.0 project is a regression — in 5.x configs it was merely the default.

***

#### Modules and Module Resolution

##### Choosing `module` / `moduleResolution`

| Situation | `module` | `moduleResolution` (implied) |
| --- | --- | --- |
| Code run by Node.js (compiled by `tsc` or run via type stripping) | `nodenext` (or a fixed `node20`/`node18`) | `nodenext` / `node16` |
| Code consumed only by a bundler (Vite, esbuild, webpack, Rspack, Bun) | `preserve` (or `esnext`) | `bundler` |
| Published library | `nodenext` | `nodenext` |

`node10`/`node`/`classic` resolution, `amd`/`umd`/`system`/`none` output and `outFile` do not exist in 7.0.

##### ESM under `nodenext`: write the runtime extension

Relative imports in ESM files need the extension of the *emitted* file (`.js`), or, with `rewriteRelativeImportExtensions`, the `.ts` source extension:

```ts
// tsconfig: {"declaration": false}
// file: util.ts
export const twice = (n: number): number => n * 2;
// file: main.ts
import { twice } from "./util"; // error TS2835
import { twice as ok } from "./util.js";
console.log(twice(2), ok(3));
```

TS2835 reads "Relative import paths need explicit file extensions in ECMAScript imports when '--moduleResolution' is 'node16' or 'nodenext'. Did you mean './util.js'?". Under `bundler` resolution the extensionless import compiles — and then fails in Node.js with `ERR_MODULE_NOT_FOUND`. That mismatch is one of the most common review findings.

##### `rewriteRelativeImportExtensions`: `.ts` specifiers that work in Node.js and in `tsc` output

```ts
// tsconfig: {"rewriteRelativeImportExtensions": true}
// file: util.ts
export const twice = (n: number): number => n * 2;
// file: main.ts
import { twice } from "./util.ts";
console.log(twice(2));
```

With `rewriteRelativeImportExtensions`, `tsc` emits `import { twice } from "./util.js"`, while Node.js type stripping runs the `.ts` file directly. Only *relative* specifiers are rewritten; `paths` aliases and package imports are not. Without the flag (and without `allowImportingTsExtensions` + `noEmit`), `"./util.ts"` is TS5097.

##### `verbatimModuleSyntax`: no import elision guesswork

With `verbatimModuleSyntax`, what you write is what is emitted: imports without `type` stay, imports with `type` disappear. Importing a type without `type` is an error, so a bundler or type stripper that compiles one file at a time never keeps an import of a type-only export:

```ts
// file: model.ts
export interface User { id: string }
export const anonymous: User = { id: "" };
// file: main.ts
import { User, anonymous } from "./model.js"; // error TS1484
import { type User as U2 } from "./model.js";
import type { User as U3 } from "./model.js";
const a: U2 = anonymous;
const b: U3 = a;
console.log(b);
```

TS1484: "'User' is a type and must be imported using a type-only import when 'verbatimModuleSyntax' is enabled." Note the difference between `import { type A }` (the import statement survives as `import {} from "./model.js"`, keeping side effects) and `import type { A }` (removed entirely).

Under `nodenext`, `verbatimModuleSyntax` also forbids ESM syntax in files that are CommonJS (a `.ts` file in a package without `"type": "module"`, or a `.cts` file): `export const x = 1` in such a file is TS1287 ("A top-level 'export' modifier cannot be used on value declarations in a CommonJS module when 'verbatimModuleSyntax' is enabled"), and `import` statements are TS1295 ("ECMAScript imports and exports cannot be written in a CommonJS file under 'verbatimModuleSyntax'"). Fix the package type or write `.mts` — not by removing `verbatimModuleSyntax`.

##### `erasableSyntaxOnly`: TypeScript that strip-only runtimes can run

Node.js (type stripping enabled by default since 22.18/23.6, stable since 24.12/25.2), Bun, Deno and many bundlers in "strip" mode replace types with whitespace and do not transform anything else.[^7] Constructs that generate code are rejected by `erasableSyntaxOnly` (TS1294 "This syntax is not allowed when 'erasableSyntaxOnly' is enabled."):

```ts
// tsconfig: {"erasableSyntaxOnly": true}
enum Color { Red, Green } // error TS1294
namespace Util { export const x = 1; } // error TS1294
class Point {
  constructor(private x: number) {} // error TS1294
}
const n = <number>(1 as unknown); // error TS1294
namespace OnlyTypes { export type Id = string; }
declare enum Ambient { A }
const p: OnlyTypes.Id = "id";
console.log(Color, Util, Point, n, p);
```

Type-only namespaces and `declare` declarations are fine; `import x = require("…")` in a module is also rejected. Verified at runtime: Node.js 22.23, 24.22 and 26.12 run a file with interfaces and `import type` directly, and all three throw `ERR_UNSUPPORTED_TYPESCRIPT_SYNTAX` ("TypeScript enum is not supported in strip-only mode") on an `enum`. Node.js ignores `tsconfig.json` entirely — no `paths`, no down-levelling, and relative imports must spell `.ts`.[^7]

##### JSON and other non-JS imports

```ts
// tsconfig: {"resolveJsonModule": true}
// file: config.json
{ "port": 8080 }
// file: main.ts
import config from "./config.json" with { type: "json" };
import bad from "./config.json"; // error TS1543
const port: number = config.port;
console.log(port, bad);
```

Under `nodenext`, an ESM file must import JSON with the `with { type: "json" }` attribute (TS1543 otherwise) — that is what Node.js requires at runtime. `assert { type: "json" }` is TS2880 in 6.0/7.0. JSON imports in ESM only have a default export.

For non-code assets with `noUncheckedSideEffectImports` (default `true`), declare the module shape once:

```ts
// file: assets.d.ts
declare module "*.css";
declare module "*.svg" {
  const url: string;
  export default url;
}
// file: main.ts
import "./global.css";
import logo from "./logo.svg";
import "./missing-polyfill.js"; // error TS2882
console.log(logo);
```

##### `paths` are type-checker aliases, not runtime aliases

`baseUrl` is gone; `paths` entries resolve relative to the `tsconfig.json` that declares them. `tsc` never rewrites the specifier, so the emitted file still says `import … from "@core"`:

```json
{
  "compilerOptions": {
    "module": "nodenext",
    "paths": { "@core": ["../core/src/index.ts"] }
  }
}
```

Verified: with this config `tsc --build` succeeds and Node.js 24 fails on the emitted `import { add } from "@core"` with `ERR_INVALID_MODULE_SPECIFIER`. For Node.js use workspace packages (`"dependencies": { "core": "workspace:*" }`) or `package.json` `"imports"` (`#core`, `#lib/*` — `#/…` needs Node.js ≥ 24); keep `paths` for bundled apps whose bundler is configured with the same aliases.

##### CommonJS interop

`esModuleInterop` is always on in 7.0. Under `nodenext`:

- An ESM file can `import fs from "node:fs"` / `import express from "express"` — the default import is `module.exports`.
- A CommonJS file may import/`require()` an ES module (Node.js 20.19+/22.12+ without a flag). TypeScript models this since 5.8 under `module: nodenext`/`node20` (5.7 reported TS1479 for an `import` of an `.mts` module from a CommonJS `.ts` file; 5.8 and 7.0 accept it). A module with top-level `await` still cannot be `require`d at runtime.
- CommonJS files (`.cts`, or `.ts` without `"type": "module"`) under `verbatimModuleSyntax` must use `import fs = require("node:fs")` and `export =` — TypeScript-only syntax that `erasableSyntaxOnly` rejects (TS1294). A plain `const p = require("node:path")` type-checks through `@types/node` but yields `any`. New code should be ESM.
- Dual packages: point `"exports"` conditions at separately built ESM/CJS outputs with matching `.d.mts`/`.d.cts` files; a single `.d.ts` for both formats produces "masquerading" types (types say ESM, runtime is CJS, or vice versa).

##### Module detection

`moduleDetection: "force"` treats every non-declaration file as a module, so top-level declarations in two scripts do not collide in the global scope. Without it, a file with no `import`/`export` is a global script — `const name = …` then conflicts with `lib.dom.d.ts`'s `name`.

***

#### Types and Inference

The examples follow the code conventions from the header: they compile with `tsc` 7.0.2 under the strict Node.js configuration, except for lines marked `// error TSxxxx`.

##### `unknown` at the boundaries, never `any`

`any` switches checking off in both directions and spreads: anything read from an `any` is `any`. `unknown` forces a check before use.

```ts
// BAD: JSON.parse returns any; the annotation is an unchecked promise
interface User { id: string; email: string }
const u1: User = JSON.parse('{"id":1}'); // compiles; u1.id is a number at runtime

// GOOD: parse to unknown, then validate
function isUser(v: unknown): v is User {
  return (
    typeof v === "object" && v !== null &&
    "id" in v && typeof v.id === "string" &&
    "email" in v && typeof v.email === "string"
  );
}
const raw: unknown = JSON.parse('{"id":"1","email":"a@b.c"}');
if (!isUser(raw)) throw new Error("invalid user payload");
const u2: User = raw;
u2.email.toLowerCase();

const n: unknown = 42;
n.toFixed(); // error TS18046
console.log(u1);
```

- `in` narrowing adds the property to the type (`"id" in v` makes `v.id` accessible as `unknown`), so hand-written guards no longer need casts.
- A type predicate is an unchecked claim: a wrong `isUser` lies to the compiler. In review, check that every property the type promises is actually tested — or use a schema library (Zod, Valibot, ArkType, TypeBox) and derive the type from the schema.
- `catch (e)` binds `e: unknown` under `strict` (`useUnknownInCatchVariables`).

##### Type aliases vs interfaces

```ts
interface Animal { name: string }
interface Dog extends Animal { bark(): void }      // extension is checked: conflicting members error
type Shape = { kind: "circle"; r: number } | { kind: "square"; side: number }; // unions need `type`
type Handler = (e: Event) => void;                  // function types read better as aliases
type Pair<T> = readonly [T, T];                     // tuples, mapped and conditional types need `type`

// Interfaces merge across declarations — useful for augmentation, surprising otherwise:
interface Window { appVersion: string }
const d: Dog = { name: "Rex", bark() {} };
const s: Shape = { kind: "circle", r: 1 };
const h: Handler = () => {};
const p: Pair<number> = [1, 2];
console.log(d, s, h, p);
```

Prefer `interface` for object shapes that others extend or augment, `type` for everything else. Accidental declaration merging (two `interface Config` in one scope) is a bug to flag. An `interface` cannot be assigned to `Record<string, unknown>` (no implicit index signature) while an object `type` alias can — relevant when passing to generic serialisers.

##### Discriminated unions and exhaustiveness

```ts
type Result<T, E = Error> =
  | { ok: true; value: T }
  | { ok: false; error: E };

type Event =
  | { type: "created"; id: string }
  | { type: "deleted"; id: string; soft: boolean }
  | { type: "renamed"; id: string; from: string; to: string };

function assertNever(x: never): never {
  throw new Error(`unhandled variant: ${JSON.stringify(x)}`);
}

function describe(e: Event): string {
  switch (e.type) {
    case "created": return `created ${e.id}`;
    case "deleted": return e.soft ? `trashed ${e.id}` : `deleted ${e.id}`;
    case "renamed": return `${e.from} -> ${e.to}`;
    default: return assertNever(e); // adding a variant makes this line a compile error
  }
}

function parse(s: string): Result<number> {
  const n = Number(s);
  return Number.isNaN(n) ? { ok: false, error: new Error(`not a number: ${s}`) } : { ok: true, value: n };
}
const r = parse("42");
if (r.ok) console.log(r.value + 1); else console.error(r.error.message);
console.log(describe({ type: "created", id: "1" }));
```

Destructured discriminants narrow too: with `type Action = { kind: "num"; payload: number } | { kind: "str"; payload: string }`, `const { kind, payload } = a; if (kind === "num") payload.toFixed()` type-checks (both names must come from one `const` destructuring of a parameter or `const`). Review points: a `default:` branch that returns a fallback instead of `assertNever` hides new variants; `satisfies never` (`e satisfies never`) is an alternative that needs no helper.

##### Narrowing — what works and what silently does not

```ts
declare const input: string | number | null | undefined;

// Truthiness narrowing drops "" and 0 as well as null/undefined:
function bad(v: string | number | null | undefined): string {
  if (v) return String(v);
  return "missing";             // BUG for 0 and ""
}
function good(v: string | number | null | undefined): string {
  if (v != null) return String(v); // != null removes exactly null and undefined
  return "missing";
}

// typeof null === "object":
function keys(o: object | null): string[] {
  if (typeof o === "object") return Object.keys(o); // error TS2769
  return [];
}

// Closures see narrowing after the last assignment (5.4+):
function later(x: string | undefined) {
  if (x === undefined) x = "default";
  return () => x.toUpperCase(); // x is string here: no further assignment follows
}

// Constant-key element access narrows (5.5+):
function readField(o: Record<string, unknown>, k: string): string {
  if (typeof o[k] === "string") return o[k];
  return "";
}

// Expressions that are always truthy/nullish are errors (5.6+):
declare function isReady(): boolean;
if (isReady) {} // error TS2774
if (/ready/) {} // error TS2872
const port = {} ?? 8080; // error TS2869
console.log(input, bad(0), good(0), keys(null), later(undefined), readField({}, "a"), port);
```

- TS2774 ("This condition will always return true since this function is always defined. Did you mean to call it instead?") catches the missing `()` on predicates — and TS2801 does the same for un-awaited `Promise` conditions (`if (promise)`).
- Narrowing does **not** survive across `await`, callbacks, or property reads on mutable objects after an intervening call: `if (this.user) { await save(); this.user.name }` compiles but `this.user` may have been cleared meanwhile. Copy to a `const` first.
- `typeof o === "object"` keeps `null` in the type (TS2769 above: no `Object.keys` overload accepts `object | null`); check `o !== null` too.
- `Array.isArray(x)` narrows `x` to `any[]` when `x` is `unknown`; prefer checking elements afterwards.

##### Type predicates: inferred, explicit, assertion functions

```ts
// Inferred type predicates (5.5+): filter callbacks narrow without annotations
const ids = ["1", undefined, "3"].filter(id => id !== undefined);
const firstId: string[] = ids;

// But a truthiness filter is not inferred as a predicate (it would also drop ""):
const names = ["a", "", undefined].filter(Boolean);
const bad: string[] = names; // error TS2322

// Assertion function: narrows the rest of the scope or throws
function assertDefined<T>(v: T, msg: string): asserts v is NonNullable<T> {
  if (v === undefined || v === null) throw new Error(msg);
}
const env: string | undefined = process.env["DATABASE_URL"];
assertDefined(env, "DATABASE_URL is not set");
const url = new URL(env);
console.log(firstId, bad, url.host);
```

Assertion functions must be called through a name with an explicit type annotation (a `const assertX = (v) => …` without an annotated type does not narrow — TS2775).

##### `satisfies`, `as const` and literal types

`satisfies` checks a value against a type without widening the value's inferred type; an annotation widens; `as` asserts and checks almost nothing.

```ts
type Route = { path: string; method: "GET" | "POST" };

const annotated: Record<string, Route> = {
  home: { path: "/", method: "GET" },
};
annotated.hom; // no error: the keys were widened to string

const routes = {
  home: { path: "/", method: "GET" },
  login: { path: "/login", method: "POST" },
} satisfies Record<string, Route>;
routes.hom; // error TS2551
const m: "POST" = routes.login.method; // literal type kept

const asserted = { path: "/", method: "PUT" } as Route; // error TS2352
const lie = { path: "/" } as Route; // compiles: `as` allows a subset — method is missing at runtime

const LEVELS = ["debug", "info", "warn", "error"] as const;
type Level = (typeof LEVELS)[number]; // "debug" | "info" | "warn" | "error"
function isLevel(s: string): s is Level {
  return (LEVELS as readonly string[]).includes(s);
}
console.log(m, asserted, lie, isLevel("info"));
```

Review rules: `as SomeType` on object literals, on `JSON.parse`, on `fetch().json()` and on `as unknown as T` double casts are unchecked claims — demand a validator or `satisfies`. `as const` on configuration objects keeps literal types and makes them `readonly`.

##### Generics

```ts
// Constrain with extends; return the precise type rather than the constraint
function pluck<T, K extends keyof T>(items: readonly T[], key: K): T[K][] {
  return items.map(i => i[key]);
}
const users = [{ id: 1, name: "a" }, { id: 2, name: "b" }];
const userNames: string[] = pluck(users, "name");
pluck(users, "email"); // error TS2345

// const type parameters (5.0+): infer literal types without `as const` at the call site
function routeNames<const T extends readonly string[]>(names: T): T { return names; }
const rn = routeNames(["home", "about"]); // readonly ["home", "about"]

// NoInfer (5.4+): stop a parameter from widening the inferred type
function pick<T extends string>(options: readonly T[], fallback: NoInfer<T>): T {
  return options[0] ?? fallback;
}
pick(["sm", "md"], "md");
pick(["sm", "md"], "xl"); // error TS2345

// Defaults and explicit type arguments
interface Page<T = unknown> { items: T[]; next?: string }
const page: Page<{ id: number }> = { items: [{ id: 1 }] };
console.log(userNames, rn, page);
```

- A type parameter that appears only once (e.g. `function f<T>(x: T): void`) adds nothing — use the constraint type directly. A type parameter used only in the return type (`function get<T>(key: string): T`) is a disguised `as` — flag it.
- `T extends object` excludes primitives but allows arrays and functions; `T extends Record<string, unknown>` rejects interfaces without index signatures.
- Conditional types distribute over unions only when the checked type is a naked type parameter; wrap in `[T] extends [U]` to prevent distribution.

##### Mapped, conditional and template literal types

```ts
type Getters<T> = { [K in keyof T & string as `get${Capitalize<K>}`]: () => T[K] };
type PersonGetters = Getters<{ name: string; age: number }>;
const g: PersonGetters = { getName: () => "a", getAge: () => 1 };

type ElementOf<T> = T extends readonly (infer E)[] ? E : never;
const e: ElementOf<string[]> = "x";

type EventName<T extends string> = `on${Capitalize<T>}`;
const ev: EventName<"click"> = "onClick";

// 7.0: inference from template literals consumes whole code points
type Head<S extends string> = S extends `${infer H}${string}` ? H : never;
const head: Head<"😀abc"> = "😀"; // 6.0 inferred the lone surrogate "\uD83D" here
type Chars<S extends string, A extends string[] = []> =
  S extends `${infer C}${infer R}` ? Chars<R, [...A, C]> : A;
const count: Chars<"a😀">["length"] = 2; // 6.0: 3
console.log(g, e, ev, head, count);
```

Deeply recursive conditional types slow the checker and fail with TS2589 ("Type instantiation is excessively deep and possibly infinite"). In review, prefer simple types at API boundaries; type-level programs belong in well-tested utility modules.

##### Optional properties, `undefined` and `exactOptionalPropertyTypes`

```ts
interface Patch { name?: string; email?: string }
const p1: Patch = { name: undefined }; // error TS2375

function update(p: Patch) {
  // "name" in p distinguishes "absent" from "present"; with exactOptionalPropertyTypes
  // a present key is never undefined, so PATCH semantics are reliable
  if ("name" in p) console.log(p.name.trim());
}
const input: { name?: string | undefined } = { name: undefined };
update(input); // error TS2379
update({});
console.log(p1);
```

When a property may legitimately hold `undefined`, write `name?: string | undefined`. Spreading objects with explicit `undefined` values overwrites defaults (`{ ...defaults, ...{ port: undefined } }` yields `port: undefined`) — `exactOptionalPropertyTypes` prevents such values from typing as `Partial<…>` in the first place.

##### Index signatures and `noUncheckedIndexedAccess`

```ts
const counts: Record<string, number> = {};
counts["a"] += 1; // error TS2532

const arr = [10, 20, 30];
const first: number = arr[0]; // error TS2322
const firstOk = arr[0] ?? 0;
for (const v of arr) console.log(v + 1);               // for-of yields number, no undefined
for (let i = 0; i < arr.length; i++) console.log(arr[i]! + 1); // index access: `!` after a bounds check

const m = new Map<string, number>();
m.set("a", (m.get("a") ?? 0) + 1);                     // Map.get always returns T | undefined
console.log(firstOk);
```

- `Record<string, T>` claims every key exists; use `Partial<Record<K, T>>` or `Map` for sparse data, or turn on `noUncheckedIndexedAccess`.
- Plain objects used as dictionaries inherit `Object.prototype`: `counts["constructor"]` is a function at runtime, and keys like `__proto__` from user input enable prototype pollution. Use `Map`, or `Object.create(null)` / `Object.hasOwn` checks.
- Prefer `for…of`/`entries()` over index loops; a `!` after a bounds check is acceptable, a `!` without one is a bug report waiting to happen.

##### Readonly, tuples and arrays

```ts
function total(xs: readonly number[]): number {
  xs.push(1); // error TS2339
  return xs.reduce((a, b) => a + b, 0);
}
const point: readonly [x: number, y: number] = [1, 2];
point[0] = 3; // error TS2540

const sorted = [3, 1, 2].toSorted((a, b) => a - b);     // es2023: non-mutating
const frozen = Object.freeze({ retries: 3 });
frozen.retries = 4; // error TS2540
console.log(total([1]), sorted);
```

- `readonly` is shallow and compile-time only: `Object.freeze` is the runtime counterpart (also shallow).
- `arr.sort()` sorts in place *and* compares as strings by default: `[10, 9, 1].sort()` is `[1, 10, 9]`. Always pass a comparator for numbers; prefer `toSorted`/`toReversed`/`with` (es2023) when the original must stay intact.

##### Functions, overloads and `this`

```ts
// Overloads: callers see only the overload signatures, not the implementation signature
function parseDate(v: string): Date;
function parseDate(v: number): Date;
function parseDate(v: string | number): Date {
  return new Date(v);
}
parseDate(true); // error TS2769

// Prefer a union parameter when the return type does not depend on the argument type:
function parseDate2(v: string | number): Date { return new Date(v); }

// Method shorthand in types is checked bivariantly even under strictFunctionTypes;
// property syntax is checked contravariantly.
interface Bivariant { handle(e: Event | string): void }
interface Contravariant { handle: (e: Event | string) => void }
const onlyStrings = (e: string) => e.length;
const b: Bivariant = { handle: onlyStrings };      // compiles — unsound
const c: Contravariant = { handle: onlyStrings };  // error TS2322

// `this` parameters document and check the receiver
function describeThis(this: { id: string }, prefix: string): string { return prefix + this.id; }
describeThis("x"); // error TS2684
console.log(parseDate2(0), b, c, describeThis.call({ id: "1" }, "#"));
```

Method shorthand is how callback interfaces end up accepting handlers that cannot handle all inputs. In APIs you own, declare callbacks with property syntax.

##### Enums and their alternatives

```ts
enum Status { Active, Disabled }
const fromApi: number = 42;
const s: Status = fromApi;          // compiles: any number is assignable to a numeric enum
const lit: Status = 42; // error TS2322

// String enums are nominal: a matching string literal is not assignable
enum Color { Red = "red" }
const c: Color = "red"; // error TS2322

// Preferred in new code: a const object + union type (erasable, works with type stripping)
const Role = { Admin: "admin", User: "user" } as const;
type Role = (typeof Role)[keyof typeof Role];
const r: Role = "admin";
const r2: Role = Role.User;
console.log(s, lit, c, r, r2);
```

- Enums and `const enum` generate code, so they fail under `erasableSyntaxOnly` (TS1294) and in Node.js type stripping. Under `isolatedModules`, using an ambient `declare const enum` (e.g. from a `.d.ts`) is TS2748 ("Cannot access ambient const enums when 'isolatedModules' is enabled"). For new code use `as const` objects or plain string-literal unions.
- `const enum` declared in a `.d.ts` you publish breaks consumers that compile with `isolatedModules`; never ship them.
- Bit-flag enums are a legitimate use where the code base allows enums.

##### Classes

```ts
class Account {
  readonly id: string;
  #balance = 0;                 // runtime-private (ES private field)
  private secret = "x";        // compile-time only: accessible via (acc as any).secret or ["secret"]
  static #count = 0;

  constructor(id: string) {
    this.id = id;
    Account.#count++;
  }

  deposit(amount: number): void {
    if (!Number.isFinite(amount) || amount <= 0) throw new RangeError(`invalid amount: ${amount}`);
    this.#balance += amount;
  }

  get balance(): number { return this.#balance; }
  static get count(): number { return Account.#count; }
}

class Savings extends Account {
  override deposit(amount: number): void {
    super.deposit(amount * 1.01);
  }
  override withdraw(): void {} // error TS4113
}
const acc = new Savings("a");
acc.deposit(10);
console.log(acc.balance, Account.count, acc["secret"]); // bracket access bypasses `private`
```

###### Class fields use `[[Define]]` semantics with the 7.0 default target

`useDefineForClassFields` defaults to `true` for `target >= es2022`, i.e. always with 7.0's default `es2025`. Field initialisers run *before* the constructor body — and before parameter properties are assigned: `class A { b = this.a * 2; constructor(public a: number) {} }` is TS2729 ("Property 'a' is used before its initialization") and computes `NaN` at runtime. A redeclared field in a subclass overwrites (shadows) an accessor or the value set by the base constructor:

```ts
class Base {
  name = "base";
  constructor() { this.init(); }
  init(): void {}
}
class Derived extends Base {
  value?: string;              // re-initialised to undefined AFTER Base's constructor ran init()
  override init(): void { this.value = "set in init"; }
}
console.log(new Derived().value); // undefined with [[Define]] (target >= es2022)
```

Verified at runtime: the program prints `undefined` when compiled with the 7.0 default target (`es2025`), and printed `set in init` with 5.9's default `es5`. Fix: `declare value?: string;` (type-only redeclaration, no emit) or initialise in the subclass constructor. The same pitfall affects frameworks that set fields from a base constructor (ORMs, older Lit/MobX code). Calling overridable methods from a constructor is a bug pattern in its own right.

###### Parameter properties

```ts
class Service {
  constructor(private readonly repo: Map<string, string>, public timeoutMs = 1000) {}
  find(id: string): string | undefined { return this.repo.get(id); }
}
console.log(new Service(new Map()).find("x"));
```

Parameter properties are concise but not erasable (TS1294 under `erasableSyntaxOnly`, rejected by Node.js type stripping). In code bases that run TypeScript through strip-only tools, declare fields explicitly.

###### Decorators: standard vs `experimentalDecorators`

```ts
// Standard (TC39) decorators: available since 5.0 without a flag
function logged<This, Args extends unknown[], R>(
  target: (this: This, ...args: Args) => R,
  context: ClassMethodDecoratorContext<This, (this: This, ...args: Args) => R>,
) {
  return function (this: This, ...args: Args): R {
    console.log(`-> ${String(context.name)}`);
    return target.call(this, ...args);
  };
}

class Api {
  @logged
  get(path: string): string { return `GET ${path}`; }
}
console.log(new Api().get("/users"));
```

- Standard decorators are not part of ES2025, so `tsc` down-levels them for `target <= es2025` (the output calls an `__esDecorate` helper). With `target: esnext` they are emitted verbatim, and **Node.js 24.22 and 26.12 throw `SyntaxError` on the `@` syntax** — verified. Do not combine decorators with `target: esnext` for Node.js.
- `experimentalDecorators: true` switches all decorators in the project to the legacy semantics (different signatures; parameter decorators and `emitDecoratorMetadata` only exist there). The two models cannot be mixed in one project. Changing the flag silently changes runtime behaviour of every decorator.
- Decorators are not erasable syntax: Node.js type stripping rejects them.

##### Modules of declarations: augmentation and globals

```ts
// file: augment.ts
export {};
declare global {
  interface Array<T> { last(): T | undefined }   // type only — an implementation must also exist
}
declare module "node:http" {
  interface IncomingMessage { requestId?: string }
}
// file: main.ts
import "./augment.js";
import type { IncomingMessage } from "node:http";
function id(req: IncomingMessage): string { return req.requestId ?? "-"; }
console.log(id);
```

Review: a `declare global` augmentation of a built-in without a matching runtime polyfill is a crash at the first call; augmentations of third-party modules must match the installed version's module name (`"express-serve-static-core"`, not `"express"`, for `Request`).

##### Utility types and their sharp edges

```ts
interface User { id: string; name: string; email?: string; role: "admin" | "user" }

type UserPatch = Partial<Omit<User, "id">>;           // id cannot be patched
type NewUser = Omit<User, "id"> & { password: string };
type PublicUser = Pick<User, "id" | "name">;
type ById = Record<User["id"], User>;
type Roles = User["role"];                             // "admin" | "user"
type Strict = Required<User>;                          // email becomes required

declare function fetchUser(id: string): Promise<User>;
type Fetched = Awaited<ReturnType<typeof fetchUser>>;  // User
type FetchArgs = Parameters<typeof fetchUser>;         // [id: string]

// Omit is not distributive: on a union it keeps only the common keys
type Shape = { kind: "circle"; r: number } | { kind: "square"; side: number };
type NoKind = Omit<Shape, "kind">;          // {} — r and side are gone
const broken: NoKind = { r: 1 };
broken.r; // error TS2339
type DistributiveOmit<T, K extends PropertyKey> = T extends unknown ? Omit<T, K> : never;
const fine: DistributiveOmit<Shape, "kind"> = { r: 1 };

// Omit with a misspelled key is not an error (K extends PropertyKey, not keyof T)
type Oops = Omit<User, "emial">;
const o: Oops = { id: "1", name: "n", role: "user", email: "still here" };

const patch: UserPatch = { name: "x" };
const fetched: Fetched | undefined = undefined;
const args: FetchArgs = ["1"];
const s: Strict | undefined = undefined;
const nu: NewUser | undefined = undefined;
const pu: PublicUser | undefined = undefined;
const by: ById = {};
const role: Roles = "user";
console.log(broken, fine, o, patch, fetched, args, s, nu, pu, by, role);
```

Other built-ins worth knowing: `Readonly`, `NonNullable`, `Extract`/`Exclude` (filter unions), `InstanceType`, `ThisParameterType`, `Uppercase`/`Lowercase`/`Capitalize`/`Uncapitalize`, `NoInfer`. Deep variants (`DeepPartial`, `DeepReadonly`) are not built in; a hand-written `DeepReadonly` that recurses into functions, `Date`, `Map` or `Set` produces unusable types — test it with those members.

##### Branded (nominal) types

```ts
declare const brand: unique symbol;
type Brand<T, B extends string> = T & { readonly [brand]: B };

type UserId = Brand<string, "UserId">;
type OrderId = Brand<string, "OrderId">;

function asUserId(s: string): UserId {
  if (!/^u_[a-z0-9]+$/.test(s)) throw new Error(`invalid user id: ${s}`);
  return s as UserId; // the only place a cast is allowed: after validation
}

declare function loadOrder(id: OrderId): void;
const uid = asUserId("u_42");
loadOrder(uid); // error TS2345
loadOrder("o_1"); // error TS2345
const raw: string = uid; // brands widen to the base type freely
console.log(raw);
```

Brands cost nothing at runtime. Use them for identifiers, units (`Cents`, `Milliseconds`) and validated strings (`Email`, `SafeHtml`). A brand created with `as` anywhere except in the validating constructor defeats the purpose.

##### Accessors, `accessor` fields and abstract classes

```ts
abstract class Shape {
  abstract area(): number;
  describe(): string { return `${this.constructor.name} with area ${this.area().toFixed(2)}`; }
}
class Circle extends Shape {
  #r = 0;
  // getter and setter may have unrelated types (5.1+)
  get radius(): number { return this.#r; }
  set radius(v: number | string) { this.#r = Number(v); }
  override area(): number { return Math.PI * this.#r ** 2; }
}
const c = new Circle();
c.radius = "2";
const r: number = c.radius;
new Shape(); // error TS2511

class Counter {
  accessor count = 0; // auto-accessor (standard decorators' storage); emitted as a private field + get/set
}
console.log(c.describe(), r, new Counter().count);
```

##### Variance annotations (advanced)

`in`/`out` modifiers on type parameters (`interface Producer<out T>`, `interface Consumer<in T>`) document and speed up variance checks for large generic types. A wrong annotation makes unsound assignments compile — only add them where the variance is obvious, and never to "fix" an assignability error.

***

#### Naming, Style and Module Hygiene

- `PascalCase` for types, interfaces, classes, enums and type parameters with meaning (`TItem` is acceptable, single letters for simple generics); `camelCase` for values and functions; `UPPER_SNAKE_CASE` only for true constants. No `I` prefix on interfaces.
- Named exports instead of `export default`: renames stay consistent and re-exports are explicit. (Framework conventions that require default exports — Next.js pages, Storybook stories — are exceptions.)
- `import type`/`export type` for type-only symbols (enforced by `verbatimModuleSyntax`).
- Import order: Node.js built-ins (`node:` prefix), external packages, internal aliases, relative imports. Always use the `node:` prefix for built-ins.
- Barrel files (`index.ts` re-exporting everything) slow down checking and bundling and create import cycles; prefer direct imports inside a package and a single public entry point per package.
- Avoid `namespace` for organising code (modules do that); keep `declare namespace` for describing globals in `.d.ts` files.
- Use `unknown` and narrowing over overloaded `any`-typed helpers; keep functions small enough that inferred return types are obvious, and annotate return types of exported functions.
- Prefer `readonly` arrays and properties in public APIs; return new arrays instead of mutating inputs.
- Comments explain *why*; types and names explain *what*. Use TSDoc (`/** … */`, `@param`, `@returns`, `@throws`, `@deprecated`) on exported APIs — editors show `@deprecated` as strikethrough.

***

#### Node.js Specifics

```ts
// tsconfig: {"noPropertyAccessFromIndexSignature": true}
import { readFile } from "node:fs/promises";
import { join } from "node:path";

// ESM has no __dirname; Node.js >= 20.11 provides import.meta.dirname/filename
const here: string = import.meta.dirname;
const pkg: unknown = JSON.parse(await readFile(join(here, "..", "package.json"), "utf8")); // dist/ -> package root

// process.env values are string | undefined; with noPropertyAccessFromIndexSignature use brackets
const port = Number.parseInt(process.env["PORT"] ?? "3000", 10);
if (!Number.isInteger(port) || port <= 0 || port > 65535) throw new Error(`invalid PORT`);
const dbUrl = process.env.DATABASE_URL; // error TS4111

// es2024 helpers (Node.js >= 21/22)
const byRole = Object.groupBy([{ n: "a", admin: true }, { n: "b", admin: false }], u => (u.admin ? "admin" : "user"));
const { promise, resolve } = Promise.withResolvers<number>();
setTimeout(() => resolve(port), 0);
console.log(pkg !== undefined, byRole, await promise, dbUrl);
```

- Validate configuration once at startup into a typed object; do not read `process.env` throughout the code base.
- `Object.groupBy` returns a null-prototype object with optional keys (`Partial<Record<K, T[]>>`), so `byRole.admin` is `… | undefined`.
- Prefer `node:test` + `node:assert/strict` or Vitest for tests; both run `.ts` through type stripping or a transform, never type-check.

***

#### JSX (React and others)

- `"jsx": "react-jsx"` (automatic runtime) or `"preserve"` when the bundler transforms JSX; `jsxImportSource` for Preact/Solid/Emotion. The classic `"jsx": "react"` needs `React` in scope.
- Component props: plain function components with a typed props parameter; `React.FC` is unnecessary. Children are typed explicitly (`children?: React.ReactNode`).
- Event handler types come from the library (`React.ChangeEvent<HTMLInputElement>`), not the DOM `Event`.
- Generic components need `<T,>` in `.tsx` arrow functions (the comma disambiguates from JSX).
- Vue/Svelte/Astro/MDX type checking in October 2026 still runs on TypeScript 6.0 tooling (their language tools need the 6.0 API).[^1]

***

#### Observability and Debugging

- Emit source maps (`sourceMap: true`, optionally `inlineSources: true`) and run Node.js with `--enable-source-maps` so stack traces point at `.ts` lines. With type stripping, positions are preserved without source maps because types are replaced by whitespace.
- Keep `Error` objects intact through logging (`logger.error({ err }, "msg")`) — `JSON.stringify(err)` yields `{}` because `message`/`stack` are non-enumerable.
- Name async work (`AbortSignal` reasons, error messages with identifiers) so failures are traceable; `Error.captureStackTrace` is V8-specific.
- For compiler performance: `--extendedDiagnostics` (times, memory, type counts), `--generateTrace <dir>` (open in Perfetto/`@typescript/analyze-trace`), and in 7.0 `--pprofDir <dir>` for Go profiles.

***

#### Errors and Exceptions

```ts
class HttpError extends Error {
  override readonly name = "HttpError";
  constructor(readonly status: number, message: string, options?: ErrorOptions) {
    super(message, options);
  }
}

async function load(url: string): Promise<unknown> {
  let res: Response;
  try {
    res = await fetch(url, { signal: AbortSignal.timeout(5_000) });
  } catch (e) {
    // e is unknown (useUnknownInCatchVariables, part of strict)
    throw new Error(`request to ${url} failed`, { cause: e }); // keep the original error
  }
  if (!res.ok) throw new HttpError(res.status, `GET ${url}: ${res.status}`);
  return res.json();
}

function message(e: unknown): string {
  if (e instanceof Error) return e.message;
  return String(e);
}

try {
  await load("http://127.0.0.1:1/");
} catch (e) {
  console.error(e.message); // error TS18046
  console.error(message(e));
}
```

- Never `catch (e: any)`; `catch (e: Error)` is a compile error (TS1196) because anything can be thrown.
- `instanceof Error` fails across realms (iframes, `vm` contexts, some test runners); `Error.isError()` (in `lib: esnext`, Node.js ≥ 24) handles that — but it is not in the `es2025` lib.
- Keep `cause` when wrapping. Swallowing (`catch {}` with no comment, or `catch (e) { return null }`) hides failures — flag it.
- Throw `Error` subclasses, not strings or plain objects; set `name` so logs and `instanceof` checks are meaningful.
- With `target >= es2022`, subclassing `Error` works without `Object.setPrototypeOf(this, new.target.prototype)` hacks — those were only needed for ES5 output, which 7.0 cannot produce anymore. Remove them in migrations.

***

#### Async Code

##### Floating promises and async callbacks

`tsc` does not report un-awaited promises; typescript-eslint's `no-floating-promises` and `no-misused-promises` do (run on 6.0, see Tooling). What a reviewer must catch by reading:

```ts
declare function save(id: string): Promise<void>;
const ids = ["a", "b"];

// BAD: forEach ignores the returned promises — errors become unhandled rejections,
// and the function returns before any save completes
ids.forEach(async id => { await save(id); });

// BAD: fire-and-forget without error handling
save("x");

// GOOD: run concurrently and wait for all
await Promise.all(ids.map(id => save(id)));

// GOOD: sequential when order or rate limits matter
for (const id of ids) await save(id);

// GOOD: independent tasks where partial failure is acceptable
const results = await Promise.allSettled(ids.map(save));
for (const r of results) if (r.status === "rejected") console.error(r.reason);

// Intentional fire-and-forget: say so and handle errors
void save("y").catch(err => console.error("background save failed", err));
```

- `if (promise)` is TS2801 ("This condition will always return true since this 'Promise<boolean>' is always defined") — the classic missing `await` in a condition.
- `return await` inside `try` is required for the `catch`/`finally` to see the rejection; outside `try` it is redundant.
- `Promise.all` rejects on the first failure but does not cancel the others — pass an `AbortSignal` to the operations if they must stop.
- Unbounded `Promise.all(items.map(fetch…))` over thousands of items exhausts sockets/memory; batch or use a concurrency limiter.

##### Cancellation and timeouts

```ts
async function fetchJson(url: string, signal?: AbortSignal): Promise<unknown> {
  const res = await fetch(url, { signal: signal ? AbortSignal.any([signal, AbortSignal.timeout(10_000)]) : AbortSignal.timeout(10_000) });
  if (!res.ok) throw new Error(`HTTP ${res.status}`);
  return res.json();
}
const controller = new AbortController();
const pending = fetchJson("https://example.com/data.json", controller.signal);
controller.abort(new Error("user navigated away"));
await pending.catch((e: unknown) => console.log("aborted:", e));
```

`fetch` without a timeout hangs for as long as the server keeps the connection open. `AbortSignal.any` needs Node.js ≥ 20.3 (types come from `@types/node` or `lib.dom`).

##### `using` and `await using` (explicit resource management)

```ts
import { open } from "node:fs/promises";

class Lock implements Disposable {
  constructor(readonly name: string) { console.log(`lock ${name}`); }
  [Symbol.dispose](): void { console.log(`unlock ${this.name}`); }
}

function critical(): void {
  using lock = new Lock("db");
  console.log("working with", lock.name);
} // lock released here, also when the body throws

async function firstLine(path: string): Promise<string | undefined> {
  await using file = await open(path); // FileHandle implements Symbol.asyncDispose (Node.js >= 18.18/20.4)
  for await (const line of file.readLines()) return line;
  return undefined;
}

critical();
console.log(await firstLine(new URL(import.meta.url).pathname));
```

- The `Disposable`/`AsyncDisposable` types and `Symbol.dispose` are declared in `lib: esnext` (or `esnext.disposable`) and by `@types/node` — **not** in `es2025`. With `lib: ["es2025"]` and no `@types/node`, the class above fails with TS2318 "Cannot find global type 'Disposable'" and TS2550 on `Symbol.dispose`.
- For `target <= es2025`, `tsc` down-levels `using` to `__addDisposableResource`/`__disposeResources` helpers (works on Node.js 22+, which defines `Symbol.dispose`). With `target: esnext` the `using` keyword is emitted as-is: Node.js 24.22 and 26.12 run it, **Node.js 22.23 throws `SyntaxError`**.
- `DisposableStack`/`AsyncDisposableStack` and `SuppressedError` exist at runtime from Node.js 24.
- Do not `using` a value that is returned from the function — it is disposed at scope exit.

##### Iterators and generators

```ts
// strictBuiltinIteratorReturn (5.6+, in strict): the done-result value is undefined, not any
const it = [1, 2, 3].values();
const step = it.next();
const n: number = step.value; // error TS2322

// Iterator helpers (lib es2025; Node.js >= 22): lazy, single-pass
const evens = [1, 2, 3, 4, 5, 6].values().filter(x => x % 2 === 0).map(x => x * 10).take(2).toArray();

function* idGen(prefix: string): Generator<string, void, undefined> {
  let i = 0;
  while (true) yield `${prefix}-${i++}`;
}
const ids = idGen("req").take(3).toArray();

// Set methods (lib es2025; Node.js >= 22)
const admins = new Set(["ann", "bob"]);
const active = new Set(["bob", "cid"]);
const activeAdmins = admins.intersection(active);
console.log(evens, ids, [...activeAdmins]);
```

Iterator helper objects are consumed once: iterating `evens`-style pipelines twice yields nothing the second time — call `.toArray()` when the result is reused.

***

#### Targets, Libraries and Runtimes

`target` decides which **syntax** is down-levelled; `lib` decides which **declarations** exist; the runtime decides what actually works. 7.0 never polyfills APIs.

| API (lib) | Type-checks with | Node.js 22.23 | Node.js 24.22 | Node.js 26.12 |
| --- | --- | --- | --- | --- |
| `Set.prototype.union` etc., iterator helpers | `es2025` | yes | yes | yes |
| `RegExp.escape`, `Promise.try`, `Float16Array` | `es2025` | **no** | yes | yes |
| `Error.isError`, `DisposableStack`, `SuppressedError` | `esnext` | **no** | yes | yes |
| `Array.fromAsync` | `esnext` | yes | yes | yes |
| `Temporal`, `Map.prototype.getOrInsert`, `Uint8Array.prototype.toBase64` | `esnext` | **no** | **no** | yes |

(Runtime columns measured with `typeof` checks on those exact Node.js builds.)

Rules for review:

- A project on Node.js 22 with no `lib` and no `target` (7.0 default `es2025`) type-checks `RegExp.escape("…")` and crashes at runtime with `TypeError: RegExp.escape is not a function`. Set `target`/`lib` to `es2024` for Node.js 22, or guard and polyfill.
- `lib: ["esnext"]` is a moving target: a TypeScript upgrade adds declarations for APIs your runtime lacks. Prefer a year-numbered lib plus specific `esnext.*` entries you have verified (`"lib": ["es2025", "esnext.disposable"]`).
- `@types/node` must match the **oldest** Node.js you support, not the newest: `@types/node@26` declares APIs that Node.js 22 lacks.
- Browser code: `target` should match the browserslist baseline; `lib` includes `dom` (and `webworker` for workers — not both in one project).

***

#### JavaScript and JSDoc Projects (`allowJs` / `checkJs`)

7.0 checks `.js` files with TypeScript's rules, expressed through JSDoc. Code that type-checked under 6.0 can fail under 7.0. All examples below were compiled with `allowJs`, `checkJs` and `strict` on 7.0.2.

##### Constructor functions → classes

```js
// BAD in 7.0: no longer understood as a class
function Point(x) { // error TS7006
  this.x = x; // error TS2683
}
const p = new Point(3); // error TS7009
console.log(p);
```

```js
// GOOD
export class Point {
  /** @param {number} x */
  constructor(x) {
    this.x = x;
  }
  norm() { return Math.abs(this.x); }
}
console.log(new Point(3).norm());
```

##### `@enum`, values as types, typedefs

```js
/** @enum {number} */
const Color = { Red: 1, Green: 2 };
/** @param {Color} c */ // error TS2749
export function paint(c) { return c; }

const FORWARD = 1, BACKWARD = 2;
/** @typedef {typeof FORWARD | typeof BACKWARD} Direction */
/** @typedef {(typeof Color)[keyof typeof Color]} ColorValue */

/** @param {Direction} d @param {ColorValue} c */
export function go(d, c) { return d + c; }
go(FORWARD, Color.Red);
```

- 6.0 accepted `@param {Color}` for an `@enum` object and `{FORWARD | BACKWARD}` for values; 7.0 reports TS2749 ("'Color' refers to a value, but is being used as a type here. Did you mean 'typeof Color'?").
- `/** @typedef {T} Name */` is the only typedef form; `/** @typedef {T} */ Name;` is gone.
- `{?}` (unknown type) and Closure function types `{function(string): void}` are parse errors; write `{any}` and `{(s: string) => void}`. Postfix `!` (`{string!}`) is unsupported — write `{string}`.

##### Rest parameters, `arguments`, optional parameters

```js
export function first() { return arguments[0]; }
first("x"); // error TS2554

/** @param {...number} ns */
export function sum(...ns) { return ns.reduce((a, b) => a + b, 0); }
sum(1, 2, 3);

/** @param {number} [x] */
export function opt(x) { return x ?? 0; }
opt();
```

`arguments` no longer creates an implicit `...args: any[]`; `@param {...T}` only means `T[]` and needs real rest syntax. Under `"strict": false`, 7.0 no longer lets JS callers omit arguments for parameters typed `undefined`, `unknown` or `any`.[^6]

##### Other 7.0 JS changes

- `@overload` on arrow functions/function expressions is ignored — put a `@type {{ (x: string): string; (x: number): number }}` on the variable.
- `@type` casts (`/** @type {any} */ (x)`) no longer narrow the original variable — same as `as` in TypeScript.
- `@typedef`/`@callback` inside a class body are hoisted out of the class and can collide with outer names.
- `this.x;` with a JSDoc type in a constructor no longer declares a property — declare a class field.
- CommonJS: a module may assign `module.exports = …` **or** `module.exports.x = …`, not both; `var mod = module.exports` aliasing, `this.x = …` at module top level, `require("fs").readFile` single-property requires, and nested undeclared expandos (`exports.N.X.p = 1`) are no longer recognised.
- `.d.ts` emit from JS was rewritten; review generated declarations of JS libraries when upgrading.

***

#### Declaration Files and Libraries

##### `isolatedDeclarations`

```ts
// tsconfig: {"noEmit": false, "declaration": true, "emitDeclarationOnly": true, "outDir": "out", "isolatedDeclarations": true, "allowJs": false, "checkJs": false}
export function id(a: number) { return a; } // error TS9039
export function idOk(a: number): number { return a; }
export const sum = (a: number, b: number) => a + b; // error TS9013
export const sumOk = (a: number, b: number): number => a + b;
export const list = [1, 2]; // error TS9017
export const listOk: number[] = [1, 2];
export const version = "1.0";        // literal initialisers are fine
export class Repo {
  items: string[] = [];
  count() { return this.items.length; } // error TS9013
}
```

Every exported declaration must spell its type when it cannot be read syntactically. The payoff: declaration files can be produced per file by any tool, and `tsc --build` can start dependants without waiting for type checking. Diagnostic codes changed between 6.0 and 7.0: 6.0.3 reports TS9007 for `id` and `sum` and TS9008 ("Method must have an explicit return type annotation") for `count`, where 7.0.2 reports TS9039/TS9013/TS9013. `isolatedDeclarations` cannot be combined with `allowJs` (TS5053).

##### Shipping types

- `package.json` `"exports"`: put `"types"` first in each condition block; ship `.d.ts` next to the `.js` they describe (`.d.mts`/`.d.cts` for dual packages).
- `skipLibCheck: true` is standard for applications, but libraries should type-check their own emitted `.d.ts` in CI (e.g. compile a consumer project without `skipLibCheck`).
- Never publish `const enum`, `paths` aliases, or `declare global` augmentations without a runtime implementation.
- `stripInternal` removes `/** @internal */` declarations; it does not stop runtime access.
- `types`/`typesVersions` are legacy; `exports` supersedes them for `nodenext`/`bundler` consumers.

***

#### Builds, Project References and Performance

##### Project references

```json
{
  "files": [],
  "references": [{ "path": "packages/core" }, { "path": "packages/app" }]
}
```

```json
{
  "compilerOptions": {
    "composite": true,
    "rootDir": "src",
    "outDir": "dist",
    "module": "nodenext",
    "types": [],
    "isolatedDeclarations": true
  },
  "include": ["src"]
}
```

- `tsc -b` (or `--build`) builds referenced projects in dependency order; `composite` requires `declaration` (implied) and all files covered by `include`/`files`.
- If a referenced project has errors, its outputs are not written and dependants fail with TS6305 ("Output file '…/dist/index.d.ts' has not been built from source file …") — fix the upstream error first.
- `--builders <n>` (7.0) builds independent projects concurrently; total checker threads = builders × checkers.
- `--stopBuildOnErrors` skips building downstream projects when an upstream project has errors.
- A referenced project cannot set `noEmit` (TS6310 "Referenced project '…' may not disable emit"); use `emitDeclarationOnly` when a bundler produces the JavaScript.

##### Fast, deterministic CI checks

```bash
# type-check only, with an explicit, reproducible checker count
npx tsc -p tsconfig.json --noEmit --checkers 4

# low-memory runners
npx tsc -p tsconfig.json --noEmit --singleThreaded

# monorepo
npx tsc -b --builders 2 --checkers 2

# diagnose slow builds
npx tsc -p . --noEmit --extendedDiagnostics
npx tsc -p . --noEmit --pprofDir ./profiles
```

- `--checkers` changes only performance — except for rare order-dependent results; teams that want bit-identical diagnostics between laptop and CI pin it.
- `incremental` + `tsBuildInfoFile` still work; cache the `.tsbuildinfo` in CI.
- `--noCheck` (5.6+) emits without type checking — useful for fast emit when a separate `tsc --noEmit` job checks; never as the only CI step.
- `skipLibCheck: true` cuts time substantially; 7.0 may now report declaration conflicts that touch your own files even with it on.

##### Type-level performance

- Prefer interfaces with `extends` over large intersections; name complex types rather than inlining them.
- Avoid huge unions (thousands of literal members) and deep recursive conditional types; TS2589 means the type must be simplified, not that a `// @ts-ignore` should be added.
- Explicit return types on exported functions make declaration emit cheap and errors local.

***

#### Tooling: Linting, Testing, Running

##### Running TypeScript

| Tool | Type-checks? | Notes for 7.0 projects |
| --- | --- | --- |
| `tsc` 7.0 | yes | The only type checker. Run `tsc --noEmit` (or `tsc -b`) in CI regardless of how code is executed. |
| Node.js type stripping (`node file.ts`) | no | Default-on since Node.js 22.18/23.6, stable since 24.12/25.2. Needs erasable syntax and `.ts` extensions in relative imports; ignores `tsconfig.json`.[^7] |
| `tsx`, esbuild, SWC, Bun, Deno | no | Transpile per file; need `isolatedModules`/`verbatimModuleSyntax` discipline. |
| `ts-node` | (uses the 6.0 API) | Unmaintained since 10.9.2; replace. |

##### Linting

- **typescript-eslint** 8.x supports TypeScript `>=4.8.4 <6.1.0`. In a 7.0 repo install 6.0 under the `typescript` name (`"typescript": "npm:@typescript/typescript6@^6.0.2"`) and 7.0 under an alias so that `tsc` is 7.0; the linter then type-checks with 6.0, which agrees with 7.0 for code that is clean under `stableTypeOrdering`.[^1][^5]
- Rules that catch real bugs and are worth requiring: `no-floating-promises`, `no-misused-promises`, `await-thenable`, `no-unnecessary-condition`, `switch-exhaustiveness-check`, `no-unsafe-assignment`/`no-unsafe-member-access`/`no-unsafe-call`/`no-unsafe-return`, `restrict-template-expressions`, `prefer-nullish-coalescing`, `consistent-type-imports` (or rely on `verbatimModuleSyntax`), `no-explicit-any`, `ban-ts-comment` (allow `@ts-expect-error` with description only).
- **oxlint type-aware mode** is the one type-aware linter that runs on the 7.0 compiler itself: `oxlint-tsgolint` v7 (7.0.2004 on npm in October 2026; the version encodes the TypeScript release it embeds, 7.0.2) is built on typescript-go and implements 59 of typescript-eslint's 61 type-aware rules. Enable with `oxlint --type-aware` (add `--type-check` to also report compiler diagnostics) or `"options": { "typeAware": true }` in the oxlint config.[^9] Biome lints without the TypeScript compiler.

##### Testing

- **Vitest** (5.x) and **Jest with SWC/esbuild/Babel transforms** transpile only — keep a separate `tsc --noEmit` step that includes the test files (with `"types": ["vitest/globals"]` or `["jest"]` in the test tsconfig if globals are used).
- **ts-jest** 29.4.15 declares `typescript <7`: run it against the 6.0 alias (its `isolatedModules` mode still calls the 6.0 `transpileModule` API), or switch to a transpile-only transformer such as `@swc/jest`.
- Type tests: `expectTypeOf` (Vitest), `tsd`, or plain `// @ts-expect-error` lines compiled by `tsc`:

```ts
type Equal<A, B> = (<T>() => T extends A ? 1 : 2) extends (<T>() => T extends B ? 1 : 2) ? true : false;
function assertType<T extends true>(): T | undefined { return undefined; }

interface User { id: string; name: string }
assertType<Equal<Pick<User, "id">, { id: string }>>();
// @ts-expect-error -- User requires id
const invalid: User = { name: "x" };
console.log(invalid);
```

- Prefer `@ts-expect-error` over `@ts-ignore`: it fails when the error disappears, so stale suppressions are found. Require a reason after `--`.

##### Editors

The 7.0 language server replaces tsserver. Embedded-language tooling (Vue, Svelte, Astro, MDX, Angular templates) still needs the 6.0 language service in October 2026. If a team reports "the editor shows no error but CI fails" (or the reverse), check which TypeScript version each side runs.[^1]

***

#### Security

- **Types are not validation.** Every value crossing a trust boundary (HTTP bodies, query strings, `JSON.parse`, environment variables, message queues, `localStorage`, `postMessage`) is `unknown` until validated at runtime. An interface on `req.body` is a claim, not a check.
- **Prototype pollution.** Merging user-controlled objects into plain objects (`Object.assign(target, body)`, recursive "deep merge", `obj[key] = value` with `key` from input) can write `__proto__`/`constructor.prototype`. Use `Map`, `Object.create(null)`, explicit allow-lists of keys, and reject `__proto__`/`constructor`/`prototype` keys.
- **Injection.** Template literals do not escape: `` db.query(`SELECT * FROM users WHERE id = ${id}`) `` is SQL injection regardless of `id: string`. Use parameterised queries or tagged-template APIs that bind parameters. The same applies to shell commands (`child_process.exec` with interpolation → use `execFile`/`spawn` with an argument array), HTML (`innerHTML`), and regular expressions built from input (escape with `RegExp.escape` where available — `es2025` lib, Node.js ≥ 24 — or a vetted helper).
- **Secrets in source and logs.** No tokens in `.ts` files, `tsconfig.json` or test fixtures; redact before logging errors that include request objects. Source maps uploaded to public CDNs expose the original source.
- **`any` and `as` around auth code.** A cast in authorization logic (`(user as Admin).permissions`) skips the check that the type system would otherwise force. Demand explicit narrowing.
- **Timing-safe comparisons** for tokens: `crypto.timingSafeEqual` on equal-length `Buffer`s, not `===`.
- **Supply chain.** The `typescript` package now installs a native binary through platform-specific optional dependencies (`@typescript/typescript-<os>-<arch>`); lockfiles must contain them, and installs that omit optional dependencies or use a mismatched platform cache fail at `tsc` time.

***

#### Typical Bugs a Reviewer Must Catch

All of the following compile cleanly with `tsc` 7.0.2 in strict mode (outputs measured on Node.js 24.22):

```ts
// 1. || treats 0 and "" as missing — use ??
const cfg: { retries?: number; name?: string } = { retries: 0, name: "" };
const retries = cfg.retries || 3;        // 3, not 0
const retriesOk = cfg.retries ?? 3;      // 0

// 2. Default sort is lexicographic and in place
const nums = [10, 9, 1];
nums.sort();                             // [1, 10, 9]

// 3. Stateful global regex: test() advances lastIndex
const re = /a/g;
const twice = [re.test("a"), re.test("a")]; // [true, false]

// 4. Object.keys loses the key type; casting it back is a lie for objects with extra keys
const keys = Object.keys({ a: 1 }) as ("a")[];

// 5. Array(n).fill(obj) shares one object
const grid = Array(2).fill([]) as number[][];
grid[0]!.push(1);                        // grid[1] also contains 1

// 6. Number parsing
const fromEmpty = Number("");            // 0, not NaN
const partial = parseInt("08px", 10);    // 8 — trailing garbage accepted

// 7. JSON.stringify drops/throws silently-typed values
const lost = JSON.stringify({ m: new Map([["k", 1]]), u: undefined, d: new Date(0) });
// '{"m":{},"d":"1970-01-01T00:00:00.000Z"}' — Map becomes {}, undefined key vanishes, Date becomes string

// 8. Spreading a class instance drops prototype methods and getters
class Money { constructor(readonly cents: number) {} get euros(): number { return this.cents / 100; } }
const copy = { ...new Money(150) };
const euros = (copy as Money).euros;     // undefined at runtime

// 9. for…in over an array iterates string keys (and inherited enumerable props)
let sum = "";
for (const i in [5, 6]) sum += i;         // "01"

// 10. Date months are 0-based; string dates parse as UTC or local depending on format
const jan = new Date(2026, 1, 1).getMonth();   // 1 = February
console.log(retries, retriesOk, nums, twice, keys, grid, fromEmpty, partial, lost, euros, sum, jan);
```

More patterns to flag:

- **Non-null assertions (`!`) without a preceding check**, `as` casts on external data, `as unknown as T`, `// @ts-ignore`, `any` in public signatures, `Function`/`Object`/`{}` as types (`{}` accepts every non-nullish value, including numbers).
- **Missing `await`**: floating promises, `forEach(async …)`, `if (promise)`, `return promise` inside `try` without `await`, async functions passed where a sync callback is expected (`array.filter(async …)` keeps every element because a promise is truthy).
- **Exhaustiveness**: `switch` on a union with a `default` that silently accepts new variants; `Record<Union, T>` lookup tables are safer than `switch` because missing keys are compile errors.
- **Optional chaining fallbacks**: `a?.b.c` still throws when `a.b` is undefined; `user?.age > 18` is a compile error (TS18048), but `(user?.age ?? 0) > 18` silently treats "unknown" as 0 — check that the default is meaningful.
- **Equality**: `==` (except `== null`), comparing objects/arrays/dates with `===` (identity, not value), `NaN === NaN` (false — use `Number.isNaN`).
- **Floating-point money**: `0.1 + 0.2 !== 0.3`; use integer minor units or a decimal library; `BigInt` for IDs beyond 2^53 (`JSON.parse` silently rounds 64-bit IDs).
- **Mutation of inputs**: `sort`, `reverse`, `splice` on parameters or on `readonly` data obtained through a cast; prefer `toSorted`/`toReversed`/`toSpliced`.
- **Class field semantics** under `target >= es2022` (`[[Define]]`): subclasses redeclaring fields wipe values set by base constructors; use `declare`.
- **Module/runtime mismatch**: extensionless imports or `paths` aliases in code run by Node.js; `module: esnext` + `moduleResolution: bundler` in a server package; `target: esnext` with decorators/`using` on Node.js 24/22; `lib` newer than the runtime.
- **Leaks**: event listeners, intervals, `AbortController`s and subscriptions never removed; prefer `using`/`DisposableStack` or `{ signal }` options (`addEventListener(type, fn, { signal })`).
- **Error handling**: `catch {}` swallowing, losing `cause`, `throw "string"`, `Promise` rejection with non-`Error` values, `process.on("unhandledRejection")` used to hide bugs.
- **Date/time**: `new Date("2026-10-22")` is UTC midnight but `new Date("2026-10-22T00:00")` is local midnight; month index off by one; time-zone arithmetic with fixed 24-hour days.
- **`typeof x === "object"`** without `x !== null`; `Array.isArray` vs `instanceof Array` across realms.
- **Type predicates that lie** (guards that check fewer properties than the predicate claims) and assertion functions that do not throw on every failing path.

***

#### Migration Notes: 5.x / 6.0 → 7.0

1. **Upgrade to 6.0 first** (or run 6.0.3 against the code): every deprecation 7.0 enforces is reported by 6.0 with an explanation. Fix them instead of setting `"ignoreDeprecations": "6.0"` — 7.0 ignores that setting.
2. **Turn on `"stableTypeOrdering": true` in 6.0** and fix whatever changes; then 7.0 should produce identical results. (The flag can slow 6.0 by up to 25 %; it is a migration aid only.)[^8]
3. **Make implicit defaults explicit**: `strict`, `target`, `module`, `moduleResolution`, `types`, `rootDir`, `lib`. In particular:
   - add `"rootDir": "./src"` when `tsconfig.json` sits beside `src/` and `outDir` is used (TS5011 otherwise, and output paths change);
   - add `"types": ["node", …]` for global declarations (TS2591/TS2593 "Cannot find name 'process'/'describe'" otherwise);
   - declare `*.css`/asset modules or fix side-effect imports (TS2882).
4. **Replace removed options**: `baseUrl` → `paths` relative to the config; `moduleResolution: node` → `nodenext` (and add `.js` extensions — TS2835 lists the fix) or `bundler`; `outFile`/`amd`/`system`/`umd` → a bundler; `target: es5` → at least `es2015` plus a separate down-level step if ES5 output is still required; delete `downlevelIteration`, `esModuleInterop: false`, `allowSyntheticDefaultImports: false`, `alwaysStrict: false`; delete options that were already removed in 5.5 (now TS5023 "Unknown compiler option").
5. **Source fixes**: `module X {}` → `namespace X {}`; `assert { type: "json" }` → `with { type: "json" }`; `Object.setPrototypeOf` hacks in `Error` subclasses can go; review subclasses that redeclare fields (now `[[Define]]` semantics with the default target).
6. **JavaScript projects with `checkJs`**: convert constructor functions to classes, `@enum` to `as const` objects + `@typedef`, value-typed JSDoc to `typeof`, Closure types to TypeScript syntax, `arguments` to rest parameters.
7. **Type-level string utilities** that count characters change results for non-BMP characters (emoji): update their tests.
8. **Tooling**: alias 6.0 for typescript-eslint/ts-jest/Angular/Vue tooling (`"typescript": "npm:@typescript/typescript6@^6.0.2"`, plus `"@typescript/native": "npm:typescript@^7.0.2"` for `tsc`); replace ts-node; update CI scripts that grep diagnostic codes (`isolatedDeclarations` codes changed); ensure optional dependencies are installed so the native binary is present.
9. **CI**: choose `--checkers`/`--builders` for the runner size; consider `--singleThreaded` on very small runners; cache `.tsbuildinfo`.
10. **Command lines**: `tsc src/index.ts` in a directory with `tsconfig.json` now fails (TS5112); use `tsc -p` or `--ignoreConfig`.

***

#### Production / Review Checklist

##### Configuration

- [ ] `typescript` resolves to 7.0.x (or is deliberately aliased to 6.0 for API consumers with 7.0 as `tsc`)
- [ ] No removed options (`baseUrl`, `outFile`, `target: es5`, `downlevelIteration`, `moduleResolution: node|node10|classic`, `module: amd|umd|system|none`, interop flags set to `false`, `alwaysStrict: false`) and no `ignoreDeprecations`
- [ ] `strict` not disabled; `noUncheckedIndexedAccess`, `exactOptionalPropertyTypes`, `noImplicitOverride`, `noImplicitReturns`, `noFallthroughCasesInSwitch` on for new code
- [ ] `module`/`moduleResolution` match the runtime: `nodenext` for Node.js-run code, `preserve`/`bundler` only for bundled code
- [ ] `target`/`lib` no newer than the oldest supported runtime (`es2024` for Node.js 22); no `target: esnext` with decorators or `using` on Node.js; `@types/node` matches the oldest Node.js
- [ ] `types` lists needed globals; `rootDir` explicit when emitting
- [ ] `verbatimModuleSyntax` + `isolatedModules` (+ `erasableSyntaxOnly` when code runs through strip-only tools)
- [ ] Libraries: `declaration`, `isolatedDeclarations`, correct `exports` with `types` first, no `paths` in emitted code, no exported `const enum`

##### Types

- [ ] No `any` at boundaries; external data validated at runtime; no `as` on parsed data, no `as unknown as`
- [ ] Discriminated unions with exhaustive `switch` (`assertNever`/`satisfies never`)
- [ ] `satisfies` instead of annotations/assertions for config objects; `as const` where literal types matter
- [ ] Type predicates check everything they claim; assertion functions throw on every failure path
- [ ] No `!` without a visible proof; no `@ts-ignore` (use `@ts-expect-error -- reason`)
- [ ] Callback types declared with property syntax (not method shorthand) in owned APIs
- [ ] `as const` objects or literal unions instead of new `enum`s where code must be erasable

##### Async and resources

- [ ] Every promise awaited, returned, or explicitly `void`ed with `.catch`
- [ ] No `forEach(async …)`, no async callbacks where sync ones are expected
- [ ] Timeouts/`AbortSignal` on network calls; bounded concurrency
- [ ] `using`/`await using` or `try/finally` for handles, locks, subscriptions

##### Classes and runtime semantics

- [ ] Subclass field redeclarations use `declare` where base constructors assign
- [ ] `#private` where runtime privacy matters; `override` on overrides
- [ ] Decorators: one model per project (standard vs `experimentalDecorators`)

##### Tooling and CI

- [ ] CI runs `tsc --noEmit`/`tsc -b` (7.0) in addition to transpile-only test runners
- [ ] Type-aware lint rules enabled (typescript-eslint on the 6.0 alias, or oxlint `--type-aware`); `no-floating-promises`, `no-misused-promises`, `switch-exhaustiveness-check`
- [ ] `--checkers` pinned if deterministic diagnostics across machines matter
- [ ] Lockfile contains the platform packages for `typescript` 7.0.2

##### Security

- [ ] No string-built SQL/shell/HTML; parameterised APIs
- [ ] No user-controlled keys written into plain objects without allow-listing
- [ ] Secrets not logged; source maps not public unless intended

***

[^1]: Announcing TypeScript 7.0 (July 8, 2026) — defaults, removed options, `--checkers`/`--builders`/`--singleThreaded`, `@typescript/typescript6`, JavaScript differences, editor status. https://devblogs.microsoft.com/typescript/announcing-typescript-7-0/
[^2]: npm registry, `typescript` package versions and publish times (6.0.2 2026-03-23, 6.0.3 2026-04-16, 7.0.1-rc 2026-06-18, 7.0.2 2026-07-08, 7.1.0-beta 2026-10-20). https://www.npmjs.com/package/typescript?activeTab=versions
[^3]: Announcing TypeScript 6.0 (March 23, 2026) — es2025 target/lib, Temporal, getOrInsert, `#/` imports, default changes and deprecations. https://devblogs.microsoft.com/typescript/announcing-typescript-6-0/
[^4]: Announcing TypeScript 7.1 Beta (October 20, 2026). https://devblogs.microsoft.com/typescript/announcing-typescript-7-1-beta/
[^5]: typescript-eslint issue #12518 "TypeScript 7.0.2 Support" (closed as duplicate of #10940). https://github.com/typescript-eslint/typescript-eslint/issues/12518
[^6]: microsoft/typescript-go CHANGES.md — intentional differences between TypeScript 6.0 and 7.0. https://github.com/microsoft/typescript-go/blob/main/CHANGES.md
[^7]: Node.js documentation — Modules: TypeScript (type stripping, unsupported syntax, recommended tsconfig). https://nodejs.org/api/typescript.html
[^8]: Announcing TypeScript 6.0 Beta — `--stableTypeOrdering` cost and purpose, deprecation plan. https://devblogs.microsoft.com/typescript/announcing-typescript-6-0-beta/
[^9]: Oxc blog — Type-aware linting is stable (tsgolint v7 built on TypeScript 7.0.2). https://oxc.rs/blog/2026-07-22-type-aware-linting-stable
