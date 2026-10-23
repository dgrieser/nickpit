### TypeScript 5.6: The Complete Production Developer Guide

#### Overview

> **Version**: TypeScript 5.6. The GA build, **5.6.2, shipped on September 9, 2024**; the latest and final patch is **5.6.3 (published to npm on October 8, 2024)**.[^1][^2][^3]
> **Support status (October 2026)**: unsupported. The TypeScript team ships patches only for the newest release line, so 5.6 received no fixes after 5.6.3. Newer lines followed: 5.7 (November 2024), 5.8 (February 2025), 5.9 (July 2025), **6.0** (the last JavaScript-based compiler, 6.0.2 on March 23, 2026; latest 6.0.3, April 16, 2026) and **7.0** (the native Go port, 7.0.2 on July 8, 2026), which is the npm `latest` tag today.[^3][^9][^10]
> **Scope**: the complete TypeScript language and compiler as usable in 5.6 — the type system, narrowing, generics, classes and decorators, `using`, modules and resolution, declaration emit, strictness flags, tsconfig, tooling — with every 5.6 change versus 5.5 marked **New in 5.6**.
> **Note**: this document is self-contained. Everything that shipped before 5.6 is carried forward. Every code block was compiled with `tsc` 5.6.3 (`strict`, `target: es2022`, `module: nodenext`, `lib: ["esnext", "dom"]`, `@types/node` 26) unless the block says otherwise; lines marked `// error` fail on 5.6 exactly as described.

TypeScript 5.6 is a mid-sized release. Its headline changes are **errors for syntactically always-truthy or always-nullish checks** (`if (/re/)`, `x => 0` instead of `x >= 0`, `a < b ?? c`), **types for ECMAScript iterator helpers** (`.map()`, `.filter()`, `.take()` … on built-in iterators, plus the new `IteratorObject` / `BuiltinIteratorReturn` types), the strict-family flag **`--strictBuiltinIteratorReturn`**, **arbitrary module namespace identifiers** (`export { x as "🍌" }`), **`--noUncheckedSideEffectImports`**, **`--noCheck`**, **`tsc --build` continuing past errors in upstream projects** (with the opt-out `--stopBuildOnErrors`), and several *behaviour changes that break existing builds*: module-format detection for `node_modules` in every module mode, `.mts`/`.cts` files that never emit the wrong module format, correct `override` checks on computed members, and `.tsbuildinfo` files written for every project in `--build` mode.[^1]

Global truths about TypeScript 5.6:

- **Types are erased.** `tsc` checks types and then removes them; nothing in a type annotation, interface, type alias, `as` assertion or `!` assertion exists at runtime. Validation of external data is a runtime job.
- **Typing is structural.** A value is assignable to a type when it has the required members; names and declarations do not matter.
- **The type system is deliberately unsound in places**: array covariance, method-parameter bivariance, `any`, type assertions, non-null assertions, unchecked index access (without `noUncheckedIndexedAccess`), and user-written type predicates all let wrong types through. A reviewer must treat each of these as a claim the author has to justify.
- **`lib` describes the runtime, it does not provide it.** `lib: ["esnext"]` in 5.6 declares iterator helpers, `Set.prototype.union`, `DisposableStack`, `Promise.withResolvers` and more; whether they exist depends on the actual engine (Node 22 has iterator helpers and Set methods but no `DisposableStack`).
- **TypeScript does not polyfill.** Downlevel emit (`target`) rewrites *syntax* (`using`, class fields, `?.`), never library APIs.
- **The default `target` is still ES5 and `strict` is still off by default** in 5.6 (both change in 6.0); always configure them explicitly.

***

#### Version-aware review

The project pins TypeScript 5.6.x. Before flagging code as wrong because of an API or compiler option, check it against 5.6:

- **Do not suggest anything from 5.7 or later** (see "Do not use or recommend"). For example, `--rewriteRelativeImportExtensions`, `--target es2024`, `--erasableSyntaxOnly`, `--module node18`/`node20`, `import defer`, generic `Uint8Array<ArrayBuffer>` and `Promise.try` type declarations do not exist in 5.6.
- **Say "not available in TypeScript 5.6" only when 5.6 really lacks it.** Iterator helpers, `IteratorObject`, `BuiltinIteratorReturn`, `--noCheck`, `--noUncheckedSideEffectImports`, `--strictBuiltinIteratorReturn`, `--stopBuildOnErrors` and string-literal export names *are* in 5.6. Inferred type predicates, `isolatedDeclarations`, `${configDir}`, regex literal checking, JSDoc `@import` and `Set` method types (5.5), `NoInfer` and `module: preserve` (5.4), import attributes (5.3), `using` (5.2), standard decorators, `const` type parameters and `verbatimModuleSyntax` (5.0) are all available.
- **Distinguish "TypeScript accepts it" from "the runtime supports it".** A `lib` entry or a `target` setting that is newer than the deployment runtime is a real bug; flag it with the concrete runtime (Node major, browser baseline).
- **Distinguish compiler errors from lint findings.** `no-floating-promises`, `strict-boolean-expressions`, `no-unnecessary-condition` and similar are typescript-eslint rules, not `tsc` diagnostics.
- **Respect the project's flags.** Code that is fine under `strict: false` may be wrong under `strict: true` and vice versa; read `tsconfig.json` (including `extends` chains) before judging nullability, `any` or index-access issues.

***

#### What's new in TypeScript 5.6 (vs. 5.5)

| Change | Kind | Section |
|---|---|---|
| Errors for always-truthy / always-nullish checks (TS2869, TS2871, TS2872, TS2873) | new errors, always on | Disallowed nullish and truthy checks |
| Iterator helper methods typed in `lib.esnext.iterator`; runtime `Iterator` value; `IteratorObject`, `AsyncIteratorObject`, `ArrayIterator`, `MapIterator`, `SetIterator`, `StringIterator`, `RegExpStringIterator` | lib | Iterator helpers |
| `BuiltinIteratorReturn` intrinsic and `--strictBuiltinIteratorReturn` (part of `strict`) | new strict flag | Strict builtin iterator checks |
| String literals as import/export names | syntax | Arbitrary module identifiers |
| `--noUncheckedSideEffectImports` | new flag (off by default) | Side-effect imports |
| `--noCheck` | new flag | `--noCheck` |
| `tsc -b` builds downstream projects despite upstream errors; `--stopBuildOnErrors`; `.tsbuildinfo` always written in `--build` | behaviour change | `--build` with intermediate errors |
| `package.json` `"type"` and `.mts`/`.cts` respected for files in `node_modules` in all module modes; `.mts` never emits CommonJS, `.cts` never emits ESM | behaviour change | Module format detection |
| `override` / `noImplicitOverride` checked on computed property names | new errors | `override` on computed members |
| Region-prioritized diagnostics, granular commit characters, `autoImportSpecifierExcludeRegexes` | editor only | Editor features |
| DOM `lib.d.ts` updates | lib | Migration notes |

***

#### New in 5.6: Disallowed nullish and truthy checks

TypeScript 5.6 reports an error whenever it can tell *from the syntax alone* that a condition or the left operand of `??` always has the same truthiness or nullishness.[^1][^11] These are errors, not suggestions, and no flag turns them off (they apply with `strict: false` too). They catch real bugs:

```ts
declare const x: number;
declare const options: { max: number };

if (/0x[0-9a-f]/) {}            // error TS2872: This kind of expression is always truthy. (forgot .test())
if (x => 0) {}                  // error TS2872 (arrow function instead of >=) -- also TS7006 implicit any

export function isValid(value: number) {
  return value < options.max ?? 100; // error TS2869: Right operand of ?? is unreachable because the left operand is never nullish.
}
```

What is flagged (verified on 5.6.3):

```ts
declare let s: string | undefined;
declare let t: string;
declare function check(v: unknown, mode: string): boolean;

if ({}) {}                      // error TS2872 always truthy (object literal)
if ([]) {}                      // error TS2872 always truthy (array literal)
if (function () {}) {}          // error TS2872 always truthy (function expression)
if (!/re/) {}                   // error TS2872 (operand of ! is always truthy)
if (check(s, "loose" || "x")) {} // error TS2872 misplaced parenthesis: "loose" is always truthy
const a = "" || "fallback";     // error TS2873: This kind of expression is always falsy.
const b = null ?? t;            // error TS2871: This expression is always nullish.
const c = (t + 1) ?? 0;         // error TS2869 arithmetic never yields null/undefined
const d = `x${t}` ?? "y";       // error TS2869 template literal
const e = !s ?? true;           // error TS2869 ! yields a boolean
const f = typeof s ?? "x";      // error TS2869 typeof yields a string
const g = s === t ?? false;     // error TS2869 precedence bug: (s === t) ?? false
const h = class {} ?? 1;        // error TS2869 class expression
```

What is **not** flagged:

```ts
declare let s: string | undefined;
declare let t: string;
declare function getUser(): { name: string } | undefined;

while (true) { break; }   // true, false, 0 and 1 stay allowed (idiomatic loops, debugging toggles)
if (false || s) {}
if (0) {}
const a = t ?? "x";       // no error: the check is syntactic, not type-based ('t' is never nullish by type)
const b = new Date() ?? 1;                 // no error: 'new' expressions are not checked
const c = (s as string | null) ?? "d";     // no error
const d = `${t}` || "x";                   // no error: a template with substitutions may be empty
export { a, b, c, d, getUser };
```

Older, still-active checks complement it: uncalled functions in conditions (`if (getUser)`, TS2774, `strictNullChecks` only) and un-awaited promises in conditions (`if (promise)`, TS2801). For type-based "unnecessary condition" findings (`t ?? "x"` above), use typescript-eslint's `no-unnecessary-condition`.

Review notes:

- When an upgrade to 5.6 produces TS2869/TS2872/TS2873, the fix is almost always a **real bug fix**, not a cast. Do not accept `as any` or `// @ts-expect-error` to silence it.
- `void 0 ?? x` is reported with the misleading message "left operand is never nullish" (TS2869); the code is still dead and should be removed.
- The check is syntactic, so it does not replace `strict-boolean-expressions` / `no-unnecessary-condition`.

***

#### New in 5.6: Iterator helpers and `IteratorObject`

ECMAScript iterator helpers (TC39 Stage 4, part of ES2025) add `map`, `filter`, `take`, `drop`, `flatMap`, `reduce`, `toArray`, `forEach`, `some`, `every` and `find` to built-in iterators, a global `Iterator` constructor that can be subclassed, and `Iterator.from()`.[^12] TypeScript 5.6 declares them in `lib.esnext.iterator.d.ts` — so you need `lib: ["esnext"]` (or `"esnext.iterator"`); there is no `es2025` lib in 5.6.[^1]

```ts
// @run
function* positiveIntegers() {
  let i = 1;
  while (true) yield i++;
}

// Lazy: nothing runs until take(5) is consumed.
const evens = positiveIntegers().map((x) => x * 2);
console.log([...evens.take(5)]);                        // [ 2, 4, 6, 8, 10 ]

function invert<K, V>(map: Map<K, V>): Map<V, K> {
  return new Map(map.entries().map(([k, v]) => [v, k] as const));
}
console.log(invert(new Map([["a", 1]])));                // Map(1) { 1 => 'a' }

console.log([1, 2, 3].values().filter((n) => n % 2 === 1).reduce((a, b) => a + b, 0)); // 4

class Zeroes extends Iterator<number> {                  // the runtime Iterator class
  next() { return { value: 0, done: false } as const; }
}
console.log(new Zeroes().map((z) => z + 1).take(3).toArray()); // [ 1, 1, 1 ]

const wrapped = Iterator.from({ next: () => ({ value: "x", done: false as const }) });
console.log(wrapped.take(2).toArray());                  // [ 'x', 'x' ]
```

