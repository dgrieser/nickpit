### TypeScript 5.5: The Complete Production Developer Guide

> **Version**: TypeScript 5.5 — 5.5.2 was the first stable build (June 20, 2024; 5.5.0 was the beta, 5.5.1 the RC). The latest and final patch release is **5.5.4 (July 22, 2024)**.[^1][^2]
> **Support status (October 2026)**: unmaintained. The TypeScript team ships fixes only for the newest release and does not backport them. 5.6 replaced 5.5 on September 9, 2024. Since then 5.6–5.9, **6.0** (last JavaScript-based compiler; 6.0.3, April 16, 2026) and **7.0** (native Go compiler; 7.0.2, July 8, 2026, now the npm `latest` tag) have shipped.[^2][^3] DefinitelyTyped tests `@types` packages only against TypeScript versions younger than two years, so 5.5 (June 2024) has dropped out of that window. Pin `@types/*` to their `ts5.5` dist-tags (see Toolchain Compatibility).[^4]
> **Scope**: the complete TypeScript language and type system as usable in 5.5, the 5.5 changes relative to 5.4, tsconfig and module resolution for Node.js and bundlers, declaration files, async code, errors, security, performance, testing and tooling. The focus is on what a code reviewer must catch.
> **Note**: this document is self-contained. Sections that cover 5.5-specific behaviour are marked **New in 5.5**. Every example was compiled with `tsc` 5.5.4; "new in 5.5" claims were checked against 5.4.5, and "not available" claims against the release that introduced the feature (5.6.3, 5.7.3, 5.8.3, 5.9.3, 6.0.3). Runtime claims were run on Node.js 18.20, 20.20, 22.23, 24.22 and 26.12. Unless a block says otherwise, examples assume `strict`, `target: es2022`, `module: nodenext`, an ES module package (`"type": "module"`), `lib: ["esnext"]` (which in 5.5 means ES2023 plus the listed proposals) and `@types/node`. Comments of the form `// error TS2322` mark the line where 5.5 reports that error; `// 5.4: error ...` describes the old behaviour.

***

#### Table of Contents

1. What's New in TypeScript 5.5
2. Do Not Use or Recommend
3. Version-Aware Review
4. Toolchain Compatibility and Release Facts
5. tsconfig for 5.5
6. Modules and Module Resolution
7. Core Types and Inference
8. Narrowing and Control-Flow Analysis
9. Strictness Flags Explained
10. Iterators, Generators and Collections in 5.5
11. Numbers, BigInt and Dates
12. Node.js APIs with TypeScript
13. JSX and React
14. Objects, Interfaces, Classes
15. Generics and Type-Level Programming
16. Enums, Namespaces and Non-Erasable Syntax
17. Functions, Overloads and `this`
18. Errors and Exceptions
19. Async Code and Promises
20. Resource Management (`using`)
21. Decorators
22. Idioms and Things to Avoid (BAD/GOOD)
23. Runtime Validation and Untrusted Data
24. Security
25. Declaration Files and Library Authoring
26. JavaScript Projects (`checkJs`, JSDoc)
27. Performance (Compiler and Runtime)
28. Testing
29. Tooling, Linting and Formatting
30. Observability and Debugging
31. Removed / Changed in 5.5 — Migration Notes
32. Production / Review Checklist

***

#### What's New in TypeScript 5.5

TypeScript 5.5 is a type-checker and tooling release. It has no new runtime syntax. Its headline changes are:[^1]

| Change | What it means for review |
|---|---|
| **Inferred type predicates** | `arr.filter(x => x !== undefined)` now narrows the array element type. Arrays that later receive `null`/`undefined` can stop compiling. |
| **Narrowing of constant indexed accesses** | `obj[key]` narrows when neither `obj` nor `key` is reassigned. |
| **JSDoc `@import` tag** | Type-only imports in JavaScript files without a runtime import. |
| **Regular-expression syntax checking** | Regex *literals* are parsed and checked: syntax errors, bad backreferences, unknown flags, flags/features newer than `target`. |
| **ECMAScript `Set` methods** in `lib.esnext.collection` | `union`, `intersection`, `difference`, `symmetricDifference`, `isSubsetOf`, `isSupersetOf`, `isDisjointFrom`, plus `ReadonlySetLike`. |
| **`--isolatedDeclarations`** | Errors on exported API whose declaration types cannot be produced without a type checker. |
| **`${configDir}` template variable** | Paths in a shared (extended) tsconfig resolve relative to the *extending* config. |
| **`package.json` dependencies consulted for declaration emit** | Fewer TS2742 "inferred type cannot be named" errors in pnpm-style layouts. |
| **5.0-deprecated options stop working** | `importsNotUsedAsValues`, `preserveValueImports`, `suppressImplicitAnyIndexErrors`, `out`, `target: ES3`, ... are now ignored (still need `ignoreDeprecations: "5.0"` to be accepted). |
| **Stricter decorator parsing** | `@super.decorate` must be parenthesized. |
| **`type undefined = ...` is an error** | The built-in name is now reserved like `null`/`number`. |
| **Reference directives no longer synthesized/preserved in `.d.ts`** | `/// <reference types="node" />` is no longer emitted automatically; hand-written ones survive only with `preserve="true"`. |
| Tooling | `transpileDeclaration` API; `transpileModule` skips full checking (faster ts-jest / ts-loader `transpileOnly`); named ESM imports from the `typescript` package work; package size 30.2 MB → 20.4 MB on disk; editor/watch reliability fixes. |

Everything from earlier 5.x releases is available and should be used where it fits: standard ECMAScript decorators, `const` type parameters, `--verbatimModuleSyntax`, `--moduleResolution bundler`, `export type *`, `extends` arrays in tsconfig (5.0); unrelated getter/setter types and implicit `undefined` returns (5.1); `using`/`await using` declarations and decorator metadata (5.2); import attributes (`with { type: "json" }`), `switch (true)` narrowing and `resolution-mode` in import types (5.3); preserved narrowing in closures after the last assignment, the `NoInfer<T>` utility type, `Object.groupBy`/`Map.groupBy` typings and `--module preserve` (5.4).[^5]

##### New in 5.5: Inferred Type Predicates

A function with no return-type annotation, a **single `return` statement**, no mutation of its parameter, and a `boolean` result tied to a refinement of a parameter now gets a type-predicate signature.[^1][^6]

```ts
interface Bird { name: string; sing(): void }
declare const nationalBirds: Map<string, Bird>;

export function makeBirdCalls(countries: string[]) {
  const birds = countries
    .map(c => nationalBirds.get(c))
    .filter(b => b !== undefined);     // 5.5: Bird[]; 5.4: (Bird | undefined)[]
  for (const b of birds) b.sing();     // 5.4: error TS18048 'b' is possibly 'undefined'
}

// const isNumber: (x: unknown) => x is number
export const isNumber = (x: unknown) => typeof x === "number";
// const isNonNullish: <T>(x: T) => x is NonNullable<T>
export const isNonNullish = <T,>(x: T) => x != null;
```

Type predicates have *if and only if* semantics: `false` must mean "not `T`". This is why a truthiness check on a type with falsy members is **not** inferred — and why it should not be: it hides a real bug.

```ts
export function average(students: string[], scores: Map<string, number>) {
  const s = students.map(st => scores.get(st)).filter(sc => !!sc); // drops 0 scores!
  return s.reduce((a, b) => a + b) / s.length; // error TS2532 error TS18048
}
```

Verified on 5.5.4 — which callbacks are inferred:

| Callback | Inferred predicate? |
|---|---|
| `x => x !== undefined` / `x => x != null` / `x => typeof x === "string"` | yes |
| `x => { return x !== undefined; }` (block body, one `return`) | yes |
| `x => !!x` where `x: Obj \| undefined` | yes (objects are never falsy) |
| `x => !!x` where `x: string \| undefined` (or `number`) | **no** (`""` / `0` are falsy) |
| `x => x` (returns the value, not a `boolean`) | no |
| `.filter(Boolean)` and `x => Boolean(x)` | **no** — still `(T \| undefined)[]` |
| `(x): boolean => ...` (explicit return type) | no |
| two or more `return` statements | no |
| parameter reassigned inside the callback | no |
| class method `isStr(x: unknown) { return typeof x === "string"; }` | yes (the method itself gets `x is string`) |
| `this`-based checks (`isLeaf() { return this instanceof Leaf; }`) | no — write `this is Leaf` explicitly |

**Breaking change to watch for:** a more precise inferred array type can reject later writes.

```ts
const nums = [1, 2, 3, null, 5].filter(x => x !== null); // 5.5: number[]
nums.push(null); // error TS2345 (compiled on 5.4)

// Fix: annotate the type you actually want
const keep: (number | null)[] = [1, 2, 3, null, 5].filter(x => x !== null);
keep.push(null);
export { nums, keep };
```

Review rules:

- Prefer an explicit comparison (`x !== undefined`, `x != null`) over `filter(Boolean)` or `!!x` when the goal is "remove nullish values"; it narrows *and* keeps `0`/`""`.
- Remove hand-written helpers like `function isDefined<T>(x: T | undefined): x is T` only if callers compile identically; they remain correct.
- Explicit predicates (`x is T`) are **not checked** against the body — they are as unsafe as `as`. A wrong one compiles:

```ts
// Compiles, but lies: returns true for strings that are not numbers
export function isNumeric(x: unknown): x is number {
  return typeof x === "string" || typeof x === "number";
}
```

##### New in 5.5: Control-Flow Narrowing for Constant Indexed Accesses

`obj[key]` now narrows when both `obj` and `key` are effectively constant (parameters or `const`s never reassigned).[^1]

```ts
export function upper(obj: Record<string, unknown>, key: string) {
  if (typeof obj[key] === "string") {
    return obj[key].toUpperCase(); // 5.5: ok; 5.4: error TS2571
  }
  return undefined;
}

export function noNarrowing(obj: Record<string, unknown>, key: string) {
  key = key.trim(); // key is reassigned -> no narrowing
  if (typeof obj[key] === "string") {
    return obj[key].toUpperCase(); // error TS2571
  }
  return undefined;
}
```

It also removes the need for a temporary under `noUncheckedIndexedAccess`:

```ts
export function len(obj: Record<string, string | undefined>, key: string): number {
  if (obj[key] !== undefined) {
    return obj[key].length; // 5.5: ok; 5.4: error TS2532
  }
  return 0;
}
```

Like all property narrowing, it is **not invalidated by function calls**. This compiles and throws at runtime (verified on Node 22): `if (typeof obj[key] === "string") { clear(obj); obj[key].toUpperCase(); }`. When a call between the check and the use can mutate the object, copy the value into a `const` first.

##### New in 5.5: The JSDoc `@import` Tag

JavaScript files checked with `checkJs` can import types with ECMAScript import syntax inside a JSDoc comment, with no runtime import:[^1]

```js
/** @import { User } from "./types.js" */
/** @import * as T from "./types.js" */

/** @param {User} u @returns {number} */
export function age(u) { return u.age; }

/** @param {T.User} u */
export function id(u) { return u.id.toUpperCase(); }
```

TypeScript 5.4 reports `Cannot find name 'User'` (TS2304) for the same file. Prefer `@import` over repeated `@typedef {import("./types.js").User} User` blocks. Do **not** write a real `import { User }` for a type-only export in a `.js` file — it fails at runtime because the binding does not exist.

##### New in 5.5: Regular-Expression Syntax Checking

Regular-expression **literals** are now parsed and checked. `new RegExp("...")` strings are not.[^1]

```ts
export const r1 = /@robot(\s+(please|immediately)))? do/; // error TS1508 Unexpected ')'
export const r2 = /(a)(b)\3/u;            // error TS1533 backreference to a missing group
export const r3 = /(?<year>\d{4})-\k<yr>/; // error TS1532 no group named 'yr'
export const r4 = /abc/gg;                // error TS1500 duplicate flag
export const r5 = /a{2,1}/;               // error TS1506 numbers out of order in quantifier
export const r6 = /\p{Letter}/;           // error TS1530 \p{...} needs the u or v flag
export const r7 = /[\p{L}--\p{N}]/v;      // error TS1501 the v flag needs target esnext
export const unchecked = new RegExp("(unclosed"); // not checked; throws at runtime
```

Also verified on 5.5.4:

