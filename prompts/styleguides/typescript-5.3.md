### TypeScript 5.3: The Complete Production Developer Guide

#### Overview

> **Version**: TypeScript 5.3. The GA build, **5.3.2, shipped on November 20, 2023**; the latest and final patch is **5.3.3 (published to npm on December 6, 2023)**.[^1][^2][^3]
> **Support status (October 2026)**: unsupported. The TypeScript team ships patches only for the newest release line, so 5.3 received no fixes after 5.3.3. Newer lines followed: 5.4 (March 6, 2024), 5.5 (June 2024), 5.6 (September 2024), 5.7 (November 2024), 5.8 (February 2025), 5.9 (July 2025), **6.0** (the last JavaScript-based compiler, 6.0.2 on March 23, 2026; latest 6.0.3, April 16, 2026) and **7.0** (the native Go port, 7.0.2 on July 8, 2026), which is the npm `latest` tag today (7.1 is in beta).[^3] DefinitelyTyped tests `@types` packages only against TypeScript versions younger than two years; since June 2026 new `@types/node` releases require TypeScript 5.6 (see [Tooling](#tooling-and-type-packages)).[^4]
> **Scope**: the complete TypeScript language and compiler as usable in 5.3 — types, narrowing, generics, `satisfies`, `const` type parameters, classes and decorators, `using`, enums, modules and module resolution, strictness flags, tsconfig, tooling — with every 5.3 change versus 5.2 marked **New in 5.3**.
> **Note**: this document is self-contained. Everything that shipped before 5.3 is carried forward. Every `ts` block was compiled with `tsc` 5.3.3 (`strict`, `target: es2022`, `module: nodenext`, `lib: ["esnext", "dom"]`, `@types/node` 22.19.21) unless a `// @ts-config:` comment in the block says otherwise; lines marked `// error TSnnnn` fail on 5.3.3 with exactly that diagnostic. `// @file: name` starts another file of a multi-file example, `// @pkgtype: commonjs` marks examples that live in a `"type": "commonjs"` package (the default is `"type": "module"`), and blocks marked `// @run` were also emitted and executed on Node.js 22.23.4.

TypeScript 5.3 is a small, mostly additive release. Its headline changes are **import attributes** (`import data from "./data.json" with { type: "json" }` and `import(spec, { with: … })`), **stable `resolution-mode` in `import type` and `import()` types** — now usable under every `moduleResolution` — **narrowing inside `switch (true)`**, **narrowing on comparisons to `true`/`false`**, **`instanceof` narrowing through a `static [Symbol.hasInstance]` type predicate**, and a **new error (TS2855) for `super.x` when `x` is a class field** of the base class. Tooling changes: interactive (clickable) type inlay hints, the editor preference `preferTypeOnlyAutoImports`, `tsc` skipping JSDoc parsing (with a new `JSDocParsingMode` API), faster comparison of non-normalized intersections, and `tsserverlibrary.js` reduced to a re-export of `typescript.js`.[^1][^5]

Global truths about TypeScript 5.3:

- **Types are erased.** `tsc` checks types and then removes them; annotations, interfaces, type aliases, `as` and `!` assertions do not exist at runtime. Validating external data is a runtime job.
- **Typing is structural.** A value is assignable to a type when it has the required members; declared names do not matter (except for classes with `private`/`#private` members and enums).
- **The type system is deliberately unsound in places**: array covariance, method-parameter bivariance, `any`, type assertions, non-null assertions, index access without `noUncheckedIndexedAccess`, and user-written type predicates (including the new `Symbol.hasInstance` predicates) all let wrong types through. Each one is a claim the author has to justify.
- **`lib` describes the runtime, it does not provide it.** `lib: ["esnext"]` in 5.3 declares `using`-related types (`Disposable`, `DisposableStack`, `Symbol.dispose`), decorator context types and the ES2023 array methods; whether they exist depends on the actual engine.
- **TypeScript does not polyfill.** `target` downlevels *syntax* (`using`, decorators, class fields, `?.`), never library APIs (`Symbol.metadata`, `DisposableStack`, `Array.prototype.toSorted`).
- **The defaults are permissive and module-dependent**: `strict` is off unless set; the default `target` is ES5, except that `module: nodenext` implies `target: esnext` and `module: node16` implies `target: es2022` (verified with 5.3.3 emit). Read the project's `tsconfig.json` (including every `extends` level) before judging nullability, `any` or index-access issues.
- **5.3 is the last release before `NoInfer`, closure narrowing preservation and `--module preserve` (5.4).** Code written for 5.3 often contains workarounds for these; do not "simplify" them with 5.4+ features.

***

#### Version-aware review

The project pins TypeScript 5.3.x. Before flagging code as wrong because of an API, syntax form or compiler option, check it against 5.3:

- **Do not suggest anything from 5.4 or later** (see [Do not use or recommend](#do-not-use-or-recommend-not-in-typescript-53)). For example `NoInfer<T>`, `Object.groupBy`/`Map.groupBy` types, `Promise.withResolvers` types, `--module preserve`, inferred type predicates, `Set.prototype.union` types, `--isolatedDeclarations`, `${configDir}`, iterator helper types, `--noCheck`, `--rewriteRelativeImportExtensions`, `--erasableSyntaxOnly`, `--module node18`/`node20`, `--target es2023`/`es2024` and `import defer` do not exist in 5.3. Each was verified as an error on 5.3.3 and as accepted by the release that introduced it.
- **Say "not available in TypeScript 5.3" only when 5.3 really lacks it.** Import attributes (`with`), `resolution-mode` in `import type`, `switch (true)` narrowing and `Symbol.hasInstance` narrowing *are* new in 5.3. `using`/`await using`, decorator metadata (`context.metadata`), mixing labeled and unlabeled tuple elements, `.ts` extensions in `import type` paths and `Array.prototype.toSorted`/`toSpliced`/`with` types (5.2); unrelated getter/setter types and `undefined`-returning functions without `return` (5.1); standard decorators, `const` type parameters, `--moduleResolution bundler`, `--verbatimModuleSyntax`, `export type *`, `--allowImportingTsExtensions`, `--customConditions` and multiple `extends` in tsconfig (5.0); `satisfies` and `accessor` (4.9) are all available.
- **Distinguish "TypeScript accepts it" from "the runtime supports it".** A `lib` entry or `target` newer than the deployment runtime is a real bug. 5.3 type-checks `with { type: "json" }` but Node.js 16 cannot parse it; `context.metadata` type-checks but is `undefined` at runtime without a `Symbol.metadata` polyfill; `target: "esnext"` emits `using` verbatim, which only Node.js 24+ parses.
- **Distinguish compiler errors from lint findings.** `no-floating-promises`, `no-misused-promises`, `strict-boolean-expressions`, `no-unnecessary-condition`, `switch-exhaustiveness-check` and `consistent-type-imports` are typescript-eslint rules, not `tsc` diagnostics.
- **Respect the project's flags.** Code that is fine under `strict: false` may be wrong under `strict: true` and vice versa; `noUncheckedIndexedAccess`, `exactOptionalPropertyTypes` and `useDefineForClassFields` change what is correct.
- **`@types/*` versions must match 5.3.** Current `@types/node` (26.x, and 22.20+ / 24.13.3+) no longer compiles with 5.3; a reviewer should not suggest "upgrade `@types/node` to latest" without also upgrading TypeScript.

***

#### What's new in TypeScript 5.3 (vs. 5.2)

| Change | Kind | Section |
|---|---|---|
| Import attributes: `import … with { type: "json" }`, `export … from … with {…}`, `import(spec, { with: {…} })`, `ImportCallOptions` | syntax | [Import attributes](#new-in-53-import-attributes) |
| `resolution-mode` attribute on `import type … with {…}` and `import("pkg", { with: {…} })` types — stable, no longer nightly-only | syntax | [`resolution-mode`](#new-in-53-resolution-mode-in-import-types-and-all-module-modes) |
| `resolution-mode` honoured under `bundler` and `node10`, ignored without error under `classic` | module resolution | [`resolution-mode`](#new-in-53-resolution-mode-in-import-types-and-all-module-modes) |
| Narrowing on each `case` condition of `switch (true)` | narrowing | [`switch (true)`](#new-in-53-switch-true-narrowing) |
| Narrowing on `=== true` / `=== false` / `!== true` comparisons (PR #53681) | narrowing | [Boolean comparisons](#new-in-53-narrowing-on-comparisons-to-booleans) |
| `instanceof` narrows through a `[Symbol.hasInstance]` type predicate; a right operand with `[Symbol.hasInstance]` need not be a constructor | narrowing | [`Symbol.hasInstance`](#new-in-53-instanceof-narrowing-through-symbolhasinstance) |
| TS2855: `super.x` where `x` is a base-class field | new error (breaking) | [`super` checks](#new-in-53-super-property-access-checks) |
| Clickable type inlay hints; `preferTypeOnlyAutoImports` | editor | [Editor and API](#new-in-53-editor-compiler-and-api-changes) |
| `tsc` skips JSDoc parsing; `JSDocParsingMode` API; faster intersection comparisons | performance / API | [Editor and API](#new-in-53-editor-compiler-and-api-changes) |
| `tsserverlibrary.js` is now a thin wrapper around `typescript.js` | API (breaking for browser-global loading) | [Editor and API](#new-in-53-editor-compiler-and-api-changes) |
| `const T extends string[]` infers a mutable tuple; `as const satisfies string[]` accepted (undocumented) | inference | [`satisfies` and `const` type parameters](#satisfies-as-const-and-const-type-parameters) |
| DOM `lib.d.ts` updates (e.g. a generic `SubtleCrypto.exportKey` overload, `generateKey("Ed25519", …)`) | lib | [Migration notes](#removed--changed-in-53--migration-notes) |

There are **no new compiler options, no new `target`/`lib` values and no new `lib` files in 5.3**: a file-by-file comparison of `typescript/lib` between 5.2.2 and 5.3.3 shows the same set of `lib.*.d.ts` files (5.4 is the release that adds `lib.esnext.collection`, `lib.esnext.object`, `lib.esnext.promise` and others). The deprecations announced in 5.0 still apply unchanged (see [Deprecated](#deprecated-in-53-the-50-deprecation-set)).

***

#### New in 5.3: Import attributes

Import attributes (TC39 proposal, Stage 4 in ES2025) replace import *assertions*: the keyword is `with` instead of `assert`, and hosts may use the attributes to guide how a module is loaded, not just to check it afterwards.[^1][^6] TypeScript 5.3 parses and preserves them; it does **not** check their contents — any key and string value is accepted and passed through to the emitted JavaScript.

```ts
// @ts-config: {"resolveJsonModule": true}
// @file: data.json
{ "port": 8080 }
// @file: main.ts
import config from "./data.json" with { type: "json" };
export { config };

// Dynamic form: second argument of type ImportCallOptions ({ with?: ImportAttributes; assert?: ... })
export const lazy = await import("./data.json", { with: { type: "json" } });

// Not validated by TypeScript - only the host rejects it:
// import * as foo from "./foo.js" with { type: "fluffy bunny" };
```

Before 5.3, the same line is a syntax error (verified on 5.2.2: `TS1005: ';' expected`).

##### Where import attributes are allowed (verified on 5.3.3)

| `module` setting | static `import … with {…}` | `import(x, { with: … })` |
|---|---|---|
| `esnext` | allowed | allowed |
| `nodenext` | allowed | allowed |
| `node16` | **TS2823** | allowed |
| `es2015`/`es2020`/`es2022` | **TS2823** | **TS1324** |
| `commonjs`, `amd`, `umd`, `system` | **TS2823** | **TS1324** |

TS2823 reads "Import attributes are only supported when the '--module' option is set to 'esnext' or 'nodenext'." The same restriction applies to the old `assert` form (TS2821). A project on `module: node16` that wants JSON imports with attributes must move to `nodenext` (or keep JSON out of ESM).

##### JSON imports on Node.js: TypeScript 5.3 does not enforce `type: "json"`

Node.js requires the `type: "json"` attribute for every JSON import from an ES module. TypeScript 5.3 does **not** check this under `module: nodenext` — the error TS1543 ("Importing a JSON file into an ECMAScript module requires a 'type: "json"' import attribute") only arrives in 5.7. The code compiles cleanly on 5.3 and crashes at startup:

```ts
// @ts-config: {"resolveJsonModule": true}
// @file: data.json
{ "port": 8080 }
// @file: main.ts
// BAD: compiles on 5.3 under nodenext; Node.js 18/20/22/24 throw at load time:
// TypeError [ERR_IMPORT_ATTRIBUTE_MISSING] (ERR_IMPORT_ASSERTION_TYPE_MISSING on Node.js 18/20)
import config from "./data.json";
export const port: number = config.port;
```

```ts
// @ts-config: {"resolveJsonModule": true}
// @run
// @file: data.json
{ "port": 8080 }
// @file: main.ts
// GOOD
import config from "./data.json" with { type: "json" };
console.log(config.port); // 8080
```

Runtime support for the two syntaxes (each tested with a plain `.mjs` file on the exact patch releases):

| Node.js | `with { type: "json" }` | `assert { type: "json" }` |
|---|---|---|
| 16.20.2 | SyntaxError | works |
| 18.20.8 | works (since 18.20.0) | works |
| 20.20.2 | works (since 20.10.0) | works |
| 22.23.4 | works | **SyntaxError** (removed in 22.0) |
| 24.22.1 | works | **SyntaxError** |

Review rules:

- In ESM code compiled with 5.3 for Node.js, every JSON import needs `with { type: "json" }`; flag a missing attribute — the compiler will not.
- Flag new uses of `assert { … }`. TypeScript 5.3 still accepts `assert` without any warning (a deprecation error arrives only in 5.8 for `nodenext`), but Node.js 22+ cannot parse it. Migrate to `with`.
- `with` emits verbatim, so the *runtime* must support it: Node.js ≥ 18.20 / ≥ 20.10 / any 22+, and current browsers. For bundlers, check the bundler version.
- `with` attributes on `import type` are only meaningful for `resolution-mode` (next section); any other key there is pointless.

***

#### New in 5.3: `resolution-mode` in import types and all module modes

Under `node16`/`nodenext`, whether an import is resolved with `import` or `require` conditions depends on the importing file's module format. For *type-only* imports this can be overridden. 4.7 allowed `/// <reference types="pkg" resolution-mode="require" />`; 5.3 makes the import-attribute form stable:[^1][^7]

```ts
// @ts-config: {"module": "nodenext"}
// @file: node_modules/pkg/package.json
{ "name": "pkg", "version": "1.0.0", "exports": { ".": { "import": "./esm.js", "require": "./cjs.js" } } }
// @file: node_modules/pkg/esm.d.ts
export interface TypeFromImport { esm: true }
// @file: node_modules/pkg/cjs.d.ts
export interface TypeFromRequire { cjs: true }
// @file: main.ts
import type { TypeFromRequire } from "pkg" with { "resolution-mode": "require" };
import type { TypeFromImport } from "pkg" with { "resolution-mode": "import" };
export interface MergedType extends TypeFromRequire, TypeFromImport {}

// Also on import() types:
export type R = import("pkg", { with: { "resolution-mode": "require" } }).TypeFromRequire;
```

On 5.2.2 the `with` form is a syntax error, and the older `assert { "resolution-mode": … }` form fails with TS4125 "'resolution-mode' assertions are unstable. Use nightly TypeScript". In 5.3 both `with` and `assert` work for `resolution-mode`, and the attribute is honoured under `moduleResolution: bundler` and `node10` too (5.2: TS1452 "only supported when moduleResolution is node16 or nodenext"); under `classic` it is accepted and ignored.

**Bug in 5.3 (fixed in 5.8):** under `bundler` or `node10`, when one file imports the *same specifier* with both `"resolution-mode": "require"` and `"resolution-mode": "import"`, the second import reuses the first resolution. With the package above, `--moduleResolution bundler` resolves both to `cjs.d.ts` and reports `TS2305: Module '"pkg"' has no exported member 'TypeFromImport'` (reproduced on 5.3.3–5.7.3; correct on 5.8.3). `nodenext` is not affected. Workaround on 5.3: put the two imports in separate files.

Use `resolution-mode` only in declaration-heavy code (dual-package typings, `.d.ts` shims); application code should not need it.

***

#### New in 5.3: `switch (true)` narrowing

Each `case` condition in a `switch (true)` now narrows like an `if`. Fall-through accumulates (`string | any[]` below), and `default` receives the type narrowed by all cases that exited.[^1][^8]

```ts
export function describe(x: unknown): string {
  switch (true) {
    case typeof x === "string":
      return x.toUpperCase();          // x: string (5.2: TS18046 'x' is of type 'unknown')
    case Array.isArray(x):
      return `array of ${x.length}`;   // x: any[]
    case x instanceof Date:
      return x.toISOString();          // x: Date
    default:
      return String(x);
  }
}
```

**Limitation**: the *case expressions* are not narrowed by earlier cases, only the case bodies and `default` are. This still fails on 5.3 (and on 5.9):

```ts
type Shape = { kind: "circle"; r: number } | { kind: "square"; side: number };

export function area(s: Shape | null): number {
  switch (true) {
    case s === null:
      return 0;
    case s.kind === "circle":   // error TS18047
      return Math.PI * s.r ** 2;
    default:
      return s.side ** 2;       // fine: default is narrowed to the square
  }
}
```

TS18047 is "'s' is possibly 'null'". Write `case s !== null && s.kind === "circle":` or test `null` before the `switch`. For discriminated unions, a plain `switch (s.kind)` is still clearer and gives exhaustiveness checking; prefer `switch (true)` only for heterogeneous predicates.

***

#### New in 5.3: Narrowing on comparisons to booleans

Comparing a type-guard call or a boolean expression with `true`/`false` now narrows.[^1]

```ts
interface A { a: string }
interface B { b: string }
type AB = A | B;
function isA(x: AB): x is A { return "a" in x; }

export function f(x: AB, v: string | undefined) {
  if (isA(x) === true) console.log(x.a);          // 5.2: TS2339 Property 'a' does not exist on type 'AB'
  if (isA(x) === false) console.log(x.b);
  if (isA(x) !== true) console.log(x.b);
  if ((v !== undefined) === true) console.log(v.length);
}
```

The 5.3.2 implementation had a regression where `if (foo?.bar === false) {}` (with `bar: boolean`) narrowed `foo` to non-undefined *after* the `if` statement, hiding real "possibly undefined" errors; it was fixed in **5.3.3** (issue #56482).[^9] Projects pinned to exactly 5.3.2 should move to 5.3.3.

Style: `=== true` comparisons are still noise for plain booleans; they are useful when the operand may be `boolean | undefined` (where `=== false` differs from `!x`).

***

#### New in 5.3: `instanceof` narrowing through `Symbol.hasInstance`

When the right operand of `instanceof` has a `[Symbol.hasInstance]` method declared as a type predicate, 5.3 narrows the left operand by that predicate instead of to the class instance type.[^1][^10]

```ts
interface PointLike { x: number; y: number }

export class Point implements PointLike {
  constructor(public x: number, public y: number) {}
  distanceFromOrigin() { return Math.hypot(this.x, this.y); }
  static [Symbol.hasInstance](val: unknown): val is PointLike {
    return typeof val === "object" && val !== null && "x" in val && "y" in val &&
      typeof val.x === "number" && typeof val.y === "number";
  }
}

export function f(value: unknown) {
  if (value instanceof Point) {
    console.log(value.x + value.y);   // value: PointLike
    value.distanceFromOrigin();       // error TS2339
  }
}
```

On 5.2 the same code compiles and narrows `value` to `Point`, so `value.distanceFromOrigin()` passes the checker and throws at runtime for `{ x: 1, y: 2 }`. Upgrading to 5.3 surfaces those latent bugs as TS2339.

5.3 also accepts a non-constructor right operand that has `[Symbol.hasInstance]` (5.2: TS2359 "The right-hand side of an 'instanceof' expression must be of type 'any' or of a type assignable to the 'Function' interface type"). The left operand must still be `any`, `unknown`, an object type or a type parameter — a primitive union is rejected with TS2358:

```ts
// @run
const Even = {
  [Symbol.hasInstance](v: unknown): v is number {
    return typeof v === "number" && v % 2 === 0;
  },
};

export function half(v: unknown): number {
  if (v instanceof Even) return v / 2;   // v: number
  return NaN;
}
export function bad(v: string | number) {
  return v instanceof Even;               // error TS2358
}
console.log(half(4), half(3)); // 2 NaN
```

Review notes:

- A `Symbol.hasInstance` predicate is a hand-written type guard and as unsound as any `x is T` function: check that the body really proves the predicate.
- Overriding `Symbol.hasInstance` on a class changes `instanceof` for *all* callers, including libraries and `catch (e) { if (e instanceof MyError) … }`. Flag it unless it is deliberate and documented; prefer a named `isPointLike()` function.

***

#### New in 5.3: `super` property access checks

`super.x` reads from the base class *prototype*. Class fields (including arrow-function fields) live on the instance, so `super.field` is `undefined` at runtime. 5.3 reports this as **TS2855**; 5.2 accepted it.[^1][^11]

```ts
class Base {
  handle = () => "base";     // field, not a prototype method
  method() { return "m"; }
}

export class Derived extends Base {
  run() {
    super.method();          // fine: prototype method
    return super.handle();   // error TS2855
  }
}
```

TS2855: "Class field 'handle' defined by the parent class is not accessible in the child class via super." Emitted with `target: es2022` and run on Node.js 22, the call throws `TypeError: (intermediate value).handle is not a function`. Fixes: call `this.handle()`, or turn the base field into a method if subclasses need to extend it. This is a breaking change for upgrades from 5.2: every new TS2855 is a real runtime bug, not something to suppress.

***

#### New in 5.3: Editor, compiler and API changes

- **Interactive inlay hints**: type inlay hints are clickable and jump to the type's definition (needs a 5.3-based editor TypeScript).[^1]
- **`preferTypeOnlyAutoImports`**: editor preference (VS Code: `typescript.preferences.preferTypeOnlyAutoImports`) that makes auto-imports in type positions use `import { type X }` even without `verbatimModuleSyntax`. It first appears in 5.3's `UserPreferences`. The older `autoImportFileExcludePatterns` (4.8) still excludes paths from auto-import; `autoImportSpecifierExcludeRegexes` is 5.6 and not available.[^1]
- **JSDoc parsing mostly skipped by `tsc`**: the command-line compiler now parses with `JSDocParsingMode.ParseForTypeErrors`, which still parses JSDoc in JavaScript files but, in TypeScript files, only JSDoc comments containing `@see` or `@link`; this reduces parse time and memory. API consumers choose via the new `JSDocParsingMode` enum (`ParseAll`, `ParseNone`, `ParseForTypeErrors`, `ParseForTypeInfo`) on `createSourceFile` options, the compiler host or the language-service host; 5.3.3 also exposed it to `transpileModule` (issue #56625).[^1][^9] `ParseAll` remains the API default when no mode is given, so doc generators and lint rules that read JSDoc only lose it if they opt into another mode (or reuse `tsc`'s host settings).
- **Faster intersection comparisons**: assignability checks such as `T & (A | B | …)` → `T` use the original intersection form as a fast path.[^1]
- **`tsserverlibrary.js` → `typescript.js`**: `lib/tsserverlibrary.js` is now a ~1 KB re-export of `typescript.js` (which now contains the server API). Loading `tsserverlibrary.js` as a browser global throws; import `typescript` instead. Package size dropped by about 20 %.[^1][^12]
- **API d.ts types**: 5.3.2 redefined the deprecated `AssertClause`/`AssertEntry` API types as type aliases and broke compilation of `typescript.d.ts` under `exactOptionalPropertyTypes`; both were fixed in 5.3.3 (issues #56477, #56478).[^9]

***

#### TypeScript 5.3.x patch releases

| Version | npm publish date | Notes |
|---|---|---|
| 5.3.0-beta | 2023-10-03 | beta |
| 5.3.1-rc | 2023-11-03 | release candidate |
| 5.3.2 | 2023-11-20 | GA |
| 5.3.3 | 2023-12-06 | final: boolean-comparison narrowing regression (#56482), `typescript.d.ts` under `exactOptionalPropertyTypes` (#56478), `AssertClause`/`AssertEntry` back to interfaces (#56477), `JSDocParsingMode` for `transpileModule` (#56625) |

Always pin `~5.3.3` (or exactly `5.3.3`), never `5.3.2`.[^2][^9]

***
#### Do not use or recommend (not in TypeScript 5.3)

The project's compiler does not have any of the following. Each item was checked on 5.3.3 (error) and, for the 5.4 items, on 5.4.5 (accepted).[^13][^14]

| Feature | Introduced | What 5.3 says |
|---|---|---|
| `NoInfer<T>` utility type | 5.4 | TS2304 Cannot find name 'NoInfer' |
| Narrowing preserved in closures after the last assignment | 5.4 | closure sees the declared (wide) type |
| `Object.groupBy` / `Map.groupBy` declarations | 5.4 | TS2339 Property 'groupBy' does not exist |
| `--module preserve` (and `require()` under `bundler`) | 5.4 | TS6046 invalid `--module` value |
| Checked import attributes against the global `ImportAttributes` type | 5.4 | not checked |
| `Promise.withResolvers` declarations (`lib.esnext.promise`) | 5.4 | TS2339 |
| Inferred type predicates (`arr.filter(x => typeof x === "number")` narrowing to `number[]`) | 5.5 | element type stays the union |
| Control-flow narrowing for constant indexed accesses `obj[key]` | 5.5 | not narrowed |
| `Set.prototype.union`/`intersection`/`difference`/… declarations | 5.5 | TS2339 |
| `String.prototype.isWellFormed`/`toWellFormed` declarations | 5.5 | TS2339 |
| JSDoc `@import` tags, regular-expression syntax checking | 5.5 | not available |
| `--isolatedDeclarations`, `${configDir}` in tsconfig paths | 5.5 | TS5023 unknown option / literal string |
| `--target es2023` | 5.5 | TS6046 invalid `--target` value |
| Disallowed always-truthy/nullish checks (TS2869/TS2872) | 5.6 | not reported |
| Iterator helper declarations (`.map()`, `.filter()`, `.take()` on iterators), `IteratorObject`, `BuiltinIteratorReturn` | 5.6 | TS2339 Property 'map' does not exist on type 'IterableIterator<number>' |
| `--noCheck`, `--noUncheckedSideEffectImports`, `--strictBuiltinIteratorReturn`, `--stopBuildOnErrors` | 5.6 | TS5023 |
| Arbitrary module namespace identifiers (`export { x as "a-b" }`) | 5.6 | syntax error |
| `--rewriteRelativeImportExtensions`, `--target es2024`, `lib: ["es2024"]` (resizable `ArrayBuffer`, `SharedArrayBuffer.grow`, `Object.groupBy` moved to es2024) | 5.7 | TS5023 / TS6046; `new ArrayBuffer(8, { maxByteLength: 16 })` is TS2554 |
| Checks for never-initialized variables, `--module nodenext` requiring `with { type: "json" }` | 5.7 | not reported |
| `--erasableSyntaxOnly`, `--libReplacement`, `--module node18`, granular return-expression checks | 5.8 | TS5023 / TS6046 |
| `import defer`, `--module node20` | 5.9 | syntax error / TS6046 |
| TypeScript 6.0/7.0 defaults (`strict` on, `module: esnext`, `types: []`), `--stableTypeOrdering`, the native `tsgo` compiler | 6.0 / 7.0 | not applicable |

Note on targets: 5.3 accepts `--target` values `es3` (deprecated), `es5`, `es6`/`es2015` … `es2022` and `esnext`. There is **no `es2023` target** in 5.3, although `lib: ["es2023"]` exists. Use `target: "es2022"` and add `"es2023"` to `lib` when the runtime has ES2023 APIs (Node.js 20+).

Typical wrong suggestions to avoid in 5.3 code:

```ts
// @ts-config: {"lib": ["esnext"]}
export function pick<C extends string>(colors: C[], fallback?: NoInfer<C>) { // error TS2304
  return fallback ?? colors[0];
}
export const byParity = Object.groupBy([1, 2, 3], (n) => (n % 2 ? "odd" : "even")); // error TS2339, TS7006
export const nums: number[] = [1, "a"].filter((x) => typeof x === "number");       // error TS2322
export const evens = [1, 2].values().map((x) => x * 2);                           // error TS2339, TS7006
export const both = new Set([1]).union(new Set([2]));                            // error TS2339
export const deferred = Promise.withResolvers<number>();                         // error TS2339
```

5.3-compatible equivalents:

```ts
// The classic NoInfer workaround: put the parameter in a deferred conditional type.
type NoInferCompat<T> = [T][T extends any ? 0 : never];

export function pick<C extends string>(colors: C[], fallback?: NoInferCompat<C>): C {
  return fallback ?? colors[0];
}
pick(["red", "green"], "red");
pick(["red", "green"], "blue");  // error TS2345

// groupBy without Object.groupBy types (works on every runtime):
export function groupBy<T, K extends PropertyKey>(items: readonly T[], key: (t: T) => K): Partial<Record<K, T[]>> {
  const out: Partial<Record<K, T[]>> = {};
  for (const item of items) (out[key(item)] ??= []).push(item);
  return out;
}

// filter with an explicit type predicate (no inferred predicates before 5.5):
export const nums = [1, "a"].filter((x): x is number => typeof x === "number");

// Promise.withResolvers replacement:
export function deferred<T>() {
  let resolve!: (v: T) => void;
  let reject!: (e: unknown) => void;
  const promise = new Promise<T>((res, rej) => { resolve = res; reject = rej; });
  return { promise, resolve, reject };
}
```

Even when the runtime has `Object.groupBy` (Node.js 21+), 5.3 has no declaration for it; a hand-written `declare global` augmentation is acceptable, but the cleaner fix is a TypeScript upgrade.

***

#### Deprecated in 5.3: the 5.0 deprecation set

TypeScript 5.0 deprecated the following options; in 5.0–5.4 each one is an **error (TS5101/TS5107) unless `"ignoreDeprecations": "5.0"` is set**, and from 5.5 on they no longer have any effect (5.5 still reports them unless `ignoreDeprecations` is set; verified on 5.5.4).[^15][^19] On 5.3.3:

- `target: "ES3"` (TS5107), `out`, `noImplicitUseStrict`, `keyofStringsOnly`, `suppressExcessPropertyErrors`, `suppressImplicitAnyIndexErrors`, `noStrictGenericChecks`, `charset`, `importsNotUsedAsValues`, `preserveValueImports`, and `prepend` in project references.
- `"ignoreDeprecations": "5.0"` is the only valid value in 5.3 (`"5.3"` or `"6.0"` give TS5103 "Invalid value").
- Silencing them is a stop-gap: a 5.3 codebase with `ignoreDeprecations` cannot move to 5.5+ without removing the options. Replace `importsNotUsedAsValues`/`preserveValueImports` with `verbatimModuleSyntax`, `suppressImplicitAnyIndexErrors` with real index-signature types, `out` with `outFile`.
- `keyofStringsOnly` plus `ignoreDeprecations` breaks the bundled `lib.decorators.d.ts` (TS2344) unless `skipLibCheck` is on — more evidence the option is dead.

Still valid and not deprecated in 5.3 (they are deprecated only in 6.0): `moduleResolution: "node10"`/`"node"`/`"classic"`, `module: "amd"`/`"umd"`/`"system"`, `baseUrl`, `esModuleInterop: false`, `downlevelIteration`, `target: "es5"`, `outFile`. Do not report them as deprecated in a 5.3 project, although `bundler`/`nodenext` are the better choices for new code.

`newLine` defaults to LF and `forceConsistentCasingInFileNames` defaults to `true` since 5.0.

***
#### Compiler configuration for 5.3

##### Recommended `tsconfig.json` (Node.js ESM service)

```jsonc
{
  "compilerOptions": {
    // Language and runtime
    "target": "es2022",               // highest target in 5.3 besides esnext; fine for Node.js 18+
    "lib": ["es2023"],                // add "dom" only for browser code; never add esnext-only APIs the runtime lacks
    "module": "nodenext",             // follows package.json "type" and .mts/.cts; without "target" it would imply esnext
    // "moduleResolution" is implied by module: nodenext
    "types": ["node"],                // pin @types/node to a 5.3-compatible version (see Tooling)

    // Type checking
    "strict": true,
    "noUncheckedIndexedAccess": true,
    "exactOptionalPropertyTypes": true,
    "noImplicitOverride": true,
    "noImplicitReturns": true,
    "noFallthroughCasesInSwitch": true,
    "noPropertyAccessFromIndexSignature": true,
    "useUnknownInCatchVariables": true, // already part of strict; listed for emphasis

    // Modules and emit
    "verbatimModuleSyntax": true,     // 5.0+: imports/exports emitted exactly as written (minus type-only ones)
    "isolatedModules": true,          // implied by verbatimModuleSyntax for most checks; required by esbuild/swc/tsx
    "resolveJsonModule": true,
    "declaration": true,
    "sourceMap": true,
    "outDir": "dist",
    "rootDir": "src",

    // Hygiene
    "forceConsistentCasingInFileNames": true, // default true since 5.0
    "skipLibCheck": true,
    "incremental": true
  },
  "include": ["src"]
}
```

Bundled front-end code (Vite, esbuild, webpack) uses `"module": "esnext"`, `"moduleResolution": "bundler"` and usually `"noEmit": true` (the bundler emits); `--module preserve` does not exist in 5.3. A library published to npm should be checked with `module: nodenext` even if it is bundled, because consumers resolve it with Node.js rules.

Review notes:

- `strict: false` (or missing) in a production codebase is a finding. Turning it on in one step is risky; per-flag enablement (`strictNullChecks` first) is acceptable.
- `skipLibCheck: true` is normal; it hides errors in `.d.ts` files, including *your own* emitted declarations. Libraries should run one CI job without it.
- `"lib"` must match the oldest deployment runtime. `lib: ["esnext"]` on a Node.js 18 service lets `Array.prototype.toSorted` (Node.js 20+) type-check and crash at runtime.
- `paths` only affects type checking and does **not** rewrite emitted specifiers; the runtime or bundler needs the same mapping (or use package.json `imports` with `#` specifiers, which Node.js understands natively).
- `"extends"` accepts an array of configs since 5.0; later entries override earlier ones.

##### Strictness flags (5.3)

| Flag | In `strict` | Catches |
|---|---|---|
| `noImplicitAny` | yes | parameters/variables silently typed `any` |
| `strictNullChecks` | yes | `null`/`undefined` not in every type; without it most nullability bugs are invisible |
| `strictFunctionTypes` | yes | contravariant checking of function-typed *properties* (methods stay bivariant) |
| `strictBindCallApply` | yes | typed `bind`/`call`/`apply` |
| `strictPropertyInitialization` | yes | class fields not assigned in the constructor |
| `noImplicitThis` | yes | `this` of type `any` |
| `alwaysStrict` | yes | emits `"use strict"` |
| `useUnknownInCatchVariables` | yes (4.4+) | `catch (e)` is `unknown`, not `any` |
| `noUncheckedIndexedAccess` | no | `arr[i]` and `record[key]` include `undefined` |
| `exactOptionalPropertyTypes` | no | `{ a?: string }` no longer accepts an explicit `a: undefined` |
| `noImplicitOverride` | no | overriding without `override` |
| `noPropertyAccessFromIndexSignature` | no | forces `obj["key"]` for index-signature access |
| `noImplicitReturns`, `noFallthroughCasesInSwitch` | no | missing returns, accidental fall-through |
| `noUnusedLocals`, `noUnusedParameters` | no | dead code (often delegated to the linter) |

`--strictBuiltinIteratorReturn` (5.6) does not exist in 5.3.

***

#### Code style and house rules

These conventions are not enforced by the compiler but keep 5.3 code reviewable:

- **Naming**: `PascalCase` for types, interfaces, classes, enums and type parameters with meaning (`TItem`, or `T`/`K`/`V` for short generic helpers); `camelCase` for values and functions; `UPPER_SNAKE_CASE` only for true module-level constants. No `I` prefix on interfaces.
- **`interface` vs `type`**: use `interface` for object shapes that may be extended or implemented (they merge and give clearer error messages); use `type` for unions, intersections, mapped/conditional types, tuples and function types. Do not mix both for the same concept.
- **Named exports** over `export default` (renames propagate; auto-import is reliable). Framework conventions (Next.js pages, config files) are the exception.
- **Explicit types at module boundaries** (exported functions, public class members); rely on inference inside functions. `const items = []` without annotation is `any[]` under `noImplicitAny: false` and evolving under `strict`; annotate empty collections.
- **Readonly by default** for parameters and data structures that are not mutated (`readonly T[]`, `ReadonlyMap`, `Readonly<T>`).
- **No `namespace`** in modules (use ES modules); `declare namespace` stays acceptable in `.d.ts` files for global augmentation.
- **Imports** grouped as built-ins (`node:` prefix), external packages, internal aliases, relative; let the formatter or `eslint-plugin-import`/`simple-import-sort` order them. Type-only imports marked with `type`.
- **Comments** explain why, not what; JSDoc on exported API (`/** … */`), including `@deprecated`, which editors surface. Note that `tsc` 5.3 skips most JSDoc in `.ts` files (it keeps comments with `@see`/`@link`), which does not affect editors or type checking.

```ts
// Result type instead of throwing for expected failures
export type Result<T, E = Error> = { ok: true; value: T } | { ok: false; error: E };

export function divide(a: number, b: number): Result<number, "division-by-zero"> {
  return b === 0 ? { ok: false, error: "division-by-zero" } : { ok: true, value: a / b };
}

// Builder returning `this` keeps subclass types
export class QueryBuilder<T> {
  private readonly filters: Array<(item: T) => boolean> = [];
  where(predicate: (item: T) => boolean): this {
    this.filters.push(predicate);
    return this;
  }
  run(items: readonly T[]): T[] {
    return items.filter((item) => this.filters.every((f) => f(item)));
  }
}
```

***

#### Types: `any`, `unknown`, `never` and assertions

`any` disables checking in both directions and spreads silently; `unknown` forces narrowing before use. External data — `JSON.parse`, `response.json()`, `process.env`, message payloads — enters as `any` or `string` and must be validated before it is treated as a domain type.

```ts
interface User { id: string; age: number }

// BAD: JSON.parse returns any; the assertion is a lie the compiler cannot check
export function parseUserBad(raw: string): User {
  return JSON.parse(raw) as User;
}

// GOOD: parse into unknown, then prove the shape
export function parseUser(raw: string): User {
  const data: unknown = JSON.parse(raw);
  if (
    typeof data === "object" && data !== null &&
    "id" in data && typeof data.id === "string" &&
    "age" in data && typeof data.age === "number"
  ) {
    return { id: data.id, age: data.age };
  }
  throw new TypeError("invalid user payload");
}
```

`"id" in data` narrows `data` to `object & Record<"id", unknown>` (since 4.9), which makes hand-written validators practical; schema libraries (zod, valibot, typebox, ajv with JSON Schema) are the scalable choice.

Rules for reviews:

- `as T` is acceptable only when the author has information the compiler lacks (a validated value, a DOM element known to exist). `as unknown as T` (double assertion) is almost always a bug.
- `x!` (non-null assertion) hides `undefined`; prefer a check that throws with a message.
- `// @ts-ignore` silences *any* error on the next line, including future ones; use `// @ts-expect-error` with a reason, which fails when the error disappears.
- `Function`, `Object` and `{}` are not "any object": `{}` accepts every non-nullish value including `1` and `"x"`. Use `object`, `Record<string, unknown>` or a concrete type.

```ts
const a: {} = 42;                  // compiles: {} means "not null/undefined"
const b: object = 42;              // error TS2322
const c: Record<string, unknown> = { x: 1 };
export { a, b, c };
```

##### `catch` variables are `unknown`

```ts
export async function load(url: string): Promise<string> {
  try {
    const res = await fetch(url);
    if (!res.ok) throw new Error(`HTTP ${res.status}`);
    return await res.text();
  } catch (err) {
    console.error(err.message);                       // error TS18046
    if (err instanceof Error) console.error(err.message);
    throw new Error(`load failed: ${url}`, { cause: err }); // ErrorOptions (lib es2022)
  }
}
```

Note `return await` inside `try`: without `await`, a rejected promise escapes the `catch`.

##### Exhaustiveness with `never`

```ts
type Event =
  | { type: "created"; id: string }
  | { type: "deleted"; id: string; soft: boolean };

function assertNever(x: never): never {
  throw new Error(`unhandled: ${JSON.stringify(x)}`);
}

export function handle(e: Event): string {
  switch (e.type) {
    case "created":
      return `+${e.id}`;
    case "deleted":
      return e.soft ? `~${e.id}` : `-${e.id}`;
    default:
      return assertNever(e); // adding a variant to Event makes this line fail to compile
  }
}
```

A `switch` without `default` that returns from every case also errors on a missing case under `noImplicitReturns` / an explicit return type, but `assertNever` additionally throws on unexpected runtime values (e.g. from an older client).

***

#### Narrowing (cumulative to 5.3)

TypeScript narrows on `typeof`, truthiness, `===`/`!==` (including `== null`), `in`, `instanceof`, discriminant properties, type predicates (`x is T`), assertion functions (`asserts x is T`), assignments, and — new in 5.3 — `switch (true)` cases, comparisons with `true`/`false`, and `Symbol.hasInstance` predicates.

```ts
type Result<T> = { ok: true; value: T } | { ok: false; error: Error };

export function unwrap<T>(r: Result<T>): T {
  if (!r.ok) throw r.error;          // discriminant narrowing
  return r.value;
}

function assertIsString(v: unknown, name: string): asserts v is string {
  if (typeof v !== "string") throw new TypeError(`${name} must be a string`);
}

export function env(name: string): string {
  const v = process.env[name];
  assertIsString(v, name);
  return v;                          // string
}

// Destructured discriminated unions narrow too (4.6+), when the binding is const
export function msg(r: Result<number>): string {
  const { ok } = r;
  if (ok) return String(r.value);
  return r.error.message;
}
```

Pitfalls:

- **Type predicates are unchecked.** `function isUser(x: unknown): x is User { return true; }` compiles. Review the body of every predicate and assertion function.
- **Truthiness narrowing drops valid values**: `if (count)` excludes `0`, `if (name)` excludes `""`. Use `!== undefined` / `!= null`.
- **`typeof null === "object"`**: `typeof x === "object"` keeps `null` in the type; TypeScript models this, but hand-written JS checks often forget it.
- **Narrowing does not survive callbacks for `let` variables and parameters in 5.3** (next section).

##### Closures and narrowing in 5.3 (no "preserved narrowing")

Narrowing of a `let` variable or parameter is reset inside closures created later, because the closure may run after a reassignment. TypeScript 5.4 keeps the narrowing when there are no assignments after the closure is created; **5.3 does not**.[^13]

```ts
export function urlFor(input: string | URL, names: string[]) {
  if (typeof input === "string") input = new URL(input);
  return names.map((n) => input.searchParams.get(n)); // error TS2339
}

export function urlForFixed(raw: string | URL, names: string[]) {
  const input = typeof raw === "string" ? new URL(raw) : raw;   // const binding: narrowing is stable
  return names.map((n) => input.searchParams.get(n));
}
```

TS2339 here is "Property 'searchParams' does not exist on type 'string | URL'". The correct 5.3 fix is a `const` copy, not `input!`/`as URL` and not "upgrade-only" advice.

***

#### `satisfies`, `as const` and `const` type parameters

`satisfies` (4.9) checks a value against a type without widening or replacing its inferred type; `as const` makes literals readonly and literal-typed; `const` type parameters (5.0) make a generic infer as if the caller had written `as const`.

```ts
type Route = { path: string; method: "GET" | "POST" };

// BAD: annotation widens - keys and literal values are lost
const routesWide: Record<string, Route> = {
  list: { path: "/users", method: "GET" },
};
routesWide.missing.path;                 // compiles, crashes at runtime

// GOOD: satisfies checks the shape and keeps the precise type
const routes = {
  list: { path: "/users", method: "GET" },
  create: { path: "/users", method: "POST" },
} satisfies Record<string, Route>;
routes.list.method;                      // "GET"
routes.missing;                          // error TS2339

const bad = { x: { path: "/", method: "PUT" } } satisfies Record<string, Route>; // error TS2322

// const type parameter: literal inference without as const at call sites
function defineRoles<const T extends readonly string[]>(roles: T): T { return roles; }
export const roles = defineRoles(["admin", "user"]); // readonly ["admin", "user"]

export { routes, bad };
```

```ts
// New in 5.3 (undocumented): mutable array constraints keep the literal tuple
function tuple<const T extends string[]>(x: T): T { return x; }
export const t = tuple(["a", "b"]);                          // 5.3: ["a", "b"]   (5.2: string[])
export const check: ["a", "b"] = t;                          // 5.2: TS2322
export const arr = ["a", "b"] as const satisfies string[];   // 5.3: ["a", "b"]   (5.2: TS1360)
```

Notes:

- `const T` only affects inference for literals written *at the call site*; an array variable passed in keeps its declared type.
- **Changed in 5.3 (not in the release notes)**: with a *mutable* array constraint (`<const T extends string[]>`), 5.0–5.2 fell back to the constraint (`string[]`); 5.3 infers a mutable tuple (`["a", "b"]`). Likewise `["a", "b"] as const satisfies string[]` was TS1360 on 5.2 and is accepted on 5.3 with the type `["a", "b"]` (mutable tuple). Use a `readonly` constraint (`readonly string[]`) when the result must stay readonly — that form works the same on every version.[^20]
- `as const` does not freeze the runtime object; use `Object.freeze` too when mutation must be prevented.
- `satisfies` does not change excess-property behavior of nested types; an object literal with an unknown key still errors.

***

#### Generics

```ts
// GOOD: the type parameter relates input and output
export function first<T>(items: readonly T[]): T | undefined {
  return items[0];
}

// BAD: T appears once - it is just an unchecked cast in disguise
export function parse<T>(json: string): T {
  return JSON.parse(json);
}
const n = parse<number>('"not a number"'); // compiles; n is a string at runtime
export { n };

// Constraints and keyof
export function pluck<T, K extends keyof T>(items: readonly T[], key: K): T[K][] {
  return items.map((i) => i[key]);
}

// Defaults and inference from multiple sites
export function merge<A extends object, B extends object>(a: A, b: B): A & B {
  return { ...a, ...b };
}
```

Rules:

- A type parameter used only in the return type (`parse<T>`) is a disguised `any`; return `unknown` and validate instead.
- Prefer `readonly T[]` for parameters you do not mutate; it accepts both mutable and readonly arrays.
- Spreading (`{ ...a, ...b }`) is typed `A & B`, which is wrong when `b` has optional keys set to `undefined`; with `exactOptionalPropertyTypes` the intent is clearer.
- When inference picks a candidate from the wrong argument, 5.3 has no `NoInfer`; use the `[T][T extends any ? 0 : never]` trick shown above or a second type parameter `U extends T`.
- Variance annotations (`in`/`out` on type parameters, 4.7) are for performance in large recursive types. They are checked only partly: `type Fn<out T> = (v: T) => void` is TS2636, but `interface Box<out T> { set(v: T): void }` compiles (method bivariance) and then makes `Box<string>` unassignable to `Box<"a">`. They rarely belong in application code.

##### Conditional, mapped and template literal types

```ts
// Key remapping (4.1) and template literal types
type Getters<T> = { [K in keyof T & string as `get${Capitalize<K>}`]: () => T[K] };
type UserGetters = Getters<{ name: string; age: number }>;
export const g: UserGetters = { getName: () => "a", getAge: () => 1 };

// infer with extends constraint (4.7)
type FirstString<T> = T extends [infer S extends string, ...unknown[]] ? S : never;
export type X = FirstString<["a", 1]>; // "a"

// Distributive conditional types - wrap in [] to stop distribution
type ToArray<T> = T extends unknown ? T[] : never;
type ToArrayND<T> = [T] extends [unknown] ? T[] : never;
export type D1 = ToArray<string | number>;   // string[] | number[]
export type D2 = ToArrayND<string | number>; // (string | number)[]
```

Deeply recursive conditional types slow every editor keystroke and can hit "Type instantiation is excessively deep" (TS2589). Flag clever type-level code that has no consumer benefit.

***

#### Objects, index signatures and optional properties

```ts
// @ts-config: {"noUncheckedIndexedAccess": true, "exactOptionalPropertyTypes": true}
const counts: Record<string, number> = {};
export function bump(key: string) {
  counts[key] += 1;                       // BAD: NOT reported by 5.3 (stores NaN for a new key)
  counts[key]++;                          // BAD: NOT reported by 5.3 either
  const next: number = counts[key] + 1;   // error TS2532
  counts[key] = (counts[key] ?? 0) + 1;   // GOOD
  return next;
}

const list = [1, 2, 3];
export const firstItem: number = list[0]; // error TS2322

interface Options { timeoutMs?: number }
export const o1: Options = {};
export const o2: Options = { timeoutMs: undefined }; // error TS2375
```

TS2532 is "Object is possibly 'undefined'"; TS2375 is the `exactOptionalPropertyTypes` assignability error. Without `noUncheckedIndexedAccess`, every index access above compiles.

**5.3 gap (verified, fixed in 5.5):** `noUncheckedIndexedAccess` does not check compound assignments and increments on index accesses. `counts[key] += 1` and `counts[key]++` compile cleanly on 5.3.3 and 5.4.5 and produce `NaN` for a missing key; 5.5.4 reports TS2532 on both. In a 5.3 codebase, the flag is no protection for counters — review `obj[k] += …`, `obj[k] -= …` and `obj[k]++` by hand and prefer `obj[k] = (obj[k] ?? 0) + 1`.

Further rules:

- **Excess-property checks apply only to fresh object literals.** Passing a variable with extra keys is allowed; a typo in an optional property of a literal is caught, the same typo via an intermediate variable is not.
- **`readonly` is shallow and erasable.** `Readonly<T>` does not freeze nested objects, and a `readonly` array can be passed to a function that takes a mutable array only through an assertion — flag such assertions.
- **Prototype keys**: a `Record<string, T>` built from user input is vulnerable to `__proto__`/`constructor` keys. Use `Map`, `Object.create(null)`, or validate keys.
- **`Object.keys(obj)` returns `string[]`**, not `(keyof T)[]`, because objects may have extra keys at runtime. Casting it to `(keyof T)[]` is a claim to justify.
- **`noPropertyAccessFromIndexSignature`** forces `env["PORT"]` instead of `env.PORT` for index signatures, making "this key may be absent" visible.

##### Arrays and tuples

```ts
// Labeled and unlabeled tuple elements may be mixed since 5.2
type Pair = [first: string, number];
export const p: Pair = ["a", 1];

// Methods on unions of arrays are callable since 5.2 (the array is treated as (string | number)[])
declare const arr: string[] | number[];
export const filtered = arr.filter((x) => typeof x === "string"); // (string | number)[]

// ES2023 copying methods (lib es2023): typed in 5.3, available at runtime from Node.js 20
export const sorted = [3, 1, 2].toSorted((a, b) => a - b);
export const replaced = [1, 2, 3].with(0, 9);
```

`filter` with a plain arrow function does not narrow the element type in 5.3 (inferred type predicates are 5.5); write `(x): x is string => typeof x === "string"`.

***
#### Utility types, branded types and their pitfalls

Built-ins available in 5.3: `Partial`, `Required`, `Readonly`, `Record`, `Pick`, `Omit`, `Exclude`, `Extract`, `NonNullable`, `Parameters`, `ConstructorParameters`, `ReturnType`, `InstanceType`, `ThisParameterType`, `OmitThisParameter`, `ThisType`, `Awaited` (4.5), `Uppercase`/`Lowercase`/`Capitalize`/`Uncapitalize`. `NoInfer` is 5.4.

```ts
type Shape =
  | { kind: "circle"; r: number; id: string }
  | { kind: "square"; side: number; id: string };

// Pitfall: Omit is not distributive - on a union it keeps only the common keys
type NoId = Omit<Shape, "id">;
declare const a: NoId;
export const r = a.r;                     // error TS2339

// Distributive version
type DistributiveOmit<T, K extends PropertyKey> = T extends unknown ? Omit<T, K> : never;
declare const b: DistributiveOmit<Shape, "id">;
export const size = b.kind === "circle" ? b.r : b.side;

// Branded (nominal) types for IDs and units
type UserId = string & { readonly __brand: "UserId" };
export const toUserId = (raw: string): UserId => {
  if (!/^[a-z0-9-]{8,}$/.test(raw)) throw new Error("bad id");
  return raw as UserId;                   // the one place where the cast is justified
};
declare function loadUser(id: UserId): void;
loadUser("abc");                          // error TS2345
loadUser(toUserId("abcdefgh"));

export type N = Awaited<Promise<Promise<number>>>; // number
```

- `Partial<T>` for update payloads lets every field be `undefined`; with `exactOptionalPropertyTypes` it at least distinguishes "absent" from "set to undefined". Validate patches before applying them.
- `Omit` does not check its keys (`Omit<T, "nonexistent">` compiles because the key parameter is `K extends keyof any`), so renaming a property silently stops omitting it; `Pick` is checked (TS2344). See the example in [Collections](#collections-iteration-and-common-runtime-traps).
- `ReturnType<typeof fn>` couples types to implementation details; prefer a named exported type for public APIs.
- `Record<string, T>` claims every string key exists; with `noUncheckedIndexedAccess` lookups are `T | undefined`, without it they are lies.

***

#### Declaration files, augmentation and globals

```ts
// @file: env.d.ts
declare global {
  namespace NodeJS {
    interface ProcessEnv {
      readonly DATABASE_URL?: string;
    }
  }
}
export {};
// @file: main.ts
export const url: string | undefined = process.env.DATABASE_URL;
export const typo = process.env.DATABSE_URL;   // still compiles: ProcessEnv has an index signature
```

- Augmenting `ProcessEnv` documents variables but does not catch typos (the index signature remains) or guarantee presence; parse `process.env` once at startup into a validated config object.
- `declare module "pkg" { … }` inside a file that has imports/exports *augments* the module; in a file without them it *declares* an ambient module that takes precedence over the package in `node_modules` — a common accident that hides the real package types (verified: with a stray `declare module "foo" { export const fake: string }`, importing the package's real export `real` is TS2305).
- `declare const X: T` claims something exists at runtime; review every `declare` in non-`.d.ts` files.
- Wildcard modules (`declare module "*.svg" { const url: string; export default url; }`) are fine for bundler assets; `allowArbitraryExtensions` (5.0) with `foo.d.svg.ts` files gives per-file declarations.
- `skipLibCheck` also skips *your* `.d.ts` files; keep hand-written declarations small and tested by usage.
- `/// <reference types="…" />` in source files pulls in globals for every file in the program; prefer `types` in tsconfig.

##### CommonJS interop under `nodenext`

- From an ES module, `import pkg from "cjs-pkg"` binds `module.exports` (Node.js semantics); `import { named } from "cjs-pkg"` works only for names that Node.js's static analysis detects. TypeScript 5.3 models the default import correctly under `nodenext` but trusts the package's `.d.ts` for named exports; prefer the default import for CJS-only packages with `export =` typings.
- `esModuleInterop` is implied by `module: node16`/`nodenext` (verified: a CommonJS file emitted by 5.3 with only `module: nodenext` wraps `require("legacy")` in `__importDefault`, and the call works). With `module: commonjs` it defaults to `false`, and a default import of an `export =` package is TS1259 ("can only be default-imported using the 'esModuleInterop' flag"). The dangerous combination is `allowSyntheticDefaultImports: true` with `esModuleInterop: false`: it type-checks default imports but emits no interop helper, so `legacy.default` is `undefined` at runtime.
- In `.cts` files or `"type": "commonjs"` packages under `verbatimModuleSyntax`, use `import x = require("x")` and `export =`.

***

#### Enums and constant objects

Since 5.0 every enum is a union enum: each member has its own literal type, and assigning an out-of-range literal to a numeric enum is an error. Enums are *not* erasable syntax — they emit a runtime object (except `const enum`) — and they are nominal: a string enum does not accept the equal string literal.

```ts
export enum Status { Active = "active", Disabled = "disabled" }
export enum Level { Low = 1, High = 2 }

export const s1: Status = Status.Active;
export const s2: Status = "active";        // error TS2322
export const l1: Level = 2;                // allowed: numeric enums accept matching numbers
export const l2: Level = 7;                // error TS2322 (since 5.0)
declare const n: number;
export const l3: Level = n;                // allowed: any number is assignable to a numeric enum

// Preferred for new code: a const object plus a derived union type
export const Role = { Admin: "admin", User: "user" } as const;
export type Role = (typeof Role)[keyof typeof Role]; // "admin" | "user"
export const r: Role = "admin";            // plain strings from JSON fit without casts
```

Review notes:

- Numeric enums accept any `number` (`l3`), so they are no validation of external input; check with `Object.values(Level).includes(x)` style guards or use a union.
- Numeric enums have reverse mappings (`Level[1] === "Low"`), so `Object.keys(Level)` returns both names and numbers.
- `const enum` is inlined only by a whole-program `tsc` build. Under `isolatedModules` (required by esbuild, swc, tsx, Vite) a `const enum` *declared in an ambient `.d.ts`* cannot be referenced (TS2748 "Cannot access ambient const enums when 'isolatedModules' is enabled"), and under `isolatedModules` `tsc` 5.3 itself emits even file-local `const enum`s as ordinary enum objects with property lookups (verified: `export const y = L.A;` instead of the `1 /* L.A */` it inlines without `isolatedModules`). Do not export `const enum` from libraries.
- Adding a member in the middle of an auto-numbered enum renumbers the rest; persisted values (databases, wire formats) must use explicit initializers.

***

#### Classes

##### Field semantics: `useDefineForClassFields`

With `target: "es2022"` or `esnext`, `useDefineForClassFields` defaults to `true`: fields are emitted as real class fields with `[[Define]]` semantics. A field re-declared in a subclass *without an initializer* is reset to `undefined` after the base constructor ran. 5.3 reports the direct case (TS2612), but not when the base property comes from an interface merge, a mixin or an untyped base (`any`, JavaScript):

```ts
// @run
class Base {
  name: string;
  constructor() { this.name = "set by base"; }
}
class Broken extends Base {
  name!: string;              // error TS2612
}
class Fixed extends Base {
  declare name: string;       // GOOD: type-only redeclaration, no emit
}

interface HasId { id: number }
class Entity { constructor() { (this as unknown as HasId).id = 42; } } // e.g. an ORM base
interface Entity extends HasId {}
class Order extends Entity {
  id!: number;                // BAD: no error, but resets id to undefined at runtime
}
console.log(new Broken().name, "|", new Fixed().name, "|", new Order().id); // undefined | set by base | undefined
```

TS2612: "Property 'name' will overwrite the base property in 'Base'. If this is intentional, add an initializer. Otherwise, add a 'declare' modifier or remove the redundant declaration." When reviewing an upgrade from `target: es2017`/`es2020` (where the default was `false`) to `es2022`, look for subclass field redeclarations (use `declare`), for framework-assigned fields, and for field initializers that read constructor parameters or other fields — initializers run in declaration order right after `super()` returns, before the rest of the constructor body.

##### Modifiers and members

```ts
// @ts-config: {"noImplicitOverride": true}
export abstract class Repo<T extends { id: string }> {
  readonly #items = new Map<string, T>();     // true runtime privacy (ES private field)
  protected abstract validate(item: T): void;
  save(item: T): void { this.validate(item); this.#items.set(item.id, item); }
  get size(): number { return this.#items.size; }
}

export class UserRepo extends Repo<{ id: string; email: string }> {
  protected override validate(u: { id: string; email: string }) {
    if (!u.email.includes("@")) throw new Error("bad email");
  }
  save(item: { id: string; email: string }): void { super.save(item); } // error TS4114
}

// Auto-accessors (4.9): `accessor` creates a private storage slot plus get/set
export class Counter {
  accessor count = 0;
}
```

TS4114: "This member must have an 'override' modifier because it overrides a member in the base class 'Repo<…>'".

- `private` / `protected` are compile-time only (`(obj as any).secret` works); `#private` is enforced at runtime (reading it through a `Proxy` of the instance throws a `TypeError`, which matters for mocking and reactive-proxy frameworks). Use `#` for secrets and invariants.
- Parameter properties (`constructor(private readonly db: Db)`) are TypeScript-only emit; they are allowed in 5.3 (`--erasableSyntaxOnly` that forbids them is 5.8).
- `implements` checks shape only; it does not change the class's types (method parameters still need their own annotations).
- Getter and setter of a property may have unrelated types since 5.1 (`get size(): number` / `set size(v: string | number)`), which is legitimate for normalizing setters.

##### Decorators: standard (5.0) versus `experimentalDecorators`

Without `experimentalDecorators`, TypeScript 5.3 implements the TC39 standard decorators (Stage 3): decorators receive `(value, context)` with a typed context object (`ClassMethodDecoratorContext`, `ClassFieldDecoratorContext`, `ClassAccessorDecoratorContext`, …). Parameter decorators and `emitDecoratorMetadata` exist only in the legacy `experimentalDecorators` mode, which frameworks such as NestJS, TypeORM and InversifyJS require. The two modes have incompatible signatures; mixing libraries of both kinds in one project does not work.[^16]

```ts
export function logged<This, Args extends unknown[], R>(
  target: (this: This, ...args: Args) => R,
  context: ClassMethodDecoratorContext<This, (this: This, ...args: Args) => R>,
) {
  const name = String(context.name);
  return function (this: This, ...args: Args): R {
    console.log(`-> ${name}`);
    return target.call(this, ...args);
  };
}

export class Service {
  @logged greet(who: string) { return `hello ${who}`; }
}
```

**Decorator metadata (5.2) needs a runtime polyfill.** `context.metadata` is typed as `DecoratorMetadataObject` (never `undefined`), but the emitted helper only creates it when `Symbol.metadata` exists. No Node.js release up to 26 defines `Symbol.metadata` (verified: `typeof Symbol.metadata === "undefined"` on Node.js 18, 20, 22, 24 and 26.12.1). Without a polyfill, `context.metadata.x = …` throws `TypeError: Cannot set properties of undefined` while the class is being defined:

```ts
// @run
// @file: polyfill.ts
(Symbol as { metadata?: symbol }).metadata ??= Symbol("Symbol.metadata");
export {};
// @file: main.ts
import "./polyfill.js"; // must be evaluated before any decorated class module
const { Tagged } = await import("./model.js");
console.log(Object.keys((Tagged as any)[Symbol.metadata!] ?? {})); // [ 'tagged' ]
// @file: model.ts
function tag(_: unknown, context: ClassDecoratorContext) {
  context.metadata.tagged = true;
}
@tag
export class Tagged {}
```

ESM `import` statements are hoisted and evaluated in order, so the polyfill must be a separate module imported first (or loaded with `node --import`). Review rule: any use of `context.metadata` needs a visible polyfill or a runtime known to provide `Symbol.metadata`.

Further decorator rules:

- Standard decorators cannot decorate parameters: `constructor(@Inject() x: Foo)` fails with TS1206 "Decorators are not valid here" unless `experimentalDecorators` is on.
- `experimentalDecorators` + `emitDecoratorMetadata` emit `design:paramtypes`/`design:type` metadata that depends on `reflect-metadata` and on *value* imports of the annotated types. If a constructor parameter's class is imported with `import type` (for example after a `consistent-type-imports` auto-fix), 5.3 silently emits `__metadata("design:paramtypes", [Function])` instead of the class (verified), and constructor injection fails at runtime. Keep DI-injected classes as value imports.
- With standard decorators and `target` below ES2022, `tsc` emits helpers (`__esDecorate`, `__runInitializers`); with `importHelpers` this needs `tslib` ≥ 2.5.

##### Explicit resource management: `using` and `await using` (5.2)

`using` disposes a resource at the end of the block, in reverse declaration order, even on exceptions; `await using` awaits `[Symbol.asyncDispose]()`.[^17]

```ts
// @run
class TempFile implements Disposable {
  constructor(readonly path: string) { console.log("open", path); }
  [Symbol.dispose]() { console.log("close", this.path); }
}
class Conn implements AsyncDisposable {
  async [Symbol.asyncDispose]() { console.log("conn closed"); }
}

async function job() {
  await using conn = new Conn();
  using a = new TempFile("a");
  using b = new TempFile("b");
  console.log("work", a.path, b.path, conn instanceof Conn);
}
await job(); // open a, open b, work a b true, close b, close a, conn closed
```

Runtime facts (verified by emitting with 5.3.3 and running):

- With `target: "es2022"` (or lower) `tsc` downlevels `using` with the `__addDisposableResource`/`__disposeResources` helpers. They need `Symbol.dispose`/`Symbol.asyncDispose` at runtime: added in Node.js 20.4.0 and backported to 18.18.0 (verified present on 18.20.8, 20.20.2, 22.23.4, 24.22.1; absent on 16.20.2). With `importHelpers`, `tslib` ≥ 2.6.0 is required.
- With `target: "esnext"`, 5.3 emits `using` **verbatim**. Node.js 18, 20 and 22 reject it (`SyntaxError: Unexpected identifier`); only Node.js 24 parses it. Do not use `target: "esnext"` for a server unless the runtime supports every ESnext syntax form.
- **Trap: `module: "nodenext"` without an explicit `target` implies `target: "esnext"` in 5.3**, so a tsconfig that sets only `module: nodenext` ships `using` (and any other ESnext syntax) untransformed. `module: "node16"` implies `es2022` and downlevels it. Always set `target` explicitly.
- `DisposableStack`, `AsyncDisposableStack` and `SuppressedError` are declared in `lib.esnext.disposable` but exist natively only from Node.js 24; on older runtimes they need a polyfill (the `tsc` helpers create their own `SuppressedError` fallback).
- The `lib` must include `esnext.disposable` (or `esnext`) for `Disposable`, `Symbol.dispose` and friends to type-check.

***

#### Functions

```ts
// Overloads: the implementation signature is not callable from outside
export function parse(input: string): number;
export function parse(input: string[]): number[];
export function parse(input: string | string[]): number | number[] {
  return Array.isArray(input) ? input.map(Number) : Number(input);
}
export const one = parse("1");      // number
export const many = parse(["1"]);   // number[]

// `this` parameters document and check the receiver
export function onClick(this: HTMLButtonElement, ev: MouseEvent) {
  console.log(this.id, ev.button);
}

// 5.1: a function returning `undefined` needs no return statement
export const noop: () => undefined = () => {};

// void callbacks may return values; do not rely on that value
export const doubled: number[] = [];
[1, 2].forEach((x) => doubled.push(x * 2)); // fine: push's number is ignored
```

- Prefer union parameters over overloads when the return type does not depend on the argument type.
- A callback typed `() => void` accepts functions returning anything — passing an `async` function where a `void` callback is expected compiles, and rejected promises then go unhandled (lint: `no-misused-promises`).
- Method shorthand in interfaces (`m(x: T): void`) is checked bivariantly even under `strictFunctionTypes`; property syntax (`m: (x: T) => void`) is checked contravariantly. Use property syntax for callbacks in public types.

***

#### Async code and promises

```ts
declare function fetchUser(id: string): Promise<{ id: string }>;
declare function fetchPosts(id: string): Promise<string[]>;

export async function profile(id: string) {
  // GOOD: independent requests run concurrently; tuple types are preserved
  const [user, posts] = await Promise.all([fetchUser(id), fetchPosts(id)]);

  // GOOD: partial failure handling
  const settled = await Promise.allSettled([fetchUser("a"), fetchUser("b")]);
  const ok = settled.flatMap((r) => (r.status === "fulfilled" ? [r.value] : []));

  // BAD: floating promise - errors become unhandled rejections (tsc does not report this)
  fetchPosts(id);

  // GOOD: timeouts with AbortSignal (Node.js 17.3+/18+, lib dom)
  const res = await fetch(`https://api.example.com/u/${id}`, { signal: AbortSignal.timeout(5_000) });
  return { user, posts, ok, status: res.status };
}
```

- `tsc` does not detect floating promises, `await` in loops that should be concurrent, or `async` callbacks passed to `forEach`. These need typescript-eslint (`no-floating-promises`, `no-misused-promises`, `await-thenable`, `require-await`).
- An unhandled rejection terminates Node.js 15+ processes by default.
- `Promise<T>` return types should be explicit on exported functions; they keep accidental `any` from leaking and keep declaration emit stable.
- Top-level `await` needs `module: es2022`/`esnext`/`system`/`node16`/`nodenext` and `target: es2017`+ (TS1378 otherwise), and in Node.js only works in ES modules.

***
#### Modules and module resolution

##### Choosing `module` / `moduleResolution` in 5.3

| Project | `module` | `moduleResolution` | Notes |
|---|---|---|---|
| Node.js app or library (ESM or CJS) | `nodenext` (or `node16`) | implied | Follows `package.json` `"type"` and `.mts`/`.cts`; relative imports need `.js` extensions in ESM files. `node16` rejects static import attributes (TS2823). |
| Bundled app (Vite, webpack, esbuild, Next.js) | `esnext` | `bundler` (5.0+) | Extensionless imports, `exports`/`imports` honoured, `customConditions` available. |
| Legacy CJS app | `commonjs` | `node10` (alias `node`) | No `exports` support unless `resolvePackageJsonExports`; still valid, not deprecated in 5.3. |
| Anything new | never `classic`, `amd`, `umd`, `system` | | Valid in 5.3, deprecated in 6.0. |

`--module preserve` (5.4) and `--module node18`/`node20` (5.8/5.9) do not exist in 5.3.

##### `nodenext` rules a reviewer must enforce

```ts
// @ts-config: {"module": "nodenext"}
// @file: util.ts
export const add = (a: number, b: number) => a + b;
export type Num = number;
// @file: main.ts
import { add } from "./util";       // error TS2835
import { add as add2 } from "./util.js";   // GOOD: the .js extension names the emitted file
import * as cp from "node:child_process";
export const three = add2(1, 2);
export { add, cp };
```

TS2835: "Relative import paths need explicit file extensions in ECMAScript imports when '--moduleResolution' is 'node16' or 'nodenext'. Did you mean './util.js'?". The rule applies in ES-module files (`"type": "module"` or `.mts`); in CommonJS files extensionless imports keep working because `tsc` emits `require()`.

Mixing formats:

```ts
// @ts-config: {"module": "nodenext"}
// @pkgtype: commonjs
// @file: esm-only.mts
export const v = 1;
// @file: main.ts
import { v } from "./esm-only.mjs";      // error TS1479
export async function load() {
  const { v: lazy } = await import("./esm-only.mjs"); // GOOD: dynamic import works from CJS
  return lazy;
}
export { v };
```

TS1479: "The current file is a CommonJS module whose imports will produce 'require' calls; however, the referenced file is an ECMAScript module and cannot be imported with 'require'." (Node.js 22.12+/20.19+ can `require()` ESM at runtime, but TypeScript models that only from 5.8's `--module node18`/`node20`; in 5.3 use dynamic `import()`.)

##### `verbatimModuleSyntax` (5.0)

`verbatimModuleSyntax` emits every import/export exactly as written, dropping only those marked `type`. It replaces `importsNotUsedAsValues`/`preserveValueImports` (deprecated in 5.0) and makes `tsc` output agree with single-file transpilers.

```ts
// @ts-config: {"verbatimModuleSyntax": true}
// @file: types.ts
export interface User { id: string }
export const VERSION = 1;
// @file: main.ts
import { User } from "./types.js";               // error TS1484
import { type User as U, VERSION } from "./types.js"; // GOOD: inline type modifier
import type { User as U2 } from "./types.js";      // GOOD: whole import is erased
export const v = VERSION;
export type { U, U2, User };
```

TS1484: "'User' is a type and must be imported using a type-only import when 'verbatimModuleSyntax' is enabled." In a CommonJS file (`"type": "commonjs"` or `.cts`) under `verbatimModuleSyntax`, ESM syntax is an error (TS1286 "ESM syntax is not allowed in a CommonJS module when 'verbatimModuleSyntax' is enabled" for imports, TS1287 for `export` on value declarations); such files must use `import x = require("x")` and `export =`. That makes `verbatimModuleSyntax` awkward for CJS-emitting projects — `isolatedModules` plus `consistent-type-imports` linting is the usual alternative there.

Side effects: with `verbatimModuleSyntax`, `import { type A } from "./a.js"` still emits `import {} from "./a.js"`, which *runs* the module; `import type { A }` removes the import entirely. Choose deliberately when the imported module has side effects.

##### `bundler` resolution and `.ts` extensions

```ts
// @ts-config: {"module": "esnext", "moduleResolution": "bundler", "allowImportingTsExtensions": true, "noEmit": true}
// @file: lib.ts
export const x = 1;
export type T = { x: number };
// @file: main.ts
import { x } from "./lib.ts";        // allowed with allowImportingTsExtensions + noEmit
import type { T } from "./lib.ts";   // type-only imports may use .ts since 5.2
import { x as x2 } from "./lib";     // extensionless: allowed under bundler
export const t: T = { x: x + x2 };
```

`allowImportingTsExtensions` requires `noEmit` or `emitDeclarationOnly` (TS5096), because `tsc` 5.3 does not rewrite `.ts` specifiers in emitted JavaScript (`--rewriteRelativeImportExtensions` is 5.7). Code that runs `.ts` files directly (Bun, Deno, `tsx`) uses this mode; code compiled by `tsc` for Node.js must import `./lib.js`.

##### Re-exports, `export type *` and barrels

- `export type * from "./types.js"` and `export type * as ns from "./types.js"` exist since 5.0.
- Under `isolatedModules`, re-exporting a type with a plain `export { T } from "./x.js"` is TS1205 ("Re-exporting a type when 'isolatedModules' is enabled requires using 'export type'"); use `export type { T }`.
- Barrel files (`index.ts` re-exporting everything) slow down type checking and defeat tree shaking in some bundlers; avoid them in large apps, keep them for library entry points.

##### Publishing packages (5.3 consumers)

- `package.json` `exports` must list the `"types"` condition **first** in each condition object; conditions are matched in object order, and a `"types"` entry after `"default"` is never used.
- Dual ESM/CJS packages need separate declaration files per format (`.d.mts` for the `import` branch, `.d.cts` for `require`), otherwise `nodenext` consumers get wrong default-import types ("masquerading as ESM/CJS"). Check with `@arethetypeswrong/cli`.
- A published `.d.ts` that contains `with { "resolution-mode": … }` (or any import attribute) cannot be parsed by consumers on TypeScript < 5.3 (5.2.2 reports TS1005). Libraries that support older compilers should keep attributes out of their declarations or ship a `typesVersions` fallback.

***

#### `lib`, `target` and the real runtime

TypeScript checks against the declarations selected by `lib` (default: derived from `target`), not against the engine that will run the code. Map them deliberately:

| Runtime | Suggested `target` (5.3) | Suggested `lib` | Not available at runtime despite `lib: ["esnext"]` |
|---|---|---|---|
| Node.js 18 | `es2022` | `["es2022"]` (`es2023.array` would also declare `toSorted` etc.) | `toSorted`/`toReversed`/`toSpliced`/`with`, `DisposableStack`, `SuppressedError`, `Symbol.metadata` |
| Node.js 20 | `es2022` | `["es2023"]` | `DisposableStack`, `SuppressedError`, `Symbol.metadata` |
| Node.js 22 | `es2022` | `["es2023"]` | `DisposableStack`, `SuppressedError`, `Symbol.metadata` |
| Node.js 24 | `es2022` (or `esnext` with care) | `["es2023", "esnext.disposable"]` | `Symbol.metadata` (still absent in Node.js 26) |
| Evergreen browsers | `es2022` | `["es2023", "dom", "dom.iterable"]` | depends on the browser baseline |

(Runtime availability checked with `typeof` probes on Node.js 18.20.8, 20.20.2, 22.23.4 and 24.22.1. Node.js 18 and 20 are end-of-life as of October 2026.) `findLast`/`findLastIndex` exist from Node.js 18; the copying array methods (`toSorted`, `toReversed`, `toSpliced`, `with`) from Node.js 20.

- `target` controls syntax downleveling only. `target: "es5"` with `lib: ["es2022"]` happily type-checks `Object.hasOwn` and emits code that breaks on old engines.
- `lib: ["dom"]` in a Node.js project makes browser globals (`window`, `document`, `localStorage`) type-check in server code. Node.js-only projects should omit `dom` (fetch, `URL`, `AbortController`, `structuredClone` etc. come from `@types/node`).
- Values newer than the runtime are not caught by `tsc`. Pair the config with a runtime test matrix or an `engines` field.

***

#### Security-relevant review points

- **Unvalidated casts at trust boundaries**: `req.body as CreateUser`, `JSON.parse(x) as T`, `(await res.json()) as T`. Types are erased; validate with a schema or hand-written guards.
- **Prototype pollution**: merging user JSON into objects typed `Record<string, unknown>` (`Object.assign(target, input)`, recursive merges) lets `__proto__` keys through. Use `Map`, null-prototype objects, or key allowlists.
- **`any` from libraries**: `JSON.parse`, `Response.json()`, `catch` variables with `useUnknownInCatchVariables: false`, and untyped dependencies spread `any`. typescript-eslint's `no-unsafe-*` rules make this visible.
- **Template literal types are not sanitization**: `` type SafeSql = `SELECT ${string}` `` does not prevent injection; parameterize queries.
- **`eval`, `new Function`, `child_process.exec` with string interpolation** remain injection sinks; prefer `execFile`/`spawn` with argument arrays.
- **Secrets in emitted code**: `const enum` and literal types inline values into JavaScript; never put secrets in type-level constants or `declare const` shims backed by build-time replacement without review.
- **Regular expressions**: 5.3 does not check regex syntax (5.5 does); invalid patterns fail only at runtime, and catastrophic backtracking is never detected.
- **Dependency confusion via `paths`**: a `paths` alias that shadows a real npm package name (`"utils": [...]`) type-checks against local code while the runtime may load the package.

***
#### Collections, iteration and common runtime traps

```ts
// @run
// Object.entries/keys are string-keyed by design
const limits = { cpu: 2, memory: 512 };
for (const [key, value] of Object.entries(limits)) {
  console.log(key.toUpperCase(), value * 2);          // key: string, value: number
}

// BAD: default sort is lexicographic, even for numbers
console.log([10, 9, 1].sort());                       // [ 1, 10, 9 ]
// GOOD
console.log([10, 9, 1].sort((a, b) => a - b));        // [ 1, 9, 10 ]

// BAD: Map/Set serialize to {} - the type system does not warn
const m = new Map([["a", 1]]);
console.log(JSON.stringify({ m }));                   // {"m":{}}
console.log(JSON.stringify({ m: Object.fromEntries(m) })); // {"m":{"a":1}}

// Array.isArray on unknown input yields any[]
function count(v: unknown): number {
  return Array.isArray(v) ? v.length : 0;             // v: any[] - elements are any
}
console.log(count([1, 2]));                            // 2

// for...in iterates keys as strings, including inherited enumerable ones; use for...of
for (const i in ["a", "b"]) console.log(typeof i);     // string, string
```

- `Array.isArray(x)` narrows `unknown` to `any[]`, so element accesses are unchecked; follow it with element validation (`v.every((e): e is string => typeof e === "string")`).
- `array.includes(x)` on a literal-union array rejects a wider `x` (`(["a","b"] as const).includes(s)` with `s: string` is TS2345); write `(arr as readonly string[]).includes(s)` or a `Set<string>`, not `as any`.
- `Number(x)`/`parseInt(x, 10)` produce `NaN` silently; check `Number.isFinite`.
- `Date` months are 0-based and `new Date("2026-10-22")` is UTC while `new Date(2026, 9, 22)` is local time.

```ts
const roles = ["admin", "user"] as const;
declare const input: string;
export const bad = roles.includes(input);                          // error TS2345
export const good = (roles as readonly string[]).includes(input);
type Shape = { kind: "a"; id: string };
export type Unchecked = Omit<Shape, "nonexistent">;                // compiles: Omit keys are not checked
export type Checked = Pick<Shape, "nonexistent">;                  // error TS2344
```

***

#### Errors and observability

```ts
// @run
export class AppError extends Error {
  override readonly name = "AppError";
  constructor(message: string, readonly code: string, options?: ErrorOptions) {
    super(message, options);              // ErrorOptions/cause: lib es2022
  }
}

try {
  try {
    JSON.parse("{");
  } catch (cause) {
    throw new AppError("config unreadable", "E_CONFIG", { cause });
  }
} catch (e) {
  if (e instanceof AppError) console.log(e.name, e.code, e.cause instanceof SyntaxError); // AppError E_CONFIG true
}
```

- With `target: "es5"`, subclassing built-ins (`Error`, `Map`, `Array`) breaks `instanceof` and methods, because ES5 emit calls `Error.call(this)` and loses the prototype. The example below prints `false`. Fix: use `target: "es2015"` or later, or call `Object.setPrototypeOf(this, new.target.prototype)` in the constructor.

```ts
// @ts-config: {"target": "es5", "module": "commonjs", "moduleResolution": "node10", "lib": ["es2022", "dom"]}
// @pkgtype: commonjs
// @run
class NotFound extends Error {
  constructor(msg: string) { super(msg); }
}
console.log(new NotFound("x") instanceof NotFound); // false with target es5
```

- Emit source maps (`"sourceMap": true`) and run Node.js with `--enable-source-maps` so stack traces point at `.ts` lines. Do not ship `inlineSources` to public CDNs if the source is confidential.
- Logging `err` from `catch (err)` where `err` is `unknown`: use `String(err)` or an `instanceof Error` check, not `err.message`.
- Structured error codes (`code` property, discriminated unions of error types) are easier to handle than message matching.

***

#### Performance

Compiler performance:

- Measure first: `tsc --extendedDiagnostics` (time per phase, type counts), `tsc --generateTrace <dir>` (open in Perfetto / `@typescript/analyze-trace`), `--listFiles` / `--explainFiles` for unexpected inputs.
- `"incremental": true` (writes `.tsbuildinfo`) and project references (`"composite": true`, `tsc -b`) for monorepos; `skipLibCheck: true` to avoid re-checking `node_modules` declarations.
- Large unions (thousands of members), deep conditional/recursive types, and overloaded generic APIs dominate check time. 5.3 speeds up some intersection-of-union comparisons, but type-level cleverness is still the main cost.
- Fast transpilers (esbuild, swc, tsx, Vite) do not type-check; run `tsc --noEmit` (or `tsc -b`) in CI. They also require `isolatedModules`-compatible code.
- Explicit return types on exported functions keep declaration emit cheap and stable; inferred huge types are re-serialized into every `.d.ts`.

Runtime performance (things TypeScript emit influences):

- `enum` and namespace emit create objects and IIFEs; `as const` objects are cheaper and tree-shakable.
- Downleveled `async`/generators (`target` < es2017) and `using` helpers add overhead; target the oldest runtime you support, not ES5 by habit.
- `importHelpers` + `tslib` deduplicates helpers across files; the `tslib` version must provide every helper the compiler emits (`__esDecorate` ≥ 2.5.0, `__addDisposableResource` ≥ 2.6.0).

***

#### Testing

- Type-level tests: `// @ts-expect-error` lines in a `*.test-d.ts` or a regular test file compiled with `tsc --noEmit`; libraries such as `expect-type` (also exposed as vitest's `expectTypeOf`) provide richer assertions; check their declared minimum TypeScript version before adding them to a 5.3 project. `@ts-expect-error` fails the build when the expected error disappears, which `@ts-ignore` does not.

```ts
type IsEqual<A, B> = (<T>() => T extends A ? 1 : 2) extends <T>() => T extends B ? 1 : 2 ? true : false;
function assertType<T extends true>(): T | void {}

type Id = ReturnType<typeof crypto.randomUUID>;
assertType<IsEqual<Id, `${string}-${string}-${string}-${string}-${string}`>>();
// @ts-expect-error - a plain string is not a UUID template literal
assertType<IsEqual<Id, string>>();
export {};
```

- Test runners execute transpiled code without type checking (vitest, jest with babel/swc, `node --test` with `tsx`); keep `tsc --noEmit` as a separate CI step. `ts-jest` type-checks each test file by default unless it is configured for transpile-only mode.
- Mocks typed with `as any` or `as unknown as Service` hide interface drift; prefer typed fakes (`satisfies Partial<Service>` plus explicit casts at one place) or `vi.mocked()`/`jest.mocked()`.
- Version notes (npm registry, October 2026): typescript-eslint 8.x declares `typescript >=4.8.4 <6.1.0` and supports 5.3; `ts-jest` 29.x declares `>=4.3 <7`; vitest 5 requires Node.js ≥ 22.12 and does not depend on the TypeScript version for transpiling.

***

#### Tooling and type packages

##### `@types/node` and other DefinitelyTyped packages

DefinitelyTyped tests packages only against TypeScript versions younger than two years. In June 2026 the minimum moved to 5.6: every `@types/node` published since then declares `"typeScriptVersion": "5.6"` and uses 5.6-only types (`BuiltinIteratorReturn`, `IteratorObject`). With TypeScript 5.3, `@types/node@26` fails with errors such as `TS2304: Cannot find name 'IteratorObject'` and `Cannot find name 'BuiltinIteratorReturn'` (reproduced on 5.3.3 with `skipLibCheck: false`, plus TS2416/TS2420 errors inside `fs.d.ts` and `stream.d.ts`). `skipLibCheck: true` hides those errors but not the damage: the unresolved iterator types degrade, and user code such as `for await (const e of await opendir(p))` then fails with TS2504 "Type 'Dir' must have a '[Symbol.asyncIterator]()' method".[^4]

Last `@types/node` releases that declare `typeScriptVersion` ≤ 5.3 (all published June 10, 2026):

| Node.js line | Last 5.3-compatible `@types/node` |
|---|---|
| 20 | 20.19.43 |
| 22 | 22.19.21 (22.20.0 requires 5.6) |
| 24 | 24.13.2 (24.13.3 requires 5.6) |
| 25 | 25.9.3 (npm dist-tag `ts5.3`) |

Pin the version matching the deployment runtime (`"@types/node": "~22.19.21"` for a Node.js 22 service) instead of the `ts5.3` dist-tag, which points to Node.js 25 types. `@types/node@22.19.21` compiles cleanly with 5.3.3 even with `skipLibCheck: false`. Renovate/Dependabot updates that bump `@types/*` past the 5.3 window are a frequent cause of "it suddenly stopped compiling"; reviewers should flag them unless TypeScript is upgraded in the same change. The same applies to other `@types/*` packages (their `typesVersions`/`typeScriptVersion` fields show the minimum).

##### Compiler, editor and lint

- `typescript@5.3.3` requires Node.js ≥ 14.17 to run `tsc` (package `engines`).
- Editors use their bundled TypeScript unless the workspace version is selected (VS Code: "TypeScript: Select TypeScript Version" → workspace); otherwise the editor may show 5.4+ diagnostics or accept 5.4+ syntax that the CI compiler rejects.
- typescript-eslint rules a 5.3 project should enable: `no-floating-promises`, `no-misused-promises`, `await-thenable`, `no-unnecessary-type-assertion`, `no-unsafe-assignment`/`-member-access`/`-return`, `switch-exhaustiveness-check`, `consistent-type-imports` (or `verbatimModuleSyntax`), `prefer-nullish-coalescing`, `strict-boolean-expressions` (optional). These need type information (`parserOptions.project` / `projectService`).
- Formatting belongs to Prettier/Biome/dprint; do not review formatting manually.
- `tsc --showConfig` prints the effective configuration after `extends`; use it when judging whether a flag is on.
- Tools that consume `typescript` as a library and loaded `typescript/lib/tsserverlibrary.js` as a browser global break on 5.3; Node.js `require("typescript/lib/tsserverlibrary")` keeps working.

***
#### Bugs that 5.3 compiles silently (caught only by later compilers)

Every line below compiles without error on 5.3.3; the version that starts reporting it is noted. A reviewer of 5.3 code has to catch these by reading.

```ts
// @ts-config: {"isolatedModules": true}
// @file: shapes.ts
export type Something = { a: number };
// @file: main.ts
// 5.4 TS2865: a value-named import that only refers to a type, shadowed by a local value;
// single-file transpilers cannot know whether to drop the import.
import { Something } from "./shapes.js";
let Something = 123;
export { Something };

// 5.4: same-named enums with different values were treated as compatible.
namespace First { export enum E { A = 0, B = 1 } }
namespace Second { export enum E { A = 0, B = 2 } }
export function swap(x: First.E, y: Second.E) { x = y; return x; }

// 5.6 TS2872: a regex literal is always truthy - forgot .test(input)
declare const input: string;
export const isHex = () => { if (/^[0-9a-f]+$/) return true; return input === ""; };

// 5.6 TS2869: precedence bug - parsed as (n < max) ?? 100
declare const max: number;
export const within = (n: number) => n < max ?? 100;

// 5.5 TS2532: compound assignment through noUncheckedIndexedAccess (see Objects)
// 5.7 TS1543: JSON import without with { type: "json" } under nodenext (see Import attributes)
```

Other classic bugs no TypeScript version reports: floating promises, `async` callbacks to `forEach`, `==` vs `===` on mixed types, `Array.prototype.sort()` without a comparator on numbers, `parseInt` without radix on user input, mutation of shared default objects, `Date` month off-by-one, and `for…in` over arrays.

***

#### Removed / changed in 5.3 — migration notes

Upgrading **5.2 → 5.3**:

1. **New TS2855 errors** for `super.field` where the base member is a class field: real runtime bugs; change to `this.field` or make the base member a method.
2. **`instanceof` with a class that defines `static [Symbol.hasInstance](x): x is T`** now narrows to `T` instead of the class; member accesses that relied on the instance type now fail (TS2339) — those were latent bugs.
3. **More narrowing** (`switch (true)`, `=== true/false`) can make previously necessary assertions redundant; typescript-eslint's `no-unnecessary-type-assertion` will flag them.
4. **Import attributes**: `with` replaces `assert` in new code; `assert` keeps working in 5.3 but is rejected at runtime by Node.js 22+.
5. **API consumers**: `tsserverlibrary.js` is a thin wrapper; `JSDocParsingMode` is available; `AssertClause`/`AssertEntry` are deprecated in the API in favour of `ImportAttributes`/`ImportAttribute`. `ParseAll` is still the API default; tools that set another `jsDocParsingMode` to save memory lose JSDoc nodes in `.ts` files.
6. **DOM lib**: small changes (e.g. a generic `SubtleCrypto.exportKey(format: KeyFormat, key)` overload returning `Promise<ArrayBuffer | JsonWebKey>`, `generateKey("Ed25519", …)`, and on `RTCInboundRtpStreamStats` a new optional `mid` and a new *required* `trackIdentifier`); test doubles that construct such objects may need updates.[^18]
7. **`const` type parameters / `satisfies` with mutable array constraints** now infer mutable tuples instead of the constraint (see [`satisfies`](#satisfies-as-const-and-const-type-parameters)); downstream code that relied on `string[]` may see tuple types.
8. Use **5.3.3**, not 5.3.2 (boolean-comparison narrowing regression).

Upgrading **out of 5.3** (what to expect, so reviewers can judge "upgrade TypeScript" suggestions):

- **5.4**: `NoInfer`, closure narrowing preservation, `Object.groupBy`/`Map.groupBy` types, `--module preserve`; breaking: TS2865 for type-only imports shadowed by values under `isolatedModules`, stricter enum compatibility, enum member names `Infinity`/`NaN`/`-Infinity` disallowed, more accurate conditional type constraints, template-literal checks; the last release where `ignoreDeprecations: "5.0"` keeps the 5.0-deprecated options working.[^14]
- **5.5**: the 5.0-deprecated options stop having any effect; inferred type predicates; regex checking; `isolatedDeclarations`; stricter decorator parsing; `undefined` no longer definable as a type name.[^19]
- **5.6–5.9**: always-truthy/nullish errors, iterator helpers, `--noCheck`, `--rewriteRelativeImportExtensions`, `--erasableSyntaxOnly`, `--module node18`/`node20`, `import defer`.
- **6.0**: new defaults (`strict` on, `module: esnext`, `target` es2025-era, `types: []`), deprecation of `baseUrl`, `moduleResolution: node10`/`classic`, `module: amd`/`umd`/`system`, `target: es5`, `outFile`; **7.0** is the native Go compiler. A jump from 5.3 straight to 6.0/7.0 is a large migration; 5.9 is the practical intermediate step.

***

#### Production / review checklist

##### Configuration

- [ ] `typescript` pinned to `~5.3.3` (not 5.3.2); editors use the workspace TypeScript
- [ ] `strict: true` plus `noUncheckedIndexedAccess`, `noImplicitOverride`, `noFallthroughCasesInSwitch`; `exactOptionalPropertyTypes` considered
- [ ] `target` set explicitly (with `module: nodenext` it otherwise defaults to `esnext`) and ≤ `es2022` (there is no `es2023` target in 5.3); `lib` matches the oldest deployment runtime; no `dom` in server-only code
- [ ] `module`/`moduleResolution` pair is `nodenext` (Node.js) or `esnext` + `bundler` (bundled); no `classic`
- [ ] `verbatimModuleSyntax` (ESM) or `isolatedModules` (CJS / fast transpilers) enabled
- [ ] No 5.0-deprecated options hidden behind `ignoreDeprecations: "5.0"` without a removal plan
- [ ] `@types/node` pinned to a 5.3-compatible release of the *runtime's* major (e.g. `~22.19.21`); Renovate/Dependabot rules keep `@types/*` within the 5.3 window
- [ ] `tsc --noEmit` (or `tsc -b`) runs in CI; fast transpilers are not the only compile step

##### 5.3-specific

- [ ] JSON imports in ESM use `with { type: "json" }` (5.3 does not enforce it; Node.js does)
- [ ] No new `assert { … }` import assertions; runtime supports `with` (Node.js ≥ 18.20 / 20.10 / 22)
- [ ] Static import attributes only with `module: esnext`/`nodenext` (not `node16`)
- [ ] `resolution-mode` imports of the same specifier with both modes are in separate files under `bundler`/`node10`
- [ ] `switch (true)` case expressions do not rely on narrowing from earlier cases
- [ ] `static [Symbol.hasInstance]` predicates are correct and intentional
- [ ] No `super.x` on base-class fields (TS2855 errors fixed, not suppressed)
- [ ] No suggestions of 5.4+ features (`NoInfer`, `Object.groupBy` types, `--module preserve`, inferred predicates, iterator helpers, `--erasableSyntaxOnly` …)

##### Code correctness

- [ ] No `as T`/`as unknown as T` on external data; validation at trust boundaries
- [ ] No `any` leaks (`JSON.parse`, `res.json()`, untyped deps); `catch (e)` treated as `unknown`
- [ ] Type predicates and assertion functions actually prove what they claim
- [ ] Index access results handled as possibly `undefined`, including `obj[k] += 1`/`obj[k]++` that 5.3 does not check
- [ ] Narrowed `let`/parameters copied to `const` before use in callbacks (no closure narrowing in 5.3)
- [ ] Subclass field redeclarations use `declare` under `useDefineForClassFields`
- [ ] `context.metadata` only with a `Symbol.metadata` polyfill loaded first
- [ ] `using` compiled with `target` ≤ es2022 unless the runtime parses it natively (Node.js 24+)
- [ ] Exhaustive `switch` with `assertNever`; discriminated unions instead of optional-property bags
- [ ] No exported `const enum`; numeric enums not used as input validation

##### Async, security, observability

- [ ] No floating promises or `async` callbacks where `void` is expected (typescript-eslint)
- [ ] Concurrent independent awaits (`Promise.all`), timeouts via `AbortSignal.timeout`
- [ ] No prototype-pollution-prone merges of untrusted objects; `Map` for dynamic keys
- [ ] Errors carry `cause` and a stable `code`; `target` ≥ es2015 so `Error` subclasses work
- [ ] Source maps emitted and `--enable-source-maps` in production

***

[^1]: TypeScript 5.3 release notes (Handbook). https://www.typescriptlang.org/docs/handbook/release-notes/typescript-5-3.html
[^2]: TypeScript 5.3.3 release on GitHub (published December 6, 2023). https://github.com/microsoft/TypeScript/releases/tag/v5.3.3
[^3]: npm registry metadata for `typescript` (version publish times and dist-tags: 5.3.2 2023-11-20, 5.3.3 2023-12-06, 5.4.2 2024-03-06, 6.0.2 2026-03-23, 6.0.3 2026-04-16, 7.0.2 2026-07-08). https://www.npmjs.com/package/typescript?activeTab=versions
[^4]: DefinitelyTyped README — Support window ("only tests packages on versions of TypeScript that are less than 2 years old"); `@types/node` package metadata (`typeScriptVersion`, `typesVersions`, dist-tag `ts5.3`). https://github.com/DefinitelyTyped/DefinitelyTyped#support-window , https://www.npmjs.com/package/@types/node
[^5]: Announcing TypeScript 5.3 (Microsoft DevBlogs, November 20, 2023). https://devblogs.microsoft.com/typescript/announcing-typescript-5-3/
[^6]: TC39 proposal: Import Attributes; TypeScript implementation PR #54242. https://github.com/tc39/proposal-import-attributes , https://github.com/microsoft/TypeScript/pull/54242
[^7]: `resolution-mode` in import types and all module modes, PR #55725. https://github.com/microsoft/TypeScript/pull/55725
[^8]: `switch (true)` narrowing, PR #55991. https://github.com/microsoft/TypeScript/pull/55991
[^9]: TypeScript 5.3.3 milestone issues: #56482 (boolean comparison narrowing regression), #56478, #56477, #56625. https://github.com/microsoft/TypeScript/issues?q=milestone%3A%22TypeScript+5.3.3%22
[^10]: `instanceof` narrowing through `Symbol.hasInstance`, PR #55052. https://github.com/microsoft/TypeScript/pull/55052
[^11]: Checks for `super` property accesses on instance fields, PR #54056. https://github.com/microsoft/TypeScript/pull/54056
[^12]: TypeScript wiki — API Breaking Changes (5.3: `tsserverlibrary.js` is a thin wrapper around `typescript.js`). https://github.com/microsoft/TypeScript/wiki/API-Breaking-Changes
[^13]: TypeScript 5.4 release notes (NoInfer, preserved narrowing in closures, Object.groupBy, --module preserve, notable behavioral changes). https://www.typescriptlang.org/docs/handbook/release-notes/typescript-5-4.html
[^14]: Announcing TypeScript 5.4. https://devblogs.microsoft.com/typescript/announcing-typescript-5-4/
[^15]: TypeScript 5.0 release notes — Deprecations and Default Changes; Flag Deprecation Plan #51000. https://www.typescriptlang.org/docs/handbook/release-notes/typescript-5-0.html , https://github.com/microsoft/TypeScript/issues/51000
[^16]: TypeScript Handbook — Decorators; TC39 decorators proposal. https://www.typescriptlang.org/docs/handbook/decorators.html , https://github.com/tc39/proposal-decorators
[^17]: TypeScript 5.2 release notes (using declarations, decorator metadata, union array methods, tuple labels). https://www.typescriptlang.org/docs/handbook/release-notes/typescript-5-2.html
[^18]: DOM lib updates for TypeScript 5.3, PR #55798. https://github.com/microsoft/TypeScript/pull/55798
[^19]: TypeScript 5.5 release notes (Disabling Features Deprecated in TypeScript 5.0, inferred type predicates, regex checking). https://www.typescriptlang.org/docs/handbook/release-notes/typescript-5-5.html
[^20]: Total TypeScript — "The TypeScript 5.3 feature they didn't tell you about" (const type parameters and `satisfies` with mutable array constraints). https://www.totaltypescript.com/the-typescript-5-3-feature-they-didn-t-tell-you-about
