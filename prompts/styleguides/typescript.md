### TypeScript Style Guide

> **Version**: Version-independent guide — applies to any TypeScript version. Use it when the project's TypeScript version cannot be determined; when it can, the version-specific guide for that release is more precise. The release table below covers 4.9 → 7.0. As of October 2026 the newest stable release is **TypeScript 7.0** (7.0.2, 2026-07-08), the native compiler written in Go; the last JavaScript-based compiler is **6.0** (6.0.3, 2026-04-16); **7.1** is in beta (7.1.0-beta, 2026-10-20).[^1][^2]
> **Status**: TypeScript publishes no LTS or end-of-life schedule. Fixes ship only in new releases of the current line and are not backported, so every older minor line is frozen at its last patch.[^1]
> **Scope**: Version-independent review rules for production TypeScript: compiler strictness, `any`/`unknown`, narrowing, discriminated unions, assertions, generics, enums vs unions, modules and ESM/CommonJS interop, declaration files, async code, error handling, runtime validation, immutability, utility types, `tsconfig.json` hygiene and linting. Every feature that depends on the compiler version is labelled with the release that introduced it, for example `satisfies` (4.9), `const` type parameters (5.0), `using` (5.2), `NoInfer` (5.4), `erasableSyntaxOnly` (5.8).
> **Code conventions**: Every example was compiled with `tsc` 5.9.3, 6.0.3 and 7.0.2, and with the release named in its heading or comment, and every version boundary was checked on the adjacent older release. Unless the first line of a block lists other options (`// tsconfig: {…}`), examples assume `strict: true`, `module: nodenext`, `target: es2022`, `types: ["node"]`. A line ending in `// error TSxxxx: …` fails with that diagnostic, and no other line in the block fails. Diagnostic codes are quoted from 5.9/6.0; older or newer compilers sometimes report a different code for the same problem.
> **Note**: This document is self-contained.

---

#### Table of Contents

1. Version-Aware Review
2. Code Style and Project Conventions
3. `tsconfig.json` Hygiene and Strictness Flags
4. Types: Fundamentals
5. Narrowing and Control-Flow Analysis
6. Generics and Inference
7. Enums, Unions and Erasable Syntax
8. Utility, Conditional, Mapped and Template Literal Types
9. Classes and Decorators
10. Modules, Module Resolution and ESM/CommonJS Interop
11. Declaration Files and Library Publishing
12. Async Code and Promises
13. Error Handling
14. Runtime Validation at Trust Boundaries and Security
15. Immutability and Resource Management
16. Library Typings vs. Runtime APIs
17. Testing and Type Tests
18. Linting and Tooling
19. Performance and Observability
20. Upgrading Across Versions — Migration Notes
21. Production / Review Checklist

---

#### Version-Aware Review

##### Determine the version before judging

1. Read the **compiler** version from `package.json` (`devDependencies.typescript`, an alias such as `"typescript": "npm:@typescript/typescript6@^6.0.2"`, or `"@typescript/native": "npm:typescript@^7.0.2"`) and the lockfile; monorepos may pin different versions per package. `tsc -v` prints the exact version.
2. Read the **runtime** separately (`engines.node`, `.nvmrc`, `.node-version`, the Dockerfile base image, CI matrices, `browserslist`). The compiler decides which syntax and options type-check; the runtime decides which library APIs exist. TypeScript never polyfills.
3. Read every `tsconfig*.json` in the chain (`extends`, including arrays since 5.0). The meaning of an **absent** option depends on the compiler version: since 6.0, a config without `strict` is strict, without `module` emits ES modules, and without `types` loads no `@types` globals. In 5.x the same config is loose, emits CommonJS (for the default `target: es5`) and loads every `@types` package.
4. Before recommending a feature, check that the pinned compiler supports it (table below). Before claiming that an option or API "does not exist", check it against the pinned version — many features are older than they look. Say "not available in X" only when the version table or the compiler proves it.
5. When the version cannot be determined, recommend only features available in every maintained setup (roughly 5.0+), and phrase newer ones conditionally: "if the project uses TypeScript ≥ 5.4, `NoInfer<T>` expresses this directly".

##### Release table (status as of October 2026)

| Version | First stable build | Last patch | Status | Headline changes |
|---|---|---|---|---|
| 4.9 | 4.9.3 (2022-11-15) | 4.9.5 (2023-01-30) | frozen | `satisfies`, `accessor` keyword, `in` narrowing of unlisted properties, `NaN` equality errors |
| 5.0 | 5.0.2 (2023-03-16) | 5.0.4 (2023-04-07) | frozen | standard (TC39) decorators, `const` type parameters, `moduleResolution: bundler`, `verbatimModuleSyntax`, `extends` arrays, all enums are union enums; deprecates `importsNotUsedAsValues`, `preserveValueImports`, `target: es3`, `out` and others |
| 5.1 | 5.1.3 (2023-06-01) | 5.1.6 (2023-06-28) | frozen | unrelated getter/setter types, `undefined`-returning functions need no `return` |
| 5.2 | 5.2.2 (2023-08-24) | 5.2.2 | frozen | `using` / `await using`, decorator metadata, `es2023` array copy methods (`toSorted` …) |
| 5.3 | 5.3.2 (2023-11-20) | 5.3.3 (2023-12-06) | frozen | import attributes (`with { type: "json" }`), `switch (true)` narrowing |
| 5.4 | 5.4.2 (2024-03-06) | 5.4.5 (2024-04-10) | frozen | narrowing preserved in closures, `NoInfer<T>`, `module: preserve`, `Object.groupBy`/`Map.groupBy` typings |
| 5.5 | 5.5.2 (2024-06-20) | 5.5.4 (2024-07-22) | frozen | inferred type predicates, constant indexed-access narrowing, `isolatedDeclarations`, regex syntax checking, `${configDir}`, JSDoc `@import`; options deprecated in 5.0 stop having any effect |
| 5.6 | 5.6.2 (2024-09-09) | 5.6.3 (2024-10-08) | frozen | always-truthy/never-nullish errors, iterator helper typings, `strictBuiltinIteratorReturn`, `noUncheckedSideEffectImports`, `--noCheck`; options deprecated in 5.0 become hard errors (TS5102; TS5108 for `target: es3`) |
| 5.7 | 5.7.2 (2024-11-22) | 5.7.3 (2025-01-08) | frozen | never-initialized variable checks, `rewriteRelativeImportExtensions`, `target`/`lib` `es2024`, generic `TypedArray`s |
| 5.8 | 5.8.2 (2025-02-28) | 5.8.3 (2025-04-05) | frozen | `require()` of ESM under `nodenext`, `module: node18`, `erasableSyntaxOnly`, `libReplacement`, per-branch return checks |
| 5.9 | 5.9.2 (2025-07-31) | 5.9.3 (2025-09-30) | frozen | `import defer`, `module: node20`, minimal `tsc --init`, `ArrayBuffer`/`TypedArray` lib tightening |
| 6.0 | 6.0.2 (2026-03-23) | 6.0.3 (2026-04-16) | frozen; last JavaScript-based compiler, kept for its API | new defaults (`strict`, ES modules, `types: []`, `target: es2025` …), deprecations that are errors unless `"ignoreDeprecations": "6.0"`, `es2025` target/lib, `#/` subpath imports, `--stableTypeOrdering` |
| 7.0 | 7.0.2 (2026-07-08) | 7.0.2 | **current stable** (`npm i typescript` installs it) | native Go compiler (Microsoft reports roughly 8–12× faster full builds), parallel checking; 6.0 deprecations are hard errors; no stable compiler API (tools that need one stay on 6.0) |
| 7.1 | — | 7.1.0-beta (2026-10-20) | beta only | do not recommend anything from it |

Dates are npm publish dates.[^2] Lower patch numbers in each line are beta and RC builds (for example 5.9.0-beta and 5.9.1-rc); 5.1.4 was never published, and 7.0 has no 7.0.0 package — its beta shipped only as `@typescript/native-preview`. DefinitelyTyped only supports TypeScript versions that are less than two years old; on older compilers the newest `@types/*` packages may not parse, and the project must pin them to the matching `tsX.Y` dist-tag (for example `npm i -D @types/node@ts4.9`).[^3]

##### Feature → minimum compiler version

Every row from 4.1 to 7.0 was verified by compiling a minimal sample on the listed release and on the one before it (the language and library rows on every minor release from 4.0 to 7.0): it compiles on the listed version and fails on the earlier one. Features in the first row predate 4.0 and are taken from the release notes.

| Feature | Since | Notes |
|---|---|---|
| `unknown`, optional chaining `?.`, `??`, `import type`, `#private` fields, `asserts` functions | ≤ 4.0 | available in every version this guide covers |
| variadic and labelled tuple elements, `catch (e: unknown)` | 4.0 | |
| template literal types, key remapping `as` in mapped types, `noUncheckedIndexedAccess` | 4.1 | |
| `override` keyword, `noImplicitOverride` | 4.3 | |
| `useUnknownInCatchVariables` (in `strict`), `exactOptionalPropertyTypes`, aliased-condition narrowing | 4.4 | |
| `Awaited<T>`, `type` modifiers on import names (`import { a, type B }`) | 4.5 | |
| `module`/`moduleResolution` `node16`/`nodenext`, variance annotations `in`/`out`, `extends` constraints on `infer` | 4.7 | |
| `satisfies`, `accessor`, `"x" in obj` narrowing of unlisted properties | 4.9 | |
| `const` type parameters, standard decorators, `moduleResolution: bundler`, `verbatimModuleSyntax`, `allowImportingTsExtensions`, `extends: [...]` | 5.0 | |
| `using`, `await using`, `Symbol.dispose` typings (`esnext.disposable`) | 5.2 | the down-levelled helper needs `Symbol.dispose` at run time (present in Node.js 18.20, 20.20, 22 and later) |
| import attributes `with { type: "json" }`, `switch (true)` narrowing | 5.3 | `assert { … }` is the older, now-deprecated form |
| `NoInfer<T>`, closure narrowing after last assignment, `module: preserve` | 5.4 | |
| inferred type predicates, `obj[key]` narrowing, `isolatedDeclarations`, `${configDir}` | 5.5 | |
| `noUncheckedSideEffectImports`, `strictBuiltinIteratorReturn`, `--noCheck`, always-truthy checks (TS2872) | 5.6 | |
| `rewriteRelativeImportExtensions`, `target`/`lib` `es2024` | 5.7 | |
| `erasableSyntaxOnly`, `module: node18`, `libReplacement`, `require(esm)` under `nodenext` | 5.8 | |
| `import defer`, `module: node20` | 5.9 | |
| `target`/`lib` `es2025`, `#/` subpath imports, `--stableTypeOrdering`, `--ignoreConfig`, `"types": ["*"]` | 6.0 | |
| native `tsc`, `--checkers`, `--builders`, `--singleThreaded` | 7.0 | |

##### Do not recommend features newer than the pinned compiler

The reviewer must not suggest a construct the project's compiler rejects. Typical mistakes:

- Suggesting `satisfies` (4.9), `const T` (5.0), standard decorators (5.0), `using` (5.2), `with { type: "json" }` (5.3), `NoInfer` (5.4), `erasableSyntaxOnly` (5.8) or `import defer` (5.9) to a project on an older compiler.
- Suggesting a newer `lib` API (`Object.groupBy`, `Set.prototype.union`, `Promise.withResolvers`, `Iterator.from`, `RegExp.escape`, `Temporal`) without checking **both** the compiler's lib files and the runtime (see Library Typings vs. Runtime APIs).
- Suggesting `"types": ["*"]`, `#/` imports or `"ignoreDeprecations": "6.0"` on 5.x (rejected: TS2688, TS2307 and TS5103 respectively).
- Telling a 6.0 project that a deprecated option "has been removed" — on 6.0 it is a deprecation error that `ignoreDeprecations` silences; it is removed in 7.0.
- Recommending compiler-API tools (custom transformers, `ts-patch`, `ts-node`, `ts-jest` type-checking mode, typescript-eslint type-aware rules) to run on 7.0 directly — 7.0 has no stable API, so they run on 6.0 side by side.
- Recommending anything from the 7.1 beta.

##### Deprecated and removed configuration (any version)

Verified on the adjacent releases: 5.5.4 still accepts the 5.0-deprecated options with `"ignoreDeprecations": "5.0"` (they are simply ignored — `keyofStringsOnly` no longer changes `keyof`), and 5.6.3 rejects them with TS5102 "Option '…' has been removed" (TS5108 for `target: es3`) whatever `ignoreDeprecations` says. The 5.5 release notes announced the hard error only for 6.0. 7.0 no longer knows these options at all (TS5023 "Unknown compiler option"; TS6046 for `target: es3`).

| Setting | Deprecated | Error |
|---|---|---|
| `importsNotUsedAsValues`, `preserveValueImports` | 5.0 (silenced by `"ignoreDeprecations": "5.0"`; no effect from 5.5) | 5.6 (TS5102; replacement: `verbatimModuleSyntax`) |
| `target: es3`, `out`, `charset`, `keyofStringsOnly`, `noImplicitUseStrict`, `noStrictGenericChecks`, `suppressExcessPropertyErrors`, `suppressImplicitAnyIndexErrors`, `prepend` in references | 5.0 (no effect from 5.5) | 5.6 |
| `target: es5`, `downlevelIteration`, `moduleResolution: node`/`node10`/`classic`, `module: amd`/`umd`/`system`/`none`, `baseUrl`, `outFile`, `esModuleInterop: false`, `allowSyntheticDefaultImports: false`, `alwaysStrict: false` | 6.0 (TS5101/TS5107 unless `"ignoreDeprecations": "6.0"`) | 7.0 (removed: TS5102/TS5108, TS6046 for `module: none`; `ignoreDeprecations` no longer helps) |
| import assertions `assert { type: "json" }` | 6.0 (TS2880 unless `"ignoreDeprecations": "6.0"`) | 7.0 (TS2880); under `module: nodenext` already TS2880 from 5.8 (`node20` from 5.9), and `ignoreDeprecations` does not silence it there |
| `module Foo { }` for namespaces | — | 6.0 (TS1540, not silenced by `ignoreDeprecations`) |

***

#### Code Style and Project Conventions

##### Naming and formatting

- `camelCase` for variables, functions and methods; `PascalCase` for types, interfaces, classes, enums and enum members; `SCREAMING_SNAKE_CASE` only for true module-level constants.
- Type parameters: `T`, `K`, `V` for simple generics, descriptive names (`TItem` or `Item`) when there are several or the meaning is not obvious.
- No `I` prefix on interfaces (`IUser`) and no `Type`/`T` suffixes.
- File names follow the project's convention (`kebab-case.ts` is common); one exported concept per file where practical.
- Formatting is a tool's job (Prettier, Biome, dprint). Do not review whitespace or hand-align code.

##### Exports and imports

```ts
// Named exports (preferred): refactor-safe, consistent names at import sites, tree-shakeable
export interface UserInput { name: string }
export function createUser(input: UserInput): { id: string; name: string } {
  return { id: crypto.randomUUID(), name: input.name };
}
export const DEFAULT_PAGE_SIZE = 50;
```

- Avoid `export default` in application code: each importer picks its own name, renames do not propagate, and CommonJS interop of default exports is error-prone. Framework conventions (Next.js pages, Storybook stories, tool config files) are the exception.
- Barrel files (`index.ts` re-exporting everything) slow down type checking, test start-up and bundling and create import cycles. Use them only for a package's public entry point.
- Group imports: Node.js built-ins (with the `node:` prefix), external packages, internal aliases, relative imports. Use `import type` / `type` modifiers (4.5+) for type-only imports.
- Keep modules free of top-level side effects (opening connections, starting timers, reading the environment) except in entry points; they make tests and lazy loading unpredictable.

##### Project layout

```text
src/
├── domain/        # types and pure logic, no I/O
├── services/      # business logic
├── adapters/      # HTTP, DB, queues — runtime validation at these boundaries
├── config.ts      # environment parsing and validation, once at start-up
└── index.ts       # entry point / public API
```

##### `interface` vs `type`

| Use case | Prefer |
|---|---|
| Object shapes, class contracts, public library APIs | `interface` — extendable, mergeable, clearer errors, cached by the checker |
| Unions, tuples, function types, mapped and conditional types, primitives aliases | `type` |
| Augmenting a third-party or global type | `interface` (declaration merging) |

`interface A extends B` reports incompatible members; an intersection `B & { … }` silently produces `never` for conflicting properties:

```ts
type Broken = { id: string } & { id: number };
const x: Broken = { id: "a" }; // error TS2322: Type 'string' is not assignable to type 'never'.

interface Base { id: string }
interface Child extends Base { id: number } // error TS2430: Interface 'Child' incorrectly extends interface 'Base'.
export { x };
export type { Child };
```

Declaration merging is a feature for augmentation, but in application code an accidental second `interface User` in the same scope silently adds members. Treat duplicate interface names in one module as a bug.

***
#### `tsconfig.json` Hygiene and Strictness Flags

##### `strict` is the baseline

`strict: true` enables the whole strictness family. New members join the family in later releases, so upgrading the compiler can add errors under an unchanged config — that is intended.

| Flag in the `strict` family | Since | What it catches |
|---|---|---|
| `noImplicitAny` | < 4.0 | parameters and variables silently typed `any` |
| `strictNullChecks` | < 4.0 | `null`/`undefined` assignable to every type |
| `noImplicitThis`, `alwaysStrict` | < 4.0 | untyped `this`; files emitted without `"use strict"` |
| `strictFunctionTypes` | < 4.0 | bivariant function-parameter checks (property-style members only) |
| `strictPropertyInitialization` | < 4.0 | class fields never assigned in the constructor |
| `strictBindCallApply` | < 4.0 | untyped `bind`/`call`/`apply` |
| `useUnknownInCatchVariables` | 4.4 | `catch (e)` is `unknown` instead of `any` |
| `strictBuiltinIteratorReturn` | 5.6 | built-in iterators return `undefined`, not `any`, when done |

Rules:

- Every tsconfig sets `"strict": true` explicitly — before 6.0 the default is `false`, from 6.0 it is `true`; writing it makes the intent independent of the compiler version.
- Disabling individual strict flags (`"strictNullChecks": false`) in application code is a review finding, not a style choice. When a legacy codebase must migrate gradually, prefer per-directory configs or `// @ts-expect-error` with tickets over turning checks off globally.
- Do not list strict-family flags individually next to `"strict": true` (as some older templates do): it is redundant, and a later `false` override hides in the noise.

##### Flags worth enabling beyond `strict`

| Option | Since | What it catches |
|---|---|---|
| `noUncheckedIndexedAccess` | 4.1 | `arr[i]` and `record[key]` are `T \| undefined` |
| `exactOptionalPropertyTypes` | 4.4 | `{ a?: string }` no longer accepts an explicit `a: undefined` |
| `noImplicitOverride` | 4.3 | overriding methods must say `override` |
| `noPropertyAccessFromIndexSignature` | 4.2 | forces `obj["key"]` for index-signature access, so typos in dotted names stand out |
| `noImplicitReturns`, `noFallthroughCasesInSwitch` | < 4.0 | missing `return` on some paths, fall-through `case`s |
| `noUnusedLocals`, `noUnusedParameters` | < 4.0 | dead code (often better left to the linter) |
| `allowUnreachableCode: false`, `allowUnusedLabels: false` | < 4.0 | dead code and stray labels become errors instead of suggestions |
| `verbatimModuleSyntax` | 5.0 | type-only imports must say `type`; ESM syntax rejected where CommonJS is emitted |
| `isolatedModules` | < 4.0 | constructs that single-file transpilers (esbuild, SWC, Babel) cannot compile |
| `isolatedDeclarations` | 5.5 | exported API without explicit types (libraries) |
| `noUncheckedSideEffectImports` | 5.6 (default `true` from 6.0) | `import "./x"` that does not resolve |
| `erasableSyntaxOnly` | 5.8 | enums, value namespaces, parameter properties, `import =` (anything type stripping cannot erase) |
| `skipLibCheck` | < 4.0 | speeds up checking by skipping `.d.ts` files — also skips your own `.d.ts` files |

##### Set the options whose defaults changed

Explicit values make a config mean the same thing on every compiler. The defaults below changed in 6.0 and stay changed in 7.0:[^4][^5]

| Option | Default before 6.0 | Default in 6.0 / 7.0 | Rule |
|---|---|---|---|
| `strict` | `false` | `true` | always set |
| `target` | `es5` | `es2025` (newest supported year; floats with the compiler) | set to the oldest runtime's year |
| `module` | `commonjs` (for `target` es3/es5), else `es2015` | ES modules | always set |
| `moduleResolution` | `node10`/`classic` (depends on `module`) | `bundler` unless `module` is `node16`/`node18`/`node20`/`nodenext` | set, or use a `module` that implies it |
| `types` | every package in `node_modules/@types` | `[]` | list the globals you need (`["node"]`) |
| `rootDir` | common directory of all inputs | directory containing `tsconfig.json` | set when `outDir` is set |
| `noUncheckedSideEffectImports` | `false` | `true` | declare asset modules instead of disabling it |
| `libReplacement` (5.8+) | `true` | `false` | set `true` only when a `@typescript/lib-*` replacement package is used |
| `esModuleInterop` / `allowSyntheticDefaultImports` | `false` | `true` (and `false` is deprecated) | do not set `false` |

A config with only `outDir: "dist"` and sources in `src/` emitted `dist/index.js` before 6.0 and emits `dist/src/index.js` (with error TS5011 asking for an explicit `rootDir`) on 6.0 and 7.0. A config without `module` emitted CommonJS before 6.0 and emits ES modules from 6.0 — **with no compile error**, but Node.js then fails at run time on extensionless relative imports. Review rule: every tsconfig that emits code sets `module`, `target`, `rootDir` and `outDir` explicitly.

##### Recommended profiles

Node.js application or library, emitted by `tsc` (5.0+; replace `node20` by `nodenext` on 5.8 and older, where `node20` does not exist):

```json
{
  "compilerOptions": {
    "module": "node20",
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
    "skipLibCheck": true,
    "sourceMap": true,
    "declaration": true
  },
  "include": ["src"]
}
```

- `module: nodenext` (4.7+) / `node20` (5.9+) implies the matching `moduleResolution`, makes the output format follow `package.json` `"type"` and the `.mts`/`.cts` extensions, and enforces Node.js ESM rules. `nodenext` floats with the newest Node.js semantics on every compiler upgrade; `node20` is a fixed target.
- Pick `target`/`lib` from the **oldest supported runtime**: `es2022` for Node.js 18, `es2023` for Node.js 20, `es2024` (5.7+) for Node.js 22, `es2025` (6.0+) for Node.js 24. Verify individual APIs anyway (see Library Typings vs. Runtime APIs).
- `lib: ["es2023"]` needs TypeScript 5.0+; on 4.9 use `es2022`.