The name `Iterator` already meant the *protocol* type (anything with `next()`), so 5.6 adds **`IteratorObject<T, TReturn = unknown, TNext = unknown>`** for objects that really inherit from `%IteratorPrototype%` and therefore have the helper methods, plus **`AsyncIteratorObject`** for parity (async iterator helpers are still a proposal; `AsyncIteratorObject` has no helper methods). Lib methods now return specific subtypes: `ArrayIterator<T>`, `MapIterator<T>`, `SetIterator<T>`, `StringIterator<T>`, `RegExpStringIterator<T>`; generators return `Generator<…>`, which extends `IteratorObject`.[^1][^13]

```ts
const it: ArrayIterator<number> = [1, 2].values();
const asObj: IteratorObject<number, BuiltinIteratorReturn, unknown> = it; // no error
const proto: Iterator<number> = it;                                       // no error: protocol type

declare const anyIterator: Iterator<number>;
anyIterator.map((n) => n);    // error TS2339: 'map' does not exist on the protocol type Iterator<number> (plus TS7006 for 'n')
const fixed = Iterator.from(anyIterator);
fixed.map((n) => n);          // no error: Iterator.from() wraps any iterator
export { asObj, proto };
```

**Runtime availability** — the types are there whenever `lib` includes `esnext`, but the methods exist only on engines that implement them. Node 22+ has iterator helpers; **Node 20 does not** (`positiveIntegers(...).map is not a function`). If the project still runs on Node 20 or older browsers, iterator helpers must not be used (or must be polyfilled, e.g. core-js).

##### Breaking change: hand-written `Map`/`Set` implementations

Because `Map#entries()` now returns `MapIterator<…>` (which, with `esnext` in `lib`, carries `map`, `filter`, `take` …), classes that `implements Map<K, V>` or `Set<T>` and declare `IterableIterator` return types stop compiling on 5.6:

```ts
// BAD on 5.6 with lib esnext: 'IterableIterator<[K, V]>' is missing ... map, filter, take, drop, and 9 more
export class CountingMap<K, V> implements Map<K, V> {
  private inner = new Map<K, V>();
  get size() { return this.inner.size; }
  readonly [Symbol.toStringTag] = "CountingMap";
  clear() { this.inner.clear(); }
  delete(k: K) { return this.inner.delete(k); }
  forEach(cb: (v: V, k: K, m: Map<K, V>) => void) { this.inner.forEach((v, k) => cb(v, k, this)); } // error TS2345
  get(k: K) { return this.inner.get(k); }
  has(k: K) { return this.inner.has(k); }
  set(k: K, v: V) { this.inner.set(k, v); return this; }
  *entries(): IterableIterator<[K, V]> { yield* this.inner.entries(); } // error TS2416
  *keys(): IterableIterator<K> { yield* this.inner.keys(); }            // error TS2416
  *values(): IterableIterator<V> { yield* this.inner.values(); }        // error TS2416
  [Symbol.iterator]() { return this.entries(); }                        // error TS2416
}
```

```ts
// GOOD: declare the lib's iterator types; generator objects satisfy them at runtime (Node 22+)
export class CountingMap<K, V> implements Map<K, V> {
  private inner = new Map<K, V>();
  get size() { return this.inner.size; }
  readonly [Symbol.toStringTag] = "CountingMap";
  clear() { this.inner.clear(); }
  delete(k: K) { return this.inner.delete(k); }
  forEach(cb: (v: V, k: K, m: Map<K, V>) => void) { this.inner.forEach((v, k) => cb(v, k, this)); }
  get(k: K) { return this.inner.get(k); }
  has(k: K) { return this.inner.has(k); }
  set(k: K, v: V) { this.inner.set(k, v); return this; }
  *entries(): MapIterator<[K, V]> { yield* this.inner.entries(); }
  *keys(): MapIterator<K> { yield* this.inner.keys(); }
  *values(): MapIterator<V> { yield* this.inner.values(); }
  [Symbol.iterator]() { return this.entries(); }
}
```

Prefer composition (wrap a `Map`, expose only what you need) over re-implementing the full `Map` interface. With `lib` limited to `es2022` the old code still compiles, because the helper methods are not declared.

##### `@types/node` needs 5.6 now

Current `@types/node` releases (for example 26.x) reference `IteratorObject`, `AsyncIteratorObject` and `BuiltinIteratorReturn` and ship a `typesVersions` mapping (`"<=5.6": ["ts5.6/*"]`) for older compilers. On TypeScript 5.5 they fail with `Cannot find name 'IteratorObject'` / `'BuiltinIteratorReturn'`; on 5.6 they compile. Conversely, a 5.6 project cannot assume that every future `@types/*` release keeps working: DefinitelyTyped only tests packages on TypeScript versions that are less than two years old, and 5.6 left that window in September 2026.[^14] Pin `@types/*` versions in lockfiles and treat `@types` bumps as compiler-compatibility changes.

***

#### New in 5.6: `--strictBuiltinIteratorReturn`

`IteratorResult<T, TReturn = any>` made `.value` effectively `any` once an iterator was done. 5.6 types the built-in iterators as `IteratorObject<T, BuiltinIteratorReturn, …>`, where the intrinsic `BuiltinIteratorReturn` is `any` by default and **`undefined` under `--strictBuiltinIteratorReturn`**, which is part of the `strict` family.[^1][^15] Enabling `strict` on 5.6 therefore turns on new errors:

```ts
const m = new Map<string, number>([["a", 1]]);

const r = m.values().next();
const n: number = r.value;              // error TS2322: 'number | undefined' is not assignable to 'number'

const first: number = [1, 2, 3][Symbol.iterator]().next().value; // error TS2322 (array may be empty)

// GOOD: check `done` before reading `value`
const r2 = m.values().next();
if (!r2.done) {
  const k: number = r2.value;           // no error: narrowed to the yield result
  console.log(k);
}

// BEWARE: destructuring an iterable is NOT covered by the flag. 'firstValue' is typed
// 'number' although it is undefined for an empty Map (noUncheckedIndexedAccess makes it number | undefined).
const [firstValue] = m.values();
const maybeFirst: number | undefined = firstValue; // annotate explicitly when emptiness is possible
console.log(maybeFirst ?? 0, n, first);
```

```ts
function* uppercase(iter: Iterator<string, BuiltinIteratorReturn>) {
  while (true) {
    const { value, done } = iter.next();
    yield value.toUpperCase();          // error TS18048: 'value' is possibly 'undefined' (done not checked first)
    if (done) return;
  }
}
export { uppercase };
```

Review notes:

- In your own iterator types, **spell out `TReturn`** (`Iterator<T, void>`, `Generator<T, R, N>`) instead of relying on the `any` default of `Iterator<T>`/`IteratorResult<T>`.
- Turning the flag off (`"strictBuiltinIteratorReturn": false`) is a legitimate temporary migration step for large code bases; it should be tracked, not permanent.
- On 5.5 the names `BuiltinIteratorReturn` and `IteratorObject` do not exist; libraries whose public `.d.ts` mention them require consumers on 5.6+.

***

#### New in 5.6: Arbitrary module identifiers

ES2022 allows string literals as import/export names. TypeScript 5.6 parses, checks and emits them (5.5 reports `Identifier expected`).[^1][^16] Use them for interop with generated code, WebAssembly or tools such as esbuild's `inject`, not as a style:

```ts
// @skip: two-module example, verified as separate files on 5.6.3 and 5.5.4
// fruit.ts
const banana = "🍌";
export { banana as "🍌", banana as "my-fruit" };

// main.ts
import { "🍌" as banana, "my-fruit" as fruit } from "./fruit.js";
export { "🍌" as "also banana" } from "./fruit.js";
console.log(banana, fruit);
```

- A string name must be renamed on import: `import { "x-y" } from "./m.js"` is an error (`Identifier expected`).
- With `module: commonjs` the names emit as `exports["my-fruit"] = …`; with ESM output they are emitted as written, so the runtime must support ES2022 module syntax.

***

#### New in 5.6: `--noUncheckedSideEffectImports`

Before 5.6 (and by default in 5.6), a side-effect import `import "x"` that cannot be resolved is **silently ignored**. The new flag makes it an error (TS2307).[^1][^17]

```ts
// @skip: verified with a project on 5.6.3 (option unknown on 5.5.4: TS5023)
import "./polyfils.js";          // typo: silently ignored without the flag, TS2307 with it
import "./button.css";           // TS2307 with the flag unless an ambient declaration exists
```

```ts
// @skip: ambient declaration file
// src/globals.d.ts — declare asset modules the bundler handles
declare module "*.css" {}
```

Recommendation: enable it in new 5.6 projects. It becomes the default in TypeScript 6.0.[^9] Note that a `.ts` file imported by side effect must resolve under the project's resolution rules (`./polyfill.js` under `nodenext` for a `polyfill.ts` file).

***

#### New in 5.6: `--noCheck`

`--noCheck` skips semantic type checking for all input files and still emits JavaScript (and declarations).[^1] It is a CLI flag *and* a tsconfig option in 5.6; 5.5 rejects it (TS5023).

```bash
# Emit fast while iterating, type-check separately (possibly in parallel):
tsc --noCheck              # emits even if there are type errors
tsc --noEmit               # the real gate in CI
```

Review notes:

- **A build script with `--noCheck` and no separate `tsc --noEmit` (or `tsc -b`) step ships code that was never type-checked.** Verify the CI pipeline still has a checking step.
- Without `isolatedDeclarations`, declaration emit under `--noCheck` still computes types of unannotated exports (lazily); with `isolatedDeclarations` it is purely syntactic.
- When running `--noCheck` and a checking build concurrently in watch mode, give them separate `--tsBuildInfoFile` paths.

***

#### New in 5.6: `--build` continues past errors

Before 5.6, `tsc -b` behaved as if `noEmitOnError` were set and stopped at the first project with errors. In 5.6 it **reports errors, emits outputs on a best-effort basis, and continues building downstream projects**; the exit code is still non-zero.[^1][^18] Verified on 5.6.3 with project `b` referencing composite project `a` that has a type error:

| | 5.5.4 | 5.6.3 | 5.6.3 `--stopBuildOnErrors` |
|---|---|---|---|
| errors reported | `a` only | `a` and `b` | `a` only |
| `a/out/index.js` emitted | no | **yes** | **yes** |
| `b` built | no | yes | no |
| `b/out/tsconfig.tsbuildinfo` (non-composite, non-incremental) | – | **written** | – |
| exit code | 1 | 1 | 1 |

- **The flag is spelled `--stopBuildOnErrors`.** The 5.6 release notes call it `--stopOnBuildErrors`, which 5.6.3 rejects (`Unknown build option '--stopOnBuildErrors'. Did you mean 'stopBuildOnErrors'?`).
- Erroneous outputs now land in `outDir` even in `--build` mode. If deployment scripts publish `dist/` after a failed build, set `"noEmitOnError": true` in the referenced projects and make sure CI fails on the exit code.
- `.tsbuildinfo` files are written for every project in a `--build` invocation, even without `incremental`/`composite`; add them to `.gitignore` and to clean scripts.

***

#### New in 5.6: Module format detection in `node_modules`

TypeScript 4.7 began reading `.mjs`/`.cjs`/`.mts`/`.cts` extensions and the `package.json` `"type"` field, but only under `module: node16`/`nodenext`. 5.6 uses this format information **in all module modes except `amd`, `umd` and `system`** for files in `node_modules`, and `.mts`/`.cts` files are respected everywhere.[^1][^19]

Effect 1 — unsafe default imports of ESM-only packages are now caught under `module: esnext` + `moduleResolution: bundler`:

```ts
// @skip: verified with node_modules/dep {"type":"module"} and index.d.ts `export declare function doSomething(): void;`
import dep from "dep";        // 5.5.4: no error; 5.6.3: error TS1192: Module '".../dep/index"' has no default export.
dep.doSomething();

import { doSomething } from "dep"; // GOOD
doSomething();
```

Effect 2 — `.mts` files never emit CommonJS and `.cts` files never emit ESM. `tsc --module commonjs main.mts` emitted `exports.default = …` into `main.mjs` up to 5.5.4; 5.6.3 emits `export default …`.