- Target-dependent checks: with `target: es2017`, named groups (`(?<n>...)`) report TS1503 ("only available when targeting 'ES2018' or later"), the `s` flag needs ES2018, and the `d` flag needs ES2022. The `v` flag requires `target: esnext` in 5.5 because there is no `es2024` target yet.
- **Not detected**: duplicate named groups in alternatives (`/(?<n>a)|(?<n>b)/`) and inline modifiers (`/(?i:abc)/`). Both are ES2025 syntax. They are SyntaxErrors at runtime on Node 18, 20 and 22, and work only from Node 24. Flag them in review if the project runs on Node ≤ 22 or older browsers.
- The `v` flag (`unicodeSets`) works at runtime on Node 20+.

Reviewers should treat new TS15xx regex errors after an upgrade as real bugs — the most common is a pattern that silently matched something else (for example `\p{L}` without `u` matches the literal text `p{L}`).

##### New in 5.5: ECMAScript `Set` Methods

`lib.esnext.collection.d.ts` declares the ES2025 set methods on `Set` and `ReadonlySet`. They are **not** in `lib.es2023`; you need `"lib": ["esnext"]` or `"esnext.collection"`.[^1][^7]

```ts
const a = new Set([1, 2, 3]);
const b = new Set([3, 4]);
export const u = a.union(b);                  // Set<number>
export const i = a.intersection(b);
export const d = a.difference(b);
export const sd = a.symmetricDifference(b);
export const sub = a.isSubsetOf(b);           // boolean
export const viaMap = a.union(new Map([[9, "x"]])); // any set-like (size/has/keys) works; adds the keys
export const mixed = a.union(new Set(["x"])); // Set<string | number>
export const bad = a.union([4, 5]);           // error TS2345 arrays are not set-like
```

**Runtime:** the methods exist on Node 22+ (V8 12.2), not on Node 20 or 18 (`a.union is not a function`). The type declarations do not tell you whether the runtime has them: if `lib` includes `esnext` but the deployment runs Node 20, these calls compile and crash. Match `lib` to the oldest runtime, or polyfill (core-js).

##### New in 5.5: `--isolatedDeclarations`

`isolatedDeclarations` reports an error wherever the declaration (`.d.ts`) type of an **exported** item cannot be produced from the file's syntax alone. It requires `declaration` or `composite` (TS5069 otherwise) and does not change emit. Its purpose is to let other tools (oxc, swc, the `transpileDeclaration` API) generate `.d.ts` files in parallel.[^1][^8]

```ts
// @isolatedDeclarations: true
// @declaration: true
// @noEmit: false
// @emitDeclarationOnly: true
// @outDir: out
declare function add(a: string, b: string): string;
const local = add("1", "2");               // locals need no annotation
export function foo() { return local; }    // error TS9007 needs explicit return type
export function ok(): string { return local; }
export const n = 10;                       // literal initializers are fine
export function y() { return 20; }         // literal return is fine
export const arr = [1, 2];                 // error TS9017 only const arrays can be inferred
export const obj = { a: 1, b: "x" };       // object literal of literals is fine
export const fromCall = add("a", "b");     // error TS9010 needs explicit type
export const asserted = add("a", "b") as string; // a type assertion counts as explicit
export const sat = { a: 1 } satisfies { a: number }; // error TS9010 satisfies is not enough
export class K { m() { return local; } }   // error TS9008 method needs return type
export default add("a", "b");              // error TS9037
const key = "k";
export const computed = { [key]: 1 };      // error TS9038 computed names not supported in 5.5
export function params(a = 1, b = local) { return a + b.length; } // error TS9007 error TS9011
export enum E { A = 1, B = A * 2 }         // enum initializers are fine
```

`params` gets TS9011 on `b = local` because a parameter default that is not a literal needs a type.

**5.5 bug (verified):** `isolatedDeclarations` errors are only reported when declarations are actually emitted. With `noEmit: true` (the usual "type-check only" CI job, or a bundler setup) 5.5 reports **none** of the TS90xx errors above, even with `declaration: true`; 5.6 and later report them. On 5.5, run the check as `tsc --emitDeclarationOnly --outDir <tmp>` (or a normal build), never `tsc --noEmit`.

Adopt `isolatedDeclarations` for libraries whose build uses a fast declaration emitter or parallel project builds; for applications it only adds annotation work. Computed property names in exported object literals and classes are unsupported (TS9038; still reported by 5.8).

##### New in 5.5: The `${configDir}` Template Variable

In path-valued tsconfig options (`outDir`, `rootDir`, `typeRoots`, `paths`, `include`, `exclude`, `files`, `declarationDir`, `tsBuildInfoFile`, ...), `${configDir}` is replaced by the directory of the tsconfig that is being compiled — the *extending* config, not the shared base.[^1][^9]

```jsonc
// packages/tsconfig.base.json — shared
{
  "compilerOptions": {
    "strict": true,
    "outDir": "${configDir}/dist",
    "rootDir": "${configDir}/src",
    "typeRoots": ["${configDir}/node_modules/@types"]
  },
  "include": ["${configDir}/src"]
}
```

Without the variable, `"outDir": "dist"` in a base file resolves next to the base file, so every package writes into the same directory. **Version trap (verified):** TypeScript 5.4 does not know the variable and treats it as a literal path segment — a base config with `"outDir": "${configDir}/dist"` makes 5.4 emit into a directory literally named `${configDir}/dist` beside the base file, and `"include": ["${configDir}/src"]` fails with TS18003 "No inputs were found". Do not adopt `${configDir}` in configs shared with projects still on ≤ 5.4.

##### New in 5.5: `package.json` Dependencies Consulted for Declaration Emit

When an exported value's inferred type comes from a transitive package (`a` returns a type from `b`), declaration emit needs a module specifier for `b`. With pnpm (symlinked `.pnpm` store) 5.4 reported TS2742 ("The inferred type of 'w' cannot be named without a reference to '.pnpm/b@1/node_modules/b'"). 5.5 emits `import("b").Widget` when `b` is listed in the project's `dependencies`/`peerDependencies`/`optionalDependencies` **and** is resolvable from the project. Verified with a pnpm-style layout: 5.4 fails in every configuration; 5.5 succeeds only when `b` is both declared in `package.json` and linked at the project root. If TS2742 remains, add the dependency explicitly or annotate the export.

##### New in 5.5: Behaviour Changes That Break Builds

**5.0-deprecated options no longer work.** `charset`, `target: "ES3"`, `importsNotUsedAsValues`, `noImplicitUseStrict`, `noStrictGenericChecks`, `keyofStringsOnly`, `suppressExcessPropertyErrors`, `suppressImplicitAnyIndexErrors`, `out`, `preserveValueImports`, `prepend` in project references and implicitly OS-specific `newLine`. Verified behaviour on 5.5.4:[^1][^10]

| | 5.4 | 5.5 | 5.6+ |
|---|---|---|---|
| option set, no `ignoreDeprecations` | error TS5101/TS5107 | error TS5101/TS5107 (same text) | error **TS5102** "Option ... has been removed" |
| option set, `"ignoreDeprecations": "5.0"` | option **works** | accepted but **silently ignored** | still TS5102 |

The silent-ignore step is a trap: a config that sets `suppressImplicitAnyIndexErrors` or `suppressExcessPropertyErrors` plus `ignoreDeprecations: "5.0"` compiled on 5.4 and now reports the previously suppressed TS7053/TS2353 errors; `importsNotUsedAsValues: "preserve"` no longer keeps side-effect imports of type-only modules (verified: 5.4 emits `import "./t";`, 5.5 drops it). Replace `importsNotUsedAsValues`/`preserveValueImports` with `verbatimModuleSyntax`, `out` with `outFile`, and delete the rest.

**Stricter decorator grammar.** An unparenthesized decorator must be an identifier, optionally followed by `.name` accesses and one call; anything else — including a chain that starts with `super` or `this` — must be parenthesized:

```ts
class DecoratorProvider { decorate(..._args: any[]) {} }
export class D extends DecoratorProvider {
  m() {
    class C {
      @super.decorate method1() {}   // error TS1497 must be enclosed in parentheses
      @(super.decorate) method2() {} // ok
    }
    return C;
  }
}
```

**`undefined` is not a definable type name:** `type undefined = string;` is error TS2457 (5.4 accepted it but never used it).

**Reference directives in declaration emit.** 5.4 synthesized `/// <reference types="node" />` into `.d.ts` output when a declaration mentioned Node types and dropped hand-written directives it considered unnecessary. 5.5 never synthesizes them and drops every hand-written `/// <reference types=... />` unless it has `preserve="true"`:

```ts
/// <reference types="node" preserve="true" />
import path from "node:path";
export const myPath = path.parse("/a/b"); // .d.ts keeps the directive above
```

Library authors whose published types rely on Node or React globals must either add `preserve="true"` or document that consumers need `@types/node` in their `types`.

**`lib.d.ts` changes.** DOM typings were regenerated; expect occasional new errors in DOM-heavy code.[^1]

***

#### Do Not Use or Recommend

The project's compiler is 5.5. Never suggest the following — they do not exist in 5.5 (each verified to fail on 5.5.4 and to work on the listed version):[^11]

**TypeScript 5.6** (September 2024)
- Iterator helper methods and their types: `Iterator.from(...)`, `map()`/`filter()`/`take()`/`drop()`/`flatMap()`/`reduce()`/`toArray()` on iterators (`m.values().map(...)`), and the lib types `IteratorObject`, `MapIterator`, `SetIterator`, `ArrayIterator`, `BuiltinIteratorReturn`. (The runtime methods exist on Node 22+, but 5.5 has no declarations.)
- `--strictBuiltinIteratorReturn`, `--noUncheckedSideEffectImports`, `--noCheck` on the command line/tsconfig, `--stopBuildOnErrors` (5.6 makes `--build` continue past intermediate errors by default).
- Arbitrary module namespace identifiers: `export { a as "a-b" }` is a syntax error (TS1003) in 5.5.
- The "always truthy/nullish" checks (`if (/re/)`, `if (x => 0)`, `a < b ?? c`): 5.5 does **not** report them, so a reviewer must.

**TypeScript 5.7** (November 2024)
- `--rewriteRelativeImportExtensions` (rewriting `./x.ts` imports to `./x.js` on emit).
- `--target es2024` / `--lib es2024` (and ES2024 lib members such as `ArrayBuffer.prototype.resize`/`transfer`, the `maxByteLength` constructor option, `SharedArrayBuffer.prototype.grow`). In 5.5, `Object.groupBy`, `Map.groupBy`, `Promise.withResolvers`, `Array.fromAsync` and `String.prototype.isWellFormed` live under `esnext.*` libs.
- The "never-initialized variable" error (a `let` read in a closure but never assigned): 5.5 accepts it silently.
- Validation of JSON imports under `--module nodenext` (5.7 requires `with { type: "json" }`; 5.5 does not check it — see Modules).
- Generic typed arrays (`Uint8Array<ArrayBuffer>`).

**TypeScript 5.8** (February 2025)
- `--erasableSyntaxOnly`, `--libReplacement`, `--module node18`.
- `require()` of ES modules under `--module nodenext`.
- Granular checks of each branch of a conditional `return` expression.
- The error for import *assertions* (`assert { type: "json" }`) under `nodenext` — 5.5 accepts `assert`, but Node 22+ rejects it at runtime (see Modules).

**TypeScript 5.9** (July 31, 2025)
- `import defer * as ns from "..."` (syntax error in 5.5), `--module node20`, the minimal `tsc --init` template.

**TypeScript 6.0** (March 2026)
- `--target es2025`/`--lib es2025`, `RegExp.escape`, `Map/WeakMap.prototype.getOrInsert`/`getOrInsertComputed`, `Temporal` types, `--stableTypeOrdering`, `#/` subpath imports, `--moduleResolution bundler` with `--module commonjs`, `"types": ["*"]`.
- 6.0 defaults (`strict: true`, `module: esnext`, floating latest `target`, `types: []`, `rootDir: "."`, `noUncheckedSideEffectImports: true`) do **not** apply: in 5.5 the defaults are `strict: false`, `target: es5`, `module: commonjs` (for ES5/ES3 targets), all `@types` packages included, `rootDir` inferred from the inputs.
- 6.0 deprecations (`target: es5`, `moduleResolution: node`/`node10`/`classic`, `baseUrl`, `module: amd/umd/system`, `outFile`, `downlevelIteration`, `esModuleInterop: false`, `module`-keyword namespaces) are **valid, silent options in 5.5**. Recommend moving away from them for upgradeability, but do not call them errors.

**TypeScript 7.0** (July 2026)
- The native compiler, its multi-threaded checking and its options. There is no `tsgo` or Go-based `tsc` for 5.5 projects.

**Deprecated/removed things that also must not appear in 5.5 code or configs:** the 5.0-deprecated options listed above (now ignored), `type undefined = ...`, unparenthesized complex decorator expressions, `/// <reference types="..." />` in sources that expect it to survive in `.d.ts` without `preserve="true"`.