Bundled application (Vite, esbuild, webpack, Rspack, Bun) — 5.4+ for `module: preserve`; on 5.0–5.3 use `"module": "esnext"` with `"moduleResolution": "bundler"`:

```json
{
  "compilerOptions": {
    "module": "preserve",
    "moduleResolution": "bundler",
    "target": "es2022",
    "lib": ["es2023", "dom", "dom.iterable"],
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

- `allowImportingTsExtensions` requires `noEmit` or `emitDeclarationOnly` (or `rewriteRelativeImportExtensions`, 5.7+).
- From 6.0, `dom` already contains `dom.iterable` and `dom.asynciterable`; listing them stays harmless.
- Add the bundler's ambient types (`"types": ["vite/client"]`) so asset imports and `import.meta.env` type-check.

##### Other hygiene rules

- Separate configs for build (`tsconfig.build.json`, `include: ["src"]`) and for editor/test/lint (`tsconfig.json` including tests and scripts). Every TypeScript file in the repository must belong to some project, or it is never checked.
- `extends` accepts an array since 5.0 (later entries win). Shared bases (`@tsconfig/node22`, `@tsconfig/strictest`) are fine; review what they set, especially `module` and `target`.
- `include`/`files`/`exclude` and `paths` in an extended config are resolved relative to the config that **declares** them; `${configDir}` (5.5+) makes them resolve relative to the extending config.
- `incremental`/`composite` with project references (`tsc -b`) for large repositories; `.tsbuildinfo` files belong in `.gitignore`.
- `ignoreDeprecations` is a temporary bridge with a removal ticket, never a permanent setting. `"ignoreDeprecations": "5.0"` silences the 5.0 deprecations on 5.0–5.5 (the options do nothing from 5.5 on and are hard errors from 5.6); `"6.0"` is accepted from 6.0 (5.9: TS5103 invalid value) and silences nothing on 7.0.
- `tsc file.ts` ignores `tsconfig.json`; since 6.0 it is error TS5112 when a `tsconfig.json` exists in the directory. Pre-commit hooks that run `tsc --noEmit $FILES` therefore check with default options (or break on 6.0+); use `tsc -p . --noEmit`.

***

#### Types: Fundamentals

Truths that hold in every version:

- **Types are erased.** Annotations, interfaces, type aliases and generics produce no JavaScript. Nothing is validated at run time unless code does it.
- **Typing is structural.** Two types with the same shape are interchangeable regardless of name. Nominal behaviour needs brands or `#private` fields.
- **The compiler does not polyfill.** `lib` describes what the runtime is assumed to have; `target` only controls syntax down-levelling.
- **Soundness is not a goal.** Assertions, `any`, bivariant method parameters, unchecked index access and array covariance are deliberate holes. Reviewers must know where they are.

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
  return typeof v === "object" && v !== null && "port" in v && typeof (v as { port: unknown }).port === "number";
}
const raw = parse('{"port":8080}');
if (isConfig(raw)) {
  console.log(raw.port.toFixed());
}
raw.port; // error TS18046: 'raw' is of type 'unknown'.
```

- `any` is contagious: `JSON.parse`, `Response.json()`, `Body.json()` in many typings, `require()` in JS, untyped third-party code and `catch` variables without `useUnknownInCatchVariables` all return `any`, and every property access, call and assignment on it is unchecked. Assign such results to `unknown` immediately.
- Acceptable `any`: in generic constraints where variance makes `unknown` wrong (`(...args: any[]) => unknown`), and in short-lived migration code with a lint suppression and a reason.
- `Function`, `Object` and `{}` as parameter types are near-`any` too. See below.

##### Type assertions and non-null assertions

`x as T` and `x!` are unchecked promises to the compiler. Review each one:

```ts
interface User { id: string; name: string }

const u1 = {} as User;               // compiles; u1.name is undefined at run time
const u2 = "x" as unknown as User;   // double assertion: always a smell

declare const map: Map<string, User>;
const a = map.get("id")!.name;       // throws if absent — prefer an explicit check
const b = map.get("id")?.name ?? "anonymous";

const n = "42" as number; // error TS2352: Conversion of type 'string' to type 'number' may be a mistake because neither type sufficiently overlaps with the other. If this was intentional, convert the expression to 'unknown' first.
export { u1, u2, a, b, n };
```

- Prefer, in this order: a type annotation, narrowing, `satisfies` (4.9+), a validating parser. Assertions are acceptable for values the code itself just produced with a known shape (DOM lookups the code owns, `as const`, brand constructors after validation).
- Never use angle-bracket assertions (`<User>x`): they are a syntax error in `.tsx` and rejected by `erasableSyntaxOnly` (5.8+).
- `!` on a class field (`name!: string`) opts out of `strictPropertyInitialization` — acceptable only when a framework provably assigns it (dependency injection, ORM hydration).
- `as const` is not an assertion in this sense: it narrows literals and adds `readonly`, and never lies.

##### Optional properties and `exactOptionalPropertyTypes` (4.4+)

```ts
// tsconfig: {"exactOptionalPropertyTypes": true}
interface Options { timeoutMs?: number }

const o1: Options = {};                       // OK: property absent
const o2: Options = { timeoutMs: undefined }; // error TS2375: Type '{ timeoutMs: undefined; }' is not assignable to type 'Options' with 'exactOptionalPropertyTypes: true'. Consider adding 'undefined' to the types of the target's properties.

interface Patch { timeoutMs?: number | undefined } // allow undefined explicitly when "unset" must be expressible
const p: Patch = { timeoutMs: undefined };
export { o1, o2, p };
```

`{ ...defaults, ...overrides }` with an explicit `undefined` in `overrides` overwrites the default — a common configuration bug that this flag surfaces. `"key" in obj` and `obj.key !== undefined` are different checks under it.

##### Index access and `noUncheckedIndexedAccess` (4.1+)

```ts
// tsconfig: {"noUncheckedIndexedAccess": true}
const ports: number[] = [80, 443];
const first = ports[0];          // number | undefined
first.toFixed();                 // error TS18048: 'first' is possibly 'undefined'.

const env: Record<string, string> = { HOME: "/root" };
const home = env["HOME"] ?? "/";  // string

for (const p of ports) p.toFixed();               // iteration is not affected
const [x = 0, y = 0] = ports;                      // destructuring defaults
export { home, x, y };
```

Without the flag, `ports[5]` is typed `number` and `record[key]` is never `undefined`. When reviewing a project without it, treat every index access on arrays, records and `Map`-like objects as potentially missing.

##### Excess property checks and structural typing

```ts
interface Point { x: number; y: number }

const p1: Point = { x: 1, y: 2, z: 3 }; // error TS2353: Object literal may only specify known properties, and 'z' does not exist in type 'Point'.

const tmp = { x: 1, y: 2, z: 3 };
const p2: Point = tmp;                   // OK: excess checks apply only to fresh object literals

function save(p: Point): string { return JSON.stringify(p); }
save(tmp);                               // serialises z too
export { p1, p2 };
```

A structural type does not strip extra properties. When an object is serialised, sent to a client or written to a database, copy the allowed fields explicitly or parse it through a strict schema — otherwise internal fields (password hashes, internal IDs) leak.

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

Use brands for identifiers, validated strings (emails, URLs, sanitized HTML) and units (milliseconds vs. seconds) where mixing values is a real bug.

##### `object`, `{}`, `Object`, `Function`

- `object` — any non-primitive. Use it for "some object".
- `{}` — any value except `null`/`undefined`, including strings and numbers. Rarely what is meant.
- `Object` — the wrapper interface; never use it as a type.
- `Function` — any callable, called with unchecked arguments and returning `any`. Use a precise signature (`(...args: never[]) => unknown` accepts any function safely).
- `Record<string, unknown>` — a plain dictionary whose values must be checked.

##### Functions

```ts
// Overloads: the implementation signature is not callable from outside
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

// A callback returning a value where void is expected is allowed — a source of ignored promises
const handlers: Array<() => void> = [];
handlers.push(async () => { await Promise.resolve(); }); // compiles; lint with no-misused-promises
```

- Prefer union parameters to overloads when the return type does not depend on the argument type.
- Prefer an options object to long positional parameter lists, especially booleans (`render(true, false)`).
- Method-shorthand members (`m(x: T): void`) are checked **bivariantly** even under `strictFunctionTypes`; property-style members (`m: (x: T) => void`) are checked contravariantly. Use property style where variance matters (callbacks, event handlers):

```ts
interface Animal { name: string }
interface Dog extends Animal { bark(): void }

interface MethodStyle { handle(a: Animal): void }
interface PropertyStyle { handle: (a: Animal) => void }