***

#### New in 5.6: `override` checks on computed members

```ts
// @opts: {"noImplicitOverride": true}
const foo = Symbol("foo");
const bar = Symbol("bar");

class Base {
  [bar]() {}
}

export class Derived extends Base {
  override [foo]() {}   // error TS4113: cannot have an 'override' modifier because it is not declared in the base class
  [bar]() {}            // error TS4114: must have an 'override' modifier (noImplicitOverride)
}
```

Both lines compiled without error on 5.5.[^1]

***

#### New in 5.6: Editor features

These affect editors only, not `tsc` output:[^1]

- **Region-prioritized diagnostics**: the language server can check the visible region of a large file first (VS Code with TypeScript 5.6+).
- **Granular commit characters** for completions.
- **Auto-import exclusions**: `"typescript.preferences.autoImportSpecifierExcludeRegexes": ["^lodash/.*$", "^node:"]` (and `javascript.preferences…`) in VS Code settings. Useful to stop auto-imports from deep or internal paths; it is not a substitute for an ESLint `no-restricted-imports` rule.

***

#### Do not use or recommend (not in TypeScript 5.6)

The project's compiler is 5.6. None of the following exists in it; never suggest them, and flag them if they appear in code or config (they will fail to compile, or silently do nothing in editors that bundle a newer TypeScript):[^5][^6][^7][^9][^10]

**TypeScript 5.7** (November 2024)
- `--rewriteRelativeImportExtensions` (5.6: `Unknown compiler option`). In 5.6, `.ts` import specifiers require `allowImportingTsExtensions`, which in turn requires `noEmit` or `emitDeclarationOnly`.
- `--target es2024` / `--lib es2024` (5.6 accepts targets up to `es2023` plus `esnext`; `Promise.withResolvers`, `Object.groupBy`, `Map.groupBy` and `String#isWellFormed` live in `esnext.*` libs in 5.6).
- Generic typed arrays: `Uint8Array<ArrayBuffer>`, `Buffer<ArrayBuffer>` (5.6: `Type 'Uint8Array' is not generic`); `ArrayBuffer#transfer`, resizable `ArrayBuffer` and growable `SharedArrayBuffer` declarations.
- The "never-initialized variable" check across closures (5.6 does not report `let x: number; function f() { use(x) }` when `x` is never assigned).
- Required `with { type: "json" }` for JSON imports under `module: nodenext` (5.6 compiles the import without the attribute, which then fails in Node with `ERR_IMPORT_ATTRIBUTE_MISSING` — see Modules).
- `module.enableCompileCache()` use by `tsc` itself, and the editor search for ancestor `tsconfig.json` project ownership.

**TypeScript 5.8** (February 2025)
- `--erasableSyntaxOnly`, `--module node18`, `--libReplacement`, `require()` of ESM under `--module nodenext`.
- Per-branch checking of conditional expressions in `return` statements (5.6 does not report `return cond ? anyValue : wrongType` when one branch is `any`).
- `Float16Array`, `Math.f16round`, `Promise.try` declarations.

**TypeScript 5.9** (July 2025)
- `import defer * as ns from "…"`, `--module node20`, the minimal `tsc --init` template.
- `Error.isError`, `Atomics.pause` declarations; expandable hovers.

**TypeScript 6.0** (March 2026) — the last JavaScript-based compiler
- `--target es2025` / `--lib es2025`, `--stableTypeOrdering`, `#/` subpath-import support, `--moduleResolution bundler` with `--module commonjs`.
- Types for `Temporal`, `Map#getOrInsert`/`getOrInsertComputed`, `RegExp.escape`, `Uint8Array#toBase64`/`fromBase64`.
- New defaults (`strict: true`, `module: esnext`, floating `target`, `noUncheckedSideEffectImports: true`, `types: []`, `rootDir: "."`) — **in 5.6 none of these are defaults**; write them explicitly.

**TypeScript 7.0** (July 2026)
- The native (Go) compiler. Options deprecated in 6.0 are removed in 7.0 (see Future-proofing). Do not suggest `tsgo` or `@typescript/native-preview` as a drop-in for a 5.6 project without a deliberate upgrade.

##### Deprecated or removed as of 5.6

- **Removed in 5.5** (deprecated in 5.0): `charset`, `target: "ES3"`, `importsNotUsedAsValues`, `preserveValueImports` (use `verbatimModuleSyntax`), `noImplicitUseStrict`, `noStrictGenericChecks`, `keyofStringsOnly`, `suppressExcessPropertyErrors`, `suppressImplicitAnyIndexErrors`, `out` (use `outFile`), `prepend` in project references, implicitly OS-specific `newLine`.[^4] On 5.6 setting any of these options is a hard configuration error (the `newLine` item only means the default is now LF) (`error TS5102: Option 'charset' has been removed. Please remove it from your configuration.`, `TS5108` for `target: "ES3"`); a tsconfig that still contains them does not build.
- `ignoreDeprecations: "5.0"` no longer silences them (verified: TS5102 is still reported).
- Legacy `experimentalDecorators` still works but is not the standard; new code should use standard decorators (see Decorators).

##### Future-proofing (works in 5.6, removed in 7.0)

TypeScript 6.0 deprecates and 7.0 removes: `target: "es5"`, `--downlevelIteration`, `moduleResolution: "node"`/`"node10"` and `"classic"`, `module: "amd" | "umd" | "system"`, `baseUrl`, `outFile`, `esModuleInterop: false` / `allowSyntheticDefaultImports: false`, `alwaysStrict: false`, the `module Foo {}` namespace keyword (use `namespace`), import `assert { … }` (use `with { … }`), and `/// <reference no-default-lib="true"/>`.[^9] New 5.6 code and configs should avoid them so a future upgrade is mechanical.

***

#### Project configuration (tsconfig.json)

##### Defaults in 5.6 you must override

| Option | 5.6 default | Recommendation |
|---|---|---|
| `strict` | `false` | `true` |
| `target` | `es5`; **`esnext` when `module` is `node16`/`nodenext`** | match the oldest runtime: `es2022` (Node 18/20 era) or `es2023` (Node 22+) |
| `module` | `commonjs` when `target` is `es5`, otherwise `es2015` | `nodenext` for Node, `preserve` or `esnext` + `moduleResolution: bundler` for bundlers |
| `lib` | derived from `target` (+ `dom`) | set explicitly; add `esnext.*` pieces only for APIs the runtime has |
| `useDefineForClassFields` | `true` when `target` ≥ `es2022` | leave it; understand the semantics (see Classes) |
| `noUncheckedSideEffectImports` | `false` | `true` |
| `types` | all `node_modules/@types/*` | list them explicitly (`["node"]`) to avoid ambient leakage |

Option semantics are documented in the TSConfig reference.[^24] `tsc --init` in 5.6 still writes the long commented template with `target: es2016`, `module: commonjs`, `esModuleInterop`, `forceConsistentCasingInFileNames`, `strict` and `skipLibCheck` enabled — a starting point, not a recommendation for a modern Node or bundler project.

##### Node.js application or library compiled by `tsc` (ESM)

```json
{
  "compilerOptions": {
    "target": "es2023",
    "lib": ["es2023", "esnext.iterator", "esnext.collection", "esnext.promise", "esnext.object", "esnext.array"],
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
    "noUncheckedSideEffectImports": true,

    "verbatimModuleSyntax": true,
    "isolatedModules": true,
    "forceConsistentCasingInFileNames": true,
    "skipLibCheck": true,

    "declaration": true,
    "declarationMap": true,
    "sourceMap": true
  },
  "include": ["src"]
}
```

- The `lib` list above matches **Node 22+** (iterator helpers, Set methods, `Map.groupBy`/`Object.groupBy`, `Promise.withResolvers`, `Array.fromAsync` all exist there). `lib: ["esnext"]` additionally declares `DisposableStack`, `AsyncDisposableStack` and `SuppressedError`, which Node 22 does not have (Node 24 does) — code using them typechecks and then throws `ReferenceError` at runtime.
- `module: nodenext` implies `moduleResolution: nodenext`, `esModuleInterop`, `moduleDetection: force` and — easy to miss — **`target: esnext`** (see `tsc --showConfig`). A `nodenext` project without an explicit `target` therefore emits `using` declarations and decorators verbatim, which crash on Node 22 (`using`) or on every current Node (decorators). Always set `target`.
- Do not combine `module: nodenext` with `moduleResolution: node10`/`bundler` (error TS5109).
- With `"type": "module"` in `package.json`, `.ts` files compile to ESM; without it, to CommonJS. `.mts` / `.cts` force the format.

##### Bundler-based front end (Vite, esbuild, webpack, Rollup)

```json
{
  "compilerOptions": {
    "target": "es2022",
    "lib": ["es2023", "dom", "dom.iterable"],
    "module": "preserve",
    "moduleResolution": "bundler",
    "noEmit": true,
    "allowImportingTsExtensions": true,
    "jsx": "react-jsx",

    "strict": true,
    "noUncheckedIndexedAccess": true,
    "noImplicitOverride": true,
    "noFallthroughCasesInSwitch": true,
    "noUncheckedSideEffectImports": true,
    "verbatimModuleSyntax": true,
    "isolatedModules": true,
    "skipLibCheck": true
  },
  "include": ["src"]
}
```

- `module: preserve` (5.4+) implies `moduleResolution: bundler`, `esModuleInterop` and `resolveJsonModule`, and leaves `import`/`export`/`require` as written.
- `allowImportingTsExtensions` requires `noEmit` or `emitDeclarationOnly` (TS5096). The bundler, not `tsc`, produces JavaScript.
- The bundler's transpiler (esbuild, SWC, Babel) compiles files one by one: `isolatedModules` (implied by `verbatimModuleSyntax`) makes `tsc` reject constructs those tools cannot handle.
- Add a `*.css`/asset ambient module declaration (`declare module "*.css" {}`) when enabling `noUncheckedSideEffectImports`.

##### Shared base configs: `${configDir}` (5.5+)

Paths in a tsconfig are relative to the file that contains them. In a shared base config, use `${configDir}` so `outDir`, `rootDir`, `typeRoots`, `paths`, `include` and similar resolve relative to the *extending* project:[^4]

```json
{
  "compilerOptions": {
    "outDir": "${configDir}/dist",
    "rootDir": "${configDir}/src",
    "tsBuildInfoFile": "${configDir}/.cache/tsbuildinfo"
  },
  "include": ["${configDir}/src"]
}
```

`extends` accepts an array (5.0+); later entries override earlier ones.

##### Strictness flags

`strict: true` in 5.6 enables exactly these nine flags: `alwaysStrict`, `noImplicitAny`, `noImplicitThis`, `strictBindCallApply`, `strictBuiltinIteratorReturn` (**new in 5.6**), `strictFunctionTypes`, `strictNullChecks`, `strictPropertyInitialization`, `useUnknownInCatchVariables`. New strict-family flags are added in minor releases, so upgrading TypeScript with `strict: true` can add errors — that is intended.

Flags outside `strict` worth enabling (each verified on 5.6.3):

```ts
// @opts: {"noUncheckedIndexedAccess": true, "exactOptionalPropertyTypes": true, "noPropertyAccessFromIndexSignature": true}
const scores: Record<string, number> = {};
const s: number = scores["alice"];        // error TS2322 under noUncheckedIndexedAccess: number | undefined
const arr = [1, 2, 3];
const firstItem: number = arr[0];         // error TS2322 (index access may be out of range)
for (const v of arr) console.log(v + 1);  // no error: iteration is not index access

interface Options { timeoutMs?: number }
const o1: Options = {};
const o2: Options = { timeoutMs: undefined }; // error TS2375 under exactOptionalPropertyTypes
interface Env { [key: string]: string }
declare const env: Env;
console.log(env.HOME);                    // error TS4111 under noPropertyAccessFromIndexSignature: use env["HOME"]
export { s, firstItem, o1, o2 };
```