***

#### Version-Aware Review

- **Verify against 5.5 before claiming something is missing.** Check the `typescript` version in the lockfile (`package-lock.json`, `pnpm-lock.yaml`, `yarn.lock`), not just the `^5.5.x` range in `package.json` — a caret range `^5.5.0` allows 5.6–5.9 to be installed. Say "not available in 5.5" only for items proven absent (lists above).
- **Separate type availability from runtime availability.** `lib` decides what *compiles*; the runtime decides what *runs*. `lib: ["esnext"]` in 5.5 declares `Set.prototype.union`, `Object.groupBy`, `Promise.withResolvers`, `Array.fromAsync`, `DisposableStack` and `Symbol.metadata` — on Node 20 the first four are missing (verified), and `DisposableStack` needs Node 24; `Symbol.metadata` exists in no Node release up to 26 (verified). Flag uses that the deployment target lacks.
- **Pin by exact or tilde range** (`"typescript": "~5.5.4"` or `"5.5.4"`) if the project intends to stay on 5.5. TypeScript does not follow semver: every minor release contains breaking type-check changes.
- **Do not suggest upgrading as a fix** for a bug in code under review; suggest the 5.5-compatible fix and mention the upgrade separately if relevant.
- **Watch for 5.6+ snippets copied from current docs**: iterator helpers on `Map#values()`, `--noUncheckedSideEffectImports`, `--erasableSyntaxOnly`, `--rewriteRelativeImportExtensions`, `es2024` targets, `import defer`.

***

#### Toolchain Compatibility and Release Facts

| Item | Fact (verified October 2026) |
|---|---|
| 5.5 releases on npm | 5.5.0-beta (April 25, 2024), 5.5.1-rc (June 6), **5.5.2 (June 20)**, 5.5.3 (July 1), **5.5.4 (July 22, 2024)**[^2] |
| Node.js needed to run `tsc` 5.5 | `engines: { "node": ">=14.17" }` |
| Newer releases | 5.6.2 (Sep 9, 2024), 5.7.2 (Nov 22, 2024), 5.8.2 (Feb 28, 2025), 5.9.2 (Jul 31, 2025), 6.0.2 (Mar 23, 2026), 7.0.2 (Jul 8, 2026, npm `latest`) |
| `@types/node` | The `ts5.5` dist-tag points to **25.9.3**. `@types/node` 26.x declares `typeScriptVersion: "5.6"` and does **not** compile on 5.5 (verified: TS2304 `Cannot find name 'BuiltinIteratorReturn'`/`IteratorObject`, TS2416 in `fs.d.ts`/`stream.d.ts`) unless `skipLibCheck` hides it — and even then iterator-typed APIs degrade. Install with `npm i -D @types/node@ts5.5`. |
| Other `@types/*` | Same mechanism: e.g. `@types/react@ts5.5` is 19.2.17 while `latest` is 19.3.0. A caret range in `package.json` can pull in a version that requires 5.6+. |
| typescript-eslint | v8 supports TypeScript `>=4.8.4 <6.1.0` (8.72.0); 5.5 is in range. |
| Node.js type stripping | Node 22.18+/23.6+ run `.ts` files directly by erasing types; it recommends TS 5.8+ with `erasableSyntaxOnly` and `rewriteRelativeImportExtensions`, which 5.5 lacks (see Tooling).[^12] |

Upgrade notes for reviewers: moving off 5.5 means passing through 5.6's hard errors for the removed 5.0 options (TS5102), 5.7's JSON-import and never-initialized checks, 5.8's `assert` error under `nodenext`, and 6.0's default changes. That is outside a 5.5 review, but do not let new code add to that debt (no new `baseUrl`, `moduleResolution: node10`, `target: es5`, `outFile`, `module`-keyword namespaces).

***

#### tsconfig for 5.5

##### Defaults You Must Not Assume

Defaults below are from `tsc --all` and the TSConfig reference for 5.5, and were checked by compiling.[^14] In 5.5, an empty `compilerOptions` means: `target: es5`, `module: commonjs`, `moduleResolution: node10`, `strict: false`, every package under `node_modules/@types` is loaded, `useDefineForClassFields: false` (it is `true` only for `target >= es2022`), `esModuleInterop: false` (implied `true` by `module: node16/nodenext/preserve`), `forceConsistentCasingInFileNames: true`. `tsc --init` in 5.5 writes `target: es2016`, `module: commonjs`, `strict`, `esModuleInterop`, `skipLibCheck`, `forceConsistentCasingInFileNames`.

Traps verified on 5.5.4:

- `"module": "esnext"` (or `es2022`/`es2015`) **without** `moduleResolution` selects the `classic` resolver, which does not look in `node_modules` packages properly — `@types/node` itself fails with TS2792 "Cannot find module 'undici-types'". Always pair `esnext` with `"moduleResolution": "bundler"`.
- `module: node16`/`nodenext` imply `target: es2022`/`esnext` respectively and `moduleResolution` of the same name; specify `target` explicitly anyway, because `esnext` in 5.5 emits syntax Node 22 cannot parse (see Resource Management).
- `allowImportingTsExtensions` requires `noEmit` or `emitDeclarationOnly` (TS5096).
- `exactOptionalPropertyTypes` requires `strictNullChecks` (TS5052).

##### Node.js Application (Node 20/22, ESM)

```jsonc
{
  "compilerOptions": {
    "target": "es2022",
    "lib": ["es2023"],              // add "esnext.collection" etc. only if every runtime has them
    "module": "nodenext",           // implies moduleResolution nodenext + esModuleInterop
    "types": ["node"],              // do not load every @types package
    "strict": true,
    "noUncheckedIndexedAccess": true,
    "noImplicitOverride": true,
    "noImplicitReturns": true,
    "noFallthroughCasesInSwitch": true,
    "verbatimModuleSyntax": true,
    "isolatedModules": true,
    "skipLibCheck": true,
    "sourceMap": true,
    "rootDir": "src",
    "outDir": "dist"
  },
  "include": ["src"]
}
```

Add `"exactOptionalPropertyTypes": true` and `"noPropertyAccessFromIndexSignature": true` for new code bases; retrofitting them is noisy. Set `"type": "module"` in `package.json` (or use `.mts` files) — under `nodenext` the module format of each file is decided by the nearest `package.json` `type` field and the file extension, exactly as Node does.

`lib` per runtime in 5.5: Node 18 → `es2022` (`es2023` also declares `toSorted`/`toReversed`/`with`, which Node 18 lacks, while `findLast` exists there); Node 20 → `es2023`; Node 22 → `es2023` plus `esnext.collection`, `esnext.object`, `esnext.promise`, `esnext.array`, `esnext.string` (all verified present at runtime on Node 22.23); `esnext.disposable` types are fine with Node 18.18+ for `Symbol.dispose`, but `DisposableStack`/`AsyncDisposableStack` exist natively only from Node 24.

##### Bundled Front-End or Bundled Node Code (Vite, esbuild, webpack, Bun)

```jsonc
{
  "compilerOptions": {
    "target": "es2022",
    "lib": ["es2023", "dom", "dom.iterable"],
    "module": "esnext",
    "moduleResolution": "bundler",  // or "module": "preserve" (5.4+), which implies bundler
    "noEmit": true,                  // the bundler emits; tsc only type-checks
    "allowImportingTsExtensions": true,
    "isolatedModules": true,
    "verbatimModuleSyntax": true,
    "strict": true,
    "noUncheckedIndexedAccess": true,
    "skipLibCheck": true,
    "jsx": "react-jsx"
  },
  "include": ["src"]
}
```

`bundler` resolution honours `package.json` `exports`/`imports` with the `import` and `types` conditions and allows extensionless relative imports. It does **not** verify that the bundler or runtime agrees: a library built with `moduleResolution: bundler` that is later run directly by Node fails on extensionless imports. Libraries that run unbundled on Node must use `nodenext`.

##### Library (published to npm)

```jsonc
{
  "compilerOptions": {
    "target": "es2022",
    "lib": ["es2022"],              // the lowest runtime you support
    "module": "nodenext",
    "types": [],                     // add "node" only if the library uses Node APIs
    "strict": true,
    "declaration": true,
    "declarationMap": true,
    "sourceMap": true,
    "verbatimModuleSyntax": true,
    "isolatedModules": true,
    "noUncheckedIndexedAccess": true,
    "rootDir": "src",
    "outDir": "dist"
    // "isolatedDeclarations": true  // 5.5+: if a fast d.ts emitter is used
  },
  "include": ["src"]
}
```

Do not set `skipLibCheck` in a library's *release* type-check job without also running a consumer-side check of the emitted `.d.ts` (for example with `@arethetypeswrong/cli`), because `skipLibCheck` also hides errors in your own generated declarations when they are consumed.

##### Monorepos