const dogOnly = (d: Dog) => d.bark();
const m: MethodStyle = { handle: dogOnly };   // compiles (bivariant) — crashes when called with a cat
const p: PropertyStyle = { handle: dogOnly }; // error TS2322: Type '(d: Dog) => void' is not assignable to type '(a: Animal) => void'.
export { m, p };
```

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

- Model states as a union with a literal discriminant (`kind`, `type`, `status`) instead of one object with many optional fields (`{ loading?: boolean; data?: T; error?: Error }`), which permits impossible combinations.
- Every `switch` over a union ends in a `never` check (or `s satisfies never`, 4.9+). A `default:` that silently returns a fallback hides new variants — flag it.
- Destructuring the discriminant into a local keeps narrowing since 4.4 (`const { kind } = s; if (kind === "circle") s.radius`), and destructured discriminated parameters narrow together since 4.6.
- The typescript-eslint rule `switch-exhaustiveness-check` enforces exhaustiveness without a `never` variable.

##### Built-in narrowing

```ts
function describe(v: string | number | Date | null | undefined | string[]): string {
  if (v == null) return "missing";                 // null and undefined
  if (typeof v === "string") return v.toUpperCase();
  if (typeof v === "number") return v.toFixed(2);
  if (v instanceof Date) return v.toISOString();
  return v.join(",");                               // string[]
}
export { describe };
```

- `typeof x === "object"` includes `null`: check `x !== null` as well.
- `Array.isArray` narrows `unknown` to `any[]` — a hidden `any`. Check element types after it.
- `instanceof` fails across realms (iframes, `vm` contexts, duplicated package copies in `node_modules`). Prefer discriminants or brand checks for values that may cross such boundaries.
- `"prop" in obj` narrows unions by property presence; since 4.9 it also narrows an object type that does not declare `prop` to `obj & Record<"prop", unknown>`:

```ts
// 4.9+: compiles; 4.8: error TS2339: Property 'id' does not exist on type 'object'.
export function idOf(x: object): unknown {
  if ("id" in x) return x.id;
  return undefined;
}
```

##### Truthiness narrowing excludes valid values

```ts
function setPort(port: number | undefined): number {
  if (port) return port;        // BAD: port 0 falls through
  return 8080;
}
function setPortGood(port: number | undefined): number {
  return port ?? 8080;          // GOOD: only undefined/null fall back
}
function label(name: string | undefined): string {
  return name || "anonymous";   // BAD if "" is a valid, deliberate value
}
export { setPort, setPortGood, label };
```

`0`, `""`, `NaN`, `0n` and `false` are falsy. Use `!== undefined`, `!= null` or `??` when zero or empty are valid values. `||` for defaults is a bug whenever the left side is a number, string or boolean that may legitimately be falsy.

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

- A type guard is an unchecked claim: the compiler does not verify that the body proves `v is T`. A guard that checks fewer fields than the type declares (`"id" in v` for a 12-field type) is a lie that corrupts every caller. Prefer schema validators at trust boundaries.
- `x is T` with a `false` result narrows to "not `T`" in the else branch. A guard like `isNonEmptyString` above therefore makes the compiler believe that an empty string is "not a string" in the else branch. Keep guards exact, or name them precisely.
- Assertion functions must be called through an explicitly typed reference: `const check = assertIsDefined;` without an annotation produces TS2775.

##### Inferred type predicates (5.5+)

```ts
// 5.5+: compiles; 5.4: error TS2322: Type '(string | undefined)[]' is not assignable to type 'string[]'.
const raw: Array<string | undefined> = ["a", undefined, "b"];
export const names: string[] = raw.filter((s) => s !== undefined);
```

Before 5.5, write the predicate explicitly: `raw.filter((s): s is string => s !== undefined)`. No predicate is inferred for `(s) => !!s`, because it also removes `""` — the result stays `(string | undefined)[]`.

##### Narrowing in closures, after indexed access, and in `switch (true)`

```ts
// 5.4+: narrowing survives into closures created after the last assignment
// (5.3: error TS2339: Property 'searchParams' does not exist on type 'string | URL'.)
export function late(url: string | URL, names: Map<string, string>) {
  if (typeof url === "string") {
    url = new URL(url);
  }
  return [...names.keys()].map((k) => url.searchParams.get(k));
}
```

```ts
// 5.5+: obj[key] narrows when neither obj nor key is reassigned
// (5.4: error TS2571: Object is of type 'unknown'.)
export function byKey(obj: Record<string, unknown>, key: string) {
  if (typeof obj[key] === "string") {
    return obj[key].toUpperCase();
  }
  return undefined;
}
```

```ts
// 5.3+: switch (true) narrows in each case clause (5.2: error TS18046: 'v' is of type 'unknown'.)
export function classify(v: unknown): string {
  switch (true) {
    case typeof v === "string":
      return v.toUpperCase();
    default:
      return "other";
  }
}
```

On older compilers, copy into a `const` before the closure (`const u = url;`) and narrow a local (`const value = obj[key]; if (typeof value === "string") …`). Rewriting such code is not a finding on new compilers, and the newer form is not available on old ones.

##### Narrowing is not invalidated by calls

```ts
interface Job { state: "idle" | "running" }
function start(job: Job, run: () => void) {
  if (job.state === "idle") {
    run();                       // may set job.state = "running"
    if (job.state === "running") { // error TS2367: This comparison appears to be unintentional because the types '"idle"' and '"running"' have no overlap.
      return;
    }
  }
}
export { start };
```

TypeScript assumes function calls do not mutate narrowed properties. After a call that may mutate shared state (callbacks, `await`, event handlers), re-read the value into a fresh variable (`const state: Job["state"] = job.state`) instead of trusting the earlier narrowing. The same applies across `await`: another task may have changed the object.

##### Checks newer compilers add

Since 5.6, conditions that are always truthy and `??` operands that can never be nullish are errors; since 5.7, variables never initialised before use in nested functions are reported; since 5.8, each branch of a conditional `return` is checked separately against the return type:

```ts
// 5.6+ (5.5 compiles this silently)
declare const input: string;
if (/^\d+$/) { }                                  // error TS2872: This kind of expression is always truthy.
export const big = input.length < 3 ?? true;      // error TS2869: Right operand of ?? is unreachable because the left operand is never nullish.
```

These checks are syntactic: `Number(input) ?? 80` is not flagged, although `Number()` never returns a nullish value (`Number("")` is `0`, `Number("x")` is `NaN`) — reviewers must catch that pattern themselves on any version.

***

#### Generics and Inference

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

// BAD: T appears only in the return type — a disguised assertion
function parseBad<T>(json: string): T {
  return JSON.parse(json); // the caller picks T, nothing checks it
}
const user = parseBad<{ admin: boolean }>('{"admin":"no"}'); // "no" is truthy at run time

export { firstOf, byId, user };
export type { Page };
```

- A type parameter used once (only in the return type, or only in one parameter) adds nothing; write the concrete type or `unknown`.
- Constrain as loosely as the body requires (`T extends { id: string }`, not `T extends User`).
- Generic `fetchJson<T>(url): Promise<T>` helpers are unchecked casts. Accept a schema/parser instead: `fetchJson(url, UserSchema)`.

##### `satisfies` (4.9+)

`satisfies` checks that an expression matches a type **without** widening the expression's own type:

```ts
// 4.9+ (4.8: error TS1005: ',' expected.)
type Route = { path: string; method: "GET" | "POST" };

const routes = {
  list: { path: "/users", method: "GET" },
  create: { path: "/users", method: "POST" },
} satisfies Record<string, Route>;

routes.list.method;      // "GET" (literal kept)
routes.missing;          // error TS2339: Property 'missing' does not exist on type '{ list: { path: string; method: "GET"; }; create: { path: string; method: "POST"; }; }'.

const annotated: Record<string, Route> = routes;
annotated.missing;       // compiles: the annotation widened the type to "any key"

export const bad = { path: "/x", method: "PUT" } satisfies Route; // error TS2322: Type '"PUT"' is not assignable to type '"GET" | "POST"'.
```

Use `as const satisfies Config` for immutable configuration tables. Before 4.9, the closest equivalent is an identity helper: `const defineRoutes = <T extends Record<string, Route>>(r: T) => r;`.

##### `const` type parameters (5.0+)

```ts
// 5.0+ (4.9: syntax errors, starting with TS1139: Type parameter declaration expected.)
function tuple<const T extends readonly unknown[]>(...xs: T): T {
  return xs;
}
export const t = tuple("a", 1, true); // readonly ["a", 1, true] — no `as const` at the call site
```

`const` only affects literals written directly at the call; an array stored in a variable first is already widened. Before 5.0, callers write `as const`.

##### `NoInfer<T>` (5.4+)

```ts
// 5.4+ (5.3: error TS2304: Cannot find name 'NoInfer'.)
function createFsm<S extends string>(states: S[], initial: NoInfer<S>): { states: S[]; current: S } {
  return { states, current: initial };
}
createFsm(["idle", "running"], "idle");
createFsm(["idle", "running"], "stopped"); // error TS2345: Argument of type '"stopped"' is not assignable to parameter of type '"idle" | "running"'.
```

Without `NoInfer`, `"stopped"` becomes an inference candidate and `S` silently widens to include it. Before 5.4 the usual workaround is a second type parameter: `<S extends string, I extends S>(states: S[], initial: I)`.

##### Variance annotations (4.7+) and inference changes

- `interface Producer<out T>` / `interface Consumer<in T>` document and speed up variance checks for complex generic types. Never add them to silence an error — a wrong annotation makes unsound assignments compile.
- Inference changes between minor releases are normal (TypeScript does not follow semantic versioning). When an upgrade produces errors in generic calls far from the edited code, add an explicit type argument or annotate the callback parameter rather than casting.

***
#### Enums, Unions and Erasable Syntax

##### Prefer union types and `as const` objects

```ts
// GOOD: erasable, tree-shakeable, works with every transpiler and with type stripping
const Status = {
  Pending: "pending",
  Active: "active",
  Closed: "closed",
} as const;
type Status = (typeof Status)[keyof typeof Status]; // "pending" | "active" | "closed"

function isOpen(s: Status): boolean {
  return s !== Status.Closed;
}

const fromApi: string = "active";
const ok = (Object.values(Status) as readonly string[]).includes(fromApi); // runtime membership check
export { isOpen, ok };
```

A plain string-literal union (`type Status = "pending" | "active"`) is enough when no runtime list of values is needed.

##### If enums are used

```ts
enum Color { Red, Green }
declare const fromApi: number;
const c: Color = fromApi;           // compiles: any `number` is assignable to a numeric enum
const name = Color[Color.Red];      // reverse mapping exists only for numeric members

enum Mode { Read = "r", Write = "w" }
const m: Mode = "r";                // error TS2322: Type '"r"' is not assignable to type 'Mode'.

enum Flags { None = 0, A = 1 << 0, B = 1 << 1, AB = A | B } // bit flags: the remaining good use of numeric enums
export { c, name, m, Flags };
```

- Numeric enums are not closed. Since 5.0 an out-of-range **literal** is rejected (`const x: Color = 42` is TS2322; 4.9 accepted it), but any computed `number` from JSON, a database or arithmetic is accepted on every version. A `switch` over an enum still needs a `default` that throws, and external values must be validated.
- String enums are nominal: plain strings are not assignable, so parsing external input needs an explicit mapping.
- `const enum` is inlined by `tsc` but cannot be compiled by single-file transpilers (esbuild, SWC, Babel, Node.js type stripping) when it is declared in another file; under `isolatedModules`, using an ambient `const enum` is TS2748. Never publish `const enum`s in a library's `.d.ts`.
- Enums emit runtime code (an IIFE), are rejected by `erasableSyntaxOnly` (5.8+) and by Node.js type stripping. Existing enums are not a defect by themselves; new ones in a codebase that runs through type stripping are.

##### `erasableSyntaxOnly` (5.8+)

```ts
// tsconfig: {"erasableSyntaxOnly": true}  (5.8+; 5.7: error TS5023: Unknown compiler option 'erasableSyntaxOnly'.)
enum Dir { Up, Down }                                         // error TS1294: This syntax is not allowed when 'erasableSyntaxOnly' is enabled.
namespace Runtime { export const v = 1; }                     // error TS1294: This syntax is not allowed when 'erasableSyntaxOnly' is enabled.
export class Repo { constructor(private readonly url: string) {} } // error TS1294: This syntax is not allowed when 'erasableSyntaxOnly' is enabled.
export { Dir, Runtime };
```

TypeScript-only constructs with runtime semantics — enums, namespaces containing values, parameter properties, `import x = require()` / `export =` in ES modules, angle-bracket assertions — cannot be removed by replacing types with whitespace. Erasable alternatives: `as const` objects, ES modules, explicit fields assigned in the constructor, `import`/`createRequire`. Enable the flag whenever code runs through Node.js type stripping, `ts-blank-space`, or other strip-only tools.

##### Namespaces

ES modules replace namespaces for organising code. Legitimate remaining uses: type-only namespaces, `declare namespace` in `.d.ts` files, and merging with functions or classes in declarations. `module Foo { }` as a namespace keyword is an error since 6.0 (TS1540: use `namespace`); `declare module "pkg" { }` (ambient module declarations and augmentation) is unaffected.

***

#### Utility, Conditional, Mapped and Template Literal Types

##### Built-in utility types

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
type Complete = Required<UserPatch>;
type Frozen = Readonly<User>;
type Ctor = ConstructorParameters<typeof URL>;      // [url: string | URL, base?: string | URL]
type Loaded = Awaited<Promise<Promise<User>>>;     // User (Awaited: 4.5+)
type Fetch = ReturnType<typeof fetch>;              // Promise<Response>
type Args = Parameters<(a: string, b: number) => void>; // [a: string, b: number]
type Present = NonNullable<string | null | undefined>;  // string
type Kinds = Exclude<"a" | "b" | "c", "c">;         // "a" | "b"
type Upper = Uppercase<"get">;                      // "GET"
export type { UserPatch, PublicUser, UserIndex, Complete, Frozen, Ctor, Loaded, Fetch, Args, Present, Kinds, Upper };
```

- `Omit<T, K>` does not check that `K` exists in `T` (`Omit<User, "pasword">` silently omits nothing). For security-relevant projections (removing secrets), use `Pick` with an allow-list.
- `Record<string, T>` claims every key exists; reading `rec[key]` returns `T` (not `T | undefined`) without `noUncheckedIndexedAccess`. For dynamic keys prefer `Map<string, T>` or `Partial<Record<string, T>>`.
- `Partial<T>` for "update" payloads also allows `{}` — validate that at least one field is present when that matters.
- `ReturnType<typeof fn>` couples a public type to an implementation detail; export an explicit type for APIs.

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

// `infer … extends` constraint: 4.7+
type FirstArg<F> = F extends (first: infer P extends string, ...rest: never[]) => unknown ? P : never;
type F1 = FirstArg<(id: "x") => void>;        // "x"
export type { A, D, N, F1 };
```