- `noUncheckedIndexedAccess`: catches the most common runtime `undefined` bugs (array index, record lookup). Strongly recommended for new code.
- `exactOptionalPropertyTypes`: distinguishes "missing" from "present but `undefined`" — matters for `Object.keys`, `in`, spreading defaults (`{...defaults, ...opts}` where `opts.timeoutMs` is `undefined` overwrites the default).
- `noImplicitOverride`, `noImplicitReturns`, `noFallthroughCasesInSwitch`: cheap, catch real mistakes.
- `noUnusedLocals`/`noUnusedParameters`: usually better handled by ESLint (warnings instead of build failures).
- `skipLibCheck: true`: speeds up builds and isolates you from broken third-party `.d.ts`; it also hides conflicts between duplicate `@types` versions. Libraries should run at least one CI job with `skipLibCheck: false` against their own emitted declarations.

***

#### Type system essentials

##### `any`, `unknown`, `never`

- `any` switches the checker off for everything it touches and spreads silently (`any.foo.bar()` is `any`). Treat every `any` in new code as a defect unless it is at a well-documented boundary. `noImplicitAny` only catches *implicit* `any`.
- `unknown` is the type-safe top type: you must narrow before use. Use it for parsed JSON, `catch` variables (default under `strict` via `useUnknownInCatchVariables`) and untrusted input.
- `never` is the empty type: the return type of functions that always throw, and the type left after exhaustive narrowing.

```ts
// BAD: JSON.parse returns any; the annotation is an unchecked claim
interface User { id: string; name: string }
const u: User = JSON.parse('{"id": 1}');          // no error, wrong at runtime
console.log(u.id.toUpperCase());                  // TypeError at runtime: u.id is a number

// GOOD: parse into unknown and validate
function isUser(v: unknown): v is User {
  return typeof v === "object" && v !== null
    && "id" in v && typeof v.id === "string"
    && "name" in v && typeof v.name === "string";
}
const raw: unknown = JSON.parse('{"id": "1", "name": "a"}');
if (isUser(raw)) console.log(raw.name);
```

`"id" in v` narrows `v` to `object & Record<"id", unknown>` (4.9+), so `v.id` is accessible without casts. For anything bigger than a few fields, use a schema validator (Zod, Valibot, ArkType, TypeBox/Ajv) and derive the static type from the schema so the two cannot drift.

##### Structural typing and excess property checks

Excess property checks only apply to **fresh object literals** assigned directly to a typed target. They do not apply once the object goes through a variable, and they never apply at runtime:

```ts
interface Point { x: number; y: number }
const p1: Point = { x: 1, y: 2, z: 3 };          // error TS2353: 'z' does not exist in type 'Point'
const tmp = { x: 1, y: 2, z: 3 };
const p2: Point = tmp;                            // no error: not a fresh literal
function save(p: Point) { return JSON.stringify(p); }
console.log(save(p2));                            // {"x":1,"y":2,"z":3} -- extra data leaks into output
export { p1 };
```

Review consequence: when an object with extra fields is serialized, logged or written to a database, copy the allowed fields explicitly (or validate with a strict schema). Types do not strip properties.

**Branded (nominal) types** prevent mixing structurally identical primitives:

```ts
type UserId = string & { readonly __brand: "UserId" };
type OrderId = string & { readonly __brand: "OrderId" };
const asUserId = (s: string) => s as UserId;     // the single place that asserts
declare function loadUser(id: UserId): void;
const order = "o-1" as OrderId;
loadUser(asUserId("u-1"));
loadUser(order);                                 // error TS2345: OrderId is not assignable to UserId
```

##### `object`, `{}`, `Object`

- `{}` means "any non-nullish value" — `const x: {} = 42` compiles. Do not use it for "an object".
- `object` means any non-primitive. `Record<string, unknown>` means an object with unknown values that can be indexed.
- `Object` (capital) is almost never right.

```ts
const a: {} = 42;                     // no error
const b: object = 42;                 // error TS2322
const c: Record<string, unknown> = { k: 1 };
function f<T extends {}>(v: T) { return v; } // "NonNullable" constraint idiom
f(null);                              // error TS2345
export { a, b, c };
```

##### `interface` vs. `type`

Both describe object shapes. Prefer `interface` for object types that are extended (better error messages, cached relations, `extends` checks conflicts) and `type` for unions, intersections, mapped/conditional types, tuples and function types. Be aware that interfaces **merge**: two `interface Window` declarations in the same scope combine, which is the intended way to augment globals/modules — and a source of surprises when it happens by accident.

```ts
interface Config { port: number }
interface Config { host: string }               // merged: Config has port and host
const cfg: Config = { port: 1 };                // error TS2741: Property 'host' is missing

type A = { x: number } & { x: string };         // intersection: x is never
declare const a: A;
const n: number = a.x;                          // no error: never is assignable to everything -- a bug magnet
export { cfg, n };
```

##### Literal types, `as const` and `satisfies`

```ts
type Method = "GET" | "POST";
declare function request(url: string, method: Method): void;

const opts = { url: "/api", method: "GET" };
request(opts.url, opts.method);                 // error TS2345: string is not assignable to Method

const opts2 = { url: "/api", method: "GET" } as const;
request(opts2.url, opts2.method);               // no error: method is "GET"

// satisfies (4.9+): validate against a type without widening the inferred type
type Routes = Record<string, { method: Method; auth: boolean }>;
const routes = {
  health: { method: "GET", auth: false },
  login: { method: "POST", auth: false },
  admin: { method: "GET", auth: true, extra: 1 },  // error TS2353: excess property is still checked
} satisfies Routes;
routes.health.method;                           // type "GET" (an annotation ": Routes" would widen to Method)
routes.unknownRoute;                            // error TS2339: keys are preserved, typos are caught
```

Prefer `satisfies` over `as` to check a literal: `as` silences missing-property errors (`{} as Config` compiles), `satisfies` does not.

##### Enums

```ts
enum Status { Active, Disabled }                 // numeric: reverse mapping, Status[0] === "Active"
enum Color { Red = "RED", Green = "GREEN" }     // string: no reverse mapping

const s: Status = 7;                            // error TS2322 (5.0+: out-of-range literal rejected)
declare const n: number;
const s2: Status = n;                           // no error: arbitrary numbers still assignable
const c: Color = "RED";                         // error TS2322: string enums are nominal
console.log(Object.keys(Status));               // [ '0', '1', 'Active', 'Disabled' ] -- reverse mapping keys
export { s, s2, c };
```

- Numeric enums are unsound (any `number` is assignable) and their `Object.keys/values` include reverse-mapping entries. Prefer string enums or, better, a `const` object plus a union type:

```ts
export const Status = { Active: "active", Disabled: "disabled" } as const;
export type Status = (typeof Status)[keyof typeof Status];   // "active" | "disabled"
export function isStatus(v: string): v is Status {
  return (Object.values(Status) as string[]).includes(v);
}
```

- `const enum` is inlined by `tsc`, but single-file transpilers (esbuild, SWC, Babel, Node type stripping) cannot see other files' enums. Under `isolatedModules`, referencing an *ambient* `const enum` (from a `.d.ts`) is an error, and exporting `const enum`s from libraries is a hazard. Avoid `const enum` in code compiled by anything other than a full `tsc` program.
- Enums, namespaces with values, parameter properties and `import x = require()` are not "erasable" syntax: Node's built-in type stripping rejects them (see Modules). 5.6 has no `--erasableSyntaxOnly` to enforce this (5.8+).
- Enum members cannot have numeric names (`"1" = 2` is TS2452). The 5.4 release notes also announce errors for members named `Infinity`, `-Infinity` and `NaN`, but 5.4.5 through 5.6.3 accept them — do not rely on the compiler to reject them; avoid such names.

##### Tuples and arrays

```ts
type Range = [start: number, end?: number];       // labelled, optional element
type Args = [string, ...number[], boolean];       // leading/middle rest (4.2+)
const r: Range = [1];
const a: Args = ["x", 1, 2, true];

const xs: readonly number[] = [1, 2];
xs.push(3);                                        // error TS2339: 'push' does not exist on readonly number[]
function mutate(v: number[]) { v.push(0); }
mutate(xs);                                        // error TS2345: 'readonly number[]' is not assignable to 'number[]'

// Array covariance is unsound:
const dogs: { bark(): void }[] = [];
const animals: object[] = dogs;                    // no error
animals.push({});                                  // no error -- dogs now contains a non-dog
export { r, a };
```

`Readonly<T>` / `readonly` are shallow and compile-time only; use `Object.freeze` (also shallow) or immutable data structures when mutation must be prevented at runtime.

##### Index signatures, `Record`, `Object.keys`

- `Object.keys(obj)` returns `string[]`, not `(keyof T)[]`, because objects may have more keys at runtime than their type shows. Do not "fix" this with `as (keyof T)[]` without thinking about extra keys.
- `for (const k in obj)` also iterates inherited enumerable properties.
- `Record<string, V>` claims every string key exists; with `noUncheckedIndexedAccess` lookups become `V | undefined`. For dictionaries built from user input use `Map<string, V>` (no prototype keys, no `__proto__` issues).

```ts
const counts: Record<string, number> = {};
const key = "__proto__";
counts[key] = 1;                         // no error; hits the __proto__ setter (an object value would replace the prototype)
console.log(Object.keys(counts).length); // 0
const safe = new Map<string, number>([[key, 1]]);
console.log(safe.size);                  // 1
```

##### Utility types worth knowing

`Partial`, `Required`, `Readonly`, `Pick`, `Omit` (not key-checked: `Omit<T, "typo">` compiles), `Record`, `Exclude`, `Extract`, `NonNullable`, `ReturnType`, `Parameters`, `ConstructorParameters`, `InstanceType`, `Awaited` (4.5+), `NoInfer` (5.4+), `ThisType`, and the intrinsic string types `Uppercase`, `Lowercase`, `Capitalize`, `Uncapitalize`.

```ts
interface User { id: string; name: string; password: string }
type PublicUser = Omit<User, "pasword">;        // no error -- typo silently keeps 'password'!
type Strict<T, K extends keyof T> = Omit<T, K>; // key-checked wrapper
type PublicUser2 = Strict<User, "pasword">;     // error TS2344
type Resolved = Awaited<Promise<Promise<number>>>; // number
export type { PublicUser, PublicUser2, Resolved };
```

***

#### Narrowing and control flow

##### Built-in narrowing

```ts
function describe(v: string | number | null | undefined | Date | string[]) {
  if (v == null) return "nullish";                  // == null catches both null and undefined
  if (typeof v === "string") return v.toUpperCase();
  if (typeof v === "number") return v.toFixed(2);
  if (Array.isArray(v)) return v.join(",");
  return v.toISOString();                           // Date
}

// Pitfall: typeof null === "object"
function keys(v: object | null) {
  if (typeof v === "object") return Object.keys(v); // error TS2769: v is still object | null here
  return [];
}

// Pitfall: truthiness drops 0 and ""
function port(p?: number) {
  return p || 8080;     // BAD: 0 becomes 8080
}
function port2(p?: number) {
  return p ?? 8080;     // GOOD
}
export { describe, keys, port, port2 };
```

Other narrowing forms: `in` (`"kind" in v`), `instanceof` (including `Symbol.hasInstance`-aware narrowing, 5.3+), equality with literals, discriminant property checks, `switch (true)` with `typeof`/predicate cases (5.3+), and comparisons to `true`/`false` (5.3+).

`instanceof` fails across realms (iframes, `vm` contexts, duplicated package copies in `node_modules`); for errors from other modules prefer duck-typing on a `code`/`name` property.

##### Discriminated unions and exhaustiveness

```ts
type Shape =
  | { kind: "circle"; r: number }
  | { kind: "square"; side: number }
  | { kind: "rect"; w: number; h: number };

function assertNever(x: never): never {
  throw new Error(`Unhandled variant: ${JSON.stringify(x)}`);
}

function area(s: Shape): number {
  switch (s.kind) {
    case "circle": return Math.PI * s.r ** 2;
    case "square": return s.side ** 2;
    default: return assertNever(s);   // error TS2345: "rect" not handled -- add the case
  }
}
export { area };
```