- Use project references (`composite: true`, `references`) and `tsc -b`; `composite` implies `declaration` and `incremental`.
- Put shared settings in a base config and use `${configDir}` (5.5+) for paths that must be relative to each package (see What's New). `extends` may be an array (5.0+).
- `prepend` in project references is gone (ignored since 5.5); use a bundler to concatenate outputs.

***

#### Modules and Module Resolution

##### Choosing `module`/`moduleResolution`

The module settings decide both how imports are resolved and which output format each file gets.[^13]

| Code runs on | `module` | `moduleResolution` | Relative import specifiers |
|---|---|---|---|
| Node.js directly (tsc emit) | `nodenext` (or `node16`) | (implied) | must include the **output** extension: `./x.js` |
| A bundler | `esnext` or `preserve` | `bundler` (implied by `preserve`) | extensionless or `.ts` (with `allowImportingTsExtensions` + `noEmit`) |
| Node type stripping / Deno / Bun running `.ts` | `nodenext` or `preserve` | — | `./x.ts` (with `allowImportingTsExtensions` + `noEmit`) |
| Legacy CJS-only Node (`moduleResolution: node10`) | `commonjs` | `node10` | extensionless — but `exports` maps are ignored (deprecated in 6.0) |

Verified on 5.5.4 with the same sources:

- `nodenext`: an ESM file importing `./helper` without extension → TS2835 ("Did you mean './helper.js'?"). A `.cts` file importing an ESM-only package → TS1479 (Node 20.19+/22.12+ can `require()` ESM at runtime, but 5.5 does not model that — 5.8 does).
- `node10`: a package that exposes types only through `exports` → TS2307 "There are types at ..., but this result could not be resolved under your current 'moduleResolution' setting". Default import of an `export =` module without `esModuleInterop` → TS1259.
- `bundler`: none of the above are reported — including a `.cts` file importing an ESM-only package, which fails at runtime when not bundled.

##### ESM/CJS Interop Rules

```ts
// @filename: legacy.cts
function legacy(): number { return 1; }
export = legacy;                      // CommonJS: module.exports = legacy
// @filename: main.mts
import legacy from "./legacy.cjs";     // ESM default import = module.exports
import * as fs from "node:fs";         // namespace import of a builtin is fine
import { readFile } from "node:fs/promises";
export const v = [legacy(), typeof fs.statSync, typeof readFile];
```

- Under `nodenext`, an ESM file may default-import CommonJS (`module.exports` becomes the default). **Named** imports from CJS work at runtime only if Node's `cjs-module-lexer` detects them; TypeScript cannot know. If a named import from a CJS package fails at runtime with "does not provide an export named", switch to the default import.
- A CommonJS file (`.cts`, or `.ts` in a package without `"type": "module"`) cannot `import` an ESM-only package statically in 5.5 (TS1479). Use `await import("pkg")` — TypeScript preserves dynamic `import()` in CJS output under `node16`/`nodenext` (it rewrites it to `require` under `module: commonjs`, which breaks for ESM-only packages).
- `import x = require("x")` and `export =` are the CJS forms that work with `verbatimModuleSyntax`; under `verbatimModuleSyntax` a CJS-format file may **not** use `import`/`export` syntax at all (TS1286/TS1287, verified).

##### `verbatimModuleSyntax` and `isolatedModules`

`verbatimModuleSyntax` (5.0+) means: what you write is what is emitted — imports without `type` stay, imports with `type` disappear. It replaces the removed `importsNotUsedAsValues`/`preserveValueImports`.

```ts
// @verbatimModuleSyntax: true
// @filename: types.ts
export interface Foo { n: number }
export interface Bar { m: number }
export const runtimeValue = 1;
// @filename: main.ts
import type { Foo } from "./types.js";
import { type Bar, runtimeValue } from "./types.js"; // inline type modifier
import { Bar as B2 } from "./types.js";              // error TS1484 must use a type-only import
export const v: Foo | Bar | B2 = { n: runtimeValue };
```

`isolatedModules` (implied by `verbatimModuleSyntax` for its main checks) catches constructs a single-file transpiler (esbuild, swc, Babel, Node type stripping) cannot compile: re-exporting a type without `export type` (TS1205), using an ambient `const enum` (TS2748). Enable one of them in every project that is not compiled solely by `tsc`.

##### Import Attributes and JSON

```ts
// @resolveJsonModule: true
// @filename: data.json
{ "name": "x", "version": 1 }
// @filename: main.ts
import data from "./data.json" with { type: "json" };
export const name: string = data.name;
```

- Use `with { type: "json" }` (TypeScript 5.3+). The older `assert { type: "json" }` still compiles on 5.5 but is a **SyntaxError on Node 22 and 24** (verified: "Unexpected identifier 'assert'"); Node 18/20 accept both. TypeScript reports TS2880 for `assert` only from 5.8.
- Importing JSON from an ES module **without** the attribute compiles on 5.5 (5.7+ reports TS1543) but throws `ERR_IMPORT_ATTRIBUTE_MISSING` on Node 20, 22 and 24 (verified). Flag it in review.
- `resolveJsonModule` is required for JSON imports. Under `node10` a default import of JSON also needs `esModuleInterop` (TS1259 otherwise); `bundler` and `nodenext` allow it as is.

##### `paths`, `baseUrl` and Subpath Imports

`paths` only affects type-checking; tsc does **not** rewrite the specifiers in emitted JS. A project that uses `paths` and emits with `tsc` for Node must also configure the runtime (subpath `imports` in `package.json`, a loader, or a bundler) — otherwise the output fails with `ERR_MODULE_NOT_FOUND`. Prefer `package.json` `imports` (`"#internal/*": "./dist/internal/*"`), which both Node and 5.5's `node16`/`nodenext`/`bundler` resolvers understand. In 5.5, subpath import keys must start with `#` followed by a character other than `/`; the `#/` prefix is a TypeScript 6.0 addition. `baseUrl` is not needed for `paths` since 4.1 and is deprecated in 6.0; do not add it to new configs.

##### Dual Packages and `exports`

```jsonc
{
  "name": "my-lib",
  "type": "module",
  "exports": {
    ".": {
      "import": { "types": "./dist/index.d.ts", "default": "./dist/index.js" },
      "require": { "types": "./dist/index.d.cts", "default": "./dist/index.cjs" }
    }
  }
}
```

- The `types` condition must come **first** inside each block; conditions are matched in object order.
- ESM and CJS builds need separate declaration files (`.d.ts` for ESM, `.d.cts` for CJS) — one `.d.ts` shared by both lies about the module format ("masquerading as ESM/CJS"). Check packages with `@arethetypeswrong/cli`.
- `typesVersions` is still honoured in 5.5 and is the mechanism DefinitelyTyped uses to serve older TypeScript versions.

***

#### Core Types and Inference

##### `any`, `unknown`, `never`, `{}`, `object`

```ts
declare function parse(text: string): unknown;

const value = parse("{}");
value.id; // error TS18046 'value' is of type 'unknown'

if (typeof value === "object" && value !== null && "id" in value) {
  console.log(value.id); // ok: narrowed to object & Record<"id", unknown>
}

const n: {} = 42;          // {} accepts every non-nullish value, including primitives
const o: object = 42;      // error TS2322 object excludes primitives
const r: Record<string, unknown> = { a: 1 }; // use for "plain object with unknown values"
export { n, o, r };
```

- `any` disables checking in both directions and spreads (`const x = anyValue.foo` is `any`). Treat a new `any` in a diff as a finding unless it is justified in a comment; prefer `unknown` plus narrowing.
- `JSON.parse`, `Response.json()` and `catch` bindings without `useUnknownInCatchVariables` return `any`. Annotating the result (`const u: User = JSON.parse(s)`) is an unchecked cast. Validate instead (see Runtime Validation).
- `never` is the empty type: use it for exhaustiveness checks and impossible branches.
- `Function` and `Object` (capitalized) are almost never the right annotations; use a function signature or `object`/`Record`.

##### Literal Types, `as const` and `satisfies`

```ts
type Route = { path: string; method: "GET" | "POST" };

// satisfies (4.9+): check the type, keep the narrow inferred type
export const routes = {
  home: { path: "/", method: "GET" },
  submit: { path: "/submit", method: "POST" },
} satisfies Record<string, Route>;
routes.home.method; // type "GET", not "GET" | "POST"

// an annotation widens to the annotation
export const routes2: Record<string, Route> = { home: { path: "/", method: "GET" } };
routes2.missing.path; // compiles (no noUncheckedIndexedAccess) — the key set is lost

// as const: deeply readonly literal types
export const ROLES = ["admin", "user"] as const;
export type Role = (typeof ROLES)[number]; // "admin" | "user"
```

- `as const` is not a type assertion that can lie; `as SomeType` is. A reviewer should treat `as` on data from outside (network, storage, user input) as an unchecked claim.
- `x as unknown as T` is a double cast that bypasses all checking — flag it unless it is in a tested adapter.
- Non-null assertion `x!` is an unchecked claim too; prefer narrowing or an explicit error.

##### `const` Type Parameters (5.0+) and `NoInfer` (5.4+)

```ts
declare function routesOf<const T extends readonly string[]>(paths: T): T;
export const r = routesOf(["/a", "/b"]); // readonly ["/a", "/b"] without "as const" at call site

declare function createFSM<S extends string>(states: S[], initial: NoInfer<S>): void;
createFSM(["idle", "busy"], "idle");
createFSM(["idle", "busy"], "stopped"); // error TS2345 ("stopped" no longer widens S)
```

##### Arrays, Tuples, Index Signatures

```ts
const list: string[] = ["a"];
const first = list[0];            // string — wrong when the array is empty
first.toUpperCase();              // compiles without noUncheckedIndexedAccess

const scores: Record<string, number> = {};
const s = scores["alice"];        // number — wrong when the key is missing
export { first, s };
```

With `noUncheckedIndexedAccess`, `list[0]` and `scores["alice"]` are `T | undefined`. Recommend it for new code; it catches real out-of-bounds bugs. The flag does not affect `for...of`/`forEach` (iteration never yields missing elements) or destructuring of fixed-length tuples. `arr.at(i)` returns `T | undefined` with or without the flag.

```ts
// @noUncheckedIndexedAccess: true
const pair: [string, number] = ["a", 1];
const [name, count] = pair;   // string, number: tuple positions are known
const list = ["a", "b"];
for (const item of list) item.toUpperCase();  // fine: iteration never yields undefined
const maybe = list[5];        // string | undefined
export { name, count, maybe };
```

- `readonly T[]` / `ReadonlyArray<T>` is not assignable to `T[]` (TS4104/TS2345); accept `readonly T[]` in parameters you do not mutate.
- `Object.keys(o)` returns `string[]`, not `(keyof typeof o)[]`, because objects may have extra keys at runtime. Indexing with those keys is TS7053. Do not "fix" it with `as (keyof T)[]` when the object can come from outside; iterate `Object.entries` or use a `Map`.
- `for (const k in obj)` iterates inherited enumerable keys and gives `string`; prefer `Object.keys`/`Object.entries`.
- `const roles = ["admin", "user"] as const; roles.includes(s)` with `s: string` is TS2345. Widen the array (`(roles as readonly string[]).includes(s)`) or write a type guard; do not cast `s`.

##### `exactOptionalPropertyTypes`

```ts
// @exactOptionalPropertyTypes: true
interface Options { timeout?: number }
export const a: Options = {};                       // ok: absent
export const b: Options = { timeout: undefined };   // error TS2375
interface Options2 { timeout?: number | undefined } // say so explicitly if undefined is allowed
export const c: Options2 = { timeout: undefined };
```

Without the flag, `?:` means "absent or `undefined`", and `"timeout" in opts` can be true while the value is `undefined`. Libraries that spread user options over defaults (`{ ...defaults, ...opts }`) get overwritten with `undefined` — a classic bug the flag exposes.

##### Template Literal Types and Branded Types

```ts
type EventName = `on${Capitalize<"click" | "focus">}`; // "onClick" | "onFocus"
export const e: EventName = "onClick";

// Branded (nominal) types for IDs that must not be mixed
declare const brand: unique symbol;
type Brand<T, B extends string> = T & { readonly [brand]: B };
export type UserId = Brand<string, "UserId">;
export type OrderId = Brand<string, "OrderId">;

export function toUserId(raw: string): UserId {
  if (!/^u_[a-z0-9]+$/.test(raw)) throw new Error("invalid user id");
  return raw as UserId; // the only place a cast is allowed: after validation
}
declare function loadOrder(id: OrderId): void;
loadOrder(toUserId("u_1")); // error TS2345
```

***

#### Narrowing and Control-Flow Analysis

##### Discriminated Unions and Exhaustiveness

```ts
type Shape =
  | { kind: "circle"; radius: number }
  | { kind: "square"; size: number }
  | { kind: "rect"; w: number; h: number };

function assertNever(x: never): never {
  throw new Error(`unexpected: ${JSON.stringify(x)}`);
}

export function area(s: Shape): number {
  switch (s.kind) {
    case "circle": return Math.PI * s.radius ** 2;
    case "square": return s.size ** 2;
    case "rect": return s.w * s.h;
    default: return assertNever(s); // adding a new kind makes this a compile error
  }
}
```

A `switch` without the `default: assertNever` branch silently returns `undefined` for a new variant when the function's return type includes `undefined` or `void`. With an explicit non-`undefined` return type, `noImplicitReturns`/TS2366 catches it. Prefer the `assertNever` pattern in reducers, state machines and protocol handlers.

##### Narrowing Forms

```ts
declare const v: string | number | Date | null | undefined;

if (typeof v === "string") v.toUpperCase();
if (v instanceof Date) v.getTime();
if (v != null) { const x: string | number | Date = v; console.log(x); } // != null removes null and undefined
if (v) { /* string | number | Date — but "" and 0 are excluded too! */ }

// switch (true) narrowing (5.3+)
export function describe(x: unknown): string {
  switch (true) {
    case typeof x === "string": return x.toUpperCase();
    case Array.isArray(x): return `array(${x.length})`;
    default: return String(x);
  }
}

// "in" narrowing
type Cat = { meow(): void }; type Dog = { bark(): void };
export function speak(a: Cat | Dog) { if ("meow" in a) a.meow(); else a.bark(); }
```

- Truthiness narrowing on `string | number` drops `""` and `0`. In review, ask whether `0`/`""` are valid values (ports, counts, prices, empty names) before accepting `if (x)` or `x || default`. Use `??` for defaults when only `null`/`undefined` should fall back.
- `Array.isArray(x)` narrows `unknown` to `any[]`; elements are `any`.

##### Closures (5.4+ improvements) and Their Limits

Since 5.4, narrowing of a `let` or parameter is preserved inside closures created **after the last assignment**:

```ts
export function urls(base: string | undefined) {
  if (!base) base = "https://example.com";
  return ["a", "b"].map(p => new URL(p, base)); // ok since 5.4: base is string
}

export function stillWidened(id: string | undefined) {
  if (id === undefined) id = "x";
  const f = () => id.length; // error TS18048 — an assignment follows
  id = undefined;
  return f;
}
```

Narrowing of object properties (`obj.x`, `obj[key]`) survives `await` and function calls even though other code may mutate the object in between — TypeScript does not invalidate it (see the constant-index example above). For values read after an `await` or a call that may mutate the object, re-check or copy into a `const` before the check.

##### Assertion Functions

```ts
export function assert(cond: unknown, msg: string): asserts cond {
  if (!cond) throw new Error(msg);
}
export function assertIsString(x: unknown): asserts x is string {
  if (typeof x !== "string") throw new TypeError("expected string");
}

declare const input: unknown;
assertIsString(input);
input.toUpperCase(); // narrowed

// Pitfall: an arrow-function assertion needs an explicit type annotation on the binding
const assertPositive = (n: number): asserts n is number => { if (n <= 0) throw new RangeError("n"); };
assertPositive(1); // error TS2775 assertions require every name in the call target to be declared with an explicit type annotation
```

Declare assertion helpers as `function` declarations, or annotate the variable (`const assertPositive: (n: number) => asserts n is number = ...`).

***

#### Strictness Flags Explained

`strict: true` in 5.5 enables: `noImplicitAny`, `noImplicitThis`, `alwaysStrict`, `strictNullChecks`, `strictFunctionTypes`, `strictBindCallApply`, `strictPropertyInitialization`, `useUnknownInCatchVariables`. (5.6 adds `strictBuiltinIteratorReturn` to this set; 5.5 does not have it.)

| Flag (not in `strict`) | What it catches | Recommendation |
|---|---|---|
| `noUncheckedIndexedAccess` | `arr[i]`, `record[key]` typed as possibly `undefined` | on for new code |
| `exactOptionalPropertyTypes` | `{ a?: T }` rejects explicit `undefined` | on for new code/libraries |
| `noImplicitOverride` | missing `override` on overriding members | on |
| `noImplicitReturns` | code paths without `return` | on |
| `noFallthroughCasesInSwitch` | non-empty `case` falling through | on |
| `noPropertyAccessFromIndexSignature` | forces `obj["key"]` for index-signature keys | optional |
| `noUnusedLocals` / `noUnusedParameters` | dead code | prefer the lint rule (less disruptive while editing) |
| `allowUnreachableCode: false` | unreachable statements become errors | optional |
| `forceConsistentCasingInFileNames` | `./User` vs `./user` (breaks on Linux) | default `true` |

Turning `strict` off (or any member flag) in a diff is a significant regression — ask why. Turning `strictNullChecks` off makes every type implicitly nullable and disables most narrowing.

```ts
// @strictNullChecks: false
// With strictNullChecks off, null is assignable to everything — no error here:
export const s: string = null;
export function len(x: string) { return x.length; } // len(null) compiles and throws
len(null);
```

***

#### Iterators, Generators and Collections in 5.5

5.5 predates the 5.6 iterator rework. `Map#keys()/values()/entries()`, `Set#values()`, `Array#entries()` and `String#matchAll()` return `IterableIterator<T>`; there are no `MapIterator`/`IteratorObject` types and no iterator helper methods.

```ts
const m = new Map<string, number>([["a", 1], ["b", 2]]);
const it: IterableIterator<number> = m.values();
export const doubled = [...m.values()].map(v => v * 2);    // 5.5 idiom: spread, then array methods
export const viaFrom = Array.from(m.values(), v => v * 2);  // same, one pass
export const helpers = m.values().map(v => v * 2);         // error TS2339 error TS7006 (iterator helpers are 5.6+)

function* ids(): Generator<number, string, undefined> {
  let i = 0;
  while (i < 3) yield i++;
  return "done";
}
const g = ids();
const r = g.next();                       // IteratorResult<number, string>
if (!r.done) console.log(r.value + 1);    // number only after checking done
export { it, r };
```

- `IteratorResult<T, TReturn = any>`: when `TReturn` is `any` (the default for many lib signatures in 5.5), `result.value` is `any` until you check `done`. Always check `done` before using `value` (5.6's `strictBuiltinIteratorReturn` would enforce it; 5.5 does not).
- `for...of` over `Map`/`Set`/generators requires `target >= es2015` or `downlevelIteration`; with `target: es5` and no `downlevelIteration` it is TS2802.
- `Object.groupBy`/`Map.groupBy` (lib `esnext.object`/`esnext.collection` in 5.5, Node 21+) return `Partial<Record<K, T[]>>` — every group lookup is possibly `undefined`.

```ts
const people = [{ name: "a", team: "x" }, { name: "b", team: "y" }];
const byTeam = Object.groupBy(people, p => p.team);
byTeam["x"].length;          // error TS2532 group may be undefined
export const xs = byTeam["x"]?.length ?? 0;
```

- Arrays: prefer the non-mutating ES2023 methods (`toSorted`, `toReversed`, `toSpliced`, `with`; lib `es2023`, Node 20+) over `sort()`/`reverse()` on shared arrays. `arr.sort()` without a comparator sorts numbers lexicographically (`[10, 9, 1].sort()` → `[1, 10, 9]`).
- `Array.prototype.includes`/`indexOf` use SameValueZero/strict equality — objects are compared by reference.
- `Map` keys compare by identity; using objects or arrays (`map.get([1, 2])`) never hits. Use string keys or a dedicated key function.

***

#### Numbers, BigInt and Dates

```ts
// @target: es2020
export const big = 2n ** 64n;               // bigint literals need target >= es2020
export const unsafe = Number.MAX_SAFE_INTEGER + 2; // 9007199254740993 is not representable
export const mixed = big + 1;               // error TS2365 cannot mix bigint and number
export const ok = big + BigInt(1);
```

- IDs from databases (`BIGINT`, Snowflake/Twitter IDs) exceed 2^53; keep them as `string` or `bigint`. `JSON.parse` silently rounds large integers; `JSON.stringify` throws on `bigint`.
- Money: integers in minor units or a decimal library — never binary floating point.
- `parseInt(x)` without a radix, `Number("")` (→ `0`) and `isNaN` (coerces) are common bugs; use `Number.parseInt(x, 10)`, check `Number.isFinite`, `Number.isNaN`.
- `Date` months are 0-based; `new Date("2024-06-20")` is UTC midnight while `new Date("2024-06-20T00:00")` is local time. `Temporal` types are not in 5.5 (6.0 adds them).
- Division by zero yields `Infinity`/`NaN`, not an exception; validate before formatting or persisting.

***

#### Node.js APIs with TypeScript

```ts
import { readFile, writeFile } from "node:fs/promises";
import { setTimeout as sleep } from "node:timers/promises";
import { pipeline } from "node:stream/promises";
import { createReadStream, createWriteStream } from "node:fs";
import { createGzip } from "node:zlib";

export async function loadConfig(path: string): Promise<unknown> {
  const text = await readFile(path, "utf8");   // string (encoding given) vs Buffer (no encoding)
  return JSON.parse(text) as unknown;
}

export async function gzip(src: string, dst: string): Promise<void> {
  await pipeline(createReadStream(src), createGzip(), createWriteStream(dst)); // propagates errors, closes streams
}

export async function retry<T>(fn: () => Promise<T>, attempts = 3): Promise<T> {
  for (let i = 1; ; i++) {
    try {
      return await fn();
    } catch (err) {
      if (i >= attempts) throw new Error(`failed after ${attempts} attempts`, { cause: err });
      await sleep(100 * 2 ** i);
    }
  }
}

export async function save(path: string, data: unknown) {
  await writeFile(path, JSON.stringify(data), { encoding: "utf8", mode: 0o600 });
}
```

- Use the `node:` prefix for built-ins (clearer, cannot be shadowed by a package).
- Prefer `fs/promises` and `stream/promises.pipeline` over callback APIs and manual `.pipe()` chains (which do not forward errors).
- `setTimeout` returns `NodeJS.Timeout` under `@types/node` and `number` in the DOM lib; with both loaded, use `ReturnType<typeof setTimeout>`.
- `__dirname`/`require` do not exist in ES modules; use `import.meta.dirname`/`import.meta.filename` (Node 20.11+/21.2+, declared in recent `@types/node`) or `fileURLToPath(new URL(".", import.meta.url))`.
- Top-level `await` needs an ES module, `module` set to `es2022`/`esnext`/`system`/`node16`/`nodenext`/`preserve` and `target` `es2017` or later (TS1378 otherwise); in a CommonJS-format file under `nodenext` (`.cts`, or no `"type": "module"`) it is TS1309.

***

#### JSX and React

- `jsx: "react-jsx"` (automatic runtime, React 17+) for new projects; `"preserve"` when a bundler/Babel handles JSX. `.tsx` is required for JSX syntax; generic arrow functions in `.tsx` need `<T,>` or `<T extends unknown>`.
- Pin `@types/react`/`@types/react-dom` to their `ts5.5` tags (19.2.x at the time of writing) — newer versions declare a higher `typeScriptVersion`.
- Do not type components as `React.FC` just to get `children`; declare props explicitly (`children?: React.ReactNode`).
- Event handler types: `React.ChangeEvent<HTMLInputElement>`; avoid `any` for events.
- `useState<T>()` without an initial value is `T | undefined`; `useRef<T>(null)` gives a `RefObject` whose `current` may be `null`.

***

#### Objects, Interfaces, Classes

##### `interface` vs `type`

Use `interface` for object shapes that are extended or implemented, and `type` for unions, intersections, mapped/conditional types, tuples and function types. Interfaces with the same name in the same scope **merge** (declaration merging); type aliases cannot be redeclared. Accidental merging with a global (`interface Request {}` in a script file that augments the DOM `Request`) is a real bug source — keep files as modules (any `import`/`export`) so declarations stay local.

##### Structural Typing and Excess Property Checks

```ts
interface Point { x: number; y: number }
export const p1: Point = { x: 1, y: 2, z: 3 }; // error TS2353 excess property on a fresh literal
const tmp = { x: 1, y: 2, z: 3 };
export const p2: Point = tmp;                   // ok: no check on non-fresh objects
```

Excess-property checking only applies to object literals assigned directly. A misspelled optional property (`{ tiemout: 5 }`) passed through a variable compiles; review options objects built in variables and prefer passing literals or annotating the variable.

##### Method Parameter Bivariance

`strictFunctionTypes` checks parameters contravariantly only for **function-typed properties**, not for **method** declarations:

```ts
interface Ev { type: string }
interface ClickEv extends Ev { x: number }

interface HandlerMethod { handle(e: Ev): void }          // method syntax: bivariant
interface HandlerProp { handle: (e: Ev) => void }        // property syntax: checked

export const h1: HandlerMethod = { handle(e: ClickEv) { console.log(e.x.toFixed()); } }; // compiles, crashes for non-click events
export const h2: HandlerProp = { handle: (e: ClickEv) => console.log(e.x) };          // error TS2322
```

Write callback members of interfaces as properties (`handle: (e: Ev) => void`) when soundness matters.

##### Classes

```ts
// @noImplicitOverride: true
export abstract class Repo<T extends { id: string }> {
  readonly #items = new Map<string, T>();     // runtime-private (ES private field)
  protected abstract validate(item: T): void;
  save(item: T): void { this.validate(item); this.#items.set(item.id, item); }
  get size(): number { return this.#items.size; }
}

interface User { id: string; email: string }
export class UserRepo extends Repo<User> {
  protected override validate(u: User): void {
    if (!u.email.includes("@")) throw new Error("invalid email");
  }
  save(u: User): void { super.save(u); } // error TS4114 must have 'override'
}
```

- `private`/`protected` are compile-time only: `(obj as any).secret` and `JSON.stringify(obj)` see them. Use `#field` for real encapsulation (or for secrets that must not be serialized). `#private` fields are not accessible from subclasses or via `Proxy` wrappers — do not wrap such classes in proxies (a `Proxy` method call with `this` = proxy throws `TypeError`).
- Enable `noImplicitOverride`: renaming a base method otherwise silently turns overrides into new methods.
- `strictPropertyInitialization` (part of `strict`) requires fields to be assigned in the constructor. `field!: T` silences it — acceptable only when a framework initializes the field (DI, ORMs) and documented.
- **`useDefineForClassFields`** is `true` for `target >= es2022` (and `esnext`). Class fields then use `Object.defineProperty` semantics and run **after** `super()` returns. Redeclaring a base-class property in a subclass without an initializer overwrites it with `undefined` at runtime; TypeScript reports TS2612 for the obvious case. Use `declare` for type-only redeclarations:

```ts
class Base {
  value: string | undefined;
  constructor() { this.init(); }
  init() { this.value = "from-base"; }
}
export class Derived extends Base {
  declare value: string;  // type-only: no runtime field
}
export class Broken extends Base {
  value!: string;         // error TS2612 (at es2022 this field would reset value to undefined)
}
```

- Field initializers run after the base constructor. A base constructor that calls an overridden method which reads a subclass field sees `undefined`. Avoid calling overridable methods from constructors.
- Parameter properties (`constructor(private readonly db: Db)`) are TypeScript-only runtime syntax: Node type stripping and `--erasableSyntaxOnly` tooling reject them. Fine for `tsc`/bundler builds; flag them in code meant to run under Node's type stripping.
- `accessor` fields (`accessor count = 0`, 4.9+) create a getter/setter pair backed by a private slot; they exist for decorators.

##### Getters and Setters

Since 5.1 the getter and setter types may be unrelated (with explicit annotations). A setter that accepts `string | number` while the getter returns `string` is legitimate; a getter whose type includes `undefined` when the setter rejects it usually signals a design problem.

***

#### Generics and Type-Level Programming

```ts
// Constrain to what you use; return precise types
export function pluck<T, K extends keyof T>(items: readonly T[], key: K): T[K][] {
  return items.map(i => i[key]);
}

// Do not use a type parameter only once: it is just a disguised any/unknown
export function bad<T>(x: unknown): T { return x as T; } // caller-chosen T = unchecked cast
export function good(x: unknown): string { return String(x); }

// Default type parameters
export interface Page<T = unknown> { items: T[]; next?: string }
```

Utility types to know (all available in 5.5): `Partial`, `Required`, `Readonly`, `Pick`, `Omit`, `Record`, `Exclude`, `Extract`, `NonNullable`, `Parameters`, `ConstructorParameters`, `ReturnType`, `InstanceType`, `Awaited`, `ThisParameterType`, `OmitThisParameter`, `ThisType`, `Uppercase`/`Lowercase`/`Capitalize`/`Uncapitalize`, `NoInfer` (5.4).

```ts
interface User { id: string; name: string; password: string }
export type PublicUser = Omit<User, "password">;
export type Typo = Omit<User, "pasword">;      // compiles! Omit does not check keys
export type Strict = Pick<User, "pasword">;     // error TS2344 Pick does check keys

// Conditional types distribute over unions; wrap in [] to stop it
type ToArray<T> = T extends unknown ? T[] : never;
type ToArrayNonDist<T> = [T] extends [unknown] ? T[] : never;
export type A1 = ToArray<string | number>;         // string[] | number[]
export type A2 = ToArrayNonDist<string | number>;  // (string | number)[]

// Mapped types with key remapping
export type Getters<T> = { [K in keyof T as `get${Capitalize<string & K>}`]: () => T[K] };
export type UserGetters = Getters<PublicUser>;      // { getId(): string; getName(): string }
```

- `Omit<T, K>` accepts any key, so a typo silently keeps the property (a password leaks into a "public" type). Prefer `Pick` for allow-lists, or a strict omit: `type StrictOmit<T, K extends keyof T> = Omit<T, K>`.
- `Awaited<T>` unwraps nested promises; `ReturnType<typeof asyncFn>` is `Promise<...>`.
- Deeply recursive conditional types slow the checker and hit "Type instantiation is excessively deep" (TS2589). Prefer simple, explicit types in public APIs.

***

#### Enums, Namespaces and Non-Erasable Syntax

```ts
export enum Color { Red, Green }        // numeric enum: reverse mapping, any number is assignable
let c: Color = Color.Red;
c = 5;                                  // error TS2322 (5.0+: out-of-range literal rejected)
declare const n: number;
c = n;                                  // compiles: any number variable is accepted
export const name = Color[0];           // "Red" — reverse mapping exists for numeric enums only

export enum Status { Active = "active", Disabled = "disabled" } // string enum: nominal
export const s: Status = "active";      // error TS2322 string literal not assignable

// Usually better: a const object + union type (erasable, works with type stripping)
export const Role = { Admin: "admin", User: "user" } as const;
export type Role = (typeof Role)[keyof typeof Role];
export { c };
```

- `const enum` is inlined by `tsc`; single-file transpilers (esbuild, swc, Babel, Node type stripping) cannot inline enums from other files. `isolatedModules`/`verbatimModuleSyntax` forbid reading ambient const enums (TS2748). Never publish `const enum` in a library's `.d.ts` (consumers using `isolatedModules` cannot use them).
- Enums, `namespace`s with values, parameter properties, `import x = require()` aliases and legacy decorators emit runtime code. They are valid in 5.5 but break under Node's strip-only mode (`ERR_UNSUPPORTED_TYPESCRIPT_SYNTAX`, verified on Node 22–26). 5.5 has no `--erasableSyntaxOnly` (5.8) to enforce this; use an ESLint rule (for example `no-restricted-syntax` on `TSEnumDeclaration`, `TSModuleDeclaration`, `TSParameterProperty`) if the code runs through type stripping.
- `namespace` is for declaration files and legacy code; use ES modules. The `module Foo {}` keyword form for namespaces is deprecated in 6.0 — write `namespace`.
- Bit-flag enums are a legitimate numeric-enum use; document that any number is accepted.

***

#### Functions, Overloads and `this`

```ts
// Overloads: implementation signature is not callable from outside
export function parse(input: string): number;
export function parse(input: string[]): number[];
export function parse(input: string | string[]): number | number[] {
  return Array.isArray(input) ? input.map(Number) : Number(input);
}
const x = parse("1");        // number
const y = parse(["1"]);      // number[]
declare const z: string | string[];
parse(z);                    // error TS2769 no overload accepts the union — add one or use a union signature

// Optional vs. "| undefined" parameters
export function f1(a?: number) { return a; }        // may be omitted
export function f2(a: number | undefined) { return a; } // must be passed explicitly
f2();                        // error TS2554

// Explicit this parameter (erased at runtime)
export function onClick(this: { label: string }, ev: { x: number }) { return `${this.label}@${ev.x}`; }
export { x, y };
```

- **Return type `void` in a callback type does not mean "returns nothing"**: a function returning a value (or a `Promise`) is assignable to `() => void`. Passing an `async` function to `arr.forEach`, an `EventEmitter` listener, or Express middleware compiles; its rejection is unhandled. typescript-eslint's `no-misused-promises` catches this.

```ts
const results: number[] = [];
[1, 2].forEach(n => results.push(n));           // ok: push's return value is ignored
[1, 2].forEach(async n => { await Promise.resolve(n); throw new Error("lost"); }); // compiles; rejection unhandled
export { results };
```

- Prefer union parameter types over overloads when the return type does not depend on the input. Order overloads from most to least specific; the first match wins.
- Default parameter values are evaluated at call time (no shared mutable default like Python), but object defaults are shallow: `{ retries: 3, ...opts }` keeps `opts.retries === undefined` (see `exactOptionalPropertyTypes`).

***

#### Errors and Exceptions

```ts
export class HttpError extends Error {
  constructor(message: string, readonly status: number, options?: ErrorOptions) {
    super(message, options);          // ErrorOptions/cause: lib es2022
    this.name = "HttpError";
  }
}

export async function getJson(url: string): Promise<unknown> {
  let res: Response;
  try {
    res = await fetch(url);
  } catch (err) {                    // err: unknown (useUnknownInCatchVariables, part of strict)
    throw new Error(`network failure for ${url}`, { cause: err });
  }
  if (!res.ok) throw new HttpError(`GET ${url} failed`, res.status);
  return res.json() as Promise<unknown>;
}

export function message(err: unknown): string {
  return err instanceof Error ? err.message : String(err);
}
```

- `catch (e)` is `unknown` under `strict` (4.4+). Do not annotate `catch (e: any)` to get around it; narrow with `instanceof` or a guard. Anything can be thrown in JavaScript (strings, `undefined`, plain objects).
- **Target ES5 breaks `instanceof` on `Error` subclasses** (5.5's default target is still ES5). Verified on Node 22: compiled with `target: es5`, `new HttpError(...) instanceof HttpError` is `false` and methods declared on the subclass are `undefined`; with `target: es2015`+ both work. If ES5 output is unavoidable, call `Object.setPrototypeOf(this, new.target.prototype)` in the constructor.
- Always set `this.name` so logs and `err.toString()` show the class.
- Keep the original error: `new Error(msg, { cause })` (Node 16.9+, lib `es2022`). Losing the stack of the root cause is a common review finding.
- `Promise.reject("string")` / `throw "string"` lose stack traces; throw `Error` objects.
- Do not swallow errors with empty `catch {}` (optional catch binding, ES2019) unless the comment says why.

***

#### Async Code and Promises

```ts
declare function fetchUser(id: string): Promise<{ id: string }>;
declare function ready(): Promise<boolean>;

export async function bad(ids: string[]) {
  if (ready()) {                         // error TS2801 condition always true — missing await
    ids.forEach(async id => { await fetchUser(id); }); // fire-and-forget: errors unhandled, no ordering
  }
}

export async function good(ids: string[]) {
  if (await ready()) {
    // parallel, fail fast:
    const users = await Promise.all(ids.map(id => fetchUser(id)));
    // parallel, collect failures:
    const settled = await Promise.allSettled(ids.map(id => fetchUser(id)));
    const failed = settled.filter(r => r.status === "rejected");
    return { users, failed: failed.length };
  }
  return undefined;
}
```

What `tsc` does **not** catch (and typescript-eslint's type-aware rules do):

| Bug | tsc 5.5 | Lint rule |
|---|---|---|
| Promise not awaited as a statement (`save(x);`) | silent | `@typescript-eslint/no-floating-promises` |
| Async callback where `void` is expected (`forEach`, listeners) | silent | `@typescript-eslint/no-misused-promises` |
| Promise used as a condition | TS2801 for `if (p)`, `p ? a : b`, `if (x && p)`; **silent** for `while (p)`, `!p`, `p \|\| fallback` (verified) | `no-misused-promises` (`checksConditionals`) |
| `await` on a non-promise | silent | `@typescript-eslint/await-thenable` |
| `return promise` inside `try` without `await` (catch/finally do not cover it) | silent | `@typescript-eslint/return-await` (`in-try-catch`) |

```ts
declare function save(): Promise<void>;
export async function wrong() {
  try {
    return save();          // rejection escapes the catch below
  } catch (e) {
    console.error("never runs for save() rejections", e);
  }
}
export async function right() {
  try {
    return await save();    // awaited inside try: caught
  } catch (e) {
    console.error(e);
  }
}
```

More rules:

- Sequential `await` in a loop is fine when order matters or for rate-limiting; otherwise batch with `Promise.all` (bounded concurrency for large inputs — e.g. `p-limit`).
- Pass an `AbortSignal` (`AbortSignal.timeout(ms)`; `AbortSignal.any([...])` on Node 20.3+) to `fetch` and long operations; there is no default timeout on `fetch`.
- `Promise.withResolvers()` (lib `esnext.promise` in 5.5) needs Node 22+ at runtime.
- `async` functions always return a promise; an `async` function annotated `: Promise<void>` that should return data is a smell.
- Unhandled rejections terminate Node processes (default `--unhandled-rejections=throw` since Node 15).

***

#### Resource Management (`using`)

`using` and `await using` (5.2+) call `[Symbol.dispose]()` / `[Symbol.asyncDispose]()` at scope exit, in reverse order, even on exceptions. They need `lib` `esnext.disposable` (or `esnext`), and a runtime `Symbol.dispose` (Node 18.18+/20+; polyfill elsewhere).

```ts
class TempFile implements Disposable {
  constructor(readonly path: string) {}
  [Symbol.dispose]() { console.log("removing", this.path); }
}

export function work() {
  using a = new TempFile("/tmp/a");
  using b = new TempFile("/tmp/b");
  console.log("body");
} // prints body, removing /tmp/b, removing /tmp/a
```

**Emit trap (verified):** with `target: es2022` and lower, 5.5 downlevels `using` to helper code that runs on Node 18–24. With **`target: esnext`**, 5.5 emits `using` unchanged, which is a SyntaxError on Node 22 and earlier and runs only on Node 24+. Projects on Node ≤ 22 must not use `target: esnext` with `using`. `DisposableStack`/`AsyncDisposableStack` are declared by `lib.esnext.disposable` but exist natively only on Node 24+.

***

#### Decorators

5.5 supports two incompatible decorator systems:

| | Standard (TC39, 5.0+) | Legacy (`experimentalDecorators: true`) |
|---|---|---|
| Signature | `(value, context: ClassMethodDecoratorContext) => ...` | `(target, key, descriptor) => ...` |
| Parameter decorators | not supported | supported (used by Angular, NestJS, TypeORM, InversifyJS) |
| `emitDecoratorMetadata` | not supported | supported (`reflect-metadata`) |
| Metadata | `context.metadata` + `Symbol.metadata` (5.2+) | `reflect-metadata` |

```ts
export function logged<This, Args extends unknown[], R>(
  target: (this: This, ...args: Args) => R,
  context: ClassMethodDecoratorContext<This, (this: This, ...args: Args) => R>,
) {
  const name = String(context.name);
  return function (this: This, ...args: Args): R {
    console.log(`enter ${name}`);
    return target.call(this, ...args);
  };
}

export class Service {
  @logged greet(who: string) { return `hi ${who}`; }
}
```

- Do not mix the systems: frameworks built on legacy decorators (NestJS, Angular before its own migration, TypeORM) require `experimentalDecorators`; their decorators do not type-check as standard decorators.
- Decorator metadata: `Symbol.metadata` is undefined in every Node release up to 26 (verified). Code relying on `context.metadata` must polyfill: `(Symbol as { metadata?: symbol }).metadata ??= Symbol("Symbol.metadata");` before the decorated classes load.
- 5.5 requires parentheses around decorator expressions such as `@(super.decorate)` (TS1497); element access (`@obj["key"]`) was never valid decorator syntax (TS1005 on 5.4 and 5.5) and must be written `@(obj["key"])`.
- Node type stripping does not support decorators.

***

#### Idioms and Things to Avoid (BAD/GOOD)

##### Defaults: `||` vs `??`

```ts
interface Cfg { port?: number; name?: string; retries?: number }

// BAD: 0 and "" are valid values but fall back
export function portBad(c: Cfg) { return c.port || 8080; }       // port 0 → 8080
// GOOD
export function portGood(c: Cfg) { return c.port ?? 8080; }

// BAD: spreading options whose keys are present but undefined clobbers defaults
const defaults = { retries: 3, name: "svc" };
export const merged = { ...defaults, ...{ retries: undefined } }; // retries: undefined
// GOOD: drop undefined values first (or enable exactOptionalPropertyTypes)
export function withDefaults(c: Cfg) {
  return {
    retries: c.retries ?? defaults.retries,
    name: c.name ?? defaults.name,
  };
}
```

##### `find`, `at`, and Missing Elements

```ts
interface User { id: string; admin: boolean }
declare const users: User[];

// BAD: non-null assertion on a lookup
export const adminBad = users.find(u => u.admin)!;
// GOOD: handle absence explicitly
export function getAdmin(): User {
  const admin = users.find(u => u.admin);
  if (!admin) throw new Error("no admin configured");
  return admin;
}
```

##### Equality and Type Checks

```ts
declare const a: Date, b: Date;
export const sameBad = a === b;                      // reference equality: false for equal dates
export const sameGood = a.getTime() === b.getTime();

declare const v: unknown;
export const isObjBad = typeof v === "object";       // true for null too
export const isObjGood = typeof v === "object" && v !== null;

declare const x: string | number;
export const looseBad = x == 0;                      // "" == 0 and "0" == 0 are true
export const strictGood = x === 0;
```

##### Copies and Mutation

```ts
interface State { items: string[]; meta: { version: number } }

// BAD: shallow copy shares nested objects
export function bumpBad(s: State): State {
  const next = { ...s };
  next.meta.version++;            // mutates the caller's object
  return next;
}
// GOOD: copy what you change (or structuredClone for deep copies of plain data)
export function bumpGood(s: State): State {
  return { ...s, meta: { ...s.meta, version: s.meta.version + 1 } };
}
export const deep = structuredClone({ items: ["a"], meta: { version: 1 } });

// BAD: sort() mutates and returns the same array
export function sortedBad(xs: number[]) { return xs.sort(); }   // lexicographic, mutates input
// GOOD
export function sortedGood(xs: readonly number[]) { return xs.toSorted((p, q) => p - q); }
```

##### Readonly Parameters and Return Types

```ts
// BAD: accepts mutable array and leaks internal state
export class Registry {
  private items: string[] = [];
  all(): string[] { return this.items; }            // caller can push into internal array
}
// GOOD
export class SafeRegistry {
  readonly #items: string[] = [];
  add(item: string): void { this.#items.push(item); }
  all(): readonly string[] { return this.#items; }  // compile-time protection
  snapshot(): string[] { return [...this.#items]; } // runtime protection
}
```

`readonly` is compile-time only; `Object.freeze` is shallow and runtime-only. For APIs that hand out internal collections, return a copy if callers are untrusted.

##### Type Guards Instead of Casts

```ts
type Event = { type: "login"; user: string } | { type: "logout" };

// BAD
export function userOfBad(e: Event) { return (e as { user: string }).user; } // undefined for logout
// GOOD
export function userOf(e: Event): string | undefined {
  return e.type === "login" ? e.user : undefined;
}
```

##### Optional Chaining Hiding Bugs

```ts
interface Order { customer?: { email?: string } }
declare function send(to: string): void;

// BAD: silently does nothing when data is missing that must exist
export function notifyBad(o: Order) { if (o.customer?.email) send(o.customer.email); }
// GOOD: decide explicitly what missing data means
export function notify(o: Order): void {
  const email = o.customer?.email;
  if (email === undefined) throw new Error("order without customer email");
  send(email);
}
```

***

#### Runtime Validation and Untrusted Data

Types are erased. Nothing checks at runtime that a value matches its declared type. Every boundary — HTTP bodies, query strings, environment variables, `JSON.parse`, files, message queues, `localStorage`, third-party API responses — needs runtime validation that *produces* the type.

```ts
interface User { id: string; age: number }

// BAD: an annotation on JSON.parse output is an unchecked cast
export function parseUserBad(body: string): User {
  return JSON.parse(body);              // any -> User, no check at all
}

// GOOD: a hand-written guard (or a schema library: zod, valibot, typebox, ajv)
export function isUser(x: unknown): x is User {
  return typeof x === "object" && x !== null
    && "id" in x && typeof x.id === "string"
    && "age" in x && typeof x.age === "number" && Number.isFinite(x.age);
}
export function parseUser(body: string): User {
  const data: unknown = JSON.parse(body);
  if (!isUser(data)) throw new TypeError("invalid user payload");
  return data;
}
```

- Prefer schema libraries that infer the static type from the schema (`z.infer<typeof Schema>`), so type and check cannot drift. Check each library's documented minimum TypeScript version before upgrading it — a new major can require a newer compiler than 5.5.
- `process.env.X` is `string | undefined`. Parse and validate configuration once at startup; do not sprinkle `process.env.X!`.
- `as`-casts on `await res.json()` are the most common unchecked boundary in TypeScript code. Treat them like `JSON.parse` annotations.
- Narrowing with `"key" in x` then reading `x.key` gives `unknown` (4.9+) — that is correct; continue narrowing.

***

#### Security

TypeScript's types do not protect against injection, prototype pollution or unsafe deserialization. Review for:

- **Prototype pollution:** merging untrusted objects (`Object.assign(target, JSON.parse(body))`, recursive "deep merge", `obj[key] = value` with user-controlled `key`). Reject `__proto__`, `constructor`, `prototype` keys; use `Object.create(null)` or `Map` for user-keyed dictionaries; `Record<string, T>` typing does not prevent `__proto__` writes.
- **Injection:** SQL built with template literals, shell commands with `exec(\`...${input}\`)`. Use parameterized queries and `execFile`/`spawn` with argument arrays. Tagged-template SQL builders (`sql\`... ${x}\``) are safe only if the tag parameterizes.
- **Regular-expression DoS:** 5.5 checks regex *syntax* only. Nested quantifiers (`/(a+)+$/`) on user input remain catastrophic-backtracking risks.
- **Path traversal:** `path.join(base, userInput)` does not confine; resolve and check `resolved.startsWith(base + path.sep)`.
- **Type assertions on auth data:** `const claims = jwt.decode(token) as Claims` neither verifies the signature nor the shape. Verify then validate.
- **`eval`, `new Function`, `vm`** with user input; `vm` is not a sandbox.
- **`skipLibCheck` and `any`-typed dependencies** can hide unsafe APIs; types from DefinitelyTyped are community-written and may be wrong.
- **Secrets in `private` fields** are visible to `JSON.stringify` and logging; use `#private` or explicit `toJSON`.
- **Supply chain:** pin `typescript` exactly, use lockfiles, and avoid `postinstall`-heavy type packages. `@types/*` packages contain no runtime code but `typesVersions` redirects can change which files load.

***

#### Declaration Files and Library Authoring

- Generate declarations (`declaration: true`, `declarationMap: true`); do not hand-write `.d.ts` for TypeScript sources.
- Annotate exported functions' return types in libraries even without `isolatedDeclarations`: it stabilizes the public API and speeds up checking.
- Inferred types that reference non-exported or transitive types cause TS2742/TS4023/TS4058 errors; export the referenced types or annotate.
- 5.5 no longer synthesizes `/// <reference types="node" />` into `.d.ts` files. If your published types reference Node globals (`Buffer`, `NodeJS.Timeout`), consumers need `@types/node`; either list it in `peerDependencies`/`dependencies` or add `/// <reference types="node" preserve="true" />` to the source.
- Avoid `export default` in libraries consumed from CommonJS: `require("lib").default` is needed, and `esModuleInterop` differences make types and runtime disagree. Named exports are portable.
- Test published types with the oldest TypeScript version you claim to support (`typesVersions` if you need per-version declarations) and with `@arethetypeswrong/cli`.
- Ambient module declarations (`declare module "x" { ... }`) in an application are a last resort for untyped packages; module augmentation (`declare module "express-serve-static-core" { interface Request { user?: User } }`) must live in a module file (with an `import`/`export`), or it replaces the module instead of augmenting it.
- `declare global { ... }` augmentations also need a module file.

***

#### JavaScript Projects (`checkJs`, JSDoc)

```jsonc
{
  "compilerOptions": {
    "allowJs": true,
    "checkJs": true,
    "noEmit": true,
    "strict": true,
    "module": "nodenext",
    "target": "es2022",
    "types": ["node"]
  },
  "include": ["src"]
}
```

- Use `/** @import { T } from "./types.js" */` (**new in 5.5**) for type-only imports in `.js` files.
- `// @ts-check` at the top of a file enables checking for that file only; `// @ts-nocheck` disables it.
- JSDoc casts are written `/** @type {Foo} */ (expr)` — the parentheses are required.
- `@satisfies` (4.9+), `@overload` (5.0+), `@template` with constraints and `@typedef` all work in 5.5.

***

#### Performance (Compiler and Runtime)

##### Type-Checking Performance

- Measure with `tsc --extendedDiagnostics` (check time, memory, instantiations) and `tsc --generateTrace <dir>` (open in a trace viewer / `@typescript/analyze-trace`).
- Use `incremental: true` (writes `.tsbuildinfo`) and project references with `tsc -b` for large repos. In 5.5, `tsc -b` stops building downstream projects when an upstream project has errors (5.6 changed this).
- Set `types` explicitly; loading every `@types` package costs time and leaks globals (e.g. `@types/jest` globals into production code).
- `skipLibCheck: true` is standard for applications. It skips *all* `.d.ts` files, including your own generated ones.
- Large unions (thousands of members), deeply recursive conditional types and huge inferred object literal types slow everything down. Prefer interfaces over intersections of object types for extension (`interface A extends B, C` is cached; `B & C` is recomputed).
- Annotate return types of exported functions in hot modules; inference across big call graphs is expensive.
- 5.5 itself is faster than 5.4 for language-service operations, and `transpileModule` (ts-jest, ts-loader `transpileOnly`) skips full checking — roughly 2x faster in the TypeScript team's tests.[^1]

##### Emitted Code

- Every `target` lower than the runtime supports adds helpers and slower downlevel code (async/generators to state machines at ES5/ES2015-2016, class fields to constructor assignments). Match `target` to the oldest runtime.
- `importHelpers: true` + `tslib` deduplicates helpers in libraries/bundles; the `tslib` major must match what 5.5 emits (`tslib@^2`).
- `downlevelIteration` (needed for correct `for...of` on Maps/Sets at ES5) is slow; avoid ES5 output.
- Enums and namespaces produce runtime objects; `const` objects are cheaper and erasable.

***

#### Testing

##### Type Tests

```ts
// Compile-time assertions without a library
type Equals<A, B> = (<T>() => T extends A ? 1 : 2) extends <T>() => T extends B ? 1 : 2 ? true : false;
type Expect<T extends true> = T;

const isNumber = (x: unknown) => typeof x === "number";
export type _t1 = Expect<Equals<typeof isNumber, (x: unknown) => x is number>>; // 5.5 inferred predicate

// @ts-expect-error — the next line must fail to compile; the directive errors if it does not
const n: number = "nope";
// @ts-expect-error -- error TS2578 (reported on this directive line: the next line has no error)
const ok: number = 1;
export { n, ok };
```

- Prefer `@ts-expect-error` over `@ts-ignore`: the former fails when the error disappears (TS2578), so suppressions do not outlive their reason. Require a comment explaining every suppression.
- Vitest's `expectTypeOf`/`assertType` and the `tsd`/`tstyche` tools run type tests; check their supported TypeScript range against 5.5.

##### Running Tests

- Test runners that transpile per file (Vitest/esbuild, `ts-jest` with `isolatedModules`, `tsx`, Node type stripping) **do not type-check**. CI must run `tsc --noEmit` (or `tsc -b`) separately; a green test run proves nothing about types.
- `ts-jest` benefits from the 5.5 `transpileModule` speed-up; with `isolatedModules: true` it skips type-checking entirely.
- Mock with types: `vi.mocked(fn)` / `jest.mocked(fn)` instead of `fn as jest.Mock`.
- Keep test files out of the production build (`exclude` or a separate `tsconfig.build.json`), but inside a type-checked project so test types are checked too.

***

#### Tooling, Linting and Formatting

- **tsc** is the type checker of record. Run `tsc --noEmit` (applications), `tsc -b` (references), or `tsc -p tsconfig.build.json` (libraries) in CI, pinned to the same 5.5.4 as the editor. In VS Code, select the workspace version ("Use Workspace Version") — the bundled TypeScript is far newer and will show 5.6+ behaviour (for example no `isolatedDeclarations` noEmit bug, different truthiness errors).
- **typescript-eslint v8** supports 5.5.[^15] Enable type-aware linting (`parserOptions.projectService: true`) and at least `no-floating-promises`, `no-misused-promises`, `await-thenable`, `return-await`, `no-unnecessary-type-assertion`, `no-unsafe-*` (assignment/call/member-access/return/argument), `switch-exhaustiveness-check`, `consistent-type-imports` (or rely on `verbatimModuleSyntax`), `ban-ts-comment` (require descriptions), `prefer-nullish-coalescing`, `strict-boolean-expressions` (for codebases that want no truthiness on strings/numbers). The `no-constant-binary-expression` core rule catches part of what 5.6's truthy/nullish checks catch.
- **Prettier/Biome/dprint** for formatting; do not hand-format. Check that the formatter's parser understands the syntax you use (5.5 adds no new syntax, so any 2024+ formatter version works).
- **Running TypeScript directly:**
  - `tsx` (esbuild) and `ts-node --transpile-only`/`--swc` skip type checks; `ts-node` without them type-checks but is slow and its ESM loader is fragile. `ts-node` 10.9.2 is the last release (December 2023).
  - **Node type stripping** (Node 22.18+, 23.6+; stable in 24.12+/25.2+) ignores `tsconfig.json`, requires explicit `.ts` extensions in relative imports, requires `import type` for type-only imports, and rejects enums, value namespaces, parameter properties, `import =` aliases and decorators.[^12] With 5.5, type-check such code with `"noEmit": true`, `"allowImportingTsExtensions": true`, `"verbatimModuleSyntax": true` and `"module": "nodenext"`; enforce erasable syntax with lint rules because `--erasableSyntaxOnly` is 5.8+.
- **Bundlers** (Vite, esbuild, webpack + ts-loader `transpileOnly`, Rollup, tsup) strip types per file. Keep `isolatedModules`/`verbatimModuleSyntax` on.
- **Declaration bundlers** (`api-extractor`, `rollup-plugin-dts`, `tsup --dts`) run the TypeScript API; check their bundled/peer TypeScript version. Some declaration tools bundle a newer TypeScript and may emit `.d.ts` features 5.5 consumers cannot parse.
- **Named ESM imports of the compiler API** (`import { createProgram } from "typescript"`) work from 5.5 on; earlier versions needed the default import.

***

#### Observability and Debugging

- Emit source maps (`sourceMap: true`, or `inlineSourceMap` for scripts) and run Node with `--enable-source-maps` so stack traces point to `.ts` lines. Do not ship `inlineSources`/source maps with secrets to public CDNs unintentionally.
- Preserve error causes (`{ cause }`) and set `name` on custom errors (see Errors).
- Prefer structured loggers (pino, winston) with typed log context objects; never log whole request objects (headers contain tokens).
- `tsc --traceResolution` explains module-resolution failures; `tsc --explainFiles` shows why each file is in the program (use it when unexpected `@types` globals appear); `tsc --showConfig` prints the effective configuration after `extends` and `${configDir}` substitution.
- `tsc --listFilesOnly` and `--extendedDiagnostics` help diagnose slow builds.

***

#### Removed / Changed in 5.5 — Migration Notes

Upgrading from 5.4 to 5.5 (all verified on 5.4.5 vs 5.5.4):

1. **Deprecated options become no-ops.** Remove `importsNotUsedAsValues`, `preserveValueImports` (→ `verbatimModuleSyntax`), `suppressImplicitAnyIndexErrors`, `suppressExcessPropertyErrors`, `keyofStringsOnly`, `noStrictGenericChecks`, `noImplicitUseStrict`, `charset`, `out` (→ `outFile`), `target: ES3`, `prepend`. If a config keeps them with `ignoreDeprecations: "5.0"`, expect new type errors and changed emit; 5.6+ rejects them outright.
2. **Inferred type predicates** can narrow arrays (`.filter(x => x !== null)`) so later `push(null)` fails; annotate the variable with the wider type.
3. **New regex errors (TS1499–TS1534 range)** on literals that were always wrong or use features newer than `target`; fix the pattern or raise `target`.
4. **Decorators** like `@super.method` need parentheses (TS1497): `@(super.method)`.
5. **`type undefined`** declarations must be renamed (TS2457).
6. **Declaration output** loses synthesized `/// <reference types="..." />` lines; add `preserve="true"` to the ones that must stay.
7. **DOM lib** updates may surface new errors in browser code.
8. **Fewer TS2742** errors in pnpm workspaces when dependencies are declared.
9. **`isolatedDeclarations`** is opt-in; remember it is silent under `noEmit` in 5.5.
10. **`@types/*` versions**: newest `@types/node` (26.x) requires 5.6 — pin `@types/node@ts5.5`.

***

#### Production / Review Checklist

##### Version and Configuration

- [ ] Lockfile resolves `typescript` to 5.5.x (ideally 5.5.4); `package.json` uses an exact or `~5.5.4` range
- [ ] No 5.6+ APIs/options in code or config (iterator helpers, `noUncheckedSideEffectImports`, `erasableSyntaxOnly`, `rewriteRelativeImportExtensions`, `es2024`, `import defer`, `getOrInsert`, `RegExp.escape`, `Temporal`)
- [ ] No 5.0-deprecated options left in tsconfig (they are ignored in 5.5)
- [ ] `@types/node` and other `@types/*` resolved to their `ts5.5` tags (or verified to compile without `skipLibCheck`)
- [ ] `strict: true`; `noUncheckedIndexedAccess`, `noImplicitOverride` on for new code
- [ ] `module`/`moduleResolution` match the runtime (`nodenext` for Node, `bundler`/`preserve` for bundlers); never `module: esnext` without `moduleResolution`
- [ ] `target`/`lib` match the oldest runtime; `lib: ["esnext"]` only if every runtime has the APIs used (Set methods, `groupBy`, `withResolvers` need Node 22)
- [ ] `target: esnext` not used with `using` on Node ≤ 22
- [ ] `types` set explicitly; `verbatimModuleSyntax` or `isolatedModules` on when a transpiler other than tsc runs
- [ ] `isolatedDeclarations` (if enabled) checked by an emitting build, not by `tsc --noEmit`
- [ ] Shared configs with `${configDir}` not consumed by TypeScript ≤ 5.4

##### Types and Code Correctness

- [ ] No unexplained `any`, `as` casts on external data, `!` assertions or `as unknown as`
- [ ] External input validated at runtime (schema or guard) before being typed
- [ ] Exhaustive `switch` on discriminated unions with `assertNever`
- [ ] `filter` callbacks use explicit comparisons (`!== undefined`) rather than `Boolean`/`!!` when `0`/`""` are valid
- [ ] Explicit type predicates and assertion functions are correct (they are not checked)
- [ ] Truthiness checks and `||` defaults do not drop valid `0`/`""`/`false`; `??` used for nullish defaults
- [ ] Callback members declared as properties, not methods, where variance matters
- [ ] `Omit` keys are real keys; allow-lists use `Pick`
- [ ] Regex literals compile without TS15xx errors; ES2025 regex syntax (duplicate named groups, `(?i:)`) not used on Node ≤ 22
- [ ] JSON imports in ESM use `with { type: "json" }`, never `assert`

##### Async and Errors

- [ ] No floating promises; no `async` callbacks where `void` is expected
- [ ] `return await` inside `try`
- [ ] `catch` variables treated as `unknown`; errors wrapped with `{ cause }`; custom errors set `name`
- [ ] Not compiling to ES5 if code relies on `instanceof` for `Error` subclasses
- [ ] Timeouts/`AbortSignal` on network calls; bounded concurrency for large `Promise.all`

##### Modules and Packaging

- [ ] Relative imports include `.js` under `nodenext`; `.ts` only with `allowImportingTsExtensions` + `noEmit`
- [ ] `paths` aliases resolvable at runtime (subpath `imports`, bundler) — tsc does not rewrite them
- [ ] Libraries: `exports` with `types` first, separate `.d.ts`/`.d.cts` for dual packages, checked with `@arethetypeswrong/cli`
- [ ] Published `.d.ts` keeps required `/// <reference types preserve="true" />`
- [ ] No `const enum` in published declarations; enums/namespaces/parameter properties avoided in code run by Node type stripping

##### Security

- [ ] No prototype pollution via user-controlled keys or deep merges
- [ ] Parameterized SQL, argument-array process spawning, confined file paths
- [ ] No catastrophic-backtracking regexes on user input
- [ ] Secrets not stored in TypeScript-`private` fields that get serialized

##### Tooling and Tests

- [ ] CI runs `tsc` (tests that transpile per file do not type-check)
- [ ] typescript-eslint type-aware rules enabled (`no-floating-promises`, `no-misused-promises`, `no-unsafe-*`)
- [ ] `@ts-expect-error` with a reason instead of `@ts-ignore`
- [ ] Editor uses the workspace TypeScript 5.5, not the bundled newer version
- [ ] Source maps enabled and `--enable-source-maps` used in production Node processes

***

[^1]: TypeScript 5.5 release notes (Microsoft). https://www.typescriptlang.org/docs/handbook/release-notes/typescript-5-5.html ; Announcing TypeScript 5.5 (June 20, 2024). https://devblogs.microsoft.com/typescript/announcing-typescript-5-5/
[^2]: npm registry, `typescript` package version history and dist-tags (`npm view typescript time`). https://www.npmjs.com/package/typescript?activeTab=versions ; GitHub release v5.5.4. https://github.com/microsoft/TypeScript/releases/tag/v5.5.4
[^3]: Announcing TypeScript 7.0 / TypeScript 7.0 released (July 2026). https://devblogs.microsoft.com/typescript/ ; InfoWorld, "Go-based TypeScript 7.0 arrives". https://www.infoworld.com/article/4196378/go-based-typescript-7-0-arrives.html
[^4]: DefinitelyTyped README — Support Window and `ts<version>` dist-tags. https://github.com/DefinitelyTyped/DefinitelyTyped#support-window
[^5]: TypeScript 5.0–5.4 release notes. https://www.typescriptlang.org/docs/handbook/release-notes/typescript-5-4.html (and the linked 5.0–5.3 pages)
[^6]: microsoft/TypeScript PR #57465 — Infer type predicates from function bodies; Dan Vanderkam, "The Making of a TypeScript Feature: Inferring Type Predicates". https://github.com/microsoft/TypeScript/pull/57465 , https://effectivetypescript.com/2024/04/16/inferring-a-type-predicate/
[^7]: TC39 proposal-set-methods (ES2025). https://github.com/tc39/proposal-set-methods
[^8]: Isolated Declarations: State of the Feature (microsoft/TypeScript #58944). https://github.com/microsoft/TypeScript/issues/58944
[^9]: microsoft/TypeScript PR #58042 — `${configDir}` substitution in tsconfig. https://github.com/microsoft/TypeScript/pull/58042
[^10]: TypeScript Flag Deprecation Plan (#51000) and 5.0 deprecations (#51909). https://github.com/microsoft/TypeScript/issues/51000 , https://github.com/microsoft/TypeScript/issues/51909
[^11]: TypeScript 5.6, 5.7, 5.8, 5.9 and 6.0 release notes. https://www.typescriptlang.org/docs/handbook/release-notes/typescript-5-6.html , https://www.typescriptlang.org/docs/handbook/release-notes/typescript-5-7.html , https://www.typescriptlang.org/docs/handbook/release-notes/typescript-5-8.html , https://www.typescriptlang.org/docs/handbook/release-notes/typescript-5-9.html , https://www.typescriptlang.org/docs/handbook/release-notes/typescript-6-0.html
[^12]: Node.js documentation — Modules: TypeScript (type stripping history and limitations). https://nodejs.org/api/typescript.html
[^13]: TypeScript handbook — Modules reference (module, moduleResolution, nodenext, bundler). https://www.typescriptlang.org/docs/handbook/modules/reference.html
[^14]: TSConfig reference. https://www.typescriptlang.org/tsconfig/
[^15]: typescript-eslint — supported TypeScript versions and rules. https://typescript-eslint.io/users/dependency-versions/ , https://typescript-eslint.io/rules/