Deeply recursive conditional types slow the checker and can hit TS2589 ("Type instantiation is excessively deep and possibly infinite"). In review, question type-level cleverness that a simpler explicit type would cover; types are code that colleagues must maintain.

##### Mapped types and key remapping (4.1+)

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
type RequiredBy<T, K extends keyof T> = Omit<T, K> & Required<Pick<T, K>>;
export type { G, Mutable, DeepReadonly, PartialBy, RequiredBy };
```

A naive `DeepReadonly<T> = { readonly [P in keyof T]: T[P] extends object ? DeepReadonly<T[P]> : T[P] }` also maps over functions, `Date` and `Map`, producing types whose methods are lost or unusable; exclude functions (as above) and treat built-in classes explicitly when needed.

##### Template literal types (4.1+)

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

Template-literal inference over astral characters (emoji) splits UTF-16 surrogate pairs up to 6.0; 7.0 consumes whole code points. Do not write type-level string utilities that depend on either behaviour.

##### `keyof`, `typeof`, indexed access

```ts
const LEVELS = { debug: 10, info: 20, warn: 30, error: 40 } as const;
type Level = keyof typeof LEVELS;                  // "debug" | "info" | "warn" | "error"
type LevelValue = (typeof LEVELS)[Level];          // 10 | 20 | 30 | 40

function threshold(l: Level): LevelValue {
  return LEVELS[l];
}

type Dict = { [k: string]: unknown };
type DK = keyof Dict;                               // string | number
export { threshold };
export type { DK };
```

`Object.keys(obj)` returns `string[]`, not `(keyof T)[]`, because objects may carry extra properties at run time (structural typing). Casting the result to `(keyof T)[]` is only safe for objects the code created itself; iterate with `Object.entries` over validated data or keep keys in a `const` tuple.

***

#### Classes and Decorators

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
  override deposit(amount: number): void { // `override` (4.3+), required under noImplicitOverride
    super.deposit(amount);
  }
}
export { Savings };
```

- Prefer `#private` for real encapsulation; `private` is only a compile-time check (`(obj as any).legacyFlag` works, and `JSON.stringify` serialises it).
- Under `strictPropertyInitialization`, every field needs an initializer, a constructor assignment, or a justified `!`.
- Prefer plain functions and modules to classes with only static members.

##### Class fields and initialisation order

Subclass field initialisers run **after** the base constructor returns, on every target. A base constructor that calls an overridden method sees uninitialised subclass fields:

```ts
class Base {
  constructor() {
    this.init();
  }
  init(): void {}
}
class Child extends Base {
  items: string[] = [];
  override init(): void {
    this.items.push("x"); // runs inside Base's constructor, before `items` exists → TypeError at run time
  }
}
export { Child };
```

This compiles without any diagnostic and throws `TypeError: Cannot read properties of undefined (reading 'push')` on `new Child()` (verified with `target` es2020 and es2022). Never call overridable methods from a base constructor.

With `target` ES2022 or newer, `useDefineForClassFields` defaults to `true`: fields use JavaScript's `[[Define]]` semantics. A field that is only re-declared in a subclass then **resets** the value the base constructor assigned:

```ts
class Base {
  name: string;
  constructor() {
    this.name = "base";
  }
}
class Child extends Base {
  name!: string; // error TS2612: Property 'name' will overwrite the base property in 'Base'. If this is intentional, add an initializer. Otherwise, add a 'declare' modifier or remove the redundant declaration.
}
class Fixed extends Base {
  declare name: string; // re-types without emitting a field
}
export { Child, Fixed };
```

Emitted anyway (no `noEmitOnError`), `new Child().name` is `undefined` with `target: es2022` and `"base"` with `target: es2020` (verified on Node.js 24). Raising `target` to ES2022+ is therefore a behaviour change for class hierarchies and for libraries that rely on property setters being triggered by field initialisers; review such diffs carefully.

##### Decorators: two incompatible systems

| | Legacy (`experimentalDecorators`) | Standard (TC39, 5.0+) |
|---|---|---|
| Enabled by | `"experimentalDecorators": true` | default when the flag is off |
| Parameter decorators (`constructor(@Inject() x)`) | yes | no (TS1206) |
| `emitDecoratorMetadata` (reflect-metadata types) | yes | no |
| Signature | `(target, key, descriptor)` | `(value, context)` with `ClassMethodDecoratorContext` etc. |
| Used by | Angular, NestJS, TypeORM, InversifyJS, class-validator | new code without framework constraints |

```ts
// Standard decorator, 5.0+ (4.9: error TS2304: Cannot find name 'ClassMethodDecoratorContext'.)
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
}
console.log(new Service().greet("x"));
```

- Do not mix the systems: the flag switches every decorator in the program. Frameworks that need legacy decorators keep the flag; neither flag is deprecated in 6.0 or 7.0.
- No Node.js release up to 26 parses decorator syntax natively. With `target: esnext`, `tsc` leaves `@dec` in the output, which then fails to parse; lower targets down-level decorators. Node.js type stripping rejects decorators.
- Decorator metadata (5.2+) needs `Symbol.metadata` at run time, which Node.js does not provide up to 26 — polyfill it (`Symbol.metadata ??= Symbol("Symbol.metadata")`) before the decorated modules load.
- `accessor` fields (4.9+) create a private backing field with a getter/setter pair and are the standard way to decorate fields.

***

#### Modules, Module Resolution and ESM/CommonJS Interop

##### Choosing `module` / `moduleResolution`

| Situation | `module` | `moduleResolution` | Available since |
|---|---|---|---|
| Code run directly by Node.js (ESM or CommonJS) | `nodenext`, or a fixed `node16` / `node18` / `node20` | implied | 4.7 / 5.8 / 5.9 |
| Code consumed by a bundler, Bun or a framework build | `preserve` (or `esnext`) | `bundler` | 5.4 (`preserve`), 5.0 (`bundler`) |
| Library published to npm | `nodenext` / `node20` | implied | 4.7 / 5.9 |
| Legacy CommonJS output consumed by a bundler | `commonjs` | `bundler` (6.0+); before 6.0 `node10` | |

Red flags in review:

- `moduleResolution: node` / `node10` in a project on 5.0+: it ignores `package.json` `exports`, so types resolve differently from what Node.js and bundlers load. Deprecated in 6.0, removed in 7.0.
- `moduleResolution: bundler` for code that Node.js runs without a bundler: it accepts extensionless relative imports that Node.js ESM rejects at run time (`ERR_MODULE_NOT_FOUND`).
- `module: esnext` without a bundler and without `"type": "module"`: Node.js loads the `.js` output as CommonJS and fails on `import`.

##### ESM rules under `nodenext`

```ts
// Project: package.json {"type":"module"}, module nodenext
import { readFileSync } from "node:fs";
import { helper } from "./util";            // error TS2835: Relative import paths need explicit file extensions in ECMAScript imports when '--moduleResolution' is 'node16' or 'nodenext'. Did you mean './util.js'?
export const text = readFileSync(new URL("./data.txt", import.meta.url), "utf8") + String(helper);
```

- The `.js` extension in an import refers to the **output** file (`./util.js` for `util.ts`). With `rewriteRelativeImportExtensions` (5.7+) the source may say `./util.ts`, which is what Node.js type stripping requires.
- `__dirname`, `__filename` and `require` do not exist in ESM. Use `import.meta.dirname`/`import.meta.filename` (Node.js 20.11+) or `fileURLToPath(import.meta.url)`.
- `.mts` always emits `.mjs` (ESM), `.cts` always emits `.cjs` (CommonJS), regardless of `package.json`.
- A CommonJS file may `require()` an ES module only on runtimes that support it (Node.js 20.19+, 22.12+). TypeScript models this from 5.8 under `nodenext`/`node20`; under `node16`/`node18`, or on older compilers, it is error TS1479. The required ES module must not use top-level `await`.

##### JSON imports: `assert` vs `with`

```ts
// tsconfig: {"resolveJsonModule": true}
// 5.3+: import attributes. Before 5.3 only `assert { … }` exists; `assert` is TS2880 under nodenext from 5.8 and under every module setting from 6.0.
import pkg from "./package.json" with { type: "json" };
export const version: string = pkg.version;
```

Under `module: nodenext` (from 5.7) a JSON import in an ES module requires the `type: "json"` attribute (TS1543), and only the default import is allowed (`import { version } from "./package.json" with { type: "json" }` is TS1544), matching Node.js, which exposes JSON modules only as a default export. 5.6 accepted both silently. The older `assert { type: "json" }` form is TS2880 under `nodenext` from 5.8 (and under `node20` in 5.9) even with `"ignoreDeprecations": "6.0"`; under `esnext`/`preserve`/`node18` it stays accepted until 6.0. Verified: Node.js 18.20.8 and 20.20.2 accept both `with` and `assert`; Node.js 22 and 24 accept only `with` (`assert` is a `SyntaxError`). Check the runtime's exact version before migrating; early 18.x/20.x releases only knew `assert`.

##### CommonJS interop: `esModuleInterop` and default imports

Many npm packages are CommonJS with `module.exports = fn`. How they may be imported depends on interop settings:

```ts
// tsconfig: {"module": "commonjs", "esModuleInterop": true}
// node_modules/legacy/index.d.ts:  declare function legacy(): number; export = legacy;
import legacy from "legacy";          // GOOD: emitted with the __importDefault helper
console.log(legacy());
```

- Without `esModuleInterop` (the default before 6.0), `import legacy from "legacy"` is error TS1259. A namespace import of a callable `export =` module (`import * as legacy from "legacy"`) is TS2497 on 5.9 and 6.0 with or without the flag (7.0.2 accepts the import and reports TS2349 when `legacy()` is called): an ES namespace object is never callable, and bundlers and Node.js ESM fail at run time on code that calls one. Enable `esModuleInterop` on every version; from 6.0 it is the default and `false` is deprecated.
- **6.0 trap**: with `"esModuleInterop": false` plus `"ignoreDeprecations": "6.0"`, 6.0 type-checks as if interop were on but emits as if it were off: `import legacy from "legacy"` compiles and throws `TypeError: (0 , legacy_1.default) is not a function` at run time. Treat `esModuleInterop: false` on 6.0 as a bug (verified with 6.0.3 and Node.js 24).[^6]
- `import x = require("x")` and `export =` are the CommonJS forms; they are rejected in ES module output and by `erasableSyntaxOnly`.
- Under `verbatimModuleSyntax` (5.0+), a file emitted as CommonJS may not use `import`/`export` syntax at all (TS1295 for imports, TS1287 for `export const`): either switch the file to ESM (`"type": "module"` or `.mts`) or write `import x = require()`.

##### `verbatimModuleSyntax` and `isolatedModules`

`verbatimModuleSyntax` (5.0+) means "what you write is what is emitted": imports without `type` stay, `import type` disappears. It replaces `importsNotUsedAsValues` and `preserveValueImports` (without effect from 5.5, hard error TS5102 from 5.6).