Use `assertNever` (or `satisfies never`) in the `default` branch so adding a variant breaks the build. Without it, `noImplicitReturns` or an explicit return type catches only some cases. typescript-eslint's `switch-exhaustiveness-check` is the lint equivalent.

##### Inferred type predicates (5.5+)

Since 5.5, TypeScript infers `x is T` for functions whose single `return` expression narrows the parameter, e.g. `x => x !== undefined`.[^4]

```ts
const nums = [1, 2, null, 4].filter((x) => x !== null);   // number[] (was (number | null)[] before 5.5)
nums.push(null);                                           // error TS2345: the inferred type is narrower now

const ok: (number | null)[] = [1, 2, null].filter((x) => x !== null); // annotate when you need the wide type
ok.push(null);

declare const scores: Map<string, number>;
const vals = ["a", "b"].map((k) => scores.get(k));
const truthy = vals.filter((v) => !!v);                    // still (number | undefined)[]: 0 is falsy, no predicate inferred
const defined = vals.filter((v) => v !== undefined);       // number[]
console.log(truthy, defined);
```

- Truthiness filters (`filter(Boolean)`, `x => !!x`) do **not** infer a predicate for primitives because `0`/`""` would be dropped — and dropping them is usually a real bug in the code anyway.
- Inference requires: no explicit return type, a single `return`, no parameter mutation, and a boolean expression tied to the parameter.
- Explicit predicates (`v is User`) are **unchecked**: TypeScript does not verify that the body matches. Review every hand-written predicate like a type assertion.

```ts
// BAD: lying predicate compiles
function isString(v: unknown): v is string {
  return typeof v === "number";
}
```

##### Assertion functions

```ts
function assert(cond: unknown, msg: string): asserts cond {
  if (!cond) throw new Error(msg);
}
function assertIsDefined<T>(v: T, name: string): asserts v is NonNullable<T> {
  if (v == null) throw new Error(`${name} is required`);
}
declare const env: Record<string, string | undefined>;
const url = env["DATABASE_URL"];
assertIsDefined(url, "DATABASE_URL");
url.startsWith("postgres:");          // narrowed to string

const check = (c: unknown): asserts c => { if (!c) throw new Error(); };
// Assertion functions must be called through an explicitly typed reference:
check(url);                           // error TS2775: Assertions require every name in the call target to be declared with an explicit type annotation.
const inline: (c: unknown) => asserts c = check;
inline(url);
assert(url.length > 0, "empty");
```

##### Narrowing in closures and after `await`

5.4+ preserves narrowing inside closures created **after the last assignment** of a `let` or parameter:[^20]

```ts
function getUrls(url: string | URL, names: string[]) {
  if (typeof url === "string") url = new URL(url);
  return names.map((n) => {
    url.searchParams.set("lang", n);   // no error since 5.4: url is URL here
    return url.toString();
  });
}

function bad(x: string | number) {
  if (typeof x === "string") {
    setTimeout(() => x.toUpperCase());  // error TS2339: x is reassigned later, narrowing not kept
  }
  x = 1;
}
export { getUrls, bad };
```

Narrowing of **properties** is not invalidated by function calls or `await` — TypeScript assumes nothing else mutates the object:

```ts
interface Job { status: "queued" | "running" | "done"; result?: string }
declare function tick(): Promise<void>;
async function wait(job: Job) {
  if (job.status === "queued") {
    await tick();                       // another task may set job.status = "done" meanwhile
    if (job.status === "done") {}       // error TS2367: TS still believes status is "queued"
  }
}
export { wait };
```

When state can change across `await` or callbacks, re-read it into a fresh variable with a wide type (`const status: Job["status"] = job.status`) instead of casting.

##### Constant indexed access (5.5+)

`obj[key]` narrows when both `obj` and `key` are effectively constant:

```ts
function upper(obj: Record<string, unknown>, key: string) {
  if (typeof obj[key] === "string") return obj[key].toUpperCase(); // no error since 5.5
  return undefined;
}
export { upper };
```

##### Type assertions and non-null assertions

- `x as T` is allowed whenever `T` and `x`'s type overlap; it checks nothing at runtime. `x as unknown as T` bypasses even that.
- `x!` removes `null`/`undefined` without a check. Each `!` is a claim that needs a reason; prefer a guard, `??` with a sensible default, or an assertion function that throws a useful error.
- `<T>x` is the same as `as` (not allowed in `.tsx`).

```ts
interface Config { port: number; host: string }
const c1 = {} as Config;               // no error -- missing fields hidden
const c2: Config = { port: 1 };        // error TS2741: annotation catches it
const el = document.querySelector("#app")!;  // runtime crash later if #app is missing
export { c1, c2, el };
```

***

#### Generics

##### Constraints, defaults and inference

```ts
function pluck<T, K extends keyof T>(items: readonly T[], key: K): T[K][] {
  return items.map((i) => i[key]);
}
const names = pluck([{ id: 1, name: "a" }], "name");  // string[]
pluck([{ id: 1 }], "nmae");                           // error TS2345: "nmae" is not a key

// Generic defaults
interface Page<T, Cursor = string> { items: T[]; next?: Cursor }
const page: Page<number> = { items: [1], next: "c2" };

// Don't add a type parameter that is used only once -- it is just a disguised cast
function parseBad<T>(json: string): T { return JSON.parse(json); }   // BAD: caller picks T, nothing checks it
function parseGood(json: string): unknown { return JSON.parse(json); } // GOOD: force validation
export { names, page, parseBad, parseGood };
```

A type parameter that appears only in the return type (`parseBad<T>`) lets every caller assert any type without a cast; reviewers should treat it like `as`.

##### `const` type parameters (5.0+)

```ts
function routes<const T extends readonly string[]>(paths: T): T { return paths; }
const r = routes(["/a", "/b"]);       // readonly ["/a", "/b"] (without const: string[])

function routesMutable<const T extends string[]>(paths: T): T { return paths; }
const r2 = routesMutable(["/a"]);     // ["/a"] (mutable tuple) since 5.3; 5.0-5.2 inferred string[]
export { r, r2 };
```

##### `NoInfer<T>` (5.4+)

```ts
function createFSM<S extends string>(states: S[], initial: NoInfer<S>) { return { states, initial }; }
createFSM(["idle", "running"], "idle");
createFSM(["idle", "running"], "stopped");   // error TS2345: without NoInfer, S would widen to include "stopped"
```

##### Conditional, mapped and template literal types

```ts
type ElementOf<T> = T extends readonly (infer E)[] ? E : never;
type A = ElementOf<string[]>;                         // string

// Conditional types distribute over unions; wrap in [] to stop it
type ToArray<T> = T extends unknown ? T[] : never;
type B = ToArray<string | number>;                    // string[] | number[]
type ToArrayND<T> = [T] extends [unknown] ? T[] : never;
type C = ToArrayND<string | number>;                  // (string | number)[]

// Key remapping and template literal types
type Getters<T> = { [K in keyof T & string as `get${Capitalize<K>}`]: () => T[K] };
type G = Getters<{ name: string; age: number }>;      // { getName: () => string; getAge: () => number }

type EventName = `on${Capitalize<"click" | "focus">}`; // "onClick" | "onFocus"
const ev: EventName = "onclick";                      // error TS2820 (did you mean "onClick"?)
export type { A, B, C, G };
export { ev };
```

Keep type-level programming proportional: deeply recursive conditional types slow the checker and produce unreadable errors (TS2589 "Type instantiation is excessively deep" is a sign to simplify).

##### Variance annotations

`in`/`out` on type parameters (4.7+) document and enforce variance for interfaces and type aliases used in hot paths; they are checked against the structure. Function parameter types are checked contravariantly only for *function-typed properties*; **method syntax is bivariant** even under `strictFunctionTypes`:

```ts
interface Animal { name: string }
interface Dog extends Animal { bark(): void }

interface HandlerMethod { handle(a: Animal): void }           // method syntax: bivariant
interface HandlerProp { handle: (a: Animal) => void }         // property syntax: contravariant

const dogOnly = (d: Dog) => d.bark();
const m: HandlerMethod = { handle: dogOnly };                 // no error -- unsound
const p: HandlerProp = { handle: dogOnly };                   // error TS2322 under strictFunctionTypes
export { m, p };
```

Prefer property syntax (`handle: (a: Animal) => void`) in callback-carrying interfaces you own.

***

#### Functions

```ts
// Overloads: callers see only the overload signatures, not the implementation signature
function toDate(v: string): Date;
function toDate(v: number): Date;
function toDate(v: string | number): Date { return new Date(v); }
declare const sOrN: string | number;
toDate(sOrN);                       // error TS2769: no overload accepts the union -- add one or use a union parameter

// `this` parameter
function onClick(this: HTMLButtonElement, ev: MouseEvent) { console.log(this.id, ev.button); }
document.querySelector("button")?.addEventListener("click", onClick);

// Optional vs. default parameters
function greet(name = "world", punct?: string) { return `hi ${name}${punct ?? "!"}`; }
greet(undefined, ".");             // default applies for undefined, not for null
export { greet };
```

- Prefer a union parameter over overloads when the return type does not depend on the argument type.
- A function returning `void` in a callback type may return a value (`arr.forEach(x => list.push(x))` is fine); a declared `function f(): void { return 1 }` is an error. Since 5.1, a function whose return type is `undefined` needs no `return` statement.
- Callbacks typed `() => void` silently accept `async` functions — the returned promise is ignored. See Async.

***

#### Classes

##### Class fields and `useDefineForClassFields`

With `target` ≥ `es2022` (or explicit `useDefineForClassFields: true`), class fields use ECMAScript `[[Define]]` semantics: they are initialized **after `super()` returns** and a redeclared field **overwrites** what the base constructor set. TypeScript catches the common mistakes:

```ts
class Base {
  value: string;
  constructor() { this.value = "set by base"; }
}
class BadDerived extends Base {
  value!: string;          // error TS2612: will overwrite the base property -- becomes undefined at runtime
}
class GoodDerived extends Base {
  declare value: string;   // type-only redeclaration, emits nothing
}

class Svc {
  readonly url = this.cfg.base + "/api";   // error TS2729: 'cfg' is used before its initialization
  constructor(private cfg: { base: string }) {}
}
export { BadDerived, GoodDerived, Svc };
```

Verified at runtime on Node 22: with `target: es2022` the `Svc` version throws `Cannot read properties of undefined (reading 'base')` (field initializers run before parameter-property assignments); with `target: es2020` it works. Changing `target` therefore changes behaviour. Initialize such fields in the constructor body instead.

##### `#private` vs. `private`

```ts
class Account {
  private secret = "a";       // compile-time only: (acct as any).secret and acct["secret"] work at runtime
  #pin = "1234";              // runtime-private (ES2022), invisible to JSON.stringify and Object.keys
  check(p: string) { return p === this.#pin; }
  static isAccount(o: object): o is Account { return #pin in o; }  // ergonomic brand check (4.5+)
}
const acct = new Account();
console.log(acct["secret"]);  // no error: bracket access bypasses 'private'
acct.#pin;                    // error TS18013: not accessible outside class 'Account'
```

Use `#private` for real encapsulation (secrets, invariants); `private` is documentation.

##### Other class rules

- `override` (4.3+) with `noImplicitOverride` makes base-class renames break subclasses at compile time; since 5.6 this also covers computed member names.
- `strictPropertyInitialization` requires fields to be assigned in the constructor; `!` (definite assignment) opts out — each one is a claim to verify (DI frameworks, `init()` methods).
- Getters and setters may have **unrelated types** since 5.1 (`get size(): number`, `set size(v: string | number)`); the getter type no longer has to be assignable to the setter type.
- `accessor` fields (4.9+) create auto-accessors, mainly for standard decorators.
- `abstract` classes cannot be instantiated; `abstract new () => T` types accept them.
- `implements` checks the class shape but does **not** contextually type the members: a method parameter without an annotation is still implicitly `any`.

