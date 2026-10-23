### TypeScript 4.x (4.0–4.9): The Complete Production Developer Guide

#### Overview

> **Version**: TypeScript 4.x, targeting **4.9** and valid for every project pinned to 4.0–4.9. TypeScript 4.9 was released on November 15, 2022 (npm `4.9.3`); its latest and **final** patch is **4.9.5**, published to npm on January 30, 2023 (GitHub release dated January 31, 2023). 4.9.5 contains only two backports, one of which teaches 4.9 to accept `"ignoreDeprecations": "5.0"` for smoother upgrades.[^1][^2][^3]
> **Status as of October 2026**: the entire 4.x line is **unmaintained and receives no updates** — no bug fixes, no security fixes, no new `lib.d.ts` declarations. TypeScript has no LTS releases; fixes only ever ship in new minor versions. The successor 5.0 shipped on March 16, 2023; 5.9.3 (September 30, 2025) is the final 5.x; **6.0** (6.0.3, April 16, 2026) is the last JavaScript-based compiler; the current release is **TypeScript 7.0.2** (the native Go compiler, July 8, 2026), with 7.1 in beta.[^2][^4] DefinitelyTyped only tests `@types/*` packages against TypeScript versions less than two years old, so every 4.x version has left that window (4.9 left it in November 2024): new `@types` releases routinely use 5.x syntax and **fail to parse on 4.x** (see [`@types` pinning](#types-packages-must-be-pinned)).[^5]
> **Scope**: the TypeScript language and type system as usable in 4.x, with every feature marked by the minor version that introduced it, the compiler options and module resolution modes of 4.x, reviewer-relevant compiler bugs and pitfalls, tooling versions that still support 4.x, and migration notes to 5.0.
> **Note**: this document is self-contained. Sections and examples that need a minor newer than 4.0 are marked **(4.N+)**. Every `ts` example was compiled with the final patch of each minor — 4.0.8, 4.1.6, 4.2.4, 4.3.5, 4.4.4, 4.5.5, 4.6.4, 4.7.4, 4.8.4 and 4.9.5 — and each "(4.N+)" claim was proven by showing that the previous minor rejects or mis-types the example (and 5.0.4 was used for the 5.0 boundary). Unless an example states otherwise with a `// @option: value` line (the TypeScript playground convention), it is compiled with `strict: true`, `target: es2020`, `module: commonjs`. `// @since: 4.N` marks the minimum version; `// error` marks a line that must fail to compile, `// error 4.N+` a line that fails only from 4.N on, and `// error <4.N` a line that fails only before 4.N.

| Minor | First stable (npm) | Final patch | Headline features |
|---|---|---|---|
| 4.0 | 4.0.2 — Aug 20, 2020 | 4.0.8 — Jun 16, 2021 | variadic tuple types, labeled tuple elements, `&&=` `\|\|=` `??=`, `catch (e: unknown)`, constructor-based property inference |
| 4.1 | 4.1.2 — Nov 19, 2020 | 4.1.6 — Jun 16, 2021 | template literal types, `as` key remapping, recursive conditional types, `--noUncheckedIndexedAccess`, `jsx: react-jsx` |
| 4.2 | 4.2.2 — Feb 23, 2021 | 4.2.4 — Apr 7, 2021 | leading/middle rest elements in tuples, `abstract` construct signatures, `--noPropertyAccessFromIndexSignature`, `--explainFiles` |
| 4.3 | 4.3.2 — May 26, 2021 | 4.3.5 — Jun 30, 2021 | `override` + `--noImplicitOverride`, separate getter/setter write types, `#private` methods/accessors, `static` index signatures, `target: es2021` |
| 4.4 | 4.4.2 — Aug 26, 2021 | 4.4.4 — Oct 12, 2021 | control-flow analysis of aliased conditions, `symbol`/template-pattern index signatures, `--useUnknownInCatchVariables` (in `strict`), `--exactOptionalPropertyTypes`, `static {}` blocks |
| 4.5 | 4.5.2 — Nov 17, 2021 | 4.5.5 — Jan 20, 2022 | `Awaited<T>`, `type` modifiers on import names, tail-recursive conditional types, `module: es2022` (top-level `await`), `#x in obj`, import assertions, `--preserveValueImports` |
| 4.6 | 4.6.2 — Feb 28, 2022 | 4.6.4 — Apr 28, 2022 | code before `super()`, narrowing of destructured discriminated unions, dependent parameters, `target: es2022`, `--generateTrace` analysis tooling |
| 4.7 | 4.7.2 — May 24, 2022 | 4.7.4 — Jun 17, 2022 | `module`/`moduleResolution` `node16`/`nodenext`, `.mts`/`.cts`, `moduleDetection`, instantiation expressions, `infer X extends C`, `in`/`out` variance annotations, `moduleSuffixes` |
| 4.8 | 4.8.2 — Aug 25, 2022 | 4.8.4 — Sep 27, 2022 | improved intersection/union reduction and narrowing (`unknown` ≈ `{} \| null \| undefined`, `NonNullable<T> = T & {}`), unconstrained `T` no longer assignable to `{}`, errors on `x === []` |
| 4.9 | 4.9.3 — Nov 15, 2022 | 4.9.5 — Jan 30, 2023 | `satisfies`, `in` narrowing for unlisted properties, `accessor` auto-accessors, errors on `x === NaN`, `Promise.resolve` uses `Awaited` |

Global truths about TypeScript 4.x:

- **Types are erased.** TypeScript checks at compile time and emits JavaScript; annotations, interfaces and type aliases do not exist at runtime. A type assertion (`as`) never converts or validates anything. Data that crosses a trust boundary (HTTP, JSON, files, environment variables, `postMessage`) must be validated at runtime.
- **Structural typing.** Compatibility is decided by shape, not by declared name.
- **Soundness is not a goal.** `any`, type assertions, non-null assertions (`!`), method-parameter bivariance, unchecked index access (without `--noUncheckedIndexedAccess`) and declaration files that lie are all holes. Treat each one as an unverified claim.
- **`tsc` defaults are ancient.** Without a tsconfig, every 4.x `tsc` defaults to **`target: ES3`** (5.0 moved the default to ES5), `module: commonjs`, non-strict checking and automatic inclusion of every visible `@types/*` package. With `module` set to anything except `commonjs` and no explicit `moduleResolution`, 4.x silently uses **`classic`** resolution, which cannot find packages in `node_modules` (`TS2792: Cannot find module 'pkg'. Did you mean to set the 'moduleResolution' option to 'node'…`). Real projects must set these explicitly.
- **Emit is not bundling.** `tsc` never rewrites import specifiers, never adds polyfills (only syntax down-levelling, with helpers from `tslib` under `importHelpers`), and never resolves `paths` aliases at runtime.
- **The `lib` in 4.9 stops at ES2022.** There is no `lib`/`target` `es2023` in 4.x: `Array.prototype.findLast`, `toSorted`, `Object.groupBy`, `Promise.withResolvers`, `Set.prototype.union` and friends have no built-in declarations in any 4.x version.

***

#### What's New, Minor by Minor

This section lists every reviewer-relevant change in 4.0–4.9. A project pinned to 4.N has every feature up to and including 4.N and **none** after it. Check the pinned version (`package.json`, the lockfile, `npx tsc -v`) before flagging or recommending anything in this list.

##### 4.0 (August 2020)[^6]

- **Variadic tuple types**: spreads of generic tuples in tuple types (`[...T, ...U]`) and in rest parameters; spreads may appear anywhere in a tuple type.
- **Labeled tuple elements**: `[start: number, end: number]`, `[first: string, ...rest: number[]]`.
- **Class property inference from constructors** under `noImplicitAny` when properties have no annotation.
- **Short-circuiting assignment operators** `&&=`, `||=`, `??=` (down-levelled for targets below ES2021).
- **`catch (e: unknown)`** annotations (only `unknown` or `any` are allowed on a catch binding).
- **Custom JSX fragment factories**: the `jsxFragmentFactory` option and the `/** @jsxFrag */` pragma; `--incremental` with `--noEmit`; faster `--build` with `--noEmitOnError`.
- Editor: `/** @deprecated */` support (strike-through), "convert to optional chaining".
- **Breaking**: a property in a derived class can no longer override a base accessor (and vice versa) — always an error now (TS2610/TS2611); the operand of `delete` must be optional/`any`/`unknown`/`never` under `strictNullChecks` (TS2790); `document.origin` removed from `lib.dom.d.ts`.

##### 4.1 (November 2020)[^7]

- **Template literal types** (`` `${A}-${B}` ``) with the intrinsic string types `Uppercase`, `Lowercase`, `Capitalize`, `Uncapitalize`.
- **Key remapping in mapped types**: `{ [K in keyof T as NewKey]: … }`, including filtering with `never`.
- **Recursive conditional types** (a conditional type may reference itself).
- **`--noUncheckedIndexedAccess`**: index-signature and array element access includes `undefined`.
- `paths` without `baseUrl`; `checkJs` implies `allowJs`; `jsx: react-jsx` / `react-jsxdev` (React 17 automatic runtime); JSDoc `@see`.
- **Breaking**: `abstract` members cannot be `async`; `any`/`unknown` propagate through falsy positions of `&&` (`x && y` with `x: unknown` is `unknown`); **`resolve` in `new Promise(resolve => …)` is no longer optional** — use `new Promise<void>(…)`; conditional spreads (`...(c && obj)`) produce optional properties; unmatched parameters are no longer related to `any`.

##### 4.2 (February 2021)[^8]

- Smarter type-alias preservation in error messages and hovers.
- **Leading and middle rest elements in tuples**: `[...string[], number]`, `[boolean, ...string[], number]`.
- **Stricter `in` operator** checks: a right operand of primitive type was already TS2361; 4.2 extends the check to type parameters constrained to primitives (`function f<T extends string>(x: T) { "a" in x }`).
- Relaxed rules between optional properties and string index signatures: a type with optional properties (`{ a?: number }`) is assignable to `{ [k: string]: number }`.
- **`--noPropertyAccessFromIndexSignature`**: forces `obj["key"]` for keys that only come from an index signature.
- **`abstract` construct signatures**: `abstract new () => T`.
- **`--explainFiles`**: prints why each file is in the program.
- Improved uncalled-function checks inside `&&`/`||` (`if (user.isAdmin && …)` where `isAdmin` is a method).
- Destructured variables prefixed with `_` can be explicitly unused (`noUnusedLocals`).
- **Breaking**: `noImplicitAny` errors for unused-context `yield` expressions (TS7057); import paths ending in `.d.ts` are an error; spread-tuple size limits; parsing of `f<T>(x)` in JavaScript files changed.

##### 4.3 (May 2021)[^9]

- **Separate write types on properties**: a setter may accept a wider type than the getter returns (the getter type must still be assignable to the setter type — unrelated types arrive only in 5.1).
- **`override` keyword** and **`--noImplicitOverride`**.
- Template string type improvements (inference to and assignability between template literal types).
- **ECMAScript `#private` methods and accessors** (fields came in 3.8); `static` index signatures; `ConstructorParameters` works on abstract classes.
- Contextual narrowing for generics; **always-truthy `Promise` checks** in conditions (`if (promise)` → TS2801 "Did you forget to use 'await'?").
- `target`/`lib` **`es2021`** (`String.prototype.replaceAll`, `Promise.any`, `AggregateError`, `WeakRef`, `FinalizationRegistry`, logical-assignment kept native).
- **Breaking**: comparing a **union enum** with a numeric literal it can never equal is an error (TS2367); `useDefineForClassFields` defaults to `true` for `target: esnext`; DOM types for non-implemented APIs removed.

##### 4.4 (August 2021)[^10]

- **Control-flow analysis of aliased conditions and discriminants**: `const isString = typeof x === "string"; if (isString) x.toUpperCase()` narrows `x` (if `x` is a `const`, `readonly` property or unassigned parameter).
- **`symbol` and template-string-pattern index signatures** (`[key: symbol]: T`, `` [key: `data-${string}`]: T ``) and union index signatures.
- **`--useUnknownInCatchVariables`** — part of `strict`; makes `catch (e)` bind `unknown`.
- **`--exactOptionalPropertyTypes`** (not part of `strict`): `prop?: T` no longer accepts an explicit `undefined`.
- **`static { … }` blocks** in classes.
- Faster declaration emit and incremental builds; inlay hints in editors.
- **Breaking**: `strict` now includes `useUnknownInCatchVariables`; imported functions are called with an undefined `this` (`(0, mod.fn)()` emit); always-truthy promise checks extend to call expressions (`if (foo())`); `abstract` properties cannot have initializers.

##### 4.5 (November 2021)[^11]

- **`Awaited<T>`** utility type; `Promise.all`/`race`/`allSettled`/`any` and `await` use it.
- **`type` modifiers on import names**: `import { fn, type T } from "./m"` (and `export { type T }`).
- **Tail-recursion elimination on conditional types**: tail-recursive conditional types may recurse 1000 times; the general instantiation-depth limit rises from 50 (4.0–4.4) to 100 (from the compiler source).
- **`module: es2022`** — top-level `await` with ES modules; **`#field in obj`** private-field presence checks; template string types as discriminants.
- **Import assertions** (`import data from "./x.json" assert { type: "json" }`) — syntax only; the later `with` form (import attributes) arrives in 5.3.
- **`--preserveValueImports`** (requires `module` ≥ `es2015`, TS5095 otherwise; combined with `isolatedModules`, every type must be imported with `type`, TS1444); `lib` replacement via `@typescript/lib-*` packages in `node_modules`.
- **Breaking**: `Awaited` changes some inferred generic types; a compiler option at the root of a tsconfig without a `compilerOptions` object is an error.

##### 4.6 (February 2022)[^12]

- **Code before `super()`** in constructors, as long as it does not reference `this`.
- **Control-flow analysis for destructured discriminated unions** (`const { kind, payload } = action; if (kind === "a") payload…`).
- **Control-flow analysis for dependent parameters** (rest parameters typed as a union of tuples).
- Improved recursion-depth checks and indexed-access inference.
- **`target`/`lib` `es2022`**: `Array.prototype.at`, `Object.hasOwn`, `Error` `cause`, class fields emitted natively, top-level `await` targets.
- `--generateTrace` output analysable with the `@typescript/analyze-trace` tool; JavaScript files always get grammar/binding errors.
- **Breaking**: object rest on generic objects drops methods and non-public members (`const { a, ...rest } = genericT` — `rest` no longer has the methods).

##### 4.7 (May 2022)[^13]

- **ECMAScript-module support for Node.js**: `module`/`moduleResolution` `node16` and `nodenext`, `"type": "module"` in `package.json`, `.mts`/`.cts` source files (`.mjs`/`.cjs`, `.d.mts`/`.d.cts` output), `package.json` `exports`/`imports`, self-referencing. (In 4.5 and 4.6 `node12`/`nodenext` existed only in nightly builds; stable 4.5/4.6 reject them with TS4124.)
- **`moduleDetection`** (`auto`, `legacy`, `force`).
- Control-flow analysis for bracketed element access with literal/unique-symbol keys (`obj[key]` narrows when `key` is `const`).
- Improved function inference in objects and methods (left-to-right context-sensitive inference).
- **Instantiation expressions**: `const makeStringBox = makeBox<string>;`.
- **`extends` constraints on `infer`**: `T extends [infer H extends string, ...unknown[]] ? H : never`.
- **Optional variance annotations** `in`/`out`/`in out` on type parameters.
- **`moduleSuffixes`** (React Native style `.ios.ts` lookups); `resolution-mode` on `/// <reference types>` (on `import type` it is nightly-only in 4.7–5.2).
- **Breaking**: `readonly` tuples have a `readonly length`; JSX spreads of `unknown`/`never` rejected; template-string interpolation of a generic constrained to `symbol` is an error (TS2731).

##### 4.8 (August 2022)[^14]

- **Improved intersection reduction, union compatibility and narrowing** under `strictNullChecks`: `unknown` behaves like `{} | null | undefined`; `{}` intersected with an object type simplifies; `NonNullable<T>` is now `T & {}`; narrowing a generic `T` against `null`/`undefined` produces `T & {}`.
- Improved inference for `infer X extends number/bigint/boolean` inside template literal types (`"100"` → `100`).
- **Errors when comparing object and array literals** (`x === []`, `x === {}` → TS2839).
- Binding patterns no longer contribute inference candidates; `--build`/`--watch`/`--incremental` speedups; file-watching fixes across `git checkout`.
- **Breaking**: **an unconstrained type parameter is no longer assignable to `{}`** (`function f<T>(x: T) { const o: {} = x; }` → TS2322); `Error.cause` is typed `unknown`; unused renames in binding patterns inside type signatures (`declare function f({ a: string }): void`) are errors (TS2842; earlier versions only report implicit `any`, TS7031, under `noImplicitAny`); JavaScript files cannot `import`/`export` types.

##### 4.9 (November 2022)[^1]

- **`satisfies` operator**: validates an expression against a type without widening its inferred type.
- **Unlisted-property narrowing with `in`**: `"name" in obj` narrows `object` to `object & Record<"name", unknown>`; the left operand must be `string | number | symbol` and the right operand `object` — so `"a" in x` with an **unconstrained** type parameter `x: T` becomes an error (TS2322 "Type 'T' is not assignable to type 'object'"; 4.8 accepts it). Constrain `T extends object`.
- **Auto-accessors**: `accessor name: string` (ECMAScript decorators-proposal syntax; desugars to a private field plus `get`/`set`).
- **Errors on direct `NaN` equality** (`x === NaN`, TS2845, suggests `Number.isNaN`).
- File watching uses file-system events by default; "Remove Unused Imports" and "Sort Imports" editor commands; go-to-definition on `return`.
- **Breaking**: `Promise.resolve` uses `Awaited` (can change inferred types); `.js` emit no longer elides imports; under `node16`/`nodenext`, **`exports` is prioritized over `typesVersions`**.
- 4.9.5 accepts `"ignoreDeprecations": "5.0"` (4.8.4 rejects it with TS5023).[^3]

##### Do not use or recommend (TypeScript 5.0 and later)

The project's compiler does not have these. Writing them produces a syntax error, an unknown-option error (TS5023), an invalid-value error (TS6046) or a silent difference. Never suggest them for a 4.x codebase unless the change *is* the upgrade:

- **TC39 standard decorators** (5.0): without `experimentalDecorators`, every 4.x version rejects `@dec` with TS1219; the 2022 decorator signature `(value, context: ClassMethodDecoratorContext)` is not supported, nor are the `ClassMethodDecoratorContext`/`ClassFieldDecoratorContext` types, `Symbol.metadata`/decorator metadata (5.2).
- **`const` type parameters** `function f<const T>(x: T)` (5.0) — a parse error in 4.x.
- **`--moduleResolution bundler`**, `node10` as a name for `node` (both 5.0), `--module preserve` (5.4), `--module node18` (5.8), `node20` (5.9).
- **`--verbatimModuleSyntax`** (5.0; replaces `importsNotUsedAsValues`/`preserveValueImports`), `--allowImportingTsExtensions`, `--customConditions`, `--resolvePackageJsonExports`/`--resolvePackageJsonImports`, `--allowArbitraryExtensions` (all 5.0).
- **`extends` with an array** of base configs in tsconfig (5.0).
- **`export type * from "./m"`** and `export type * as ns` (5.0).
- JSDoc `@satisfies` and `@overload` (5.0) — the `satisfies` **operator** in `.ts` files is 4.9.
- **All enums are union enums**; out-of-range numeric literal assignment to an enum is an error (5.0) — in 4.x `let e: E = 42` compiles for numeric enums.
- **`lib`/`target` `es2023` and later** (5.0+): `Array.prototype.findLast`/`findLastIndex` (5.0, `es2023`), `toSorted`/`toReversed`/`toSpliced`/`with` (5.2), `Object.groupBy`/`Map.groupBy` and `Promise.withResolvers` (5.4), `Set` methods `union`/`intersection`… (5.5), iterator-helper types (5.6), `target: es2024` (5.7).
- **Unrelated getter/setter types** and implicit `undefined` return for functions declared to return `undefined` (5.1).
- **`using` / `await using`** declarations and `Symbol.dispose`/`asyncDispose` (5.2).
- **Import attributes** `import x from "./a.json" with { type: "json" }` (5.3); stable `resolution-mode` on `import type` (5.3); **`switch (true)` narrowing** (5.3).
- **Narrowing preserved in closures after the last assignment** and the **`NoInfer<T>`** utility type (5.4).
- **Inferred type predicates** (`arr.filter(x => x !== undefined)` narrowing the result) and constant indexed-access narrowing `obj[key]` with `const key` of non-literal type (5.5); `--isolatedDeclarations`, regular-expression syntax checking, JSDoc `@import` (5.5).
- `--noUncheckedSideEffectImports`, `--noCheck`, `--strictBuiltinIteratorReturn`, errors on always-truthy/always-nullish conditions such as `if (/regex/)` (5.6); `--rewriteRelativeImportExtensions` (5.7); `--erasableSyntaxOnly`, `--libReplacement` and `require()` of ESM under `nodenext` (5.8); `import defer` (5.9).
- TypeScript 6.0/7.0 defaults and options (`strict` on by default, `--stableTypeOrdering`, `tsgo`).

**Inside 4.x, check the minor**: a project pinned to 4.2 cannot use `override`, `satisfies`, `Awaited`, `import { type X }`, `node16`, `accessor`, or `es2022`. The marker on each feature in this guide is its first version.

##### Version-aware review

- Determine the exact compiler version from the lockfile (`node_modules/typescript/package.json`, `package-lock.json`, `yarn.lock`, `pnpm-lock.yaml`). A range such as `"typescript": "^4.7.0"` resolves to 4.9.5 on a fresh install; a lockfile may still hold 4.7.4.
- Verify an API or option against the pinned version before claiming it is unavailable; say "not available in 4.N" only when that minor's release notes or compiler prove it.
- Do not "modernize" 4.x code to 5.x idioms (`const T`, `satisfies` on 4.8, `verbatimModuleSyntax`, `bundler`) unless the PR is the upgrade.
- Distinguish **compiler** features from **lib** declarations: `Array.prototype.at` needs `lib: ["es2022"]` (4.6+) *and* a runtime that has it (Node.js 16.6+).
- Editors often run a newer bundled TypeScript than the project. "It compiles in VS Code" is not evidence; CI's `tsc` is.

***

#### Compiler Configuration (tsconfig.json)

##### Recommended baseline for 4.9

A Node.js 18/20 service on 4.9 (each option's minimum minor in the comment):

```json
{
  "compilerOptions": {
    "target": "es2022",                      // 4.6+; use "es2021" (4.3+) or "es2020" before that
    "lib": ["es2022"],                       // add "dom" only for browser code
    "module": "node16",                      // 4.7+; "commonjs" for older minors
    "moduleResolution": "node16",            // 4.7+; "node" for older minors (never leave it to default)
    "strict": true,
    "noUncheckedIndexedAccess": true,        // 4.1+
    "noImplicitOverride": true,              // 4.3+
    "noImplicitReturns": true,
    "noFallthroughCasesInSwitch": true,
    "exactOptionalPropertyTypes": true,      // 4.4+, optional but valuable
    "forceConsistentCasingInFileNames": true,
    "esModuleInterop": true,
    "isolatedModules": true,                 // when a single-file transpiler (esbuild, swc, Babel) is involved
    "skipLibCheck": true,
    "resolveJsonModule": true,
    "declaration": true,
    "sourceMap": true,
    "outDir": "dist",
    "rootDir": "src"
  },
  "include": ["src"]
}
```

Notes for reviewers:

- `tsconfig.json` allows comments and trailing commas (JSONC); tools that parse it with `JSON.parse` break.
- `target` controls **syntax** down-levelling only. `lib` controls **which declarations exist**. Neither adds polyfills. Setting `lib: ["es2022"]` for a Node.js 14 runtime type-checks `Object.hasOwn` calls that crash at runtime.
- Leaving `target` unset means **ES3** in every 4.x version: accessors fail with TS1056 and emit uses `exports.__esModule = true` instead of `Object.defineProperty`.
- `forceConsistentCasingInFileNames` defaults to `false` in 4.x (5.0 made it `true`); without it, `import "./User"` vs. `user.ts` works on macOS/Windows and fails on Linux CI.
- `newLine` defaults to the OS line ending in 4.x (5.0 made `LF` the default) — set `"newLine": "lf"` for reproducible output across platforms.
- In 4.5+, an option placed at the tsconfig root instead of under `compilerOptions` is an error when `compilerOptions` is missing; in older minors it is **silently ignored**.

##### The `strict` family by version

Option availability below was verified by passing each flag to every 4.x compiler.[^18]

`strict: true` enables every flag in this table that exists in the pinned version, and new members are added in minor releases — upgrading 4.3 → 4.4 with `strict` adds `useUnknownInCatchVariables` and can break the build.

| Flag | Since | In `strict` | Effect |
|---|---|---|---|
| `noImplicitAny` | 1.x | yes | error on inferred `any` |
| `noImplicitThis` | 2.0 | yes | error on `this` of type `any` |
| `alwaysStrict` | 2.1 | yes | emits `"use strict"`, parses in strict mode |
| `strictNullChecks` | 2.0 | yes | `null`/`undefined` are separate types |
| `strictFunctionTypes` | 2.6 | yes | contravariant parameter checking for function-type properties (not methods) |
| `strictPropertyInitialization` | 2.7 | yes | class fields must be initialized |
| `strictBindCallApply` | 3.2 | yes | typed `bind`/`call`/`apply` |
| `useUnknownInCatchVariables` | **4.4** | yes | `catch (e)` binds `unknown` |
| `noUncheckedIndexedAccess` | **4.1** | no | index access includes `undefined` |
| `noPropertyAccessFromIndexSignature` | **4.2** | no | dotted access only for declared properties |
| `noImplicitOverride` | **4.3** | no | overriding members need `override` |
| `exactOptionalPropertyTypes` | **4.4** | no | `p?: T` rejects explicit `undefined` |
| `noImplicitReturns`, `noFallthroughCasesInSwitch`, `noUnusedLocals`, `noUnusedParameters` | 1.8–2.0 | no | control-flow and dead-code hygiene |

`noUncheckedIndexedAccess`, `noImplicitOverride` and `exactOptionalPropertyTypes` are rejected by older minors with **TS5023 "Unknown compiler option"** — a config copied from a 4.9 project breaks a 4.2 build; `exactOptionalPropertyTypes` additionally requires `strictNullChecks` (TS5052).

```ts
// strict alone now types catch bindings as unknown
export function message(run: () => void): string {
  try {
    run();
    return "ok";
  } catch (e) {
    return e.message; // error 4.4+
  }
}
```

##### Target, lib and runtime

| Option value | Since | Notes |
|---|---|---|
| `target: es2020` | 3.8 | optional chaining, `??`, `BigInt` native |
| `target`/`lib: es2021` | **4.3** | `replaceAll`, `Promise.any`, `WeakRef`; `&&=`/`||=`/`??=` emitted natively |
| `target`/`lib: es2022` | **4.6** | `.at()`, `Object.hasOwn`, `Error` `cause`, native class fields and `#private`, `static {}` |
| `target: esnext` | — | moves with the compiler; avoid for reproducible output |
| `lib/target: es2023` | 5.0 | **not available in 4.x** (TS6046) |

`useDefineForClassFields` defaults to `true` when `target` is `es2022` or `esnext` (see [Class fields](#class-field-semantics-usedefineforclassfields)) — changing `target` from `es2021` to `es2022` changes runtime semantics of every class with fields.

##### `@types` packages must be pinned

New `@types/*` versions are written for current TypeScript and frequently contain 5.x-only syntax (`const` type parameters, newer lib types). With 4.x they fail to parse inside `node_modules` — and **`skipLibCheck` does not help with syntax errors**. Installing the latest `@types/node` (26.x in October 2026) on 4.9.5 produces dozens of errors such as `node_modules/@types/node/ffi.d.ts(94,21): error TS1139: Type parameter declaration expected.`

DefinitelyTyped publishes a dist-tag per supported compiler version. `npm install -D @types/node@ts4.9` installs the newest release tested with 4.9 (22.9.3); the tags `ts4.0` … `ts4.8` exist likewise (`ts4.2` → 18.15.3, `ts4.7` → 20.14.8, `ts4.8` → 22.9.0).[^5] Prefer a version that matches the **runtime** Node.js major and was published for the compiler, e.g. for 4.9: `@types/node@18.19.65`, `20.17.7` or `22.9.3`; for 4.7: `20.14.8` (these were the last releases declaring support for that TypeScript version; `@types/node` 18.19.65 does not compile on 4.7.4). Review every `@types/*` or library upgrade in a 4.x project by running `tsc` in CI — Renovate/Dependabot bumps of `@types/*` are a common cause of sudden 4.x build breaks.

***

#### Modules and Module Resolution

##### Resolution modes in 4.x

See the TypeScript modules reference for the full algorithm.[^19]

| `moduleResolution` | Since | Use for | Behaviour |
|---|---|---|---|
| `classic` | 1.x | nothing (legacy) | never looks in `node_modules`; **the default whenever `module` is neither `commonjs` nor `node16`/`nodenext`** (`es2015`, `es2020`, `es2022`, `esnext`, `amd`, `system`, `umd`) |
| `node` | 1.6 | CommonJS output, bundlers | Node.js CommonJS lookup (`index`, `main`, `types`, `typesVersions`); **ignores `package.json` `exports` and `imports`**; extensionless relative imports allowed. Renamed `node10` in 5.0 (both names accepted from 5.0 on; 4.x rejects `node10` with TS6046) |
| `node16` / `nodenext` | **4.7** | code that runs in Node.js 12.20+ without a bundler | per-file CJS/ESM format from `.mts`/`.cts` or the nearest `package.json` `"type"`; honours `exports`/`imports` with `import`/`require`/`types`/`node` conditions; ESM files need explicit extensions |

There is **no `bundler` resolution in 4.x** (5.0). A browser app built with webpack/Vite/esbuild on 4.x uses `module: esnext` (or `es2020`) **plus `moduleResolution: node`** — and therefore cannot see `exports`-only subpaths of packages; such imports need `paths` mappings or `typesVersions` in the package.

In 4.x, `module: node16`/`nodenext` must be paired with the same `moduleResolution` (it is implied when omitted). `nodenext` behaves like `node16` in all 4.x versions; in later compilers it moves (`node18` in 5.8, `node20` in 5.9), so 4.x projects should spell `node16` for stable output.

##### `node16`/`nodenext` (4.7+): what a reviewer must check

- **Format comes from the file, not from `module`.** `.mts` → ESM, `.cts` → CJS, `.ts` → the nearest `package.json` `"type"` (`"module"` → ESM, otherwise CJS). The same source line emits `import` in one file and `require()` in another.
- **Relative imports in ESM files need the runtime extension** — `.js` for a `.ts` source, `.mjs` for `.mts`:

```typescript
// package.json: { "type": "module" }, module/moduleResolution: node16
import { twice } from "./util";        // TS2835: Relative import paths need explicit file extensions ... Did you mean './util.js'?
import { twice } from "./util.js";     // OK: resolves to util.ts at compile time, util.js at runtime
import { twice } from "./util.ts";     // TS2691 in 4.x (5.0 reports TS5097 and offers allowImportingTsExtensions)
```

- **CommonJS files cannot statically import ESM-only packages.** 4.8 and 4.9 report TS1479 ("The current file is a CommonJS module whose imports will produce 'require' calls; however, the referenced file is an ECMAScript module…"); 4.7 reports TS1471 ("…The specifier only resolves to an ES module, which cannot be imported synchronously") for the same code. The fix is a dynamic `import()`, which `node16` preserves in CJS output, or converting the file to ESM (`.mts` / `"type": "module"`).
- **`import()` under `module: commonjs` is rewritten to `require()`.** With `module: commonjs`, `await import("esm-only")` emits `Promise.resolve().then(() => require("esm-only"))`. That compiles cleanly under `moduleResolution: node` (which ignores `exports`) and fails at runtime: `ERR_PACKAGE_PATH_NOT_EXPORTED` when the package only exports an `import` condition, `ERR_REQUIRE_ESM` on Node.js 18 when it exports `default` (Node.js 20.19+/22.12+ can `require()` synchronous ESM). Verified with Node.js 18.20.8, 20.20.2 and 22.23.4. A CommonJS service that must load ESM-only dependencies (`node-fetch` 3, `chalk` 5, `nanoid` 4, …) needs `module: node16` on 4.7+.
- **`exports` wins over `typesVersions`** (4.9 fix): a library publishing `typesVersions` for old compilers must also map `"types@<4.8"`-style conditions in `exports` for `node16` consumers.
- `resolution-mode` works on `/// <reference types="pkg" resolution-mode="require" />` (4.7+); on `import type … assert { "resolution-mode": … }` stable 4.7–5.2 report "Resolution mode assertions are unstable" — do not use it on 4.x.
- `moduleDetection: "force"` (4.7+) treats every non-declaration file as a module — recommended so a file without imports does not leak globals.

##### CommonJS interop: `esModuleInterop`

For a CommonJS package declared with `export =` (e.g. `module.exports = function greet() {}`), verified on 4.9.5 with Node.js 20:

| Import form | without `esModuleInterop` | with `esModuleInterop` |
|---|---|---|
| `import greet from "greet"` | TS1259; runtime `TypeError: (0 , greet_1.default) is not a function` | OK, works |
| `import * as greet from "greet"; greet()` | TS2497, but the emitted code happens to work | TS2497 and runtime `TypeError: greet is not a function` (`__importStar` wraps the function) |
| `import greet = require("greet")` | OK, works | OK, works |

Enable `esModuleInterop` (it implies `allowSyntheticDefaultImports`) and use default imports for `export =` modules. Flag `import * as x from "cjs"` followed by calling or constructing `x` — it breaks as soon as interop is on. `allowSyntheticDefaultImports` alone only changes type checking, not emit: with `module: commonjs`, `import greet from "greet"` then compiles cleanly and still crashes with the same `TypeError` (verified).

##### Type-only imports and import elision

`tsc` (4.x default `importsNotUsedAsValues: "remove"`) drops an import whose bindings are only used as types — **including its side effects**:

```typescript
// side.ts
import { User } from "./types";          // types.ts also registers a global handler at load time
export const u: User = { id: "1" };
// emitted JS: no require("./types") at all -> the registration never runs
```

Rules:

- Import a module for its side effects with a bare `import "./register";`.
- Mark type-only imports explicitly: `import type { User } from "./types";` (3.8+) or `import { createUser, type User } from "./types";` (**4.5+**; a parse error, TS1005, before 4.5). The same goes for re-exports: `export type { User } from "./types";` / `export { type User } from "./types";` (4.5+).
- Under `isolatedModules` (needed for esbuild, swc, Babel, `ts-jest` `isolatedModules`, Vite), re-exporting a type without `type` is TS1205; a file without any `import`/`export` is TS1208 in 4.x (5.0 dropped TS1208); accessing an ambient (`declare const enum` in a `.d.ts`) const enum is TS2748. Note: on 4.0–4.4 a single parse error (such as `export { type X }`) makes `tsc` report only syntax errors for the whole program, so a version mismatch can hide the isolatedModules errors entirely.
- `importsNotUsedAsValues` (`remove` | `preserve` | `error`, 3.8+) and `preserveValueImports` (**4.5+**, requires `module` ≥ `es2015`; together with `isolatedModules` it reports TS1444 for types imported without `type`) control elision in 4.x. Both are **deprecated in 5.0** (TS5101 "will stop functioning in TypeScript 5.5") stop functioning in 5.5 (still accepted with `"ignoreDeprecations": "5.0"` but ignored) and are hard errors (TS5102) from 5.6; their replacement is `verbatimModuleSyntax` (5.0). On 4.x, the portable way to enforce type-only imports is `isolatedModules` plus the `@typescript-eslint/consistent-type-imports` lint rule; avoid introducing `importsNotUsedAsValues: "error"` in a project that is about to move to 5.x.

##### `paths`, `baseUrl` and `rootDirs`

`paths` only affects type checking and editor resolution. `tsc` emits the specifier unchanged (`import { db } from "@app/db"` stays `require("@app/db")`), so the runtime needs a matching mechanism (bundler alias, `tsconfig-paths`, Node.js `imports` field with `#app/*` and `node16`). Since **4.1**, `paths` works without `baseUrl`; prefer that — `baseUrl` also makes every bare specifier resolvable relative to it (with `baseUrl: "src"`, `import "utils"` silently resolves to `src/utils.ts` even when an npm package `utils` is installed — verified on 4.9.5). (`baseUrl` is deprecated in 6.0.)

##### Top-level `await`

Top-level `await` needs `module` set to `es2022` (**4.5+**), `esnext`, `system` or `node16`/`nodenext` (4.7+, ESM files only) **and** `target` ≥ `es2017`; otherwise TS1378. It is never allowed in a CommonJS output file.

##### Declaration files and libraries

- Ship `.d.ts` built with `declaration: true` from the same compiler version the library supports as a minimum: `.d.ts` emitted by 4.9 can contain syntax (e.g. `accessor`, variance annotations, `infer X extends`) that older consumers cannot parse; declarations from 5.x (`const T`, `export type *`) break all 4.x consumers. Use `typesVersions` (with `exports` `types@` conditions for `node16` consumers) to ship down-levelled types; `downlevel-dts` automates that.
- `skipLibCheck: true` skips *type checking* of `.d.ts` files but **not parsing**: a dependency's declaration that uses newer syntax still breaks a 4.x build (verified with `@types/node` 26.6.5 on 4.9.5).
- `/// <reference types="node" />` in a library's public `.d.ts` forces `@types/node` on every consumer; declare narrower interfaces instead.

***

#### Naming and Code Style

- `PascalCase` for types, interfaces, classes, enums and type parameters with meaning (`TItem`, `TKey`, or a single `T` for short generics); `camelCase` for values, functions and members; `UPPER_SNAKE_CASE` only for true compile-time constants.
- No `I` prefix on interfaces, no Hungarian suffixes (`UserInterface`, `UserType`).
- One exported concept per module; prefer named exports over `export default` (renames, re-exports and auto-imports stay consistent).
- File names follow the project convention (`kebab-case.ts` or `camelCase.ts`); with `node16` ESM, import specifiers must match the emitted file name exactly, including case.
- Format with Prettier (2.x supports TypeScript 4.x syntax up to `satisfies` from 2.8; Prettier 3 also parses 4.x code); do not hand-align.

***

#### Types and the Type System

##### `any`, `unknown`, `never`

- `any` disables checking in both directions and spreads silently (`JSON.parse`, `Response.json()` — `Promise<any>` in the 4.x `lib.dom` —, untyped `require`). Treat every `any` that reaches business logic as a bug; prefer `unknown` at boundaries and narrow.
- `unknown` accepts everything and allows nothing until narrowed.
- `never` is the empty type: exhaustiveness checks, impossible branches, functions that always throw.

```ts
export function parsePort(raw: unknown): number {
  if (typeof raw === "string" && /^\d+$/.test(raw)) {
    const port = Number(raw);
    if (port > 0 && port < 65536) return port;
  }
  if (typeof raw === "number" && Number.isInteger(raw)) return raw;
  throw new Error(`invalid port: ${String(raw)}`);
}

export function bad(raw: unknown): number {
  return raw.length; // error
}
```

`JSON.parse` returns `any` in every 4.x `lib`. Annotate the result as `unknown` and validate (hand-written guards, `zod` 3.22 — documented for TypeScript 4.5+ —, `io-ts`, `ajv`):

```ts
interface Config { port: number; host: string }

function isConfig(v: unknown): v is Config {
  return typeof v === "object" && v !== null &&
    typeof (v as Record<string, unknown>).port === "number" &&
    typeof (v as Record<string, unknown>).host === "string";
}

export function loadConfig(text: string): Config {
  const data: unknown = JSON.parse(text);
  if (!isConfig(data)) throw new Error("invalid config");
  return data;
}

export function loadConfigBad(text: string): Config {
  return JSON.parse(text) as Config; // compiles, validates nothing
}
```

##### `unknown`, `{}` and `object` (4.8 changes)

Under `strictNullChecks`, 4.8 treats `unknown` like `{} | null | undefined`: truthiness narrows `unknown` to `{}`, and `{} | null | undefined` is assignable from `unknown`.

```ts
export function f(x: unknown, y: {} | null | undefined): void {
  y = x; // error <4.8
  if (x) {
    const nonNullish: {} = x; // error <4.8
  }
}
```

**Breaking in 4.8**: an unconstrained type parameter is no longer assignable to `{}` (it can be `null`/`undefined`). Code that compiled on 4.7 fails on 4.8:

```ts
function takesNonNullish(value: {}): string[] {
  return Object.keys(value); // throws at runtime for null/undefined
}

export function wrap<T>(x: T): string[] {
  return takesNonNullish(x); // error 4.8+
}

export function wrapFixed<T extends {}>(x: T): string[] {
  return takesNonNullish(x);
}
```

From 4.8, narrowing a generic against `null`/`undefined` produces `T & {}`, which is exactly `NonNullable<T>` (now defined as `T & {}`):

```ts
// @since: 4.8
export function throwIfNullable<T>(value: T): NonNullable<T> {
  if (value === undefined || value === null) {
    throw new Error("nullable value");
  }
  return value;
}
```

Reviewer notes: `{}` means "any non-nullish value", including `0`, `""` and functions — it is not "an empty object". Use `Record<string, never>` for an empty object, `object` for "any non-primitive", `Record<string, unknown>` for a dictionary. The `@typescript-eslint/ban-types` rule (v5–v7; split into `no-empty-object-type`/`no-unsafe-function-type`/`no-wrapper-object-types` in v8) flags `{}`, `Object` and `Function`.

##### Literal types, widening and `as const`

```ts
const method = "GET";          // type "GET"
let mutable = "GET";           // type string (widened)
const routes = ["/a", "/b"] as const; // readonly ["/a", "/b"]
type Route = typeof routes[number];   // "/a" | "/b"

declare function request(m: "GET" | "POST", url: Route): void;

const opts = { method: "GET", url: "/a" };
request(opts.method, opts.url); // error

const fixed = { method: "GET", url: "/a" } as const;
request(fixed.method, fixed.url);
export { method, mutable };
```

A template literal *expression* is typed `string`, not a template literal type, unless `as const` is added (4.2 reverted the 4.2-beta change that inferred template literal types for such expressions).

##### `satisfies` (4.9+)

`satisfies` checks that an expression conforms to a type **without** changing the expression's inferred type. Before 4.9 the alternatives were an annotation (loses the precise type) or a generic identity helper.

```ts
// @since: 4.9
type Colors = "red" | "green" | "blue";
type RGB = [red: number, green: number, blue: number];

const palette = {
  red: [255, 0, 0],
  green: "#00ff00",
  blue: [0, 0, 255],
} satisfies Record<Colors, string | RGB>;

export const green = palette.green.toUpperCase(); // still string, not string | RGB

export const typo = {
  red: "#f00",
  green: "#0f0",
  bleu: "#00f", // error
} satisfies Record<Colors, string>;
```

For a 4.0–4.8 project, the equivalent is an identity function with a constrained type parameter — recommend this instead of `satisfies`:

```ts
const defineRoutes = <T extends Record<string, { path: string; auth: boolean }>>(routes: T): T => routes;

export const routes = defineRoutes({
  home: { path: "/", auth: false },
  admin: { path: "/admin", auth: true },
});
export const adminPath: string = routes.admin.path;
```

Pitfalls: `x satisfies T` does not narrow or convert; it does not make a mutable literal `readonly` (combine `as const satisfies T`); and `satisfies` in `.js`/JSDoc (`@satisfies`) is 5.0.

##### `interface` vs `type`

Use `interface` for object shapes that may be extended or merged (public API, declaration merging, `implements`), and `type` for unions, tuples, mapped and conditional types, and function types. Interfaces are cached by name and give better error messages and faster checks for large object hierarchies; intersections (`A & B`) of object types silently produce `never` members on conflicts where `interface B extends A` reports an error.

```ts
interface Base { id: string }
interface Conflict extends Base { id: number } // error

type Merged = Base & { id: number };
declare const m: Merged;
export const id: never = m.id; // the conflict becomes never instead of an error
```

##### Excess property checks

Excess property checks apply only to **fresh object literals** assigned directly to a typed target. A variable in between, a spread, or a type assertion disables them.

```ts
interface Options { retries: number; timeoutMs?: number }

export const a: Options = { retries: 3, timeout: 100 }; // error

const tmp = { retries: 3, timeout: 100 };
export const b: Options = tmp;                      // no error: not fresh
export const c = { retries: 3, timeout: 100 } as Options; // no error: assertion
```

##### Optional properties and `exactOptionalPropertyTypes` (4.4+)

By default `prop?: T` means `T | undefined` and accepts an explicit `undefined`. `exactOptionalPropertyTypes` distinguishes "missing" from "present but `undefined`", which matters for `in`, `Object.keys`, spreading over defaults and serialisation.

```ts
// @since: 4.4
// @exactOptionalPropertyTypes: true
interface Settings { theme?: "dark" | "light" }

export const s1: Settings = {};
export const s2: Settings = { theme: undefined }; // error

const defaults = { theme: "light" as const };
export const merged = { ...defaults, ...s1 };
```

Enable it for new code; in existing code, it surfaces places where `{ ...defaults, ...overrides }` would overwrite a default with `undefined`. Libraries' `.d.ts` files written without it may need `| undefined` added explicitly.

##### Index signatures, `Record` and dynamic keys

```ts
// @since: 4.1
// @noUncheckedIndexedAccess: true
const counts: Record<string, number> = {};
export function inc(key: string): number {
  counts[key] = (counts[key] ?? 0) + 1;
  const n: number = counts[key]; // error
  return n;
}

export function first(xs: string[]): string {
  return xs[0]; // error
}
```

Without `noUncheckedIndexedAccess` (**4.1+**; TS5023 on 4.0), `xs[0]` on an empty array and `dict[missingKey]` are typed as present. Enable it in new 4.x code; in a large existing codebase, at least require explicit `?? fallback` or checks at array/dictionary reads in review.

`noPropertyAccessFromIndexSignature` (**4.2+**) forces bracket access for keys that come only from an index signature, so typos in dotted access become visible:

```ts
// @since: 4.2
// @noPropertyAccessFromIndexSignature: true
interface Env { NODE_ENV: string; [key: string]: string | undefined }
declare const env: Env;
export const mode = env.NODE_ENV;
export const port = env.PORT; // error
export const port2 = env["PORT"];
```

Index signatures with `symbol` keys, template-string patterns and unions are **4.4+**:

```ts
// @since: 4.4
interface DataAttrs {
  [attr: `data-${string}`]: string;
}
export const attrs: DataAttrs = { "data-id": "42" };
export const wrong: DataAttrs = { "data-id": 42 }; // error

interface SymbolTable {
  [key: symbol]: number;
}
export const table: SymbolTable = { [Symbol("a")]: 1 };
```

`Object.keys(obj)` returns `string[]`, not `(keyof T)[]`, because objects can have extra keys at runtime. Do not "fix" this with a cast in library code; iterate over a known key list or use `Object.entries` with validation.

##### Tuples: variadic, labeled, rest anywhere

```ts
// @since: 4.0
type Concat<A extends readonly unknown[], B extends readonly unknown[]> = [...A, ...B];
export type T1 = Concat<[1, 2], [3]>; // [1, 2, 3]

export function tail<T extends unknown[]>(xs: readonly [unknown, ...T]): T {
  const [, ...rest] = xs;
  return rest;
}
export const t: [number, string] = tail([true, 1, "x"]);

type Range = [start: number, end: number]; // labeled elements (documentation only)
export function slice(...[start, end]: Range): number {
  return end - start;
}
```

Leading and middle rest elements are **4.2+**:

```ts
// @since: 4.2
type Path = [...segments: string[], last: number];
export const p: Path = ["a", "b", 3];
type Middle = [boolean, ...string[], number];
export const m: Middle = [true, "x", "y", 1];
```

`readonly` tuples have a `readonly length` since **4.7**: assigning `tuple.length = n` is an error.

##### Template literal types and key remapping (4.1+)

```ts
// @since: 4.1
type HttpMethod = "get" | "post";
type Handler = `on${Capitalize<HttpMethod>}`; // "onGet" | "onPost"
export const h: Handler = "onPost";
export const bad: Handler = "onput"; // error

type Getters<T> = { [K in keyof T as `get${Capitalize<string & K>}`]: () => T[K] };
interface Person { name: string; age: number }
export type PersonGetters = Getters<Person>; // { getName: () => string; getAge: () => number }

type OmitByType<T, U> = { [K in keyof T as T[K] extends U ? never : K]: T[K] };
export const p: OmitByType<Person, number> = { name: "x" };
```

`string & K` (or `Extract<K, string>`) is required because `keyof T` may contain `number | symbol`. Inference of number/bigint/boolean literals from template types with `infer X extends number` is **4.8+**:

```ts
// @since: 4.8
type ParseInt<S extends string> = S extends `${infer N extends number}` ? N : never;
export const n: ParseInt<"100"> = 100;
export const wrong: ParseInt<"100"> = 101; // error
```

On 4.7, the same type is `number` (both lines compile); before 4.7, `infer N extends number` is a syntax error.

Since **4.7**, interpolating a value whose type may be `symbol` (including a generic constrained by `string | symbol` or `keyof T`) into a template string is an error (TS2731) because it throws at runtime:

```ts
// @since: 4.7
export function describe<T, K extends keyof T>(obj: T, key: K): string {
  return `reading ${key}`; // error
}
export function describeFixed<T, K extends keyof T>(obj: T, key: K): string {
  return `reading ${String(key)}`;
}
```

##### Conditional types, `infer` and recursion

```ts
type ElementOf<T> = T extends readonly (infer E)[] ? E : never;
export type E1 = ElementOf<string[]>; // string

type ToArray<T> = T extends unknown ? T[] : never;   // distributive
export type D = ToArray<string | number>;           // string[] | number[]
type ToArrayNonDist<T> = [T] extends [unknown] ? T[] : never;
export type ND = ToArrayNonDist<string | number>;  // (string | number)[]
```

- Recursive conditional types are **4.1+**. 4.0 rejects a conditional type alias that references itself (`type Flatten<T> = T extends (infer U)[] ? Flatten<U> : T` → TS2456 "Type alias 'Flatten' circularly references itself").
- Tail-recursive conditional types (**4.5+**) can recurse 1000 levels; other recursion is limited by the instantiation depth — 50 in 4.0–4.4, 100 in 4.5–4.9 (TS2589 "Type instantiation is excessively deep and possibly infinite").

```ts
// @since: 4.5
type TrimLeft<S extends string> = S extends ` ${infer R}` ? TrimLeft<R> : S;
type Spaces = "                                                                                    x";
export const trimmed: TrimLeft<Spaces> = "x";
```

On 4.1–4.4 the same 84-space input exceeds the depth limit and the assignment fails.

- `infer` with an `extends` constraint is **4.7+**:

```ts
// @since: 4.7
type FirstString<T> = T extends [infer S extends string, ...unknown[]] ? S : never;
export const fs1: FirstString<["a", 1]> = "a";
export const fs2: FirstString<[1, "a"]> = "a"; // error
```

##### `Awaited<T>` (4.5+)

`Awaited<T>` recursively unwraps promise-likes the way `await` does. Use it instead of hand-written `UnwrapPromise<T>` helpers.

```ts
// @since: 4.5
export type A = Awaited<Promise<Promise<number>>>; // number

export async function load<T>(p: Promise<T>): Promise<Awaited<T>> {
  return await p;
}
```

In 4.5+, `Promise.all` and `await` on generic values use `Awaited`, and in 4.9 `Promise.resolve` does too — which can change inferred types of existing code during an upgrade.

##### Generics: constraints, defaults, inference

```ts
export function pluck<T, K extends keyof T>(items: readonly T[], key: K): T[K][] {
  return items.map((i) => i[key]);
}
export const names: string[] = pluck([{ name: "a", age: 1 }], "name");

interface Page<T = unknown> { items: T[]; next?: string }
export const page: Page = { items: [1, "x"] };
```

Review points:

- A type parameter used only once (`function f<T>(x: T): void`) adds nothing; use the constraint type directly.
- `function parse<T>(s: string): T { return JSON.parse(s); }` is a disguised `any` cast — the caller picks `T` and nothing checks it.
- **Instantiation expressions (4.7+)** specialise a generic function without calling it:

```ts
// @since: 4.7
function makeBox<T>(value: T): { value: T } {
  return { value };
}
const makeStringBox = makeBox<string>;
export const box = makeStringBox("x");
export const badBox = makeStringBox(1); // error
```

- **Variance annotations (4.7+)** `in`/`out` document and check variance and can speed up checking of deeply generic types; they are rarely needed in application code and must match the actual usage (TS2636 otherwise). Because method parameters are bivariant, a wrong annotation on a *method*-style member is not detected:

```ts
// @since: 4.7
interface Producer<out T> { get: () => T }
interface Consumer<in T> { accept: (value: T) => void }
interface Wrong<out T> { accept: (value: T) => void } // error
interface NotChecked<out T> { accept(value: T): void } // method parameters are bivariant: no error
export type P = Producer<string>;
export type C = Consumer<string>;
```

- **There are no `const` type parameters in 4.x** (5.0). To infer literal types for a generic argument on 4.x, ask callers to write `as const` or constrain the parameter (`T extends readonly string[]` plus `as const` at the call site).
- **There is no `NoInfer<T>`** (5.4). The 4.x workaround is `T & {}` in the position that should not drive inference, or a second type parameter `U extends T`.

##### Abstract construct signatures (4.2+)

```ts
// @since: 4.2
abstract class Shape { abstract area(): number }
type AbstractCtor<T> = abstract new (...args: any[]) => T;

function withLogging<TBase extends AbstractCtor<Shape>>(Base: TBase) {
  abstract class Logged extends Base {
    log(): string { return `area=${this.area()}`; }
  }
  return Logged;
}
export class Square extends withLogging(Shape) {
  constructor(private side: number) { super(); }
  area(): number { return this.side ** 2; }
}
```

##### Branded (nominal) types

TypeScript is structural; to keep `UserId` and `OrderId` apart, use a brand:

```ts
type Brand<T, B extends string> = T & { readonly __brand: B };
type UserId = Brand<string, "UserId">;
type OrderId = Brand<string, "OrderId">;

const asUserId = (s: string): UserId => s as UserId; // validate here
declare function loadUser(id: UserId): void;

declare const orderId: OrderId;
loadUser(asUserId("u-1"));
loadUser(orderId); // error
loadUser("u-1"); // error
```

***

#### Narrowing and Control-Flow Analysis

##### Built-in narrowing

`typeof`, `instanceof`, `in`, equality with literals, truthiness, `Array.isArray`, user-defined type predicates and assertion functions all narrow. Pitfalls:

- `typeof x === "object"` includes `null`.
- Truthiness narrowing drops `0`, `""`, `NaN` and `0n` — use `!= null` / `!== undefined` for "present".
- `Array.isArray` narrows `unknown` to `any[]` — the elements are still `any`.

##### `in` narrowing (stricter in 4.2, unlisted properties in 4.9)

`in` with a primitive on the right side is an error (TS2361) in every 4.x; 4.2 and 4.9 tightened the check further. Since **4.9**, `"key" in obj` narrows types that do not declare `key` to `T & Record<"key", unknown>`, so validation of `unknown` data no longer needs casts:

```ts
// @since: 4.9
export function packageName(pkg: unknown): string | undefined {
  if (pkg && typeof pkg === "object" && "name" in pkg && typeof pkg.name === "string") {
    return pkg.name;
  }
  return undefined;
}
```

On 4.0–4.8 the property access fails with TS2339; use a cast to `Record<string, unknown>` after the `typeof … === "object"` check instead (as in `isConfig` above).

##### Aliased conditions and discriminants (4.4+)

```ts
// @since: 4.4
export function len(x: string | number[]): number {
  const isString = typeof x === "string";
  if (isString) {
    return x.toUpperCase().length;
  }
  return x.length;
}
```

The alias must be a `const`, and the narrowed reference must be a `const`, a `readonly` property or a parameter that is never assigned in the function body; otherwise the alias does not narrow and the use site fails to compile as on 4.3.

##### Discriminated unions and exhaustiveness

```ts
type Shape =
  | { kind: "circle"; radius: number }
  | { kind: "square"; size: number };

function assertNever(x: never): never {
  throw new Error(`unexpected: ${JSON.stringify(x)}`);
}

export function area(s: Shape): number {
  switch (s.kind) {
    case "circle":
      return Math.PI * s.radius ** 2;
    case "square":
      return s.size ** 2;
    default:
      return assertNever(s);
  }
}
```

Without the `default: assertNever(...)`, adding a variant compiles silently in functions that return `void` or have a `default` returning a fallback; the `never` check catches it everywhere. Template literal types work as discriminants since 4.5.

Destructuring the discriminant and payload keeps the correlation since **4.6** (destructure with `const`, not with default values):

```ts
// @since: 4.6
type Action =
  | { kind: "num"; payload: number }
  | { kind: "str"; payload: string };

export function handle(action: Action): string {
  const { kind, payload } = action;
  if (kind === "num") {
    return payload.toFixed(2);
  }
  return payload.toUpperCase();
}
```

Dependent parameters (rest parameter typed as a union of tuples) also narrow since **4.6**:

```ts
// @since: 4.6
type Args = ["str", string] | ["num", number];
export function f(...[kind, value]: Args): string {
  if (kind === "str") return value.toUpperCase();
  return value.toFixed(1);
}
```

##### Bracketed element access (4.7+)

```ts
// @since: 4.7
const key = Symbol();
const numberOrString = Math.random() < 0.5 ? 42 : "hello";
const obj = { [key]: numberOrString };

if (typeof obj[key] === "string") {
  const s: string = obj[key];
}
```

##### Narrowing is lost in callbacks (all 4.x)

Narrowing of `let` variables and parameters is **not** preserved inside closures in any 4.x version (5.4 keeps it after the last assignment). Copy to a `const` first:

```ts
export function greet(name: string | undefined): () => string {
  if (!name) name = "anonymous";
  return () => name.toUpperCase(); // error
}

export function greetFixed(name: string | undefined): () => string {
  const n = name ?? "anonymous";
  return () => n.toUpperCase();
}
```

The opposite hole exists too: TypeScript does **not** invalidate the narrowing of `obj.prop` when a function is called in between. If that function (or a callback) reassigns `obj.prop`, the narrowed type is wrong — watch for it in review.

##### Type predicates and assertion functions

```ts
export function isDefined<T>(x: T | undefined | null): x is T {
  return x !== undefined && x !== null;
}

const maybe: (string | undefined)[] = ["a", undefined];
export const strings: string[] = maybe.filter(isDefined);
export const stillMaybe: string[] = maybe.filter((x) => x !== undefined); // error

export function assertIsString(v: unknown, msg = "expected string"): asserts v is string {
  if (typeof v !== "string") throw new TypeError(msg);
}
```

In 4.x an inline arrow (`x => x !== undefined`) is **not** inferred as a type predicate (5.5); pass a named guard or annotate the arrow `(x): x is string => …`. A predicate is a promise to the compiler: review its body — a wrong predicate is worse than a cast because it looks verified. Assertion functions must be called through an explicitly typed reference (TS2775 otherwise).

##### Comparison checks added in 4.x

```ts
declare const items: string[];
declare const n: number;
declare const config: { load(): Promise<boolean> };

export const empty = items === []; // error 4.8+
export const isNan = n === NaN; // error 4.9+
export async function check(): Promise<void> {
  if (config.load()) { // error 4.4+
    console.log("loaded?");
  }
}
```

- `x === []` / `x === {}` (TS2839, **4.8+**): always `false` — use `x.length === 0`.
- `x === NaN` (TS2845, **4.9+**): always `false` — use `Number.isNaN(x)`.
- `if (promise)` / `if (fn())` returning a promise (TS2801, 4.3+ for named promises, 4.4+ for call expressions): a forgotten `await`.
- Uncalled function checks in conditions (`if (user.isAdmin)` where `isAdmin` is a method; extended to `&&`/`||` in 4.2).

On older minors these lines compile — review them manually.

***

#### Expressions and Operators

##### Optional chaining, nullish coalescing, logical assignment

Optional chaining (`?.`) and nullish coalescing (`??`) are 3.7 features and available in every 4.x. Logical assignment operators are **4.0+** and down-levelled for targets below ES2021:

```ts
// @since: 4.0
interface Opts { retries?: number; cache?: Map<string, string>; name?: string | null }

export function normalise(o: Opts): Opts {
  o.retries ??= 3;              // only if null/undefined
  o.cache ||= new Map();        // if falsy
  o.name &&= o.name.trim();     // only if truthy
  return o;
}
```

Review points:

- `a || b` replaces `0`, `""`, `false` and `NaN`; use `??` for defaults of numbers, strings and booleans.
- `obj?.a.b` short-circuits the whole chain; parentheses end the chain, so `(obj?.a).b` throws when `obj` is nullish.
- `x ??= y` assigns only when `x` is `null`/`undefined`, and does **not** evaluate `y` otherwise — side effects in `y` are skipped.

##### Class property inference from constructors (4.0+)

Under `noImplicitAny`, unannotated properties get their types from constructor assignments:

```ts
// @since: 4.0
// @strictPropertyInitialization: false
export class Square {
  sideLength;
  constructor(sideLength: number) {
    this.sideLength = sideLength;
  }
  get area(): number {
    return this.sideLength ** 2;
  }
}
```

Explicit annotations remain preferable for public classes.

##### Import assertions (4.5+)

`import data from "./data.json" assert { type: "json" }` parses since 4.5 and is accepted only with `module: esnext` (or in ESM files under `nodenext`); other `module` settings report TS2821/TS2836. The proposal later moved to the `with` keyword (import attributes; TypeScript 5.3), Node.js 22 removed support for `assert` (a `SyntaxError`; Node.js 20.20 still accepts it with a deprecation warning), and 6.0 deprecates the `assert` form — do not introduce new `assert` imports; for JSON in 4.x CommonJS code use `resolveJsonModule` with a plain import.

***

#### Built-in Utility Types and Type Operators

| Utility | Since | Note |
|---|---|---|
| `Partial`, `Required`, `Readonly`, `Pick`, `Record` | 2.1–2.8 | homomorphic mapped types keep optionality and `readonly` |
| `Exclude`, `Extract`, `NonNullable`, `ReturnType`, `InstanceType` | 2.8 | `NonNullable<T>` is `T & {}` from 4.8 |
| `Parameters`, `ConstructorParameters` | 3.1 | `ConstructorParameters` and `InstanceType` accept abstract classes from 4.3 (TS2344 on 4.2) |
| `Omit` | 3.5 | not distributive over unions — `Omit<A \| B, "k">` collapses to common keys |
| `ThisParameterType`, `OmitThisParameter`, `ThisType` | 3.3 / 2.3 | |
| `Uppercase`, `Lowercase`, `Capitalize`, `Uncapitalize` | **4.1** | intrinsic string manipulation types |
| `Awaited` | **4.5** | recursive promise unwrapping |
| `NoInfer` | 5.4 | **not available** in 4.x |

Distributive `Omit` for unions:

```ts
type DistributiveOmit<T, K extends PropertyKey> = T extends unknown ? Omit<T, K> : never;

type Event =
  | { kind: "click"; x: number; ts: number }
  | { kind: "key"; key: string; ts: number };

export const e1: DistributiveOmit<Event, "ts"> = { kind: "key", key: "a" };
export const e2: Omit<Event, "ts"> = { kind: "key", key: "a" }; // error
```

Mapped-type modifiers: `-readonly` and `-?` remove modifiers (`type Mutable<T> = { -readonly [K in keyof T]: T[K] }`), `+readonly`/`+?` add them. `keyof`, `typeof` (on values), indexed access `T["k"]`, `T[number]` for array elements and `unique symbol` are all available in 4.x.

***

#### Declaration Merging, Augmentation and Globals

- Interfaces with the same name merge; use this to augment third-party types (`Express.Request`, `NodeJS.ProcessEnv`, `Window`) — inside a module file, wrap global augmentations in `declare global { … }`.
- Module augmentation (`declare module "express-serve-static-core" { interface Request { user?: User } }`) only works if the file is a module and the augmented module is resolvable; otherwise it *declares a new ambient module* that shadows the real one (verified on 4.9.5: in a file without `import`/`export`, `declare module "pkg" { … }` makes `import { p } from "pkg"` fail with TS2305 elsewhere). Add `export {};` to augmentation files.
- Typing `process.env` via `NodeJS.ProcessEnv` augmentation does not make variables exist; validate at startup.
- `namespace` (and the older `module Foo {}` spelling) is legacy for application code; it is not erasable syntax (Node.js type stripping rejects namespaces that contain code; Babel supports them only partially) and `module Foo {}` becomes an error in 6.0.

```ts
// @types: node
declare global {
  namespace NodeJS {
    interface ProcessEnv {
      DATABASE_URL?: string;
    }
  }
}

export function databaseUrl(): string {
  const url = process.env.DATABASE_URL;
  if (!url) throw new Error("DATABASE_URL is not set");
  return url;
}
```

***

#### JSX

- `jsx: react-jsx` / `react-jsxdev` (**4.1+**, React 17 automatic runtime) — no `import React` needed; `jsxImportSource` (4.1+) selects Preact/Emotion runtimes. `jsx: react` (classic) needs `React` in scope.
- Since **4.7**, spreading values of type `unknown` or `never` into JSX elements is an error.
- With `jsx: react-jsx`, `moduleDetection: auto` (4.7+) treats every `.tsx` file as a module.
- `@types/react` 18 releases have been published for 4.x via the `ts4.x` dist-tags (`@types/react@ts4.9` → 18.3.12); the `ts5.0` tag already points to 19.x, so `@types/react` 19 is not for 4.x — pin it.

***

#### JavaScript Projects (`allowJs`, `checkJs`, JSDoc)

- `checkJs` implies `allowJs` since **4.1**.
- JSDoc support by minor: `@deprecated` (4.0), `@see` (4.1), `{@link}` (4.3), `@type {const}` assertions and default type arguments in `@template` (4.5). `@satisfies`, `@overload` (5.0) and `@import` (5.5) are not available.
- Since **4.6**, JavaScript files always get grammar and binding errors (duplicate declarations, invalid modifiers) — `// @ts-nocheck` opts a file out.
- Since **4.8**, `// @ts-check` JavaScript files cannot `import`/`export` names that are only types (use `import("mod").Type` in JSDoc or `@typedef`).
- Since **4.9**, `tsc` no longer elides unused or type-only imports from `.js` files, so a `.js` file importing a TypeScript-only name now fails at runtime instead of silently working.

***

#### Functions

##### Parameters, return types, overloads

- Annotate parameters and the return type of exported functions; let locals be inferred. An explicit return type stops an accidental `any` or union from leaking into the public API and speeds up checking.
- Optional parameters must come last; prefer an options object over more than three positional parameters or boolean flags.
- Overload signatures are checked only against the implementation signature, not against each other's bodies; keep the implementation signature wide and private, and order overloads from most to least specific. A union-typed parameter is usually simpler than overloads.

```ts
export function toArray(x: string): string[];
export function toArray(x: number[]): number[];
export function toArray(x: string | number[]): string[] | number[] {
  return typeof x === "string" ? x.split(",") : x;
}
export const a = toArray("a,b");
export const b = toArray(true); // error
```

##### `void` callbacks and `Promise<void>` (4.1)

A function type returning `void` accepts functions that return a value; an `async` function passed where `() => void` is expected creates a floating promise (see [Async code](#async-code)). Since **4.1**, `new Promise((resolve) => …)` requires `resolve(value)` unless the type argument is `void`:

```ts
// @since: 4.1
export const p1 = new Promise((resolve) => {
  setTimeout(() => resolve(), 10); // error
});
export const p2 = new Promise<void>((resolve) => {
  setTimeout(() => resolve(), 10);
});
```

##### `this` parameters and method extraction

Methods lose `this` when passed as callbacks. Declare `this: void` on functions that must not use `this`, or a `this` parameter for functions that need one; use arrow-function class properties or `.bind` for handlers.

```ts
class Counter {
  count = 0;
  increment(this: Counter): void {
    this.count++;
  }
}
const c = new Counter();
const inc = c.increment;
inc(); // error
export { c };
```

##### Method parameter bivariance

`strictFunctionTypes` applies only to properties of function type, not to methods. A method signature `handle(e: Event): void` accepts an implementation with a narrower parameter — a common unsoundness in event-handler and plugin interfaces. Declare callback members as properties (`handle: (e: Event) => void`) when they are implemented by callers.

```ts
interface Animal { name: string }
interface Dog extends Animal { bark(): void }

interface MethodStyle { handle(a: Animal): void }
interface PropertyStyle { handle: (a: Animal) => void }

const dogOnly = (d: Dog): void => d.bark();
export const m: MethodStyle = { handle: dogOnly };   // compiles: unsound
export const p: PropertyStyle = { handle: dogOnly }; // error
```

***

#### Classes

##### Class field semantics (`useDefineForClassFields`)

`useDefineForClassFields` defaults to **`true` when `target` is `es2022` or `esnext`** and `false` otherwise (the `esnext` default arrived in 4.3; `es2022` exists from 4.6). With it, fields use `[[Define]]` semantics: they are initialised in declaration order before the constructor body, they shadow base-class accessors, and a declared-but-uninitialised field (`name?: string`) becomes an own property set to `undefined`. The following was compiled with each target and run on Node.js 20 (4.9.5):

```typescript
class Dep { y = 41; }
class A {
  x = this.dep.y + 1;              // reads a parameter property
  constructor(private dep: Dep) {}
}
new A(new Dep()).x;
// target es2021 (4.3–4.9): 42
// target es2022 (4.6–4.9): TypeError at runtime — and NO compile error
// target esnext (4.3–4.9): TS2729 "Property 'dep' is used before its initialization" (and TypeError if emitted)

class B { name?: string; }
Object.keys(new B());
// es2021: []      es2022/esnext: [ 'name' ]
```

**Compiler bug in all 4.x versions (and 5.0; fixed in 5.1)**: TS2729 is reported for `target: esnext` but **not** for `target: es2022`, although both emit native class fields. Raising `target` to `es2022` therefore silently converts "field initialiser reads a parameter property" into a runtime `TypeError`. When reviewing a `target` bump to `es2022`, search for field initialisers that use constructor parameter properties, and for subclasses whose fields redeclare base-class properties (they now overwrite values set by the base constructor and shadow base accessors). Use `declare name: string;` to redeclare a property's type without emitting a field.

##### Parameter properties, visibility and `#private`

- `private`/`protected` are compile-time only: the property is visible to `JSON.stringify`, `Object.keys`, `obj["secret"]` and any JavaScript caller. Use `#private` (3.8+ for fields; **4.3+** for `#methods` and `#accessors`) when privacy must hold at runtime. `#private` requires `target` ≥ ES2015 (TS18028 below that) and is down-levelled to `WeakMap`s for targets below ES2022.
- `#field in obj` brand checks are **4.5+**.

```ts
// @since: 4.5
export class Token {
  #secret: string;
  constructor(secret: string) { this.#secret = secret; }
  #mask(): string { return this.#secret.slice(0, 2) + "***"; }
  get masked(): string { return this.#mask(); }
  static isToken(x: unknown): x is Token {
    return typeof x === "object" && x !== null && #secret in x;
  }
}
```

##### `override` and `noImplicitOverride` (4.3+)

```ts
// @since: 4.3
// @noImplicitOverride: true
class Base {
  render(): string { return "base"; }
}
export class Child extends Base {
  render(): string { return "child"; } // error
}
export class Child2 extends Base {
  override render(): string { return "child2"; }
  override rendr(): string { return "typo"; } // error
}
```

Enable `noImplicitOverride` in 4.3+ projects: renaming a base method otherwise leaves subclasses with silently dead "overrides". On 4.0–4.2 the `override` keyword is a syntax error — do not suggest it there.

##### Accessors

Getter and setter of the same property may have **different types since 4.3**, as long as the getter's type is assignable to the setter's type (completely unrelated types require 5.1):

```ts
// @since: 4.3
export class Box {
  #size = 0;
  get size(): number { return this.#size; }
  set size(value: number | string) {
    this.#size = typeof value === "string" ? Number.parseInt(value, 10) : value;
  }
}
const b = new Box();
b.size = "42";
export const n: number = b.size;
```

Since **4.0**, a subclass property that overrides a base accessor (or vice versa) is always an error:

```ts
class Base {
  get id(): number { return 1; }
}
export class Derived extends Base {
  id = 2; // error
}
```

`accessor` fields (**4.9+**) declare an auto-accessor backed by a private slot. They are part of the TC39 decorators proposal; in 4.9 they mainly serve as a decorator target for future code and need `target` ≥ ES2015 (TS18045 otherwise):

```ts
// @since: 4.9
export class Person {
  accessor name: string;
  constructor(name: string) { this.name = name; }
}
```

##### Constructors: `super()` ordering (4.6), static blocks (4.4)

Since **4.6**, code that does not touch `this` may run before `super()` even when the class has parameter properties or initialised fields (before 4.6: TS2376):

```ts
// @since: 4.6
class Base { constructor(readonly id: string) {} }
export class User extends Base {
  role = "user";
  constructor(rawId: string) {
    const id = rawId.trim().toLowerCase();
    super(id);
  }
}
```

`static { … }` initialisation blocks are **4.4+** (down-levelled for targets below ES2022):

```ts
// @since: 4.4
export class Registry {
  static readonly names: string[] = [];
  static {
    Registry.names.push("default");
  }
}
```

##### Abstract classes, `implements`, definite assignment

- `implements` checks the class shape but does not contribute types: parameters of an implemented method are **not** contextually typed by the interface (they become implicit `any` under `noImplicitAny`).
- `abstract` members cannot be `async` (4.1) and abstract properties cannot have initialisers (4.4, TS1267).
- `strictPropertyInitialization` requires fields to be assigned in the constructor. The definite-assignment assertion `field!: T` turns the check off for that field — accept it only where a framework (DI container, ORM, Angular `@Input`) provably initialises the field.

```ts
// @since: 4.4
export abstract class Repo<T> {
  abstract readonly table: string;
  abstract name = "repo"; // error
  abstract find(id: string): Promise<T | undefined>;
}
export class Service {
  private client!: { query(sql: string): Promise<unknown> }; // set by init(); review carefully
  private timeoutMs: number; // error
  init(client: { query(sql: string): Promise<unknown> }): void { this.client = client; }
}
```

##### Subclassing `Error` with ES5/ES3 targets

With `target` ES5 or ES3 (the 4.x default), subclasses of `Error` and `Array` lose their prototype (`instanceof` is `false`, subclass methods are missing), and subclasses of `Map`/`Set` throw `TypeError: Constructor Map requires 'new'` on construction. Verified with 4.9.5 on Node.js 20:

```typescript
class NotFound extends Error {
  constructor(msg: string) { super(msg); this.name = "NotFound"; }
}
new NotFound("x") instanceof NotFound;  // target es5: false   target es2015+: true
```

Fix: target ES2015 or later (every supported runtime is), or call `Object.setPrototypeOf(this, new.target.prototype)` in the constructor.

***

#### Decorators (experimental only in 4.x)

4.x supports only the **legacy (stage-1) decorators** behind `experimentalDecorators`, plus `emitDecoratorMetadata` for reflection (`reflect-metadata`, used by NestJS, TypeORM, InversifyJS, class-validator). Without the flag, every 4.x version rejects decorators with TS1219; 5.0 accepts them without a flag as **standard** decorators with a different signature and semantics.

```ts
// @experimentalDecorators: true
function logged(target: object, key: string, desc: PropertyDescriptor): void {
  const original = desc.value as (...args: unknown[]) => unknown;
  desc.value = function (this: unknown, ...args: unknown[]) {
    console.log(`call ${key}`);
    return original.apply(this, args);
  };
}
export class Api {
  @logged
  fetch(id: string): string { return id; }
}
```

Review points:

- Do not port 4.x decorators to the 5.0 standard form (`(value, context) => …`) without also removing `experimentalDecorators`; the two models are incompatible, and libraries built on `reflect-metadata` (NestJS, TypeORM, InversifyJS, class-validator) require the legacy model.
- `emitDecoratorMetadata` emits runtime references to parameter types: a class imported with `import type` is recorded as `Function` and an interface as `Object` in `design:paramtypes` (verified on 4.9.5), which breaks DI resolution silently at runtime. Keep DI-injected classes as value imports; `isolatedModules` tools (esbuild) do not emit metadata at all — use `swc` or `tsc` for such projects.
- Parameter decorators on constructor parameters receive `undefined` as the property key; 5.0 checks this strictly, 4.x does not — a decorator typed `(target, key: string, index)` compiles on 4.x and fails on 5.0.
- In 4.8+, the TypeScript **API** moved decorators into `node.modifiers` (`ts.getDecorators()`, `ts.canHaveDecorators()`); custom transformers and codegen written for ≤4.7 that read `node.decorators` see `undefined` on 4.8+.

***

#### Enums and Their Alternatives

- Numeric enums create a reverse mapping and, in every 4.x version, **accept any number** (`let e: Direction = 42` compiles; 5.0 rejects out-of-range literals). Validate numbers from the outside before treating them as an enum.
- Since **4.3**, comparing a union enum value with a numeric literal outside its domain is an error (TS2367).
- String enums are nominal: a string literal is not assignable to them.
- `const enum` is inlined by `tsc` but cannot work with per-file transpilers: under `isolatedModules`, ambient const enums from `.d.ts` files are TS2748, and libraries must not export `const enum` in their public `.d.ts`.
- Prefer a union of string literals or an `as const` object for new code: no runtime/type mismatch, works with `isolatedModules` and with type-stripping tools.

```ts
enum Direction { Up, Down }
export let d: Direction = 42; // compiles in 4.x (error in 5.0)

export function isUp(x: Direction): boolean {
  return x === Direction.Up || x === 5; // error 4.3+
}

enum Status { Active = "active", Disabled = "disabled" }
export const s: Status = "active"; // error

export const Level = { Debug: "debug", Info: "info" } as const;
export type Level = typeof Level[keyof typeof Level]; // "debug" | "info"
export const lvl: Level = "info";
```

***

#### Error Handling

##### `unknown` in `catch` (4.0 annotation, 4.4 default under `strict`)

`catch (e: unknown)` is allowed since 4.0; under `strict` (or `useUnknownInCatchVariables`) **4.4+** binds `unknown` by default. Anything can be thrown — strings, `undefined`, objects from other realms — so narrow before use:

```ts
// @since: 4.4
export function errorMessage(e: unknown): string {
  if (e instanceof Error) return e.message;
  if (typeof e === "string") return e;
  try {
    return JSON.stringify(e);
  } catch {
    return String(e);
  }
}

export function run(task: () => void): string | undefined {
  try {
    task();
    return undefined;
  } catch (e) {
    return e.message; // error
  }
}
```

Do not "fix" the 4.4 upgrade with `catch (e: any)` across the codebase; use a helper like `errorMessage` or narrow with `instanceof`.

##### Error `cause` (lib `es2022`, 4.6+)

`new Error(message, { cause })` is declared in `lib.es2022.error` (4.6+; requires `lib`/`target` `es2022`) and works in Node.js 16.9+. Since **4.8**, `Error.prototype.cause` is typed `unknown` (it was `Error` in 4.6/4.7):

```ts
// @since: 4.8
// @target: es2022
export async function readConfig(load: () => Promise<string>): Promise<string> {
  try {
    return await load();
  } catch (err) {
    throw new Error("could not read config", { cause: err });
  }
}
export function rootCause(e: Error): Error {
  const c: Error = e.cause; // error
  return c;
}
```

##### Rules

- Throw `Error` subclasses, never strings or plain objects; set `name`.
- Re-throw with `cause` instead of losing the original stack; do not swallow errors in empty `catch {}` blocks (optional catch binding is ES2019 syntax and fine in 4.x).
- Model expected failures (validation, "not found") as return values (`Result` unions) where callers must handle them; reserve exceptions for unexpected failures.
- `finally` with `return` overrides a thrown error — flag it.

***

#### Async Code

##### Floating and misused promises

The compiler does not report unhandled promises (except the always-truthy checks). Use `@typescript-eslint/no-floating-promises` and `no-misused-promises` (type-aware linting). Typical bugs:

```ts
declare function save(id: string): Promise<void>;

export function bad(ids: string[]): void {
  ids.forEach(async (id) => {
    await save(id); // forEach does not await: errors become unhandled rejections
  });
  save("x"); // floating: rejection is unhandled, ordering is lost
}

export async function good(ids: string[]): Promise<void> {
  for (const id of ids) {
    await save(id); // sequential
  }
  await Promise.all(ids.map((id) => save(id))); // concurrent, fail-fast
}
```

- `Promise.all` rejects on the first failure but does not cancel the others; use `Promise.allSettled` (lib `es2020`) when every result matters.
- `Promise.any` and `AggregateError` need lib `es2021` (4.3+).
- Unbounded `Promise.all` over user-sized arrays exhausts sockets/DB pools; batch or use a concurrency limiter.
- Since Node.js 15, an unhandled rejection terminates the process by default.
- Return `await` inside `try` blocks (`return await p`), otherwise the `catch`/`finally` does not see the rejection.

##### Cancellation and timeouts

Pass an `AbortSignal` through async APIs (`fetch`, `undici`, `fs/promises`, `events.once`, `timers/promises` in Node.js 16+). `AbortSignal.timeout()` exists in Node.js 17.3+/16.14+, but its type declaration comes from `@types/node`/`lib.dom` versions — check that the pinned `@types/node` declares it.

##### Async iteration and generators

- `for await (const chunk of stream)` needs the `AsyncIterable` declarations of `lib` `es2018` (or `es2018.asynciterable`); with `lib: es2017` it fails with TS2583 even though `target: es2017` can down-level it.
- With `target` ES5/ES3, `for...of` over `Map`, `Set` or strings, and array spread of iterables, require `downlevelIteration` (TS2802 otherwise). Target ES2015+ instead.
- Since **4.2**, `const x = yield value` without a contextual type is an implicit-`any` error (TS7057); annotate the generator's return type `Generator<Y, R, N>`.

```ts
// @since: 4.2
export function* bad() {
  const input = yield 1; // error
  return input;
}
export function* good(): Generator<number, string, string> {
  const input = yield 1;
  return input;
}
```

***

#### Reviewer-Relevant Compiler Quirks and Bugs in 4.x

None of these will be fixed in 4.x. Each was reproduced on the final patch of the stated versions.

- **`target: es2022` hides TS2729** (4.6–4.9 and 5.0.4; fixed in 5.1): a field initialiser that reads a constructor parameter property (`x = this.dep.y` with `constructor(private dep: Dep)`) is reported for `target: esnext` but not for `target: es2022`, and the emitted native class field throws `TypeError` at runtime. See [Class field semantics](#class-field-semantics-usedefineforclassfields).
- **One syntax error hides all type errors.** When any file in the program has a syntax error, `tsc` reports only syntax errors. A single 4.5+ construct (`import { type X }`, `satisfies`, `accessor`, `infer X extends`) compiled by an older 4.x compiler produces one TS1005 and masks every real type error in the project until it is fixed.
- **Narrowing is not preserved in closures** (all 4.x; fixed for the after-last-assignment case in 5.4) — code that copies to a `const` is correct, not redundant.
- **Numeric enums accept any number** (all 4.x; 5.0 rejects out-of-range literals).
- **`exports` vs `typesVersions` precedence** was wrong under `node16`/`nodenext` before 4.9: 4.7/4.8 may pick a `typesVersions` redirect where 4.9 follows `exports`. A library tested only with 4.9 can present different types to 4.7/4.8 consumers.
- **Relational comparisons with implicit coercion** (`ns > 4` where `ns: number | string`) compile in 4.x and are errors in 5.0 (TS2365).
- **Inline arrow predicates do not narrow** (`.filter(x => x != null)` keeps `T | undefined`) in all 4.x versions (5.5 infers predicates).
- **`@types` parse failures**: current `@types/*` releases contain 5.x syntax; `skipLibCheck` does not suppress syntax errors in `.d.ts` files.
- **`tsc` without a tsconfig** compiles to ES3 with non-strict checks; `tsc file.ts` ignores the project's `tsconfig.json` entirely (a common source of "it compiles locally" reports).

***

#### Security

TypeScript types disappear at runtime; every security property must be enforced by runtime code.

##### Validate untrusted input

- Treat `JSON.parse`, `req.body`, `process.env`, query strings, message events and database rows as `unknown` and validate with a schema library or hand-written guards. A cast (`as User`) or a generic `parse<T>()` is not validation.
- Pin validation libraries to versions that support the project's compiler: `zod` 3.22 documents "TypeScript 4.5+"; Zod 4 is tested only against TypeScript 5.5+.[^15] A schema library whose `.d.ts` no longer parses on 4.x leaves the build broken, not insecure — but the temptation to replace it with casts is the real risk.

##### Prototype pollution and dynamic keys

```ts
type Dict = Record<string, unknown>;

export function setPath(target: Dict, path: string, value: unknown): void {
  const keys = path.split(".");
  let cur: Dict = target;
  for (const key of keys.slice(0, -1)) {
    if (key === "__proto__" || key === "constructor" || key === "prototype") {
      throw new Error("forbidden key");
    }
    const next = cur[key];
    if (typeof next !== "object" || next === null) cur[key] = {};
    cur = cur[key] as Dict;
  }
  const last = keys[keys.length - 1];
  if (last === undefined || last === "__proto__" || last === "constructor" || last === "prototype") {
    throw new Error("forbidden key");
  }
  cur[last] = value;
}
```

- Prefer `Map<string, V>` or `Object.create(null)` for dictionaries filled from user input; a `Record<string, V>` typed object still inherits `__proto__`, `constructor` and `toString`.
- Use `Object.hasOwn` (lib `es2022`, 4.6+, Node.js 16.9+) or `Object.prototype.hasOwnProperty.call` instead of `in` for own-property checks — `"toString" in {}` is `true`.
- Deep-merge and "set by path" helpers applied to request bodies are a classic pollution vector; check that third-party ones are on patched versions.

##### Other review points

- No `eval`, `new Function`, `vm.runInNewContext` on input; no `child_process.exec` with interpolated strings (use `execFile`/`spawn` with argument arrays).
- SQL built with template literals is injection unless a tagged template from the driver (`sql\`…\``) or parameter placeholders are used.
- Resolve and contain file paths (`path.resolve` + prefix check) before reading user-chosen files.
- `inlineSources`/`sourceMap` files shipped to browsers disclose original source; ship source maps only to the error tracker.
- `.tsbuildinfo`, `tsconfig.json` with internal paths, and `src/` usually do not belong in the published npm package: check `files`/`.npmignore`.
- The 4.x toolchain is frozen: `typescript-eslint` 5/6/7, `ts-jest` 27/28 and their transitive dependencies receive no fixes. Run `npm audit`/OSV scanning; prefer the newest tool line that still supports the pinned compiler (see [Tooling](#tooling)).

***

#### Performance

##### Runtime (what TypeScript affects)

- `tsc` adds no runtime besides down-levelled syntax, `enum`/`namespace` objects, decorator helpers and import helpers. Lower `target`s produce larger, slower code: `async`/`await` below ES2017 becomes a generator state machine, generators below ES2015 become `__generator` switch loops, `#private` below ES2022 becomes `WeakMap` lookups, class fields below ES2022 become constructor assignments.
- Set `target` to what the oldest runtime supports (Node.js 16 → `es2021`; Node.js 18+ → `es2022`).
- With `importHelpers`, the helpers come from `tslib` — its version must provide every helper the compiler emits: `#field in obj` (4.5) needs `__classPrivateFieldIn`, added in tslib **2.4.0**; with tslib 2.3.1 4.9.5 reports TS2343 ("…which does not exist in 'tslib'. Consider upgrading your version of 'tslib'"). Use the newest `tslib` 2.x.

##### Compiler performance

- `skipLibCheck: true`, `incremental: true` (writes `.tsbuildinfo`), project references with `tsc --build` for monorepos.
- Diagnose with `--extendedDiagnostics`, `--listFiles`, `--explainFiles` (**4.2+**), `--traceResolution` and `--generateTrace <dir>` (**4.1+**), and analyse traces with `@typescript/analyze-trace` (released with 4.6).
- Prefer `interface extends` over large intersections; annotate return types of exported functions; avoid huge unions (> a few thousand members) and deeply recursive conditional types; tail-recursive formulations (4.5+) are cheaper and allowed to recurse deeper.
- Restrict `include` to sources and set `types` explicitly so that unrelated `@types/*` packages are not loaded.
- 4.9's file watching uses file-system events by default; on network file systems (NFS, SMB) switch `watchOptions.watchFile`/`watchDirectory` to a polling strategy such as `"dynamicPriorityPolling"` if changes are missed.

***

#### Testing

- **Type-check tests in CI with `tsc --noEmit`** (or `tsc -b`) — test runners that transpile with esbuild/swc/Babel (Vitest, `@swc/jest`, `tsx`, `ts-jest` with `isolatedModules: true`) never report type errors.
- **Jest 29 + `ts-jest` 29**: `ts-jest` 29.x declares `typescript >=4.3` (29.4.x: `>=4.3 <7`); verified: `ts-jest` 29.4.15 with Jest 29.7.0 runs and type-checks tests on 4.9.5. For TypeScript 4.0–4.2 use `ts-jest` 27.1.x (peer `>=3.8 <5.0`, Jest 27).
- **Type-level tests**: `// @ts-expect-error` (3.9+) fails when the line no longer errors — preferable to `@ts-ignore`, which hides any error. Libraries like `expect-type`/`tsd` work if their own `.d.ts` parse on the pinned version.
- Test both runtime behaviour and the type contract of public APIs; when a PR changes `target`, `useDefineForClassFields`, `module` or `esModuleInterop`, require runtime tests of the emitted code, not only `tsc` success.

```ts
interface User { id: string; name: string }

// @ts-expect-error -- missing name must be rejected
export const invalid: User = { id: "1" };

type Equal<A, B> = (<T>() => T extends A ? 1 : 2) extends (<T>() => T extends B ? 1 : 2) ? true : false;
export const check: Equal<Pick<User, "id">, { id: string }> = true;
```

***

#### Tooling

##### Compiler and runners

- **`typescript` 4.9.5** declares `engines.node >= 4.2.0` and runs on every current Node.js; 5.0 requires Node.js ≥ 12.20.
- Pin the exact compiler (`"typescript": "4.9.5"` or `~4.9.5`) and commit the lockfile. A caret range on 4.x never moves to 5.0, but within 4.x a lockfile refresh from `^4.2.0` jumps to 4.9.5 and brings every breaking change listed in this guide.
- **Editors** bundle the newest TypeScript. In VS Code, set `"typescript.tsdk": "node_modules/typescript/lib"` and select "Use Workspace Version", otherwise the editor shows 5.x/7.x diagnostics and quick fixes (`satisfies`, `const T`, …) that the project compiler rejects.
- **`ts-node` 10.9.2** (peer `typescript >= 2.7`)[^20] works with 4.9.5: CommonJS directly, ESM via `node --loader ts-node/esm` (verified on Node.js 22.23.4; Node prints deprecation warnings for `--loader`). ts-node type-checks by default; `transpileOnly`/`swc` modes skip checking.
- **`tsx`, `esbuild`, `swc`, Babel** strip types with their own parsers, independent of the project's compiler version, and never type-check. They support `isolatedModules`-compatible code only (no cross-file `const enum` inlining, no `emitDecoratorMetadata` in esbuild).
- **Node.js built-in type stripping** (Node.js 22.18+/23.6+ by default) also ignores the project's TypeScript version; it rejects non-erasable syntax such as `enum` and parameter properties (verified on Node.js 22.23.4) — common in 4.x codebases.

##### Linting: `typescript-eslint` versions for 4.x

`typescript-eslint` checks the running compiler against its supported range and warns outside it.[^16]

| typescript-eslint | Supported TypeScript | ESLint | Node.js |
|---|---|---|---|
| 5.62.0 (last v5, July 2023) | `>=3.3.1 <5.2.0` | 6, 7, 8 | `^12.22 \|\| ^14.17 \|\| >=16` |
| 6.21.0 (last v6, Feb 2024) | `>=4.3.5 <5.4.0` | 7, 8 | `^16 \|\| >=18` |
| 7.18.0 (last v7, July 2024) | `>=4.7.4 <5.6.0` | `^8.56.0` | `^18.18 \|\| >=20` |
| 8.0.0 – 8.15.0 | `>=4.7.4` (upper bound moves) | `^8.57.0 \|\| ^9` | `^18.18 \|\| ^20.9 \|\| >=21.1` |
| 8.16.0+ (8.72.0 current) | `>=4.8.4` (8.72.0: `<6.1.0`) | `^8.57.0 \|\| ^9 \|\| ^10` | `^18.18 \|\| ^20.9 \|\| >=21.1` |

So: TypeScript 4.0–4.2 → typescript-eslint 5.62.0; 4.3–4.6 → 6.21.0; 4.7 → 7.18.0 or ≤ 8.15.0; 4.8–4.9 → current 8.x. Verified: `typescript-eslint` 8.72.0 with ESLint 9.39.5, `recommendedTypeChecked` and `parserOptions.projectService` lints a 4.9.5 project (reports `no-floating-promises`, `require-await`) without a version warning.

Rules with the highest review value on 4.x: `no-floating-promises`, `no-misused-promises`, `await-thenable`, `no-unsafe-*` (assignment/member-access/call/return/argument), `no-explicit-any`, `consistent-type-imports` (replacement for `importsNotUsedAsValues: "error"`), `switch-exhaustiveness-check`, `no-unnecessary-condition`, `prefer-nullish-coalescing`, `no-non-null-assertion`, `ban-ts-comment` (require descriptions on `@ts-expect-error`), `explicit-module-boundary-types` for libraries.

***

#### Observability and Debugging

- Emit `sourceMap: true` (or `inlineSourceMap`) and run Node.js with `--enable-source-maps` (Node.js 12.12+) so stack traces point to `.ts` lines; upload maps to the error tracker instead of serving them publicly.
- `declarationMap: true` lets "go to definition" jump from a package's `.d.ts` to its `.ts` source in monorepos.
- Give `Error` subclasses a `name` and keep `cause` chains (lib `es2022`, 4.6+) so logs show the root cause.
- Debug the build with `--explainFiles` (why is this file included?), `--traceResolution` (why does this import resolve there?), `--showConfig` (the effective configuration after `extends`), `--extendedDiagnostics` and `--generateTrace`.
- Structured logging (`pino`, `winston`) with typed log context objects; never log whole request bodies or `Error` objects that carry credentials in `cause`/`config` (Axios errors include request headers).

***

#### Migrating from 4.x to 5.0 (and beyond)

TypeScript 5.0 (March 16, 2023) is the next release after 4.9; there is no 4.10.[^17]

##### Before the upgrade, on 4.9.5

- Upgrade to **4.9.5** first and fix its breaking changes (4.8: `T` → `{}`, `x === []`; 4.9: `x === NaN`, `Promise.resolve`/`Awaited` inference).
- Add `"ignoreDeprecations": "5.0"` only if a deprecated option must stay during the transition — 4.9.5 accepts it (4.8.4 and older reject it with TS5023).
- Replace `importsNotUsedAsValues`/`preserveValueImports` usage with explicit `import type`/`type` modifiers so that the switch to `verbatimModuleSyntax` is mechanical.
- Pin `@types/*` and linting to versions that support both 4.9 and 5.x during the transition (e.g. `typescript-eslint` 6.21–8.x).

##### 5.0 deprecations (errors TS5101/TS5107 in 5.0–5.4, no effect in 5.5, hard errors from 5.6)

In 5.0, each of these is an **error** unless `"ignoreDeprecations": "5.0"` is set (verified on 5.0.4: TS5101 for `importsNotUsedAsValues: "preserve"`/`"error"`, TS5107 for `target: ES3`; the default values such as `importsNotUsedAsValues: "remove"` are not reported):[^17]

- `target: ES3`
- `out` (use `outFile`)
- `noImplicitUseStrict`, `keyofStringsOnly`, `suppressExcessPropertyErrors`, `suppressImplicitAnyIndexErrors`, `noStrictGenericChecks`, `charset`
- `importsNotUsedAsValues`, `preserveValueImports` (→ `verbatimModuleSyntax`)
- `prepend` in project references

##### 5.0 default and behaviour changes

- Default `target` is ES5 (was ES3); `newLine` defaults to `LF`; `forceConsistentCasingInFileNames` defaults to `true`.
- **Enum overhaul**: all enums are union enums; assigning an out-of-domain numeric literal is an error; mixed string/number enum edge cases fixed.
- **Relational operators** reject implicit `string | number` coercion (`ns > 4`).
- **Decorators**: without `experimentalDecorators`, `@dec` now means standard (TC39) decorators with a different signature — code that relied on TS1219 to force the flag on will instead compile against the wrong decorator model; with `experimentalDecorators`, parameter decorators on constructors are checked more strictly.
- `moduleResolution: node` is renamed `node10`; new `bundler` resolution, `customConditions`, `allowImportingTsExtensions`, `resolvePackageJsonExports/Imports`; `extends` accepts arrays.
- TypeScript's own source moved from namespaces to modules and the compiler API changed (see the API breaking-changes wiki) — custom transformers and compiler patchers (`ttypescript`, `ts-patch`) must be re-verified against the new compiler.
- Later majors continue: 6.0 turns `target: es5`, `moduleResolution: node`/`classic`, `baseUrl`, `outFile`, `esModuleInterop: false` and import `assert` syntax into deprecation errors (silenced with `"ignoreDeprecations": "6.0"`), and 7.0 removes them. A 4.x project usually migrates in steps: 4.9.5 → 5.x latest with `ignoreDeprecations` cleared → 6.0 → 7.0.

```ts
enum Digit { Zero = 0, Two = 2 }
export const m: Digit = 1; // compiles on 4.x, error on 5.0

export function cmp(ns: number | string): boolean {
  return ns > 4; // compiles on 4.x, error on 5.0
}
```

***

#### Idioms and Things to Avoid

| Avoid | Prefer | Why |
|---|---|---|
| `as` casts on external data | runtime validation into `unknown` | casts verify nothing |
| `any` in signatures | `unknown`, generics with constraints | `any` disables checking transitively |
| `!` non-null assertions | explicit checks, `??` defaults | crashes move to runtime |
| `@ts-ignore` | `@ts-expect-error -- reason` | `@ts-ignore` hides every error and never expires |
| `Function`, `Object`, `{}` as types | specific function types, `object`, `Record<string, unknown>` | too wide |
| `namespace` for code organisation | ES modules | namespaces do not tree-shake and are not erasable syntax |
| `export default` | named exports | stable names across refactors and re-exports |
| `const enum` across package boundaries | union types / `as const` objects | breaks with `isolatedModules` and `.d.ts` consumers |
| `import * as x from "cjs"; x()` | `import x from "cjs"` with `esModuleInterop` | crashes under interop |
| `forEach(async …)` | `for…of` + `await` or `Promise.all` | floating promises |
| `satisfies`, `override`, `Awaited`, `accessor`, `import { type X }` on an older 4.x minor | the 4.x-version-appropriate alternative | syntax error on that compiler |
| `const T`, `NoInfer`, `using`, `bundler`, `verbatimModuleSyntax` | 4.x patterns in this guide | not available in 4.x |

***

#### Production / Review Checklist

##### Configuration

- [ ] Exact compiler version known from the lockfile; features used match that minor (see the per-version markers)
- [ ] `strict: true`; additional flags only if the pinned minor has them (`noUncheckedIndexedAccess` 4.1, `noImplicitOverride` 4.3, `exactOptionalPropertyTypes` 4.4)
- [ ] `target` and `lib` set explicitly and matching the oldest runtime; no `lib` declarations the runtime lacks
- [ ] `moduleResolution` set explicitly (`node` or `node16`) — never left to `classic`
- [ ] `module: node16` (4.7+) for Node.js services that import ESM-only packages; `.js` extensions in ESM relative imports
- [ ] `esModuleInterop: true`; `forceConsistentCasingInFileNames: true`; `isolatedModules: true` when a single-file transpiler is in the chain
- [ ] `@types/*` pinned to versions published for the compiler (`@types/node@ts4.9` etc.); dependency bot PRs for `@types` verified by CI
- [ ] `tslib` ≥ 2.4.0 when `importHelpers` is on and 4.5+ syntax is used
- [ ] `target` bumps to `es2022` reviewed for `useDefineForClassFields` runtime changes (parameter properties read in field initialisers, redeclared fields)

##### Types and code

- [ ] No `any`/casts on data from I/O; validation at boundaries
- [ ] Exhaustive `switch` on discriminated unions with `never` checks
- [ ] Narrowed values copied to `const` before use in callbacks
- [ ] No `x === []`, `x === NaN`, `if (promise)`; no uncalled-function conditions
- [ ] Enums: numeric enum values from outside validated; no union-enum comparisons against foreign literals
- [ ] `override` used consistently (4.3+), `#private` where runtime privacy matters
- [ ] Type predicates and assertion functions reviewed for correctness

##### Async and errors

- [ ] No floating promises; no `forEach(async …)`; `return await` inside `try`
- [ ] `catch (e)` handled as `unknown`; errors wrapped with `cause` (es2022 lib)
- [ ] `Promise<void>` for promises resolved without a value
- [ ] Concurrency bounded for user-sized collections; `AbortSignal` passed through

##### Security

- [ ] No prototype pollution via dynamic keys; `Map`/`Object.create(null)` for user-keyed data
- [ ] No `eval`/`new Function`/shell interpolation; parameterised SQL
- [ ] Source maps not publicly served; package `files` reviewed
- [ ] Frozen 4.x toolchain dependencies audited

##### Build, tooling and observability

- [ ] CI runs `tsc --noEmit` (or `tsc -b`) with the project compiler, plus type-aware `typescript-eslint` at a version supporting the pinned minor
- [ ] Tests run against emitted/transpiled code when module or class-field settings change
- [ ] `--enable-source-maps` in production Node.js; errors carry `name` and `cause`
- [ ] Upgrade path to 5.x documented: deprecated options removed, `ignoreDeprecations` not left in place

***

[^1]: Announcing TypeScript 4.9 / TypeScript 4.9 release notes. https://www.typescriptlang.org/docs/handbook/release-notes/typescript-4-9.html
[^2]: npm registry, `typescript` package version history and publish dates (4.0.2 … 4.9.5, 5.0.2, 6.0.3, 7.0.2, 7.1.0-beta). https://www.npmjs.com/package/typescript?activeTab=versions
[^3]: GitHub release TypeScript 4.9.5 ("Port ignore deprecations to 4.9", #52419; "Port timestamp fix to 4.9", #52426). https://github.com/microsoft/TypeScript/releases/tag/v4.9.5
[^4]: Announcing TypeScript 7.0 and TypeScript 6.0 (TypeScript blog). https://devblogs.microsoft.com/typescript/
[^5]: DefinitelyTyped README — support window ("only tests packages on versions of TypeScript that are less than 2 years old") and `ts4.x` dist-tags. https://github.com/DefinitelyTyped/DefinitelyTyped#support-window ; https://www.npmjs.com/package/@types/node?activeTab=versions
[^6]: TypeScript 4.0 release notes. https://www.typescriptlang.org/docs/handbook/release-notes/typescript-4-0.html
[^7]: TypeScript 4.1 release notes. https://www.typescriptlang.org/docs/handbook/release-notes/typescript-4-1.html
[^8]: TypeScript 4.2 release notes. https://www.typescriptlang.org/docs/handbook/release-notes/typescript-4-2.html
[^9]: TypeScript 4.3 release notes. https://www.typescriptlang.org/docs/handbook/release-notes/typescript-4-3.html
[^10]: TypeScript 4.4 release notes. https://www.typescriptlang.org/docs/handbook/release-notes/typescript-4-4.html
[^11]: TypeScript 4.5 release notes. https://www.typescriptlang.org/docs/handbook/release-notes/typescript-4-5.html
[^12]: TypeScript 4.6 release notes. https://www.typescriptlang.org/docs/handbook/release-notes/typescript-4-6.html
[^13]: TypeScript 4.7 release notes. https://www.typescriptlang.org/docs/handbook/release-notes/typescript-4-7.html
[^14]: TypeScript 4.8 release notes. https://www.typescriptlang.org/docs/handbook/release-notes/typescript-4-8.html
[^15]: Zod 3.22 README ("TypeScript 4.5+!") and Zod 4 documentation (tested against TypeScript 5.5+). https://unpkg.com/zod@3.22.4/README.md ; https://zod.dev
[^16]: typescript-eslint — dependency versions and `SUPPORTED_TYPESCRIPT_VERSIONS` in `@typescript-eslint/typescript-estree` (5.62.0, 6.21.0, 7.18.0, 8.15.0, 8.16.0, 8.72.0). https://typescript-eslint.io/users/dependency-versions
[^17]: TypeScript 5.0 release notes — Breaking Changes and Deprecations. https://www.typescriptlang.org/docs/handbook/release-notes/typescript-5-0.html
[^18]: TSConfig reference (`useDefineForClassFields`, `moduleResolution`, `target`, `strict`, `ignoreDeprecations`). https://www.typescriptlang.org/tsconfig
[^19]: Modules reference — theory and `node16`/`nodenext`. https://www.typescriptlang.org/docs/handbook/modules/reference.html
[^20]: ts-jest, ts-node and tslib package metadata (peer dependencies, helper exports). https://www.npmjs.com/package/ts-jest ; https://www.npmjs.com/package/ts-node ; https://www.npmjs.com/package/tslib