```ts
// tsconfig: {"verbatimModuleSyntax": true}
import type { Server } from "node:http";                       // erased
import { createServer, type IncomingMessage } from "node:http"; // createServer kept, IncomingMessage erased
import { IncomingHttpHeaders } from "node:http";                 // error TS1484: 'IncomingHttpHeaders' is a type and must be imported using a type-only import when 'verbatimModuleSyntax' is enabled.

export function handler(req: IncomingMessage, h: IncomingHttpHeaders): Server {
  void h;
  return createServer(() => req.socket.destroy());
}
```

Only interfaces and type aliases need the `type` modifier; imported classes are values and stay. Prefer `verbatimModuleSyntax` whenever a single-file transpiler (esbuild, SWC, Babel, Node.js type stripping) is in the pipeline; on 4.x use `isolatedModules` plus `importsNotUsedAsValues: "error"`.

##### `paths` and subpath imports

- `paths` affects **type checking only**. A review must confirm the runtime has the same alias (bundler config, `package.json` `imports`, test-runner `moduleNameMapper`/`resolve.alias`) — otherwise code type-checks and fails at run time.
- `baseUrl` is deprecated in 6.0 and removed in 7.0. Write `paths` relative to the tsconfig (`"@lib/*": ["./src/lib/*"]`, accepted without `baseUrl` since 4.1).
- Prefer `package.json` `imports` (`"#app/*": "./dist/app/*"`), which Node.js, bundlers and TypeScript (4.7+ under `node16`/`nodenext`, 5.0+ under `bundler`) all understand. The bare `#/` prefix resolves only from TypeScript 6.0 and only on Node.js 24.14+/25.4+; use a named prefix such as `#app/` for wider support.

##### Side-effect imports and `import defer`

- `import "./polyfill"` was not resolved before 5.6, so typos compiled silently. Enable `noUncheckedSideEffectImports` (5.6+, default from 6.0) and add `declare module "*.css";` (or the bundler's client types) for asset imports.
- `import defer * as m from "./heavy.js"` (5.9+) is only valid with `module: esnext`/`preserve`; TypeScript does not down-level it and Node.js (up to 26) does not parse it. Flag it in code that Node.js runs directly.

***

#### Declaration Files and Library Publishing

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
  "engines": { "node": ">=20.19" }
}
```

- The `types` condition must come **first** in each `exports` entry; conditions are matched in order.
- Once `exports` exists, deep imports that are not listed are blocked at run time and, under `node16`/`nodenext`/`bundler`, in TypeScript. Under `moduleResolution: node10` TypeScript ignores `exports` entirely and falls back to `types`/`typings` and `typesVersions`.
- Dual ESM/CommonJS packages need separate declaration files per format (`.d.mts`/`.d.cts`) or a CommonJS build whose types are correct for `require`. Check with `@arethetypeswrong/cli` and `publint`.
- Declarations generated by a new compiler may use syntax or names that older consumers cannot parse or resolve (`const` type parameters and `accessor` fail to parse, `NoInfer` is TS2304 before 5.4). Document the minimum TypeScript version and test it, or use `typesVersions` to ship downlevelled declarations.

##### `isolatedDeclarations` (5.5+)

```ts
// tsconfig: {"isolatedDeclarations": true, "declaration": true}
export function add(a: number, b: number) { return a + b; } // error TS9007: Function must have an explicit return type annotation with --isolatedDeclarations. (7.0.2: TS9013)
export function sub(a: number, b: number): number { return a - b; }
export const config = { retries: 3 };                        // OK: literal initialisers are inferable
export const fixed = [1, 2] as const;                        // OK
```

Enable it for published libraries and large monorepos: it makes declaration emit possible per file in other tools, and it makes public API changes visible in diffs. Diagnostic codes differ between compilers (7.0 reports TS9013/TS9039 for some of the cases 6.0 reports as TS9007) — do not key CI filters on them.

##### Writing `.d.ts` files and augmentations

```ts
// file: globals.d.ts — `export {}` makes it a module so `declare global` is an augmentation
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

- Typing `process.env` keys does not make them exist; they remain `string | undefined` at run time unless validated at start-up. Declaring them as non-optional `string` is a lie.
- `declare module "x";` without a body types the entire module as `any`. Prefer real types or a narrow declaration.
- `skipLibCheck: true` also skips errors in your **own** `.d.ts` files; keep hand-written declarations small, or check them in a separate step.
- Hand-written `.d.ts` files for JavaScript code drift from the implementation. Prefer generating them (`declaration: true`, `allowJs` + `checkJs` with JSDoc).
- `@types/*` package versions should match the runtime's major version (`@types/node@22` for Node.js 22). A newer `@types/node` advertises APIs the deployed runtime lacks.

***
#### Async Code and Promises

##### Floating and misused promises

The compiler does not report un-awaited promises. These compile cleanly and lose errors:

```ts
declare function save(): Promise<void>;
declare function reportError(e: unknown): void;

export function handler(): void {
  save();                          // BAD: a rejection becomes an unhandledRejection (process exit in Node.js 15+)
  void save().catch(reportError);  // GOOD when intentionally fire-and-forget
}

declare const items: string[];
items.forEach(async (i) => {       // BAD: forEach ignores the returned promises
  await Promise.resolve(i);
});
await Promise.all(items.map(async (i) => Promise.resolve(i))); // GOOD

if (save()) { }                    // error TS2801: This condition will always return true since this 'Promise<void>' is always defined.
```

- typescript-eslint's type-aware rules `no-floating-promises`, `no-misused-promises`, `await-thenable` and `return-await` catch these; enable them in every TypeScript service.
- `async` callbacks passed to APIs that expect synchronous callbacks (`forEach`, `EventEmitter.on`, `setInterval`, Express 4 handlers) need their own `try/catch`; a rejection there is unhandled.
- `async` functions always return a promise. `if (isValid(x))` where `isValid` became `async` is always true — TS2801 catches only direct calls in conditions, not stored results.

##### `return await` inside `try`

```ts
declare function fetchText(url: string): Promise<string>;

export async function bad(url: string): Promise<string> {
  try {
    return fetchText(url);            // BAD: the rejection escapes the try; catch never runs
  } catch {
    return "";
  }
}

export async function good(url: string): Promise<string> {
  try {
    return await fetchText(url);      // GOOD: awaited inside try, so catch/finally see the rejection
  } catch {
    return "";
  }
}
```

The same applies to `finally` blocks that release resources: without `await`, they run before the operation finishes.

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

// Parallel, collect failures (es2020 lib)
async function tolerant(ids: string[]) {
  const settled = await Promise.allSettled(ids.map(fetchUser));
  return settled.flatMap((s) => (s.status === "fulfilled" ? [s.value] : []));
}

// Timeouts and cancellation (AbortSignal.timeout: present in Node.js 16.20 and later; typed by lib.dom / @types/node)
async function withTimeout(url: string): Promise<unknown> {
  const res = await fetch(url, { signal: AbortSignal.timeout(5_000) });
  if (!res.ok) throw new Error(`GET ${url}: ${res.status}`);
  return (await res.json()) as unknown;
}
export { slow, fast, tolerant, withTimeout };
```

- Unbounded `Promise.all(hugeArray.map(…))` opens thousands of sockets or DB connections; use a concurrency limit (p-limit, a worker pool, batches) for large inputs.
- `Promise.all` does not cancel the other operations when one fails; pass an `AbortSignal` and abort it in the failure path when the remaining work is wasteful or harmful.
- `fetch` does not reject on HTTP errors: check `res.ok`. `res.json()` returns `Promise<any>` under `lib.dom` (and `Promise<unknown>` when only `@types/node`'s undici typings apply): treat it as `unknown` and validate.
- `Promise.race` with a timeout promise leaves the timer running; clear it in `finally`, or use `AbortSignal.timeout`.
- `Promise.withResolvers` is typed from 5.4 (`esnext.promise`, `es2024` lib from 5.7) and exists in Node.js 22+; do not suggest it for Node.js 20.

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

Breaking out of `for await` calls the generator's `return()`, which runs its `finally` blocks — put cleanup there (or `await using`, 5.2+).

***

#### Error Handling

##### `catch` variables are `unknown`

Under `strict` (since 4.4, `useUnknownInCatchVariables`) a `catch` variable is `unknown`; before 4.4, or with the flag off, it is `any`, and `catch (e: unknown)` (4.0+) opts in per clause.

```ts
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

export async function load(url: string): Promise<string> {
  try {
    const res = await fetch(url);
    if (!res.ok) throw new HttpError(res.status, `GET ${url} failed`);
    return await res.text();
  } catch (err) {
    if (err instanceof HttpError && err.status === 404) return "";
    console.error(err.message); // error TS18046: 'err' is of type 'unknown'.
    throw new Error(`load(${url}) failed`, { cause: err }); // keep the original error
  }
}
```

- Narrow before use: `err instanceof Error ? err.message : String(err)`. Do not "fix" TS18046 with `catch (err: any)` or `(err as Error).message` — anything can be thrown, including strings, `null` and objects from other realms.
- Wrap with `{ cause }` (`ErrorOptions`, `lib` es2022 and Node.js 16.9+) instead of concatenating messages; loggers and Node.js print the cause chain. On 4.6 and 4.7 the `cause` option is typed `Error`, so passing an `unknown` catch variable is TS2322 there; from 4.8 it is `unknown`.
- `throw "text"` and `reject("text")` lose the stack trace. Throw `Error` subclasses only (lint: `only-throw-error`, `prefer-promise-reject-errors`).
- Do not swallow errors: an empty `catch {}` needs a comment that explains why ignoring is correct.
- Parameter properties (`readonly status: number` above) are not erasable syntax; under `erasableSyntaxOnly` declare the field and assign it in the constructor.

##### Custom error classes

- Set `name` so logs show the class.
- With `target` ES2015 or newer, `instanceof` works for subclasses of `Error`. With `target: es5` (deprecated in 6.0, removed in 7.0), `class E extends Error` breaks the prototype chain: `instanceof E` is `false` and methods are missing unless the constructor calls `Object.setPrototypeOf(this, new.target.prototype)`. Seeing that call in an ES2015+ project is harmless legacy; missing it in an ES5 project is a bug.
- `Error.captureStackTrace` is a V8 API typed by `@types/node`, not by `lib`.
- `instanceof` fails for errors from another realm or from a duplicated package copy; for library error types, also check a discriminant (`err.code === "E_TIMEOUT"`).

##### Errors as values

For expected failures (validation, not-found, conflicts), a discriminated result type keeps the error channel visible in the signature:

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

Use exceptions for unexpected failures and programmer errors; do not mix both styles for the same failure in one API.

***

#### Runtime Validation at Trust Boundaries and Security

Types disappear at run time; security depends on runtime checks.

##### Validate every trust boundary

```ts
interface CreateUser { email: string; role: "user" | "admin" }

// BAD: the annotation is a claim, not a check — a client can send role: "admin"
export async function createBad(req: { json(): Promise<any> }): Promise<CreateUser> {
  const body: CreateUser = await req.json();
  return body;
}