```ts
interface Handler { handle(input: string): number }
class H implements Handler {
  handle(input) { return input.length; }   // error TS7006: implicit any -- 'implements' does not type parameters
}
export { H };
```

***

#### Decorators

TypeScript 5.0+ implements **standard ECMAScript decorators** (TC39 Stage 3) when `experimentalDecorators` is **off** (the default). Legacy decorators remain behind `experimentalDecorators: true`; the two have different signatures, emit and evaluation order and cannot be mixed in one project.[^8]

```ts
// @run
function logged<This, Args extends unknown[], R>(
  target: (this: This, ...args: Args) => R,
  context: ClassMethodDecoratorContext<This, (this: This, ...args: Args) => R>,
) {
  const name = String(context.name);
  return function (this: This, ...args: Args): R {
    console.log(`-> ${name}`);
    return target.call(this, ...args);
  };
}

function bound<This, Args extends unknown[], R>(
  _target: (this: This, ...args: Args) => R,
  context: ClassMethodDecoratorContext<This, (this: This, ...args: Args) => R>,
) {
  const name = context.name;
  context.addInitializer(function (this: This) {
    const self = this as Record<string | symbol, unknown>;
    self[name] = (self[name] as (this: This, ...args: Args) => R).bind(this);
  });
}

class Greeter {
  constructor(private readonly who: string) {}
  @logged @bound
  greet() { return `hello ${this.who}`; }
}

const g = new Greeter("ts");
const detached = g.greet;
console.log(detached());            // -> greet | hello ts
```

Standard vs. legacy differences a reviewer must know:

- Standard decorators receive `(value, context)`; legacy ones receive `(target, key, descriptor)`. Libraries such as older Angular, NestJS, TypeORM, class-validator and Inversify rely on **legacy** decorators *and* `emitDecoratorMetadata`; switching a project to standard decorators breaks them.
- **Parameter decorators** (`constructor(@Inject() x)`) exist only with `experimentalDecorators`. 5.6 rejects them otherwise.
- `emitDecoratorMetadata` works only with `experimentalDecorators`.
- Decorator metadata (`context.metadata`, `Symbol.metadata`, 5.2+): TypeScript's emit uses `Symbol.metadata` only if the runtime has it. **Node 22 and 24 do not define `Symbol.metadata`**, so `context.metadata` is `undefined` unless a polyfill runs before the decorated classes load: `(Symbol as { metadata?: symbol }).metadata ??= Symbol("Symbol.metadata");` (the cast is needed because the lib declares `Symbol.metadata` as `readonly`). Verified: with the polyfill, `C[Symbol.metadata]` holds the values decorators wrote.
- Since 5.5 decorator expressions are parsed strictly: `@super.decorate` must be written `@(super.decorate)`.
- **`target: esnext` emits decorators verbatim.** No current Node release parses them: on Node 22, 24 and 26 the output fails with `SyntaxError: Invalid or unexpected token` (verified). Keep `target` at `es2023` or lower for code with decorators so `tsc` downlevels them.

```ts
// @opts: {"experimentalDecorators": false}
declare function Inject(): ParameterDecorator;
class Svc {
  constructor(@Inject() dep: string) { console.log(dep); }   // error TS1206: Decorators are not valid here
}
export { Svc };
```

***

#### Explicit resource management: `using` and `await using` (5.2+)

`using` disposes a resource at the end of the enclosing block, in reverse order, even when an exception is thrown — the TypeScript form of `try/finally` cleanup.[^21]

```ts
// @run
class TempResource implements Disposable {
  constructor(readonly name: string) { console.log(`open ${name}`); }
  [Symbol.dispose]() { console.log(`close ${this.name}`); }
}

class Conn implements AsyncDisposable {
  async [Symbol.asyncDispose]() { console.log("conn closed"); }
}

async function work() {
  using a = new TempResource("a");
  using b = new TempResource("b");
  await using c = new Conn();
  console.log("working", a.name, b.name, c instanceof Conn);
}                                     // order: conn closed, close b, close a

await work();
```

Runtime requirements (verified):

- With `target` ≤ `es2022`/`es2023`, `tsc` downlevels `using` to helper code; it needs `Symbol.dispose`/`Symbol.asyncDispose` at runtime. Node 18.20, 20.20 and 22 (tested) define them; older runtimes need a polyfill. `lib` must include `esnext.disposable` (or `esnext`) for the types.
- With `target: esnext`, `using` is emitted as-is: Node 24 runs it, **Node 22 throws `SyntaxError`**.
- `DisposableStack`/`AsyncDisposableStack`/`SuppressedError` are typed by `lib.esnext.disposable` but missing at runtime on Node 22 (present on Node 24).
- `using x = null` / `undefined` is allowed (nothing is disposed); any other value without `[Symbol.dispose]` throws a `TypeError` at runtime and is a compile error if the type lacks it.

```ts
declare function openFile(): { close(): void };
using f = openFile();   // error TS2850: The initializer of a 'using' declaration must be either an object with a '[Symbol.dispose]()' method, or be 'null' or 'undefined'.
export {};
```

When disposal itself throws while another error is propagating, both are wrapped in a `SuppressedError` (`.error` = disposal error, `.suppressed` = original). With downlevel emit on Node 22 the TypeScript helper creates that object itself (`name === "SuppressedError"`) although no global `SuppressedError` exists, so `err instanceof SuppressedError` throws `ReferenceError` there — check `name` instead.

***

#### Errors and exceptions

```ts
class HttpError extends Error {
  override readonly name = "HttpError";
  constructor(readonly status: number, message: string, options?: ErrorOptions) {
    super(message, options);                       // ErrorOptions / cause: lib es2022
  }
}

async function fetchJson(url: string): Promise<unknown> {
  let res: Response;
  try {
    res = await fetch(url);
  } catch (err) {                                  // err: unknown (useUnknownInCatchVariables)
    throw new Error(`request to ${url} failed`, { cause: err });
  }
  if (!res.ok) throw new HttpError(res.status, `HTTP ${res.status} for ${url}`);
  return res.json();
}

function message(err: unknown): string {
  if (err instanceof Error) return err.message;
  return String(err);
}

try {
  await fetchJson("http://127.0.0.1:9/");
} catch (err) {
  console.log(err.message);                        // error TS18046: 'err' is of type 'unknown'
  console.log(message(err));
}
export { HttpError };
```

- Anything can be thrown; treat `catch` variables as `unknown` and narrow. Never annotate `catch (e: any)` to silence the check.
- Preserve the original error with `{ cause }` when re-throwing; do not stringify and lose the stack.
- `throw` only `Error` instances (typescript-eslint `only-throw-error`).
- With `target` < `es2015`, subclassing `Error` breaks `instanceof` (needs `Object.setPrototypeOf(this, new.target.prototype)`); with `es2015`+ targets it works natively. One more reason not to target ES5.
- A `Result<T, E>` union (`{ ok: true; value: T } | { ok: false; error: E }`) is a good fit for *expected* failures (validation, parsing); keep exceptions for unexpected ones.

***

#### Async code

TypeScript checks promise types but not whether promises are handled. The most common production bugs are invisible to `tsc` and need typescript-eslint (`no-floating-promises`, `no-misused-promises`, `await-thenable`, `return-await`, `require-await`, `promise-function-async`):

```ts
declare function save(id: number): Promise<void>;
declare const ids: number[];

// BAD: forEach ignores the promises -- unawaited, errors become unhandled rejections
ids.forEach(async (id) => { await save(id); });

// BAD: fire-and-forget
save(1);

// GOOD: concurrent, all awaited, first failure rejects
await Promise.all(ids.map((id) => save(id)));

// GOOD: sequential when order or rate matters
for (const id of ids) await save(id);

// GOOD: collect failures instead of stopping at the first
const results = await Promise.allSettled(ids.map(save));
const failed = results.filter((r): r is PromiseRejectedResult => r.status === "rejected");
console.log(failed.length);
```

None of the lines above produce a `tsc` error. Additional rules:

- `async` callbacks passed where `() => void` is expected (event emitters, `Array#forEach`, Express handlers in older typings) lose their rejections. `no-misused-promises` catches it.
- `return await` inside `try` is required for the `catch`/`finally` to see the rejection; outside `try` it is optional.
- Use `AbortSignal.timeout(ms)` / `AbortController` for cancellation and pass `signal` through (`fetch(url, { signal })`).
- A promise is always truthy. `tsc` reports TS2801 for a promise used directly as a condition (`if (isAllowed(u))`, `a && isAllowed(u)`, `isAllowed(u) ? … : …`), but not for `!!isAllowed(u)`, `Boolean(isAllowed(u))` or `users.filter(async (u) => await isAllowed(u))` — the last keeps every element. `no-misused-promises` catches these.
- Top-level `await` needs ESM output (`module` `es2022`/`esnext`/`nodenext` with an ESM file, or `preserve`) and `target` ≥ `es2017`.
- `Promise.withResolvers()` is declared in `lib.esnext.promise` in 5.6 (Node 22+ at runtime).

***

#### Modules and module resolution

##### Choose `module` by the runtime

| Runtime / tool | `module` | `moduleResolution` | Notes |
|---|---|---|---|
| Node.js (ESM or CJS), `tsc` emits | `nodenext` (or `node16`) | implied | relative imports need the **runtime** extension (`./util.js`); format from `package.json` `"type"` and `.mts`/`.cts` |
| Bundler (Vite, webpack, esbuild) | `preserve` (5.4+) or `esnext` | `bundler` | extensionless imports allowed; `noEmit` or bundler does emit |
| Library for both | `nodenext` | implied | strictest; output works in Node and bundlers |
| Legacy CommonJS | `commonjs` | `node10` | avoid for new code; `node10` ignores `exports` |

See the Modules reference for the exact rules of each mode.[^25] `--module node18` and `node20` are **not** in 5.6 (5.8/5.9). In 5.6 `nodenext` models the then-current Node behaviour: it does **not** allow `require()` of ESM from CommonJS (5.8 added that), even though Node 22.12+ supports it at runtime.

```ts
// @skip: multi-file example (module: nodenext, "type": "module"), verified on 5.6.3
import { helper } from "./helper";        // error TS2835: Relative import paths need explicit file extensions ... Did you mean './helper.js'?
import { helper } from "./helper.js";     // GOOD: refers to helper.ts at compile time, helper.js at runtime
import { helper } from "./helper.ts";     // error TS5097 unless allowImportingTsExtensions (+ noEmit)
```

##### JSON imports under `nodenext` in 5.6

5.6 compiles `import cfg from "./cfg.json"` in an ESM file without an import attribute; **Node rejects it at runtime** (`ERR_IMPORT_ATTRIBUTE_MISSING`). The attribute requirement is only enforced by `tsc` from 5.7. Always write the attribute:

```ts
// @skip: verified with resolveJsonModule + nodenext + "type": "module" on 5.6.3 and Node 22
import cfg from "./cfg.json" with { type: "json" };   // GOOD (import attributes: 5.3+)
import bad from "./cfg.json";                          // compiles on 5.6, throws ERR_IMPORT_ATTRIBUTE_MISSING in Node
import * as ns from "./cfg.json" with { type: "json" };
ns.version;                                            // error TS2339: only ns.default exists for JSON in ESM
```

Use `with`, not the older `assert` keyword (deprecated in 6.0, removed in 7.0).

##### `verbatimModuleSyntax` (5.0+) and type-only imports

`verbatimModuleSyntax` makes the rule simple: imports/exports without `type` are kept, those with `type` are dropped. It replaces `importsNotUsedAsValues` and `preserveValueImports` (both stopped functioning in 5.5; from 5.6 setting either is error TS5102).

```ts
// @opts: {"verbatimModuleSyntax": true}
import { createServer, type Server } from "node:http";   // value + inline type-only specifier
import { RequestListener } from "node:http";              // error TS1484: 'RequestListener' is a type and must be imported using a type-only import when 'verbatimModuleSyntax' is enabled.
const handler: RequestListener = (_req, res) => res.end("ok");
const server: Server = createServer(handler);
console.log(server.listening);
```

- Under `verbatimModuleSyntax`, a CommonJS-format file (no `"type": "module"`, or `.cts`) cannot use `import`/`export` syntax at all with `module: nodenext` (TS1286/TS1287); it must use `import x = require("x")` and `export =`. Projects that want ESM syntax compiled to CommonJS should not enable it — or should move to real ESM.
- `export type { T }` / `import type { T }` are erased; a type-only import of a class cannot be used with `new` (TS1361).
- A side-effect `import "./polyfill.js"` is always kept.

##### `paths`, `baseUrl` and runtime resolution

`paths` only affects type checking — **`tsc` never rewrites import specifiers** (5.6 has no import rewriting at all). `import { db } from "@/db"` compiles, and then fails in Node unless the runtime or bundler is configured with the same alias (Node `package.json` `"imports"` subpath imports such as `#db/*`, or a bundler alias). Prefer `package.json` `"imports"` for Node projects — both `tsc` (`nodenext`/`bundler`) and Node understand it. `baseUrl` is deprecated in 6.0; in 5.6, `paths` works without it.

##### Node.js type stripping

Node 22.18+ (and 23.6+) run `.ts` files directly by stripping types, without type checking; the feature is marked stable in Node 24.12/25.2.[^22] Constraints for code that must run this way under TypeScript 5.6:

- Only erasable syntax: no `enum`, no `namespace` with values, no parameter properties (`constructor(private x)`), no `import x = require()`, no legacy decorators. Node reports `ERR_UNSUPPORTED_TYPESCRIPT_SYNTAX`. 5.6 has no `--erasableSyntaxOnly` to enforce this; use a lint rule or code review.
- Imports must use the real `.ts` extension (`import "./lib.ts"`), which in 5.6 requires `allowImportingTsExtensions` and therefore `noEmit`. 5.6 cannot both accept `.ts` specifiers and emit JavaScript (`--rewriteRelativeImportExtensions` is 5.7+).
- Type-only imports must use `import type` (enable `verbatimModuleSyntax`).
- Node does not read `tsconfig.json` (`paths`, `target` and so on are ignored).

##### CommonJS interop

- `esModuleInterop` (enabled by `tsc --init`, implied by `module: preserve`/`nodenext`) makes `import fs from "fs"` / `import express from "express"` work for CommonJS modules. Keep it on; `esModuleInterop: false` is deprecated in 6.0.
- Since 5.6, default imports of ESM-only packages without a default export are rejected in every module mode except `amd`/`umd`/`system` (see New in 5.6), catching runtime `undefined` imports in bundler projects.
- In a CommonJS-format file under `nodenext`, `import { x } from "esm-only"` is error TS1479 in 5.6 (the file would `require()` an ES module). Use `await import()` or convert the file to ESM.

***

#### Declaration files and libraries

##### Publishing

```json
{
  "name": "my-lib",
  "type": "module",
  "exports": {
    ".": {
      "types": "./dist/index.d.ts",
      "default": "./dist/index.js"
    }
  },
  "files": ["dist"],
  "peerDependencies": { "typescript": ">=5.6" }
}
```

- `"types"` must come first in each `exports` condition block; a dual CJS/ESM package needs separate `.d.ts`/`.d.cts` files per format (a single `.d.ts` for both is the "masquerading as ESM/CJS" bug). Validate with `@arethetypeswrong/cli` and `publint`.
- Declarations generated by 5.6 may mention `IteratorObject`, `MapIterator`, `BuiltinIteratorReturn` etc. (for example an exported function returning `map.keys()`); consumers on 5.5 or older then fail. Either annotate public return types with older-compatible types (`IterableIterator<K>`) or document the minimum TypeScript version.
- Since 5.5, `tsc` neither synthesizes nor preserves `/// <reference types="…" />` in `.d.ts` output unless written with `preserve="true"`.

##### `isolatedDeclarations` (5.5+)

`isolatedDeclarations` requires exported API to carry enough annotations that a `.d.ts` can be produced from one file without type checking — enabling fast parallel declaration emit by other tools and `--noCheck` builds. It requires `declaration` (or `composite`).

```ts
// @opts: {"isolatedDeclarations": true, "declaration": true}
export function add(a: number, b: number) { return a + b; }   // error TS9007: Function must have an explicit return type annotation with --isolatedDeclarations.
export function sub(a: number, b: number): number { return a - b; }
export const ids = [1, 2].map((n) => n * 2);                  // error TS9010: Variable must have an explicit type annotation with --isolatedDeclarations.
export const ok: number[] = [1, 2].map((n) => n * 2);
export const name = "lib";                                    // no error: literal initializers are trivially inferable
```

Enable it for libraries and monorepo packages consumed through project references; for applications it adds annotation noise without benefit.

##### Ambient declarations and augmentation

- `declare global { … }` inside a module augments globals; `declare module "pkg" { … }` augments a module (the file must itself be a module, i.e. contain an `import`/`export`).
- A `.d.ts` file in `include` without imports/exports is a **global script**: every top-level declaration becomes global — the usual cause of "duplicate identifier" conflicts.
- `skipLibCheck` skips *all* `.d.ts` files, including your own hand-written ones.

***

#### Regular expression checking (5.5+)

TypeScript parses regular expression **literals** (not `new RegExp("…")` strings) and reports syntax errors, missing backreference groups, and flags or features newer than `target`:[^4]

```ts
const a = /@robot(\s+(please|immediately)))? do some task/;  // error TS1508: Unexpected ')'. Did you mean to escape it with backslash?
const b = /(\w+) \2/u;                                        // error TS1533: This backreference refers to a group that does not exist.
const c = /(?<year>\d{4})-\k<yaer>/;                           // error TS1532: There is no capturing group named 'yaer' in this regular expression.
const d = new RegExp("(unclosed");                            // no error at compile time -- SyntaxError at runtime
export { a, b, c, d };
```

Review regular expressions from user input for ReDoS (nested quantifiers like `(a+)+`); the compiler does not check complexity.

***

#### `lib` versus the real runtime

TypeScript only checks against the declarations you enable. This table shows where 5.6 declares commonly used newer APIs and which Node.js versions have them (runtime availability verified with Node 18.20, 20.20, 22.23 and 24.22):

| API | 5.6 lib | Node 18 | Node 20 | Node 22 | Node 24 |
|---|---|---|---|---|---|
| `Array#findLast`, `toSorted`, `toReversed`, `with` | `es2023` | partial (`findLast` only) | yes | yes | yes |
| `Object.groupBy`, `Map.groupBy` | `esnext.object`, `esnext.collection` | no | no | yes | yes |
| `Promise.withResolvers` | `esnext.promise` | no | no | yes | yes |
| `Set#union`, `intersection`, `difference`, `isSubsetOf` … | `esnext.collection` | no | no | yes | yes |
| `Array.fromAsync` | `esnext.array` | no | no | yes | yes |
| Iterator helpers, global `Iterator` | `esnext.iterator` | no | no | yes | yes |
| `Symbol.dispose`, `Symbol.asyncDispose` | `esnext.disposable` | yes | yes | yes | yes |
| `DisposableStack`, `AsyncDisposableStack`, `SuppressedError` | `esnext.disposable` | no | no | no | yes |
| `Symbol.metadata` | `esnext.decorators` | no | no | no | no |
| `Promise.try`, `RegExp.escape`, `Float16Array`, `Error.isError` | **not in 5.6** | – | – | – | yes (types only from 5.8–6.0) |

Rules:

- Set `lib` to the oldest runtime you support and add `esnext.*` sub-libs one by one, rather than `lib: ["esnext"]`.
- Code that uses an API newer than the runtime needs a polyfill import (core-js, `es-iterator-helpers`) — and the polyfill import should be a checked side-effect import (`noUncheckedSideEffectImports`).
- For browser code, `lib: ["dom", "dom.iterable"]` follows the DOM typings bundled with 5.6; APIs that are in the typings may still be missing in older browsers (check Baseline/caniuse).
- Using APIs that 5.6 does not declare (`Promise.try` on Node 24) requires a local `declare global` augmentation; keep it in one file with a comment explaining why.

***

#### Security

TypeScript's guarantees end at the process boundary. Reviewers should look for:

- **Unvalidated input typed by assertion**: `req.body as CreateUser`, `JSON.parse(x) as T`, `await res.json() as T`, generic `fetchJson<T>()` helpers. All of these are unchecked casts. Require a runtime schema at every trust boundary (HTTP, queues, files, environment, `postMessage`, `localStorage`).
- **Prototype pollution**: merging untrusted objects into plain objects (`Object.assign(target, input)`, recursive "deep merge", `obj[key] = value` with user-controlled `key`). Use `Map`, `Object.create(null)`, or reject `__proto__`/`constructor`/`prototype` keys. Types do not prevent it (see the `Record` example).
- **Excess data leakage**: objects with more properties than their type (structural typing) serialized into responses or logs. Map explicitly to DTOs.
- **Injection**: TypeScript does not distinguish trusted from untrusted strings. SQL must use parameterized queries or tagged-template builders that parameterize; shell commands must use `execFile`/`spawn` with argument arrays (not `exec` with interpolation); HTML must be escaped or set via `textContent`, never `innerHTML` with user data. Branded types (`SafeHtml`, `SqlFragment`) can make the safe path the only one that type-checks.
- **`eval`, `new Function`, `vm`** with any external input: reject.
- **`any` at boundaries**: a single `any` from an untyped dependency or `catch` silences checks downstream; enable `noImplicitAny` and lint `no-unsafe-*` rules (`no-unsafe-assignment`, `no-unsafe-member-access`, `no-unsafe-call`, `no-unsafe-return`, `no-unsafe-argument`).
- **Secrets in source maps and declaration output**: `sourceMap`/`inlineSources` publish original source; check what goes into npm packages (`files`, `.npmignore`) and front-end bundles.
- **Supply chain**: pin `typescript`, `@types/*` and build tooling in the lockfile; an `@types` update can change what type-checks across the whole program.

***

#### Performance

##### Compiler performance

- Measure with `tsc --extendedDiagnostics` (check time, instantiation count, memory) and `tsc --generateTrace <dir>` (analyze with `@typescript/analyze-trace`). `tsc --explainFiles` and `--listFilesOnly` show why files are included.
- Use `incremental: true` (or `composite` with project references and `tsc -b`) so rebuilds reuse `.tsbuildinfo`.
- Use `--noCheck` (5.6+) for fast emit-only builds, with type checking as a separate CI step; with `isolatedDeclarations`, declaration emit becomes per-file.
- Avoid huge unions (thousands of members), deeply recursive conditional types and wide intersections of object types; prefer `interface extends` over `&` for object composition (relations between interfaces are cached).
- Annotate return types of exported functions in large code bases: it saves inference work and stabilizes declaration output.
- Restrict `types` and `include`; scanning all of `node_modules/@types` and stray `.d.ts` files costs time and leaks globals.

##### Runtime performance

TypeScript emits plain JavaScript; types have no runtime cost. What does cost:

- Downlevel emit: low `target` values produce slower, larger code (generators, async, class fields, `using` helpers). Use the highest `target` your runtime supports.
- `enum` objects and `namespace` IIFEs are runtime code; `as const` objects or plain unions are lighter.
- Iterator helpers are lazy — good for streaming large sequences, but `iter.map(f).toArray()` is not faster than `array.map(f)` on an array that already exists.

***

#### Testing