// GOOD: validate shape and values; decide privileged fields server-side
export function parseCreateUser(input: unknown): CreateUser {
  if (typeof input !== "object" || input === null) throw new TypeError("body must be an object");
  const { email } = input as Record<string, unknown>;
  if (typeof email !== "string" || !email.includes("@") || email.length > 254) {
    throw new TypeError("invalid email");
  }
  return { email, role: "user" };
}
```

- Untrusted, whatever their declared type: `JSON.parse`, `Response.json()`, `req.body`/`req.query`/`req.params`, `process.env`, `localStorage`, `postMessage` data, WebSocket and queue messages, files, database rows written by other systems, and responses from third-party APIs.
- Prefer a schema library whose output type is **derived from the schema** (Zod, Valibot, ArkType, TypeBox + Ajv, io-ts). A hand-written interface next to a hand-written validator drifts; `z.infer<typeof Schema>` cannot.
- Parse once at the boundary into a typed domain value (brands for validated strings), then trust the type inside.
- Validate configuration (`process.env`) once at start-up and fail fast; do not scatter `process.env.X!` through the code.

##### Common vulnerabilities in TypeScript code

- **Prototype pollution**: object spread `{ ...defaults, ...input }` defines own properties, so a `"__proto__"` key from `JSON.parse` does not change any prototype (verified on Node.js 24). Recursive "deep merge" helpers and `target[key] = value` loops with attacker-controlled keys do: copying `JSON.parse('{"__proto__":{"admin":true}}')` key by key replaces the target's prototype, so `target.admin` becomes `true`, and a recursive merge that descends into `__proto__` writes to `Object.prototype` for the whole process. Reject `__proto__`, `constructor`, `prototype` keys, use `Map` or `Object.create(null)` for dictionaries.
- **Injection**: never build SQL, shell commands, HTML or regular expressions from untrusted strings. Use parameterised queries / query builders, `execFile`/`spawn` with argument arrays (no `shell: true`), `textContent` or a sanitiser instead of `innerHTML`, and an escaping function (`RegExp.escape` — `es2025` lib in TypeScript 6.0+, Node.js 24+) for dynamic patterns.
- **Code execution**: `eval`, `new Function`, `vm.runInContext`, `setTimeout(string)` with external input.
- **Over-posting / mass assignment**: passing a validated-looking but un-stripped request body to an ORM `update()`; structural typing keeps unknown properties.
- **Leaking fields**: returning database entities directly; map to explicit response DTOs.
- **Secrets**: compare with `crypto.timingSafeEqual`; never put credentials in bundler `define`/`import.meta.env` values (they end up in client bundles); `.d.ts` typing does not make them secret.
- **ReDoS**: nested quantifiers on user input (`/(a+)+$/`). 5.5+ checks regex *syntax*, not complexity.
- `as`, `!` and `any` at a trust boundary bypass all checks — reject them in review.

##### Supply chain and configuration

- Pin `typescript` with `~x.y.z` (or exactly) and commit the lockfile. TypeScript does not follow semantic versioning: every minor release may add errors or change inference, and 6.0 changed defaults under unchanged configs.
- `compilerOptions.plugins` load language-service plugins from `node_modules` in editors (not in 7.0, which ignores them); review additions like any dependency.
- Type-only dependencies (`@types/*`) are still code that runs in editors and CI; keep them in `devDependencies` and pinned.

***

#### Immutability and Resource Management

##### `readonly` and `as const`

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

function total(xs: readonly number[]): number { // documents and enforces "does not mutate"
  return xs.reduce((a, b) => a + b, 0);
}
export { r, total };
```

- `readonly` is shallow and compile-time only; `Object.freeze` is shallow and run-time only (and its result type is `Readonly<T>`).
- A mutable array is assignable to `readonly T[]`, not the other way round — prefer `readonly` array parameters for every function that does not mutate its input.
- `readonly` properties are not deeply immutable and are **not** enforced through aliases: an object typed `{ readonly a: number }` can be assigned to a mutable `{ a: number }` variable without error. Treat `readonly` as documentation plus a guard against accidental writes, not as a security boundary.
- `Array.prototype.sort`, `reverse` and `splice` mutate in place, which is a common bug on arrays received from callers or held in state (React, Redux). Use `toSorted`/`toReversed`/`toSpliced`/`with` (`es2023` lib, TypeScript 5.2+, Node.js 20+) or copy first (`[...xs].sort()`).

##### `using` and `await using` (5.2+)

```ts
// tsconfig: {"lib": ["es2022", "esnext.disposable"]}   (5.2+; 5.1: syntax errors TS1005/TS1434)
class Lock implements Disposable {
  #held = true;
  [Symbol.dispose](): void {
    this.#held = false;
  }
}

class TempDir implements AsyncDisposable {
  constructor(readonly path: string) {}
  async [Symbol.asyncDispose](): Promise<void> {
    const { rm } = await import("node:fs/promises");
    await rm(this.path, { recursive: true, force: true });
  }
}

export async function job(): Promise<void> {
  await using dir = new TempDir("/tmp/job-1");
  using lock = new Lock();
  // … work; disposal runs lock → dir on every exit path, including exceptions
  void dir;
  void lock;
}
```

- Typing needs `lib` `esnext.disposable` (or `esnext`); the disposable declarations are not part of `es2025`.
- TypeScript down-levels `using` for every target except `esnext`. The helper needs `Symbol.dispose`/`Symbol.asyncDispose` at run time (present in current Node.js 18, 20 and 22 releases); `DisposableStack`, `AsyncDisposableStack` and `SuppressedError` exist only from Node.js 24 — guard or polyfill them.
- With `target: esnext`, `using` is emitted as written, which Node.js 22 rejects as a syntax error.
- On compilers before 5.2, or as a general rule: resources acquired in a function (file handles, locks, DB connections, timers, subscriptions) are released in `try … finally`, never left to garbage collection.

***

#### Library Typings vs. Runtime APIs

`lib` decides which globals type-check; it does not make them exist. Two independent questions apply to every API: does the pinned compiler's `lib` declare it, and does the oldest supported runtime have it?

| API | Compiler `lib` (first version) | Node.js |
|---|---|---|
| `Array.prototype.at`, `Object.hasOwn`, `Error` `cause` | `es2022` (4.6) | 16.9+ |
| `toSorted`, `toReversed`, `with`, `findLast` | `es2023` (5.0 for `findLast`, 5.2 for the copy methods) | 20+ (`findLast` 18+) |
| `Object.groupBy`, `Map.groupBy` | `esnext` (5.4), `es2024` (5.7) | 22+ (absent in 20) |
| `Promise.withResolvers` | `esnext` (5.4), `es2024` (5.7) | 22+ |
| `Set.prototype.union` and other set methods | `esnext` (5.5), `es2025` (6.0) | 22+ |
| Iterator helpers (`Iterator.from`, `.map`, `.toArray`) | `esnext` (5.6), `es2025` (6.0) | 22+ |
| `Promise.try`, `Float16Array` | `esnext` (5.8), `es2025` (6.0) | 24+ |
| `RegExp.escape` | `es2025` / `esnext` (6.0; 5.9 lacks it even in `esnext`) | 24+ |
| `DisposableStack`, `SuppressedError` | `esnext` (5.2) | 24+ |
| `Error.isError` | `esnext` (5.9) | 24+ |
| `Temporal`, `Map.prototype.getOrInsert` | `esnext` (6.0) | 26+ |
| `Symbol.metadata` (decorator metadata) | `esnext` (5.2) | none up to 26 |

Each cell was verified: the compiler column by compiling a call on the listed release and the release before it, the runtime column with `typeof` checks on Node.js 16.20, 18.20, 20.20, 22.23, 24.22, 25.9 and 26.12 (Node.js 21 and 23 were not tested).

Rules:

- Set `lib` to the oldest runtime's ECMAScript year, plus `dom` only for browser code. A Node.js project whose `lib` includes `dom` type-checks `window`, `document` and browser-only overloads that crash at run time.
- `lib: ["esnext"]` floats with the compiler: each upgrade adds APIs the runtime may not have. Prefer granular libs (`"lib": ["es2023", "esnext.disposable"]`).
- Before 6.0, a missing `lib` derives from `target` (`target: es5` gives the ES5 library, without `Map`, `Set` or `Promise` types unless `lib` adds them); since 6.0 the default target and lib are `es2025`, which advertises APIs that Node.js 20 and 22 lack.
- `@types/node` declares Node.js APIs; its major version must match the deployed Node.js major.
- Since 5.7, `TypedArray`s are generic over their buffer (`Uint8Array<ArrayBufferLike>`), and since 5.9 such arrays are no longer assignable where `ArrayBuffer`/`BufferSource` is required (`crypto.subtle.digest(buf)` with a Node.js `Buffer`). Fix by typing producers as `Uint8Array<ArrayBuffer>` or copying (`new Uint8Array(buf)`), never with `as any`.
- Since 5.6 under `strict`, built-in iterators return `undefined` when done (`strictBuiltinIteratorReturn`): `it.next().value` is `T | undefined` until `done` is checked.

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
export type { _publicHasNoPassword };
```

- Prefer `// @ts-expect-error <reason>` (3.9+) to `// @ts-ignore`: it fails when the error disappears, so stale suppressions are noticed. Both suppress *every* error on the next line — keep that line minimal. `// @ts-nocheck` on a whole file is a finding.
- Vitest's `expectTypeOf`/`assertType` (`vitest --typecheck`), `tsd` and `expect-type` provide readable type assertions. The house-rule `Assert<T, U extends T>` pattern only checks assignability, not equality; use an `Equals` helper as above when exact types matter.

##### Runtime tests

- Test runners that strip types (Vitest, Jest with SWC/esbuild/Babel, `node --test` on `.ts` files, tsx, Bun) **do not type-check**. CI must run `tsc --noEmit -p tsconfig.json` (or `tsc -b`) separately; a green test run says nothing about type errors.
- Test files need their framework's globals: explicit imports (`import { describe, it, expect } from "vitest"`) work on every compiler; global `describe` needs `"types": ["vitest/globals"]` or `["jest"]` — which is mandatory from 6.0, where `types` defaults to `[]`.
- Mocks typed with `as any` or `as unknown as Service` hide interface drift. Use typed mock helpers (`vi.mocked`, `jest.Mocked<T>`, `satisfies Partial<Service>`).

***

#### Linting and Tooling

##### typescript-eslint

Recommended configuration: `tseslint.configs.strictTypeChecked` (or `recommendedTypeChecked`) plus `stylisticTypeChecked`. Review-relevant rules:

| Rule | Catches |
|---|---|
| `no-floating-promises`, `no-misused-promises`, `await-thenable`, `return-await` | lost rejections, async callbacks in sync slots, awaited non-promises, `return` without `await` in `try` |
| `no-explicit-any`, `no-unsafe-assignment`, `no-unsafe-member-access`, `no-unsafe-call`, `no-unsafe-return`, `no-unsafe-argument` | `any` leaking out of untyped sources |
| `no-unnecessary-condition`, `strict-boolean-expressions` | dead checks, truthiness bugs |
| `switch-exhaustiveness-check` | missing union cases |
| `no-non-null-assertion`, `consistent-type-assertions` | unchecked `!` and `as` |
| `restrict-template-expressions`, `restrict-plus-operands` | `"${obj}"` → `[object Object]`, string/number mixing |
| `only-throw-error`, `prefer-promise-reject-errors` | throwing non-`Error` values |
| `consistent-type-imports` | missing `import type` |
| `no-deprecated` | use of `@deprecated` APIs |
| `ban-ts-comment` | `@ts-ignore`, `@ts-nocheck`, `@ts-expect-error` without description |

- Type-aware rules need the TypeScript compiler API. typescript-eslint supports TypeScript `>=4.8.4 <6.1.0`, so it runs on 6.0 at most; projects that build with 7.0 install 6.0 next to it (`"typescript": "npm:@typescript/typescript6@^6.0.2"` plus `"@typescript/native": "npm:typescript@^7.0.2"`).[^7][^5] Projects on TypeScript older than 4.8.4 need an older typescript-eslint major.
- Lint rules are not a substitute for compiler flags: `noUncheckedIndexedAccess`, `exactOptionalPropertyTypes` and `strict` catch more, at lower cost.
- Disabling a rule inline needs a reason (`// eslint-disable-next-line @typescript-eslint/no-explicit-any -- third-party callback`).

##### Transpilers and type stripping

- esbuild, SWC, Babel, tsx, Bun, Deno, Vite and Node.js type stripping transpile file by file **without type checking** and ignore most of `tsconfig.json` (`paths` support varies). Code must be `isolatedModules`-compatible; `verbatimModuleSyntax` (5.0+) makes that explicit.
- Node.js runs `.ts` files by stripping types (unflagged since 22.18.0 and 23.6.0; Node.js 20 cannot). It rejects enums, value namespaces, parameter properties, `import =` and decorators, requires real file extensions in imports (`./b.ts`), ignores `tsconfig.json` (no `paths`), and refuses `.ts` files under `node_modules`. Use `erasableSyntaxOnly` (5.8+), `verbatimModuleSyntax`, and `allowImportingTsExtensions` with `rewriteRelativeImportExtensions` (5.7+) or `noEmit`.[^8]
- `ts-node` (last release 10.9.2, 2023) needs the compiler API and lags behind new module options; prefer `tsx`, Node.js type stripping, or compiling first.
- Editors bundle their own TypeScript; configure the workspace version so editor diagnostics match CI.

***

#### Performance and Observability

##### Compiler performance

- Set `types` explicitly; loading every `@types` package (the default before 6.0) is a common cause of slow builds.
- `skipLibCheck`, `incremental`, project references with `tsc -b`, and `isolatedModules` + a fast transpiler for emit with `tsc --noEmit` for checking. `--noCheck` (5.6+) emits without checking when a separate check job exists.
- Avoid huge unions (thousands of members), deeply recursive conditional types and wide template-literal mapped types. `tsc --extendedDiagnostics` and `--generateTrace <dir>` locate slow types.
- Prefer `interface extends` over large intersections; interfaces are cached by the checker.
- TypeScript 7.0's native compiler is the largest available speed-up, but only for projects free of 6.0 deprecations that do not need the compiler API in the same process.[^5]

##### Emitted code

- A `target` below ES2022 down-levels class fields, `#private`, `??=` and `static {}` into slower helper-based code; a `target` below ES2017 turns `async`/`await` into generator state machines. Pick the highest target the runtime supports.
- `importHelpers` + `tslib` avoids duplicating helpers in every file when down-levelling.
- `enum` and `namespace` emit IIFEs; `as const` objects are smaller and tree-shakeable.
- Spreading large arrays into arguments (`Math.max(...arr)`, `arr.push(...other)`) throws `RangeError` beyond the engine's argument limit; use a loop or `reduce` for unbounded input.
- `JSON.parse(JSON.stringify(x))` turns `Date` into strings, `Map`/`Set` into `{}`, drops `undefined` and functions, and throws on `BigInt`; use `structuredClone` (Node.js 17+) for deep copies.

##### Observability

- Emit `sourceMap` and run Node.js with `--enable-source-maps` so stack traces point at `.ts` lines. Do not publish source maps with embedded sources (`inlineSources`) unintentionally.
- `JSON.stringify(err)` yields `{}` for `Error` objects (`message`, `stack` and `cause` are not enumerable). Serialise errors explicitly or through the logger:

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

- Structured logs: pass typed context objects (`logger.info({ userId, orderId }, "order created")`) instead of interpolated strings.
- Register `process.on("unhandledRejection")`/`("uncaughtException")` to log and exit; never use them to keep a corrupted process running.
- OpenTelemetry instrumentation must load before the instrumented modules (`node --import ./instrumentation.js`); with ESM output a plain `import` at the top of the entry file is not early enough.

***

#### Upgrading Across Versions — Migration Notes

TypeScript minor releases routinely add errors. Pin the compiler, upgrade one line at a time in a dedicated change, and run both `tsc` and the emitted program.

##### 4.x → 5.x

- 5.0 deprecates `importsNotUsedAsValues`, `preserveValueImports` (→ `verbatimModuleSyntax`), `target: es3`, `out`, `keyofStringsOnly`, `suppressImplicitAnyIndexErrors`, `noImplicitUseStrict`, `noStrictGenericChecks`, `charset`, `prepend`; they stop having any effect in 5.5 (still accepted with `"ignoreDeprecations": "5.0"`) and are hard errors (TS5102/TS5108) from 5.6.
- 5.0 makes every enum a union enum: out-of-range numeric literals assigned to an enum type become errors.
- 5.0 switches decorators to the TC39 standard unless `experimentalDecorators` is set; frameworks that need legacy decorators must keep the flag.
- The compiler's own Node.js requirement rises: `engines.node` is `>=4.2.0` in 4.9, `>=12.20` in 5.0, `>=14.17` from 5.1 through 6.0, and `>=16.20.0` in 7.0 (whose `tsc` is a native binary launched through a small Node.js wrapper). CI images with old Node.js versions break on compiler upgrades.

##### 5.x → 6.0

- New defaults (`strict`, ES-module output, `bundler` resolution, `target: es2025`, `types: []`, `rootDir`, `noUncheckedSideEffectImports`) change errors **and emitted output** for configs that relied on them. Make every such option explicit before upgrading.
- Deprecations are errors (TS5101/TS5107/TS2880) unless `"ignoreDeprecations": "6.0"` is set; fix them, because 7.0 removes them. The `module Foo { }` namespace keyword (TS1540) and `tsc file.ts` next to a `tsconfig.json` (TS5112) are errors that `ignoreDeprecations` does not silence.
- `baseUrl` → `paths` relative to the tsconfig (`"*": ["./src/*"]` for bare lookups) or `package.json` `imports`.
- `tsc file.ts` next to a `tsconfig.json` is TS5112; use `tsc -p .`.[^4]

##### 6.0 → 7.0

- Everything deprecated in 6.0 is removed; `ignoreDeprecations` no longer helps.
- 7.0 ships no stable compiler API: typescript-eslint, ts-jest (type-check mode), ts-morph, ts-loader, fork-ts-checker, Angular's compiler, Vue/Svelte/Astro/MDX tooling keep using 6.0 via `@typescript/typescript6` (`tsc6`).[^5]
- Union member order in `.d.ts` output and error messages changes (stable type ordering; preview it on 6.0 with `--stableTypeOrdering`); template-literal inference handles whole Unicode code points; JavaScript checking drops Closure-style JSDoc and constructor functions.
- The `typescript` package installs a platform-specific native binary through optional dependencies; installs with `--omit=optional` break `tsc`.

***

#### Production / Review Checklist

##### Version and configuration

- [ ] Compiler version determined (`package.json`, lockfile) before recommending version-tied features; runtime version determined separately
- [ ] `typescript` pinned (`~x.y.z`) with a committed lockfile
- [ ] `strict`, `module`, `target`, `lib`, `types`, `rootDir`/`outDir` explicit in every emitting tsconfig
- [ ] `module`/`moduleResolution` match the runtime: `nodenext`/`node16`/`node20` for Node.js, `preserve`/`esnext` + `bundler` for bundlers; no `node10`/`classic` on 5.0+
- [ ] `esModuleInterop` on (never `false` on 6.0)
- [ ] No deprecated options; any `ignoreDeprecations` has a removal ticket
- [ ] `noUncheckedIndexedAccess`, `noImplicitOverride`, `verbatimModuleSyntax` (5.0+) in new projects; `exactOptionalPropertyTypes` where feasible
- [ ] `erasableSyntaxOnly` (5.8+) when Node.js type stripping or strip-only tools run the code

##### Types and narrowing

- [ ] No `any` at boundaries; `unknown` + validation for parsed or external data
- [ ] Every `as`, `!`, `@ts-expect-error` justified; no `@ts-ignore`, `@ts-nocheck` or double assertions
- [ ] Discriminated unions with exhaustive `switch` (`never` check or `switch-exhaustiveness-check`)
- [ ] Truthiness checks do not drop `0`, `""` or `false`; defaults use `??`, not `||`
- [ ] Type guards and assertion functions prove exactly what they claim
- [ ] Narrowing not trusted across calls/`await` that may mutate the value
- [ ] Generic type parameters relate inputs and outputs; no return-only `T` used as a cast
- [ ] `Omit`/`Pick` on unions use a distributive helper; security projections use `Pick` allow-lists
- [ ] New code uses unions / `as const` objects instead of enums; external values validated against enums

##### Modules and packaging

- [ ] Relative ESM imports carry `.js` (or `.ts` with `rewriteRelativeImportExtensions`)
- [ ] `import type` for type-only imports; JSON imports use the syntax the compiler and runtime support (`with` on 5.3+)
- [ ] `paths` aliases mirrored in the runtime/bundler/test-runner config, or replaced by `package.json` `imports`
- [ ] Libraries: `exports` with `types` first, declarations tested against the oldest supported TypeScript, checked with attw/publint

##### Async, errors and resources

- [ ] No floating promises; no `async` callbacks in synchronous slots without error handling
- [ ] `return await` inside `try`; errors wrapped with `{ cause }`; only `Error` objects thrown
- [ ] `catch` variables narrowed from `unknown`, not cast
- [ ] Bounded concurrency for large fan-outs; `AbortSignal` passed through I/O layers; `res.ok` checked
- [ ] Resources released with `using`/`await using` (5.2+) or `try … finally`

##### Security

- [ ] Request bodies, env vars, JSON, messages validated at run time with a schema
- [ ] No prototype-pollution-prone merges; no over-posting into ORM updates; explicit response DTOs
- [ ] Parameterised SQL, argument-array process spawning, no `innerHTML`/`eval` with untrusted data
- [ ] `lib` APIs available on the oldest supported runtime (no `esnext`-only APIs without polyfills)

##### Tooling and CI

- [ ] `tsc --noEmit` (or `tsc -b`) runs in CI separately from tests and bundling
- [ ] typescript-eslint type-aware rules enabled, running on a supported TypeScript (≤ 6.0)
- [ ] Editor uses the workspace TypeScript version

***

[^1]: TypeScript release notes (per version) and the TypeScript blog. https://www.typescriptlang.org/docs/handbook/release-notes/overview.html , https://devblogs.microsoft.com/typescript/
[^2]: npm registry — `typescript` versions and publish dates (`npm view typescript time`). https://www.npmjs.com/package/typescript?activeTab=versions
[^3]: DefinitelyTyped README — "Support window" (TypeScript versions younger than two years) and `tsX.Y` dist-tags. https://github.com/DefinitelyTyped/DefinitelyTyped#support-window
[^4]: TypeScript 6.0 release notes — new defaults, deprecations, `ignoreDeprecations`, `--stableTypeOrdering`. https://www.typescriptlang.org/docs/handbook/release-notes/typescript-6-0.html
[^5]: Announcing TypeScript 7.0 — native compiler, removed options, no API, `@typescript/typescript6`. https://devblogs.microsoft.com/typescript/announcing-typescript-7-0/
[^6]: TSConfig reference — `esModuleInterop`, `module`, `moduleResolution`, `verbatimModuleSyntax`, `useDefineForClassFields`. https://www.typescriptlang.org/tsconfig/
[^7]: typescript-eslint — Dependency Versions (supported TypeScript range). https://typescript-eslint.io/users/dependency-versions/
[^8]: Node.js documentation — Modules: TypeScript (type stripping and its limitations). https://nodejs.org/api/typescript.html