- Test runners: Vitest (native TypeScript through esbuild), Jest with `ts-jest` or `@swc/jest`, Node's built-in `node:test` with type stripping (Node 22.18+). Transpile-only runners do **not** type-check: run `tsc --noEmit` (or `tsc -b`) in CI as a separate step.
- Type-level tests: `@ts-expect-error` asserts that the next line has an error and itself errors when it does not (TS2578), unlike `@ts-ignore`, which silently stays forever. Use `@ts-expect-error` with a reason in tests; flag `@ts-ignore` and `@ts-nocheck` in production code (typescript-eslint `ban-ts-comment`).

```ts
interface User { id: string; name: string }
// @ts-expect-error -- id is required
const u1: User = { name: "x" };
// @ts-expect-error -- unused directive: reported as error TS2578 because the next line is valid
const u2: User = { id: "1", name: "x" };
export { u1, u2 };
```

- For precise type assertions use `expectTypeOf` (Vitest) or `tsd`/`tstyche`; a hand-rolled `type Equals<A, B>` helper is easy to get subtly wrong with `any`.
- Do not use `as` in tests to construct impossible fixtures for production types; build valid objects with factory functions so tests break when the type changes.

***

#### Tooling

- **Type-aware linting**: typescript-eslint v8 (8.72.0 supports TypeScript `>=4.8.4 <6.1.0`, so 5.6 is supported).[^23] Use `tseslint.configs.strictTypeChecked` + `stylisticTypeChecked` with `parserOptions.projectService: true`. Key rules: `no-floating-promises`, `no-misused-promises`, `await-thenable`, `no-unnecessary-condition`, `strict-boolean-expressions` (opinionated), `switch-exhaustiveness-check`, `consistent-type-imports`, `no-explicit-any`, `no-unsafe-*`, `prefer-nullish-coalescing`, `only-throw-error`, `ban-ts-comment`, `no-non-null-assertion`, `restrict-template-expressions`.
- **Formatting**: Prettier or dprint/Biome; never hand-format. Formatting changes should be separate from logic changes in review.
- **Editors**: VS Code ships its own TypeScript; select the workspace version ("TypeScript: Select TypeScript Version") so the editor uses 5.6 and does not show errors or quick fixes from a newer compiler.
- **Running TypeScript**: `tsx` / `ts-node` (`--transpile-only`) / Node type stripping for scripts; `tsc` for production builds or a bundler with a separate `tsc --noEmit` check.
- **Package checks**: `@arethetypeswrong/cli` and `publint` for published libraries.
- **Pin the compiler**: `"typescript": "~5.6.3"` (tilde, not caret: minor releases add errors by design).

***

#### Removed / changed in 5.6 — migration notes

Upgrading from 5.5 (items 1–8 verified on 5.5.4 vs. 5.6.3):

1. **New always-on errors** TS2869/TS2871/TS2872/TS2873 for constant truthy/nullish expressions. Fix the logic; they are real bugs in nearly all cases.
2. **`strict` now includes `strictBuiltinIteratorReturn`**: `.next().value` of built-in iterators becomes `T | undefined`. Check `done`, or set the flag to `false` temporarily.
3. **lib changes for iterator helpers**: built-in iterators return `ArrayIterator`/`MapIterator`/`SetIterator` etc. Classes implementing `Map`/`Set`/`Array`-like built-in interfaces with `IterableIterator` return types fail with `lib: esnext`. Use the new return types or composition.
4. **`@types/node`**: current releases reference `IteratorObject`/`BuiltinIteratorReturn` and fail on 5.5; after the upgrade to 5.6 they compile, so a pin to an old `@types/node` that existed only for 5.5 can be lifted.
5. **`tsc -b` no longer stops at the first failing project** and emits outputs of projects with errors; add `--stopBuildOnErrors` or `noEmitOnError` where CI/deploy relied on the old behaviour, and ignore the now-always-written `.tsbuildinfo` files.
6. **Module format of `node_modules` files** is respected in `esnext`/`bundler`/`preserve` modes: default imports of ESM-only packages without a default export now fail (TS1192). Switch to named imports.
7. **`.mts` emits ESM and `.cts` emits CommonJS** regardless of `module`.
8. **`override`/`noImplicitOverride` on computed names** (TS4113/TS4114).
9. **DOM lib updates** (`lib.dom.d.ts` regenerated, see the 5.6 release notes' linked issue): new errors in DOM code after the upgrade should be fixed by following the new types rather than by casting.
10. New opt-in flags to consider: `noUncheckedSideEffectImports` (recommended), `noCheck` (build pipelines only).

***

#### Production / review checklist

##### Configuration
- [ ] `typescript` pinned to `~5.6.3`; editor uses the workspace version
- [ ] `strict: true`; `target` set explicitly (not left to the `es5` or the `nodenext`-implied `esnext` default)
- [ ] `lib` matches the oldest supported runtime; no `esnext` APIs the runtime lacks (`DisposableStack` on Node 22, iterator helpers on Node 20)
- [ ] `module`/`moduleResolution` match the runtime (`nodenext` for Node, `preserve`/`bundler` for bundlers)
- [ ] `noUncheckedIndexedAccess`, `noImplicitOverride`, `noFallthroughCasesInSwitch`, `noUncheckedSideEffectImports` considered
- [ ] `verbatimModuleSyntax` (or at least `isolatedModules`) when a single-file transpiler or Node type stripping runs the code
- [ ] No removed options (`importsNotUsedAsValues`, `preserveValueImports`, `suppressImplicitAnyIndexErrors`, `out`, `charset`, `keyofStringsOnly`, `noStrictGenericChecks`, `target: ES3`)
- [ ] No 5.7+ options (`rewriteRelativeImportExtensions`, `erasableSyntaxOnly`, `libReplacement`, `module: node18/node20`, `target/lib es2024+`)
- [ ] CI runs a real type check (`tsc --noEmit` or `tsc -b`) — not only `--noCheck`, a bundler, or a transpile-only test runner — and fails on the exit code

##### Types and correctness
- [ ] No new `any`; `unknown` at boundaries with runtime validation
- [ ] No unjustified `as`, `as unknown as`, `!`, `@ts-ignore`; `@ts-expect-error` carries a reason
- [ ] Hand-written type predicates and assertion functions match their bodies
- [ ] Exhaustive `switch` over unions with `assertNever`/`satisfies never`
- [ ] `??` instead of `||` where `0`/`""`/`false` are valid
- [ ] No reliance on narrowing of object properties across `await`/callbacks
- [ ] Iterator results checked with `done` before using `value`
- [ ] Single-use generic return types (`parse<T>()`) treated as casts
- [ ] Enums: no numeric enums for external values; no `const enum` across files with isolated transpilers
- [ ] Class fields: no redeclared base fields without `declare`; no initializers depending on parameter properties

##### Modules and packaging
- [ ] Relative imports carry `.js` (nodenext) or follow the bundler's rules; no `paths` aliases that the runtime cannot resolve
- [ ] JSON imports use `with { type: "json" }`
- [ ] `import type` / inline `type` for type-only imports
- [ ] Libraries: `exports` with `types` first, per-format declarations, validated with `@arethetypeswrong/cli`; public `.d.ts` checked for 5.6-only types if older consumers matter

##### Async and errors
- [ ] No floating promises; no `async` callbacks where `void` is expected; `forEach(async …)` replaced
- [ ] `catch` variables treated as `unknown`; errors re-thrown with `cause`
- [ ] Cancellation via `AbortSignal`; timeouts on network calls
- [ ] `using`/`await using` for resources where the runtime supports `Symbol.dispose`

##### Security
- [ ] Schema validation at every trust boundary (HTTP bodies, env, files, messages)
- [ ] No prototype-pollution-prone merges of untrusted objects
- [ ] Parameterized SQL, argument-array process spawning, no `innerHTML` with user data, no `eval`
- [ ] Response DTOs constructed explicitly (no structural over-sharing)

***

[^1]: TypeScript 5.6 release notes. https://www.typescriptlang.org/docs/handbook/release-notes/typescript-5-6.html
[^2]: Announcing TypeScript 5.6 (Microsoft DevBlogs, September 9, 2024). https://devblogs.microsoft.com/typescript/announcing-typescript-5-6/
[^3]: npm registry — typescript package versions and publish dates (5.6.2: 2024-09-09, 5.6.3: 2024-10-08, 6.0.3: 2026-04-16, 7.0.2: 2026-07-08). https://www.npmjs.com/package/typescript?activeTab=versions
[^4]: TypeScript 5.5 release notes (inferred type predicates, constant indexed access narrowing, regex checking, isolatedDeclarations, `${configDir}`, disabled 5.0 deprecations). https://www.typescriptlang.org/docs/handbook/release-notes/typescript-5-5.html
[^5]: TypeScript 5.7 release notes. https://www.typescriptlang.org/docs/handbook/release-notes/typescript-5-7.html
[^6]: TypeScript 5.8 release notes. https://www.typescriptlang.org/docs/handbook/release-notes/typescript-5-8.html
[^7]: TypeScript 5.9 release notes. https://www.typescriptlang.org/docs/handbook/release-notes/typescript-5-9.html
[^8]: TypeScript 5.0 release notes (decorators, const type parameters, verbatimModuleSyntax, bundler resolution, deprecations). https://www.typescriptlang.org/docs/handbook/release-notes/typescript-5-0.html
[^9]: TypeScript 6.0 release notes (new defaults, deprecations, preparing for 7.0). https://www.typescriptlang.org/docs/handbook/release-notes/typescript-6-0.html
[^10]: Announcing TypeScript 7.0 (Microsoft DevBlogs, July 8, 2026). https://devblogs.microsoft.com/typescript/announcing-typescript-7-0/
[^11]: microsoft/TypeScript PR #59217 — Disallow truthy/nullish checks on syntactically never-nullish expressions. https://github.com/microsoft/TypeScript/pull/59217
[^12]: TC39 proposal: Iterator helpers. https://github.com/tc39/proposal-iterator-helpers
[^13]: microsoft/TypeScript PR #58222 — Iterator helper types (IteratorObject). https://github.com/microsoft/TypeScript/pull/58222
[^14]: DefinitelyTyped README — support window (packages are tested only on TypeScript versions less than two years old). https://github.com/DefinitelyTyped/DefinitelyTyped#support-window
[^15]: microsoft/TypeScript PR #58243 — BuiltinIteratorReturn and --strictBuiltinIteratorReturn. https://github.com/microsoft/TypeScript/pull/58243
[^16]: microsoft/TypeScript PR #58640 — Arbitrary module namespace identifiers. https://github.com/microsoft/TypeScript/pull/58640
[^17]: microsoft/TypeScript PR #58941 — --noUncheckedSideEffectImports. https://github.com/microsoft/TypeScript/pull/58941
[^18]: microsoft/TypeScript PR #58838 — Allow --build with intermediate errors (--stopBuildOnErrors). https://github.com/microsoft/TypeScript/pull/58838
[^19]: microsoft/TypeScript PR #58825 — Respect file extensions and package.json "type" in node_modules for all module modes. https://github.com/microsoft/TypeScript/pull/58825
[^20]: TypeScript 5.4 release notes (preserved narrowing in closures, NoInfer, module preserve). https://www.typescriptlang.org/docs/handbook/release-notes/typescript-5-4.html
[^21]: TypeScript 5.2 release notes (`using` declarations, decorator metadata). https://www.typescriptlang.org/docs/handbook/release-notes/typescript-5-2.html
[^22]: Node.js — Modules: TypeScript (type stripping); Node.js v22.18.0 release notes. https://nodejs.org/api/typescript.html , https://nodejs.org/en/blog/release/v22.18.0
[^23]: typescript-eslint — supported TypeScript versions (`SUPPORTED_TYPESCRIPT_VERSIONS = '>=4.8.4 <6.1.0'` in v8.72.0). https://typescript-eslint.io/users/dependency-versions
[^24]: TSConfig reference. https://www.typescriptlang.org/tsconfig/
[^25]: TypeScript Modules reference (module, moduleResolution, nodenext, bundler, preserve). https://www.typescriptlang.org/docs/handbook/modules/reference.html
