### TypeScript 5.2: The Complete Production Developer Guide

> **Version**: TypeScript 5.2 — beta `5.2.0-beta` (2023-06-30), RC `5.2.1-rc` (2023-08-09), stable **5.2.2 (2023-08-24)**. 5.2.2 is the first and **only** stable 5.2 release; no later 5.2.x patch was ever published.[^1][^2][^3]
> **Status (October 2026)**: TypeScript has no LTS lines and does not backport fixes, so 5.2 has been frozen since TypeScript 5.3.2 shipped on 2023-11-20. The newest stable release is **TypeScript 7.0** (7.0.2, 2026-07-08, the native Go compiler); the last JavaScript-based line is 6.0 (6.0.3, 2026-04-16); 7.1.0-beta appeared on 2026-10-20.[^3] DefinitelyTyped only tests `@types/*` packages against TypeScript versions younger than two years, so since August 2025 new `@types` releases are no longer guaranteed to compile under 5.2 (see Ecosystem compatibility).[^4]
> **Scope**: The full TypeScript language as usable with the 5.2 compiler: every 5.2 change versus 5.1 (explicit resource management, decorator metadata, tuple labels, array-union methods, ES2023 copying array methods, symbols as weak keys, type-only `.ts` import paths, breaking changes), the type system, narrowing, generics, classes and both decorator systems, enums, modules and module resolution (`bundler`, `node16`, `nodenext`), `verbatimModuleSyntax`, strictness flags, the 5.0 deprecations, tsconfig recommendations, reviewer-relevant compiler pitfalls and tooling. Every TypeScript example was compiled with `tsc` 5.2.2; boundary claims were checked against 5.1.6 and 5.3.3 (and later compilers where named); emitted JavaScript was run on Node.js 16.20.2, 18.20.8, 20.20.2, 22.23.4, 24.22.1 and 26.12.1 where runtime behaviour matters.
> **Note**: This document is self-contained. Sections marked **[New in 5.2]** cover behaviour introduced in 5.2.

---

#### What's New in TypeScript 5.2

TypeScript 5.2 is a feature release with one large addition — **explicit resource management** (`using` and `await using`) — plus decorator metadata, several type-system relaxations, new library typings and four breaking changes.[^1][^2]

| Area | Change in 5.2 | Verified boundary (5.1.6 → 5.2.2) |
|---|---|---|
| Syntax | `using` and `await using` declarations, `Symbol.dispose`/`Symbol.asyncDispose`, `Disposable`/`AsyncDisposable`, `DisposableStack`, `AsyncDisposableStack`, `SuppressedError` (lib `esnext.disposable`) | 5.1 rejects `--lib esnext.disposable` (TS6046) and does not parse `using` |
| Decorators | `context.metadata` on every standard decorator context, `Symbol.metadata`, `DecoratorMetadata` (lib `esnext.decorators`) | 5.1: TS2339 "Property 'metadata' does not exist on type 'ClassFieldDecoratorContext'" |
| Tuples | Labeled and unlabeled elements may be mixed; labels survive spreads | 5.1: TS5084 "Tuple members must all have names or all not have names." |
| Calls on array unions | `filter`, `find`, `some`, `every`, `reduce` callable on `string[] \| number[]` | 5.1: TS2349 "This expression is not callable." |
| Library | `toSorted`, `toReversed`, `toSpliced`, `with` on arrays and typed arrays (lib `es2023.array`) | 5.1: TS2339 "Property 'toSorted' does not exist" |
| Library | Non-registered symbols as `WeakMap`/`WeakSet`/`WeakRef`/`FinalizationRegistry` keys (`WeakKey`, lib `es2023.collection`) | 5.1: TS2344 "Type 'symbol' does not satisfy the constraint 'object'." |
| Modules | `import type` and `import("…")` types may use `.ts`/`.mts`/`.cts`/`.tsx` extensions without `allowImportingTsExtensions` | 5.1: TS5097 |
| Editor | Comma completions for object members, "Inline variable" refactoring, clickable inlay parameter hints | 5.1 language service offers neither |
| Performance | Faster checks for deeply nested recursive type relations | — |
| Breaking | `module`/`moduleResolution` must agree when either is `node16`/`nodenext` (TS5109, TS5110) | 5.1 accepted `--module esnext --moduleResolution node16` silently |
| Breaking | Merged declarations must be all exported or all local, also in ambient contexts (TS2395) | 5.1 accepted the mismatch in `.d.ts` files |
| Breaking | Declaration emit writes `namespace` instead of `module` for namespaces | 5.1 emitted `export declare module Legacy` |
| Breaking | DOM `lib.d.ts` regenerated; `TupleType.labeledElementDeclarations` may contain `undefined` (compiler API) | — |

What 5.2 **does not** change: the default `target` is still ES5 and the default `module` is still CommonJS when no tsconfig sets them; `strict` is still off unless enabled; there is no new `target` value (`es2023` as a *target* arrives only in 5.5, see below), and `lib: ["esnext"]` in 5.2 means `es2023` plus `esnext.intl`, `esnext.decorators` and `esnext.disposable` — nothing else.

Everything from 5.0 and 5.1 is available and should be used where it fits: standard (TC39) decorators, `const` type parameters, `moduleResolution: "bundler"`, `verbatimModuleSyntax`, `allowImportingTsExtensions`, `customConditions`, `resolvePackageJsonExports`/`Imports`, `allowArbitraryExtensions`, `export type *`, `extends` arrays in tsconfig, all enums as union enums, JSDoc `@satisfies` and `@overload` (5.0); unrelated getter/setter types, implicit `undefined` returns for `undefined`-returning functions, decoupled JSX element typing and namespaced JSX attributes (5.1).[^5][^6]

***

#### Version-Aware Review — Do Not Use or Recommend

##### How to judge code for a 5.2 project

- Determine the **compiler** version from `package.json` (`"typescript": "~5.2.2"`, `"5.2.2"`, or a lockfile entry) and the **runtime** separately (`engines.node`, `.nvmrc`, Dockerfile base image, browserslist). The compiler version decides which syntax, options and library typings type-check; the runtime decides what exists when the code runs. TypeScript never polyfills library APIs.
- A caret range such as `"typescript": "^5.2.2"` resolves to the newest 5.x on a fresh install (5.9.3 in October 2026). Look at the lockfile before assuming the project really compiles with 5.2. If the lockfile pins 5.2.2, judge the code against 5.2.
- Before claiming that an option, lib name or syntax "does not exist", check it against 5.2. Many things are older than they look: `satisfies` (4.9), `accessor` and standard decorators (5.0), `const` type parameters (5.0), `verbatimModuleSyntax` (5.0), `moduleResolution: "bundler"` (5.0), `Array.prototype.findLast` typings (5.0), `using` (5.2). Say "not available in 5.2" only for things this guide or the 5.2 compiler proves missing.
- Code that compiles with 5.2 may still fail at run time: `using`, `DisposableStack`, decorator metadata and the ES2023 array methods all need runtime support or polyfills that the compiler does not check (see the sections below for the verified Node.js matrix).

##### Do not use or recommend (not available in 5.2)

These belong to newer compilers. Syntax, compiler options, library typings and narrowing items were executed (5.2.2 rejects them, the named version accepts them); editor features and a few diagnostics are taken from the respective release notes.

- **5.3**: import attributes — `import data from "./data.json" with { type: "json" }` is a parse error in 5.2 (TS1005); `import(…, { with: … })` fails with TS2322 because 5.2's `ImportCallOptions` only knows `assert`. `switch (true)` narrowing (`case typeof v === "string":` does not narrow in 5.2). Narrowing when a type-guard call is compared to `true` (`if (isA(x) === true)`). `instanceof` narrowing through `Symbol.hasInstance`. Interactive inlay hints.[^7]
- **5.4**: preserved narrowing in closures created after the last assignment; the `NoInfer<T>` utility type; `Object.groupBy`, `Map.groupBy` and `Promise.withResolvers` typings; `--module preserve`; checked import attributes.[^8]
- **5.5**: inferred type predicates (`arr.filter((x) => x !== undefined)` stays `(T | undefined)[]` in 5.2); narrowing of constant indexed accesses (`obj[key]` after `typeof obj[key] === "string"`); regular-expression syntax checking; `Set.prototype.union`/`intersection`/`difference`/… typings; `Array.fromAsync` and `String.prototype.isWellFormed` typings; JSDoc `@import`; `isolatedDeclarations`; `${configDir}` in tsconfig; `target: "es2023"` (5.2 accepts only `es3`…`es2022` and `esnext` as targets).[^9]
- **5.6**: "always truthy/nullish" checks (TS2872/TS2869); Iterator helper typings (`IteratorObject`, `Iterator.from`); `--strictBuiltinIteratorReturn`; `--noUncheckedSideEffectImports`; `--noCheck`; arbitrary module identifiers; `tsc --build` continuing past errors.[^10]
- **5.7**: checks for never-initialized variables in nested functions; `--rewriteRelativeImportExtensions`; `target`/`lib` `es2024`; `ArrayBuffer.prototype.transfer` and other ES2024 typings; the JSON-import attribute requirement under `nodenext`.[^11]
- **5.8–5.9**: `--erasableSyntaxOnly`, `--module node18`, `require()` of ES modules under `nodenext`, `--libReplacement` (5.8); `import defer`, `--module node20`, the minimal `tsc --init` (5.9).[^12]
- **6.0 / 7.0**: new defaults (`strict` on, `module: esnext`, `types: []`, …), the 6.0 deprecations, `ignoreDeprecations: "6.0"` (TS5103 "Invalid value" in 5.2), the native compiler and its performance. None of this applies to a 5.2 build.[^13]
- **Library APIs without 5.2 typings** even with `lib: ["esnext"]`: `Promise.try`, `RegExp.escape`, `Float16Array`, `Iterator` as a value, `Map.prototype.getOrInsert`, `Temporal`, `Error.isError`, `Uint8Array.fromBase64`. Using them requires a local `declare global` augmentation **and** a runtime that has them.

##### Deprecated or removed older features

- **Deprecated in 5.0, still usable in 5.2 only with `"ignoreDeprecations": "5.0"`** (no effect from 5.5, hard errors TS5102/TS5108 from 5.6): `target: "es3"`, `out`, `prepend` in project references, `charset`, `importsNotUsedAsValues`, `preserveValueImports`, `keyofStringsOnly`, `suppressExcessPropertyErrors`, `suppressImplicitAnyIndexErrors`, `noImplicitUseStrict`, `noStrictGenericChecks`. Without the escape hatch 5.2 reports `TS5101`/`TS5107` "… is deprecated and will stop functioning in TypeScript 5.5". Treat every one of them as an upgrade blocker. See section 9.
- **Import assertions** (`assert { type: "json" }`) are the only JSON-module syntax 5.2 can parse, but Node.js 22 and later reject `assert` at run time (verified, see Modules). TypeScript 5.3 added `with`, and later compilers deprecate `assert`.
- **`experimentalDecorators`** is not deprecated in 5.2 (nor in 6.0/7.0). It is a separate decorator system; new code should use standard decorators unless a framework requires the legacy ones.
- **`moduleResolution: "node"`** is an alias of `node10` in 5.2. It does not understand `package.json` `exports`; prefer `node16`/`nodenext` or `bundler`.

***

#### Ecosystem Compatibility of a 5.2 Toolchain

A 5.2 project in 2026 runs a three-year-old compiler against current dependencies. These break most often:

- **`@types/node`**: verified against `tsc` 5.2.2 *without* `skipLibCheck`: `@types/node` 20.19.37, 22.19.21 and 24.19.2 type-check cleanly; **26.6.5 does not** — it references `IteratorObject`, `AsyncIteratorObject` and `BuiltinIteratorReturn` (TypeScript 5.6 lib types) and produces TS2304/TS2416 errors in `globals.d.ts`, `fs.d.ts` and `stream.d.ts`. `skipLibCheck: true` hides these errors, but declarations that mention the missing types degrade silently. The npm dist-tag `ts5.2` of `@types/node` points to 25.5.0 (2026-03-12); per major, the last releases whose `typeScriptVersion` field is still `5.2` are 20.19.37 and 22.19.15 (both 2026-03-06) — later 20.x/22.x releases declare `5.3`, and 24.19.2 declares `5.6` even though it currently type-checks under 5.2. Pin `@types/node` to the **major of the deployment runtime** (`~22.19.x` for Node.js 22) and do not let a caret range float to 26.
- **typescript-eslint**: 8.x (8.72.0 in October 2026) declares `typescript: ">=4.8.4 <6.1.0"` and therefore still supports 5.2.[^19] Type-aware lint rules (`no-floating-promises`, `no-misused-promises`, `await-thenable`, `no-unnecessary-condition`) are the most valuable complement to the compiler on 5.2, which lacks the 5.6 truthiness checks.
- **`tslib`**: with `importHelpers: true`, `using` needs `__addDisposableResource`/`__disposeResources`, which first appear in **tslib 2.6.0** (2023-06-26).[^18] Verified: with tslib 2.5.3 installed, 5.2 reports `TS2343: This syntax requires an imported helper named '__addDisposableResource' which does not exist in 'tslib'. Consider upgrading your version of 'tslib'.`; 2.6.2 compiles.
- Libraries that ship `.d.ts` files written for newer compilers (using `NoInfer`, `IteratorObject`, `BuiltinIteratorReturn`, import attributes in `import()` types) fail the same way. When a dependency upgrade produces errors inside `node_modules/**/*.d.ts`, the fix is to pin the dependency or upgrade TypeScript — not to add `@ts-ignore` or blanket `skipLibCheck` without understanding the cause.

***

#### Explicit Resource Management: `using` / `await using` [New in 5.2]

##### Semantics

A `using` declaration binds a value whose `[Symbol.dispose]()` method runs when the enclosing block exits — normally, through `return`/`break`/`continue`, or by an exception. `await using` does the same with `[Symbol.asyncDispose]()` and awaits it. Resources are disposed in **reverse order of declaration** (last in, first out).[^1][^14]

```ts
// flags: --lib es2022,esnext.disposable,dom
class TempFile implements Disposable {
  constructor(readonly path: string) {
    console.log("open", path);
  }
  [Symbol.dispose](): void {
    console.log("delete", this.path);
  }
}

class Connection implements AsyncDisposable {
  async query(sql: string): Promise<string[]> {
    return [sql];
  }
  async [Symbol.asyncDispose](): Promise<void> {
    console.log("connection closed");
  }
}

export async function job(): Promise<string[]> {
  await using conn = new Connection();
  using a = new TempFile("/tmp/a");
  using b = new TempFile("/tmp/b");
  using maybe = Math.random() > 2 ? new TempFile("/tmp/never") : null; // null/undefined are allowed and skipped
  void a; void b; void maybe;
  return conn.query("select 1");
  // disposal order on every exit path: b, a, conn
}
```

Rules enforced by the 5.2 compiler (verified):

```ts
// flags: --lib es2022,esnext.disposable,dom
declare function openHandle(): { close(): void };

export function notDisposable() {
  using h = openHandle(); // error TS2850
}
export function syncFunction() {
  await using x = { async [Symbol.asyncDispose]() {} }; // error TS2852
}
export function reassigned() {
  using z = { [Symbol.dispose]() {} };
  z = { [Symbol.dispose]() {} }; // error TS2588
}
export function loop(items: Disposable[]) {
  for (using it of items) {
    void it; // each element is disposed at the end of its iteration
  }
}
export async function syncInAsync() {
  await using y = { [Symbol.dispose]() {} }; // OK: await using accepts a sync Disposable
}
```

- TS2850: "The initializer of a 'using' declaration must be either an object with a '[Symbol.dispose]()' method, or be 'null' or 'undefined'." An object with only `close()` is not disposable; wrap it (`DisposableStack#adopt`, below).
- TS2852: "'await using' statements are only allowed within async functions and at the top levels of modules."
- `using` bindings are `const`-like (TS2588) and cannot be destructuring patterns (TS1492 "'using' declarations may not have binding patterns.").

##### Configuration requirements

The typings live in `lib.esnext.disposable.d.ts`. `lib: ["esnext"]` includes it; `lib: ["es2022"]` alone does not:

```text
error TS2318: Cannot find global type 'Disposable'.
error TS2339: Property 'dispose' does not exist on type 'SymbolConstructor'.
error TS2304: Cannot find name 'DisposableStack'.
```

Recommended:

```json
{
  "compilerOptions": {
    "target": "es2022",
    "lib": ["es2022", "esnext.disposable"]
  }
}
```

##### Emit and runtime support (verified)

- With `target` ≤ `es2022`, 5.2 down-levels `using` into `try`/`catch`/`finally` blocks that call the helpers `__addDisposableResource` and `__disposeResources` (inlined, or imported from `tslib` ≥ 2.6.0 with `importHelpers`). The helper looks up `Symbol.dispose`/`Symbol.asyncDispose` **at run time** and throws `TypeError: Symbol.dispose is not defined.` when they are missing. It contains its own `SuppressedError` fallback, so `SuppressedError` itself need not be polyfilled.
- With `target: "esnext"`, 5.2 emits `using` **verbatim**. Node.js 20.20.2 and 22.23.4 reject that output with `SyntaxError`; Node.js 24.22.1 and 26.12.1 execute it natively. Never combine `target: "esnext"` with `using` unless every runtime is Node.js 24+ (or a bundler down-levels it).

| Runtime (verified) | `Symbol.dispose` / `asyncDispose` | `DisposableStack`, `AsyncDisposableStack`, `SuppressedError` | native `using` syntax |
|---|---|---|---|
| Node.js 16.20.2 | missing | missing | no |
| Node.js 18.20.8, 20.20.2, 22.23.4 | present | missing | no |
| Node.js 24.22.1, 26.12.1 | present | present | yes |

So down-levelled `using` of your own `Disposable` classes runs on Node.js 18+ without polyfills, but **`new DisposableStack()` throws `ReferenceError: DisposableStack is not defined` on Node.js 18, 20 and 22** even though it type-checks. Browsers need the symbols polyfilled.

##### Polyfilling — the release-notes snippet does not type-check

The 5.2 release notes suggest `Symbol.dispose ??= Symbol("Symbol.dispose");`. In a `.ts` file with `esnext.disposable` this fails, because the lib declares the symbols `readonly`:

```ts
// flags: --lib es2022,esnext.disposable
Symbol.dispose ??= Symbol("Symbol.dispose"); // error TS2540
```

Write the polyfill through a widened view (or in a `.js` file, or use `core-js`), and load it before any module that uses `using`:

```ts
// flags: --lib es2022,esnext.disposable
// polyfills.ts — imported first by the entry point
const S = Symbol as { dispose?: symbol; asyncDispose?: symbol; metadata?: symbol };
S.dispose ??= Symbol("Symbol.dispose");
S.asyncDispose ??= Symbol("Symbol.asyncDispose");
S.metadata ??= Symbol("Symbol.metadata");
export {};
```

For `DisposableStack`/`AsyncDisposableStack` on Node.js < 24 or in browsers, use a full polyfill (for example `core-js/proposals/explicit-resource-management` or `@esfx/disposable`), or avoid the stacks and use `try`/`finally`. In ES modules, imports are hoisted: put the polyfill in its own module and import it first, never as a statement in the same file that defines the disposables.

##### `DisposableStack`, `adopt` and `defer`

`DisposableStack` collects resources whose lifetime is not tied to one declaration and turns non-disposable APIs into disposable ones:

```ts
// flags: --lib es2022,esnext.disposable,dom
import { openSync, closeSync, fstatSync } from "node:fs";

export function sizeOf(path: string): number {
  using stack = new DisposableStack();
  const fd = stack.adopt(openSync(path, "r"), (fd) => closeSync(fd)); // adapt a plain API
  stack.defer(() => console.log("done with", path));                 // run arbitrary cleanup
  return fstatSync(fd).size; // fd is closed after the return value is computed
}

export function openBoth(a: string, b: string): DisposableStack {
  using stack = new DisposableStack();
  stack.adopt(openSync(a, "r"), closeSync);
  stack.adopt(openSync(b, "r"), closeSync); // if this throws, the first fd is still closed
  return stack.move(); // success: hand ownership to the caller, nothing is disposed here
}
```

`move()` is the idiom for constructors and factories that acquire several resources: on failure everything acquired so far is released, on success ownership moves out.

##### Errors during disposal — `SuppressedError`

If the body throws and a disposer also throws, the result is a `SuppressedError` whose `error` is the newer (disposal) error and whose `suppressed` is the one it replaced. Verified with two throwing resources `a`, `b` and a throwing body: the caught error is `SuppressedError(error: "dispose a", suppressed: SuppressedError(error: "dispose b", suppressed: "body"))`. Consequences for review:

- `catch (e) { if (e instanceof MyError) … }` around a `using` scope can miss the original error, which is now nested inside `suppressed`. Log the whole chain.
- `SuppressedError` is typed with `error: any` and `suppressed: any` in 5.2 — narrow before use.
- Disposers should not throw for expected conditions (closing an already-closed handle); make them idempotent.

##### Review rules

- Resources acquired in a function (file handles, sockets, locks, DB connections, timers, subscriptions, temp directories) belong in `using`/`await using` or `try`/`finally` — never left to garbage collection or to a `close()` call that an early `return` can skip.
- `using x = await acquire()` is fine; `using x = acquire()` where `acquire` returns a **Promise** is a bug — a Promise is not `Disposable`, and 5.2 reports TS2850 only if the type is known. With `any` (untyped libraries) nothing is checked and the helper throws `TypeError: Object not disposable.` at run time.
- An `AsyncDisposable` must be bound with `await using`; `using` on it is TS2850 (verified).
- Check the tsconfig: `using` + `target: "esnext"` + Node.js < 24 is a guaranteed `SyntaxError`; `using` + `importHelpers` needs `tslib` ≥ 2.6.0; `DisposableStack` needs Node.js 24+ or a polyfill.

***

#### Decorator Metadata [New in 5.2]

Standard (TC39) decorators — available since 5.0 — receive a `context` object. In 5.2 every context gains a `metadata` property: one plain object per class, shared by all decorators of that class, and exposed afterwards as `TheClass[Symbol.metadata]`.[^1][^15]

```ts
// flags: --lib es2022,esnext,dom
const serializables = Symbol("serializables");

export function serialize<This, V>(_target: undefined, context: ClassFieldDecoratorContext<This, V>): void {
  const names = (context.metadata[serializables] ??= []) as string[];
  names.push(String(context.name));
}

export function jsonify(instance: object): string {
  const ctor = instance.constructor as { [Symbol.metadata]?: DecoratorMetadata | null };
  const names = (ctor[Symbol.metadata]?.[serializables] ?? []) as string[];
  const out: Record<string, unknown> = {};
  for (const n of names) out[n] = (instance as Record<string, unknown>)[n];
  return JSON.stringify(out);
}

export class Person {
  @serialize name: string;
  @serialize age: number;
  passwordHash = "never serialised";
  constructor(name: string, age: number) {
    this.name = name;
    this.age = age;
  }
}
```

Requirements and pitfalls (verified):

- **Typing depends on `lib`.** 5.2 always types `context.metadata` as `DecoratorMetadata`, defined as `DecoratorMetadataObject` when the lib declares `Symbol.metadata` (`esnext` or `esnext.decorators`) and as `DecoratorMetadataObject | undefined` otherwise. With `lib: ["es2022"]` the compiler therefore forces an `undefined` check; with `esnext` it does not.
- **Runtime.** The emitted helper creates the metadata object only when `Symbol.metadata` exists. No Node.js version up to 26.12.1 defines `Symbol.metadata`. Without a polyfill, the example above compiles cleanly with `lib: ["esnext"]` and then throws `TypeError: Cannot read properties of undefined (reading 'Symbol(serializables)')` while the class is being defined — verified on Node.js 24 and 26. With `Symbol.metadata ??= …` loaded first (see the polyfill above), it prints `{"name":"Ada","age":36}` on Node.js 18, 20, 22 and 24.
- Metadata is shared **per class**, and a subclass's metadata object inherits from the parent's via its prototype. Writing to an inherited array (`context.metadata[key].push(…)`) mutates the parent's list. Copy before mutating in class hierarchies.
- Prefer a module-private `symbol` or a `WeakMap` keyed by `context.metadata` over string keys, so unrelated libraries cannot collide or read the data.
- Decorator metadata is a **standard-decorators** feature. `emitDecoratorMetadata` (type metadata via `reflect-metadata`) belongs to `experimentalDecorators` and is unrelated.

***

#### Smaller Language and Library Additions in 5.2

##### Mixing labeled and unlabeled tuple elements [New in 5.2]

```ts
type Pair<T> = [first: T, T];                          // 5.1: TS5084
type AtLeastTwo<T> = [first: T, second: T, ...T[]];    // rest element no longer needs a label

type HasLabels = [a: string, b: string];
type HasNoLabels = [number, number];
type Merged = [...HasNoLabels, ...HasLabels];          // [number, number, a: string, b: string]

export const p: Pair<number> = [1, 2];
export const q: AtLeastTwo<string> = ["a", "b", "c"];
export type { Merged };
```

Labels are documentation only (they appear in signatures and editor hints); they never affect assignability.

##### Method calls on unions of arrays [New in 5.2]

Before 5.2, calling an overloaded generic method on `string[] | number[]` failed with TS2349 "This expression is not callable." 5.2 treats the union as an array of the element union for that call:

```ts
declare const ids: string[] | number[];

const nonEmpty = ids.filter((x) => !!x);                 // (string | number)[]
const hit = ids.find((x) => x === 1);                    // string | number | undefined
const lens = ids.reduce((acc: number, x) => acc + String(x).length, 0);
const same: string[] | number[] = nonEmpty;              // error TS2322
export { hit, lens, same };
```

Review note: the result is `(string | number)[]`, not `string[] | number[]`. Code that later relies on homogeneity (for example passing the result to an API that expects `number[]`) needs an explicit narrowing (`Array.isArray` plus a `typeof` check of the first element is **not** sufficient — check every element or keep the union discriminated, for example `{ kind: "ids"; values: number[] } | { kind: "names"; values: string[] }`).

##### Copying array methods (ES2023) [New in 5.2]

`toSorted`, `toReversed`, `toSpliced` and `with` return a **new** array and leave the receiver unchanged. 5.2 adds them to `Array`, `ReadonlyArray` and every typed array in `lib.es2023.array.d.ts` (so `lib: ["es2023"]` or `esnext` is needed):

```ts
export function topThree(scores: readonly number[]): number[] {
  return scores.toSorted((a, b) => b - a).slice(0, 3); // works on readonly arrays; sort() would not compile
}

const nums = [3, 1, 2];
export const sorted = nums.toSorted((a, b) => a - b); // nums is still [3, 1, 2]
export const replaced = nums.with(0, 9);              // [9, 1, 2]
export const removed = nums.toSpliced(0, 1);          // [1, 2]
export const reversed = new Int32Array([1, 2]).toReversed();
```

- Runtime: present in Node.js 20+ (verified on 20.20.2), **missing in Node.js 18** (`[].toSorted` is `undefined` on 18.20.8). A Node.js 18 project that enables `lib: ["es2023"]` type-checks code that throws `TypeError: nums.toSorted is not a function`. Match `lib` to the oldest runtime.
- Prefer `toSorted()` over `[...arr].sort()` and over in-place `sort()` on arrays received as parameters — in-place `sort()`/`reverse()`/`splice()` on an input array is a classic aliasing bug.
- `with(index, value)` throws `RangeError` for an out-of-range index instead of growing the array.

##### Symbols as `WeakMap`/`WeakSet` keys [New in 5.2]

5.2 introduces `WeakKey` (`object | symbol` with `lib` `es2023`, `object` otherwise) as the constraint of `WeakMap`, `WeakSet`, `WeakRef` and `FinalizationRegistry#register`:

```ts
const token = Symbol("request");
const cache = new WeakMap<symbol, string>();
cache.set(token, "payload");

cache.set(Symbol.for("shared"), "x"); // compiles, but throws TypeError at run time
export { cache };
```

- With `lib: ["es2022"]`, `WeakMap<symbol, …>` is still TS2344 in 5.2 — the relaxation comes from `lib.es2023.collection.d.ts`.
- **Registered symbols** (`Symbol.for(...)`) can never be collected; the runtime rejects them (`TypeError: Invalid value used as weak map key`, verified on Node.js 18–22). The type system cannot tell registered from unique symbols, so review every `Symbol.for` that flows into a weak collection.
- Runtime support: Node.js 18.20.8 and later accept unique symbols; Node.js 16 rejects them.

##### Type-only import paths with TypeScript extensions [New in 5.2]

`import type` declarations and `import("…")` types (including in JSDoc) may name `.ts`, `.mts`, `.cts` and `.tsx` files even without `allowImportingTsExtensions`, because they are erased:

```ts
// project: src/justTypes.ts exports `interface JustAType` and `const value`
import type { JustAType } from "./justTypes.ts";      // OK in 5.2 (5.1: TS5097)
type Other = import("./justTypes.ts").JustAType;      // OK in 5.2
import { value } from "./justTypes.ts";              // still TS5097 unless allowImportingTsExtensions
```

Value imports with `.ts` extensions still require `allowImportingTsExtensions`, which in turn requires `noEmit` or `emitDeclarationOnly` (TS5096). For code that `tsc` emits and Node.js runs, keep writing `.js` specifiers; 5.2 cannot rewrite `.ts` to `.js` (`rewriteRelativeImportExtensions` is 5.7).

##### Editor features [New in 5.2]

- **Comma completions**: member completions are offered after an object property even when the separating comma is missing, and accepting one inserts the comma.
- **Inline variable** refactoring replaces every use of a variable with its initializer (verified via the language service: 5.2.2 offers "Inline variable", 5.1.6 does not). Review inlined code: the initializer's side effects now run at each former use site and possibly more than once.
- **Clickable inlay parameter hints** jump to the parameter declaration.

***

#### Breaking Changes and Correctness Fixes in 5.2

##### `module` and `moduleResolution` must match for Node.js modes

If either option is `node16` or `nodenext`, the other must also be a Node.js mode. Verified on 5.2.2 (all accepted silently by 5.1.6):

```text
--module esnext   --moduleResolution node16    error TS5110: Option 'module' must be set to 'Node16' when option 'moduleResolution' is set to 'Node16'.
--module nodenext --moduleResolution node      error TS5109: Option 'moduleResolution' must be set to 'NodeNext' (or left unspecified) when option 'module' is set to 'NodeNext'.
--module commonjs --moduleResolution nodenext  error TS5110: Option 'module' must be set to 'NodeNext' when option 'moduleResolution' is set to 'NodeNext'.
```

Fixes: `module: "nodenext"` alone (it implies `moduleResolution: "nodenext"`), or `module: "esnext"` + `moduleResolution: "bundler"` for bundled code. `module: "node16"` with `moduleResolution: "nodenext"` is still accepted.

##### Consistent export checking for merged symbols

Declarations that merge (a function and a namespace with the same name, for example) must all be exported or all be local — now also inside ambient contexts such as `.d.ts` files and `declare module` blocks:

```ts
// file: merged.d.ts
// flags: --skipLibCheck false
declare module "replace-in-file" {
  export function replaceInFile(config: unknown): Promise<unknown[]>; // error TS2395
  export {};
  namespace replaceInFile { // error TS2395
    export function sync(config: unknown): unknown[];
  }
}
```

Fix: `export namespace replaceInFile { … }`. Hand-written declaration files and `declare module` augmentations break on upgrade from 5.1; `skipLibCheck` hides the error only for declaration files.

##### Namespaces are always emitted as `namespace`

Declaration emit now writes `export declare namespace Legacy` where 5.1 wrote `export declare module Legacy` (verified). Source code may still use the `module Legacy { }` keyword form in 5.2 without any diagnostic, but write `namespace` — later compilers deprecate the keyword form (6.0 makes it an error).

##### `lib.d.ts` (DOM) changes

The DOM typings were regenerated from current web specifications. Expect occasional new errors in browser code that relied on removed or renamed DOM members; fix the code, do not pin an old `lib`.

##### Compiler API: `labeledElementDeclarations` may hold `undefined`

Because tuples can mix labels, `TupleType.labeledElementDeclarations` is now `readonly (NamedTupleMember | ParameterDeclaration | undefined)[]`. Tools built on the compiler API must handle `undefined` entries.

***
#### tsconfig.json: Defaults, Profiles and Strictness

##### Defaults in 5.2 you must override

Verified with `tsc` 5.2.2 (emit, `--traceResolution`) and the TSConfig reference.[^16]

| Option | 5.2 default | Why it matters |
|---|---|---|
| `target` | `es5` — but `es2022` when `module` is `node16`, `esnext` when `module` is `nodenext` | `es5` down-levels classes and rejects `#private` (TS18028). The implied `esnext` emits `using` and decorators verbatim (verified), which Node.js < 24 cannot parse. |
| `module` | `commonjs` for `target` `es3`/`es5`, otherwise `es2015` | A config with only `"target": "es2022"` emits **ES modules** (verified) — wrong for a CommonJS package. |
| `moduleResolution` | `node10` for `commonjs`; **`classic`** for `es2015`/`es2020`/`es2022`/`esnext`; `node16`/`nodenext` for those modules | Verified with `--traceResolution`: `"module": "esnext"` without `moduleResolution` resolves with `Classic`, which never looks into `node_modules` package folders — `@types/node` 22 then fails with TS2792 "Cannot find module 'undici-types'". `classic` and `node10` also ignore `package.json` `exports`. Always set `moduleResolution` (or use `nodenext`). |
| `strict` | `false` | Every serious project sets it. |
| `useDefineForClassFields` | `true` when `target` ≥ `es2022`, else `false` | Changes class-field semantics (see Classes). |
| `esModuleInterop` | `false`, but implied `true` when `module` is `node16`/`nodenext` (verified in the 5.2 compiler source, `getESModuleInterop`)[^16] | Default imports of CommonJS packages behave differently. |
| `types` | every package under `node_modules/@types` | Globals from unrelated `@types` packages leak in; list them explicitly. |
| `lib` | derived from `target` | With `target: "es2022"` the default lib has no `toSorted` and no `Disposable`. |

`tsc --init` in 5.2 writes only `target: "es2016"`, `module: "commonjs"`, `esModuleInterop`, `forceConsistentCasingInFileNames`, `strict` and `skipLibCheck` as active settings (verified) — a starting point, not a production config.

##### Node.js application or library run directly by Node.js (ESM or CJS)

```json
{
  "compilerOptions": {
    "module": "nodenext",
    "target": "es2022",
    "lib": ["es2023", "esnext.disposable"],
    "types": ["node"],
    "rootDir": "./src",
    "outDir": "./dist",
    "strict": true,
    "noUncheckedIndexedAccess": true,
    "exactOptionalPropertyTypes": true,
    "noImplicitOverride": true,
    "noImplicitReturns": true,
    "noFallthroughCasesInSwitch": true,
    "verbatimModuleSyntax": true,
    "isolatedModules": true,
    "forceConsistentCasingInFileNames": true,
    "skipLibCheck": true,
    "sourceMap": true,
    "declaration": true,
    "declarationMap": true
  },
  "include": ["src"]
}
```

- `module: "nodenext"` implies `moduleResolution: "nodenext"`, `esModuleInterop` and — unless set — `target: "esnext"`. **Always set `target` explicitly** with `nodenext`: the implied `esnext` emits `using` declarations unchanged (verified with 5.2.2), and Node.js 20/22 then fail with `SyntaxError`.
- `target: "es2022"` is the highest target 5.2 offers besides `esnext`; it runs on Node.js 16.11+ syntax-wise. Pick `lib` from the **oldest supported runtime**: `es2023` (adds `findLast` and the copying array methods) requires Node.js 20+; for Node.js 18 use `es2022` and do not use `toSorted`/`with`.
- The output format of each file follows the nearest `package.json` `"type"` and the `.mts`/`.cts` extensions.
- `verbatimModuleSyntax` + `isolatedModules` keep the code compatible with single-file transpilers (esbuild, SWC, `tsx`, Vitest).

##### Bundled application (Vite, esbuild, webpack, Rollup)

```json
{
  "compilerOptions": {
    "module": "esnext",
    "moduleResolution": "bundler",
    "target": "es2022",
    "lib": ["es2023", "dom", "dom.iterable", "esnext.disposable"],
    "types": ["vite/client"],
    "noEmit": true,
    "strict": true,
    "noUncheckedIndexedAccess": true,
    "verbatimModuleSyntax": true,
    "isolatedModules": true,
    "allowImportingTsExtensions": true,
    "resolveJsonModule": true,
    "jsx": "react-jsx",
    "skipLibCheck": true
  },
  "include": ["src"]
}
```

- `moduleResolution: "bundler"` (5.0+) requires `module` `es2015` or later (TS5095 with `commonjs` in 5.2). `module: "preserve"` does not exist before 5.4.
- `allowImportingTsExtensions` requires `noEmit` or `emitDeclarationOnly` (TS5096, verified).
- `dom.iterable` must be listed explicitly in 5.2 for `for (const n of nodeList)` and `headers.entries()`.

##### Shared and layered configs

`extends` accepts an array since 5.0 (`"extends": ["@tsconfig/strictest/tsconfig.json", "./tsconfig.base.json"]`); later entries override earlier ones. `${configDir}` template paths do not exist before 5.5 — relative paths in an extended config are resolved relative to **that** config file, a common source of wrong `outDir`/`include` paths in monorepos.

##### Strictness flags

`strict: true` enables `noImplicitAny`, `strictNullChecks`, `strictFunctionTypes`, `strictBindCallApply`, `strictPropertyInitialization`, `noImplicitThis`, `useUnknownInCatchVariables` and `alwaysStrict`. Worth enabling on top:

| Option | Since | What it catches |
|---|---|---|
| `noUncheckedIndexedAccess` | 4.1 | `arr[i]` and `record[key]` are `T \| undefined` |
| `exactOptionalPropertyTypes` | 4.4 | `{ a?: string }` rejects an explicit `a: undefined`; requires `strictNullChecks` (TS5052) |
| `noImplicitOverride` | 4.3 | overriding members must say `override` |
| `noPropertyAccessFromIndexSignature` | 4.2 | forces `obj["key"]` for index-signature properties |
| `noImplicitReturns`, `noFallthroughCasesInSwitch` | — | missing returns, fall-through `case`s |
| `verbatimModuleSyntax` | 5.0 | type imports must say `type`; ESM syntax rejected in CommonJS output |
| `isolatedModules` | — | constructs single-file transpilers cannot compile |
| `allowUnreachableCode: false`, `allowUnusedLabels: false` | — | dead code and stray labels become errors |
| `forceConsistentCasingInFileNames` | — | `./User` vs `./user` imports that break on Linux |

A project that sets `"strict": false` or disables individual strict flags must justify it; review new code in such a project as if `strictNullChecks` were on.

***

#### The 5.0 Deprecations (still accepted with `ignoreDeprecations: "5.0"`)

TypeScript 5.0 deprecated a set of options. In 5.0–5.4 they are errors unless `"ignoreDeprecations": "5.0"` is set; 5.5 removed them completely.[^5] Verified on 5.2.2:

| Option | 5.2 without escape hatch | Replacement |
|---|---|---|
| `"target": "es3"` | TS5107 "Option 'target=ES3' is deprecated and will stop functioning in TypeScript 5.5." | `es5` or later |
| `importsNotUsedAsValues` (any value except the default `remove`), `preserveValueImports` | TS5101 | `verbatimModuleSyntax` |
| `keyofStringsOnly` | TS5101 | `string & keyof T` / `Extract<keyof T, string>` |
| `suppressImplicitAnyIndexErrors` | TS5101 | fix the index signatures or use `noUncheckedIndexedAccess` |
| `suppressExcessPropertyErrors` | TS5101 | fix the object literals |
| `noImplicitUseStrict`, `noStrictGenericChecks` | TS5101 | delete the option |
| `charset` | TS5101 | delete (files are UTF-8) |
| `out` | TS5101 | `outFile` or a bundler |
| `prepend` in project references | deprecation error | a bundler |

`"ignoreDeprecations": "5.0"` silences all of them; any other value (for example `"6.0"`) is TS5103 "Invalid value for '--ignoreDeprecations'." in 5.2. A tsconfig that needs `ignoreDeprecations` cannot be compiled by TypeScript 5.5 or later — flag it as a blocker for every compiler upgrade, not as a cosmetic issue.

***

#### Modules and Module Resolution

##### Choosing `module` / `moduleResolution` in 5.2

| Situation | `module` | `moduleResolution` |
|---|---|---|
| Code run directly by Node.js (ESM or CJS) | `nodenext` (or `node16`) | implied |
| Library published to npm | `nodenext` | implied |
| Code consumed by a bundler, Bun, or a framework build | `esnext` | `bundler` |
| Legacy CommonJS project | `commonjs` | `node10` (alias `node`) — migrate to `nodenext` |

`node16` and `nodenext` behave identically in 5.2 apart from the implied target; `nodenext` will track future Node.js behaviour when the compiler is upgraded. `bundler` with `module: commonjs` is TS5095 in 5.2 (allowed only from 6.0).

##### ESM rules under `nodenext`

```ts
// project: package.json {"type":"module"}, module nodenext, verbatimModuleSyntax — compiled with 5.2.2
import { one } from "./util";            // error TS2835: Relative import paths need explicit file extensions in EcmaScript imports when '--moduleResolution' is 'node16' or 'nodenext'. Did you mean './util.js'?
import { Id } from "./util.js";           // error TS1484: 'Id' is a type and must be imported using a type-only import when 'verbatimModuleSyntax' is enabled.
```

Correct:

```ts
// project: same setup
import { one } from "./util.js";          // .js even though the source file is util.ts
import type { Id } from "./util.js";
```

- The `.js` extension names the **output** file. 5.2 cannot rewrite `.ts` specifiers on emit.
- In a CommonJS-format file (`.cts`, or `.ts` without `"type": "module"`), `verbatimModuleSyntax` rejects `import … from` (TS1286 "ESM syntax is not allowed in a CommonJS module when 'verbatimModuleSyntax' is enabled.") and `export const` (TS1287). Use `import x = require("x")` and `export =`, or switch the file to ESM.
- A CommonJS file importing an ES module is TS1479 in 5.2 ("…the referenced file is an ECMAScript module and cannot be imported with 'require'. Consider writing a dynamic 'import("./util.js")' call instead."); the `import x = require("./util.js")` form is TS1471 ("…The specifier only resolves to an ES module, which cannot be imported with 'require'."). TypeScript only models `require(esm)` from 5.8; on 5.2 use `await import()`.
- `__dirname`, `__filename` and `require` do not exist in ESM. Use `fileURLToPath(new URL(".", import.meta.url))`, or `import.meta.dirname`/`import.meta.filename` (Node.js 20.11+; typed by `@types/node` 20.11+, not by TypeScript itself).

##### JSON modules: the 5.2 trap

TypeScript 5.2 can only parse **import assertions** (`assert { type: "json" }`); import **attributes** (`with { type: "json" }`) need 5.3. Under `nodenext` 5.2 also accepts a JSON import **without** any assertion. Verified with `resolveJsonModule`, emitted ESM run on each runtime:

| Emitted import | Node.js 18 | Node.js 20 | Node.js 22 / 24 |
|---|---|---|---|
| `import data from "./data.json" assert { type: "json" }` | works | works | `SyntaxError: Unexpected identifier 'assert'` |
| `import data from "./data.json"` (accepted by 5.2) | `ERR_IMPORT_ASSERTION_TYPE_MISSING` | `ERR_IMPORT_ASSERTION_TYPE_MISSING` | `ERR_IMPORT_ATTRIBUTE_MISSING` |

So **no static JSON import form that 5.2 accepts runs on Node.js 22 or later**[^17], and every form type-checks. Load JSON explicitly instead:

```ts
// flags: --module nodenext --moduleResolution nodenext
// file: config.mts
import { createRequire } from "node:module";
import { readFile } from "node:fs/promises";

const require = createRequire(import.meta.url);
export const pkg = require("../package.json") as { name: string; version: string };

export async function loadConfig(path: URL): Promise<unknown> {
  return JSON.parse(await readFile(path, "utf8")) as unknown; // validate before use
}
```

Bundled code (`moduleResolution: "bundler"`) is unaffected: bundlers inline JSON imports without attributes.

##### `verbatimModuleSyntax` and `isolatedModules`

`verbatimModuleSyntax` (5.0+) means "what you write is what is emitted": imports without `type` stay, `import type` disappears. It replaces the deprecated `importsNotUsedAsValues`/`preserveValueImports`.

```ts
// flags: --verbatimModuleSyntax --isolatedModules
import type { Server } from "node:http";                          // erased
import { createServer, type IncomingMessage } from "node:http";   // createServer kept
import "./register-side-effects.js";                              // kept

export function handler(req: IncomingMessage): Server {
  return createServer(() => req.socket.destroy());
}
```

```ts
// flags: --verbatimModuleSyntax --isolatedModules
export type { IncomingMessage } from "node:http"; // re-exporting a type must say `type`
export type * from "node:url";                    // `export type *` is 5.0+
```

- `import { type A, type B } from "x"` keeps an empty `import {} from "x"` — i.e. the module still executes — whereas `import type { A, B } from "x"` removes the import. Choose deliberately when the module has side effects.
- Do not suggest `importsNotUsedAsValues`/`preserveValueImports` in 5.2 code: deprecated (TS5101).

##### `paths` and `baseUrl`

`paths` affects **type checking only**. Confirm that the runtime has a matching alias (bundler `resolve.alias`, test runner `moduleNameMapper`/`alias`, or `package.json` `imports` with a named prefix such as `#app/*`). In 5.2 `paths` entries without `baseUrl` are resolved relative to the tsconfig; `baseUrl` still works and also acts as a lookup root for bare specifiers — a source of accidental resolution of `import "utils"` to `src/utils`. Prefer `paths` with `./`-relative targets and no `baseUrl` (6.0 deprecates `baseUrl`).

`package.json` `imports` must use a named prefix (`"#app/*"`) in 5.2; the bare `#/` prefix resolves only from TypeScript 6.0 (verified: 5.2.2 and 5.9.3 report TS2307 for `#/util/greet.js`, while `#app/util/greet.js` resolves).

##### Module detection and `.mts`/`.cts`

- `moduleDetection: "force"` treats every non-declaration file as a module. With the default `auto`, a file without `import`/`export` is a global **script** whose top-level declarations collide with other scripts (`Cannot redeclare block-scoped variable`). 
- `.mts` always emits `.mjs` (ESM); `.cts` always emits `.cjs` (CommonJS).
- `allowArbitraryExtensions` (5.0) lets `import styles from "./app.css"` resolve to a hand-written `app.d.css.ts`.

##### Dual packages and `exports`

```json
{
  "name": "my-lib",
  "type": "module",
  "exports": {
    ".": {
      "types": "./dist/index.d.ts",
      "import": "./dist/index.js"
    }
  },
  "files": ["dist"]
}
```

- `"types"` must come **first** inside each condition object; TypeScript picks the first matching condition.
- A package that ships both ESM and CJS needs separate declaration files per format (`index.d.mts`/`index.d.cts`), otherwise consumers under `nodenext` see the wrong module kind ("masquerading as ESM/CJS"). Check with `@arethetypeswrong/cli`.
- `customConditions` (5.0) applies only with `moduleResolution` `node16`/`nodenext`/`bundler` (TS5098 otherwise, verified).

***
#### Types: Fundamentals

Global truths that hold in 5.2:

- **Types are erased.** Annotations, interfaces, type aliases and generics produce no JavaScript. Nothing is validated at run time unless code does it — `JSON.parse`, `response.json()`, `process.env`, `req.body`, message-queue payloads and database rows are all untrusted.
- **Typing is structural.** Two types with the same shape are interchangeable regardless of name. Nominal behaviour needs brands or `#private` fields.
- **`strict` is the baseline.** Code review assumes `strictNullChecks`, `noImplicitAny`, `strictFunctionTypes`, `strictPropertyInitialization` and `useUnknownInCatchVariables`.
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
raw.port; // error TS18046
```

At trust boundaries prefer a schema validator (Zod, Valibot, ArkType, TypeBox) whose output type is derived from the schema. Check the validator's documented minimum TypeScript version first: Zod 4, for example, is tested only against TypeScript 5.5 and later, so a 5.2 project may have to stay on an older major.[^20] A hand-written guard that checks fewer fields than the type declares is a lie the compiler cannot detect.

##### Type assertions and non-null assertions

`x as T` and `x!` are unchecked promises to the compiler. Review each one:

```ts
interface User { id: string; name: string }

const u1 = {} as User;               // compiles; u1.name is undefined at run time
const u2 = "x" as unknown as User;   // double assertion: always a smell

declare const map: Map<string, User>;
const a = map.get("id")!.name;       // throws if absent — prefer an explicit check
const b = map.get("id")?.name ?? "anonymous";

const n = "42" as number; // error TS2352
export { u1, u2, a, b, n };
```

Prefer `satisfies` (checks without widening or lying), annotations, or narrowing. Never use angle-bracket assertions (`<User>x`): they are a syntax error in `.tsx` files and confuse readers. Use `as`.

##### `interface` vs `type`

| Use case | Prefer |
|---|---|
| Object shapes, class contracts, public library APIs | `interface` (extendable, mergeable, clearer errors, cached relations) |
| Unions, tuples, function types, mapped and conditional types | `type` |
| Augmenting a third-party or global type | `interface` (declaration merging) |

`interface A extends B` reports incompatible members; an intersection silently produces `never` members:

```ts
type Broken = { id: string } & { id: number };
const x: Broken = { id: "a" }; // error TS2322

interface Base { id: string }
interface Child extends Base { id: number } // error TS2430
export { x };
```

##### Optional properties and `exactOptionalPropertyTypes`

```ts
// flags: --exactOptionalPropertyTypes
interface Options { timeoutMs?: number }

const o1: Options = {};                       // OK: property absent
const o2: Options = { timeoutMs: undefined }; // error TS2375

interface Patch { timeoutMs?: number | undefined } // explicitly allow undefined when "unset" must be expressible
const p: Patch = { timeoutMs: undefined };
export { o1, o2, p };
```

`"key" in obj` and `obj.key !== undefined` mean different things under this flag. Spreading `{ ...defaults, ...overrides }` where `overrides` carries an explicit `undefined` overwrites the default — a common configuration bug.

##### Index access and `noUncheckedIndexedAccess`

```ts
// flags: --noUncheckedIndexedAccess
const ports: number[] = [80, 443];
const first = ports[0];          // number | undefined
first.toFixed();                 // error TS18048

const env: Record<string, string> = { HOME: "/root" };
const home = env["HOME"] ?? "/";  // string

for (const p of ports) p.toFixed();   // iteration is not affected
const [a = 0, b = 0] = ports;          // destructuring with defaults
export { home, a, b };
```

Without the flag, `ports[5]` is typed `number` and `record[key]` is never `undefined` — treat every unchecked index as potentially missing.

##### Readonly data

```ts
interface Settings {
  readonly name: string;
  readonly tags: readonly string[];
}
const s: Settings = { name: "a", tags: ["x"] };
s.name = "b";        // error TS2540
s.tags.push("y");    // error TS2339

const ROLES = ["admin", "user"] as const;   // readonly ["admin", "user"]
type Role = (typeof ROLES)[number];         // "admin" | "user"
const r: Role = "admin";
export const sortedTags = s.tags.toSorted(); // 5.2: copying methods exist on ReadonlyArray
export { r };
```

`readonly` is shallow and compile-time only; `Object.freeze` is shallow and run-time only. Use `readonly T[]` for every array parameter the function does not mutate. A mutable array is assignable to a `readonly` array type, not the other way round.

##### Excess property checks and structural typing

```ts
interface Point { x: number; y: number }

const p1: Point = { x: 1, y: 2, z: 3 }; // error TS2322: … Object literal may only specify known properties

const tmp = { x: 1, y: 2, z: 3 };
const p2: Point = tmp;                   // OK: excess checks apply only to fresh object literals

function save(p: Point): string { return JSON.stringify(p); }
save(tmp);                               // serialises z too — a data-leak vector for DTOs
export { p1, p2 };
```

When an object is serialised or sent to a client, copy the allowed fields explicitly or validate with a strict schema; a structural type does not strip extra properties.

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
loadUser(oid); // error TS2345
```

##### `object`, `{}`, `Object`, `unknown`

- `object` — any non-primitive.
- `{}` — any value except `null`/`undefined`, including strings and numbers. Rarely what is meant.
- `Object` — the wrapper interface; never use as a type.
- `Record<string, unknown>` — a dictionary whose values must be checked.

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
inc(1); // error TS2684

// A callback returning a value where void is expected is allowed — a source of ignored promises
const handlers: Array<() => void> = [];
handlers.push(async () => { await Promise.resolve(); }); // OK for tsc; lint with no-misused-promises
```

- Prefer union parameters to overloads when the return type does not depend on the argument type.
- Prefer an options object over long positional parameter lists, especially booleans.
- Method-shorthand members (`m(x: T): void`) are checked **bivariantly** even under `strictFunctionTypes`; property-style members (`m: (x: T) => void`) are checked contravariantly. Use property style for callbacks and handlers.
- Since 5.1, a function whose return type is `undefined` may omit the `return` statement entirely; `void` and `undefined` are still different types (a `() => void` callback's result must not be used).

***

#### Narrowing and Control-Flow Analysis

##### Discriminated unions and exhaustiveness

```ts
type Shape =
  | { kind: "circle"; radius: number }
  | { kind: "square"; side: number }
  | { kind: "rect"; w: number; h: number };

export function area(s: Shape): number {
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
```

A `default:` that silently returns a fallback hides new variants — flag it. `s satisfies never` in the `default` branch works as well (4.9+).

##### Built-in narrowing

```ts
export function describe(v: string | number | Date | null | undefined | string[]): string {
  if (v == null) return "missing";                 // null and undefined
  if (typeof v === "string") return v.toUpperCase();
  if (typeof v === "number") return v.toFixed(2);
  if (v instanceof Date) return v.toISOString();
  return v.join(",");                               // string[]
}

const thing: object = { id: 1 };
if ("id" in thing) console.log(thing.id); // `in` narrows unlisted properties to unknown (4.9+)
```

Truthiness narrowing excludes `0`, `""`, `NaN` and `0n` — `if (count)` and `if (name)` are bugs when zero or empty are valid values. 5.2 has **no** "always truthy/nullish" diagnostics (TS2872/TS2869 are 5.6): `if (/regex/)` and `a < b ?? fallback` compile silently, so reviewers must catch them:

```ts
declare const input: string;
declare const limit: number;
export function check(): boolean {
  if (/^\d+$/) { /* BAD: always true — meant /^\d+$/.test(input) */ }
  if (input.trim) { } // error TS2774 — 5.2 does catch an uncalled method
  const tooShort = input.length < limit ?? true; // BAD: ?? never applies to a boolean
  return tooShort;
}
```

5.2 does report TS2774 ("This condition will always return true since this function is always defined. Did you mean to call it instead?") for an uncalled, non-optional function in an `if` condition whose body does not call it. It does **not** catch the regex literal or the `??` on a boolean (verified) — those checks arrive in 5.6.

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

Assertion functions must be called through an explicitly typed reference (`const check = assertIsDefined;` without an annotation produces TS2775). A type guard whose body does not prove its claim silently corrupts every caller.

##### What 5.2 does not narrow (and later versions do)

```ts
// 1. Closures after reassignment — preserved only from 5.4
export function late(url: string | URL, names: string[]) {
  if (typeof url === "string") {
    url = new URL(url);
  }
  return names.map((k) => url.searchParams.get(k)); // error TS2339
}

// 2. Constant indexed access — narrowed only from 5.5
export function byKey(obj: Record<string, unknown>, key: string) {
  if (typeof obj[key] === "string") {
    return obj[key].toUpperCase(); // error TS2571
  }
  return undefined;
}

// 3. Inferred type predicates — only from 5.5
const raw: Array<string | undefined> = ["a", undefined];
export const names: string[] = raw.filter((s) => s !== undefined); // error TS2322

// 4. switch (true) — only from 5.3
export function classify(v: unknown): string {
  switch (true) {
    case typeof v === "string":
      return v.toUpperCase(); // error TS18046
    default:
      return "other";
  }
}
```

The correct 5.2 idioms — and the ones to suggest in review:

```ts
export function late(input: string | URL, names: string[]) {
  const url = typeof input === "string" ? new URL(input) : input; // a new const binding
  return names.map((k) => url.searchParams.get(k));
}

export function byKey(obj: Record<string, unknown>, key: string) {
  const value = obj[key];                       // copy into a local
  return typeof value === "string" ? value.toUpperCase() : undefined;
}

const raw: Array<string | undefined> = ["a", undefined];
export const names = raw.filter((s): s is string => s !== undefined); // explicit predicate

export function classify(v: unknown): string {
  if (typeof v === "string") return v.toUpperCase();
  if (Array.isArray(v)) return `array(${v.length})`;
  return "other";
}
```

Do **not** "simplify" these back to the patterns above in a 5.2 project — they only compile on 5.4/5.5/5.3 respectively. Also beware of hand-written predicates that lie: `(s): s is string => !!s` also drops `""`.

##### Narrowing is not invalidated by calls

```ts
interface Job { state: "idle" | "running" }
export function start(job: Job, run: () => void) {
  if (job.state === "idle") {
    run();          // may set job.state = "running"
    job.state;      // still typed "idle" — TypeScript assumes calls do not mutate
  }
}
```

Re-read mutable state after callbacks, `await` points and event emissions instead of trusting an earlier narrowing.

***

#### Generics, Inference and `satisfies`

##### Constraints, defaults and when not to be generic

```ts
// GOOD: the type parameter relates input and output
export function firstOf<T>(items: readonly T[]): T | undefined {
  return items[0];
}

export function byId<T extends { id: string }>(items: readonly T[], id: string): T | undefined {
  return items.find((i) => i.id === id);
}

export interface Page<T, Cursor = string> {
  items: T[];
  next?: Cursor;
}

// BAD: T appears only in the return type — an unchecked cast chosen by the caller
function parseBad<T>(json: string): T {
  return JSON.parse(json);
}
export const user = parseBad<{ admin: boolean }>('{"admin":"no"}'); // "no" is truthy at run time
```

A type parameter used only in the return position is an assertion in disguise. Return `unknown` and validate.

##### `satisfies` (4.9+)

```ts
type Route = { path: string; method: "GET" | "POST" };

const routes = {
  list: { path: "/users", method: "GET" },
  create: { path: "/users", method: "POST" },
} satisfies Record<string, Route>;

routes.list.method;      // "GET" (literal kept)
routes.missing;          // error TS2339

const annotated: Record<string, Route> = routes;
annotated.missing;       // compiles: the annotation widened the type

export const bad = { path: "/x", method: "PUT" } satisfies Route; // error TS2322
```

Combine with `as const` for immutable configuration: `{ … } as const satisfies Config`.

##### `const` type parameters (5.0+)

```ts
export function tuple<const T extends readonly unknown[]>(...xs: T): T {
  return xs;
}
export const t = tuple("a", 1, true); // readonly ["a", 1, true]

export function routesOf<const T extends readonly string[]>(paths: T): T[number][] {
  return [...paths];
}
export const r = routesOf(["/a", "/b"]); // ("/a" | "/b")[]
```

Use `readonly` in the constraint; with a mutable constraint (`T extends unknown[]`) a literal array argument falls back to the wider type.

##### Blocking inference without `NoInfer`

`NoInfer<T>` is 5.4+. In 5.2 the established workaround is an extra type parameter constrained to the first:

```ts
export function createFsm<S extends string, I extends S>(states: S[], initial: I): { states: S[]; current: S } {
  return { states, current: initial };
}
createFsm(["idle", "running"], "idle");
createFsm(["idle", "running"], "stopped"); // error TS2345
```

Another 5.2-compatible form is `initial: S & {}` — the intersection lowers the inference priority of that position (verified: `"stopped"` is rejected with TS2345).

##### Variance annotations (`in`/`out`, 4.7+)

`interface Producer<out T>` and `interface Consumer<in T>` document and speed up variance checks. Never add them to silence an error: a wrong annotation makes unsound assignments compile.

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
type Ctor = ConstructorParameters<typeof URL>;
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
const b: Bad = { type: "key", key: "Enter" }; // error TS2322 (excess property 'key')
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
type F1 = FirstArg<(id: "x") => void>;        // "x" — `infer … extends` is 4.7+
export type { A, D, N, F1 };
```

Deeply recursive conditional types slow the checker and can hit TS2589 ("Type instantiation is excessively deep and possibly infinite"). Question type-level cleverness that a simpler explicit type would cover.

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
const bad: Key = "PATCH /api/users"; // error TS2322

type Parts<S extends string> = S extends `${infer Head}.${infer Rest}` ? [Head, ...Parts<Rest>] : [S];
type P = Parts<"a.b.c">; // ["a", "b", "c"]
export { k, bad };
export type { P };
```

##### Tuples, labels and variadic tuples

```ts
type Args = [id: string, retries?: number, ...tags: string[]];
declare function call(...args: Args): void;
call("a");
call("a", 3, "x", "y");

type Concat<A extends readonly unknown[], B extends readonly unknown[]> = [...A, ...B];
type C = Concat<[a: string], [number]>; // [a: string, number] — mixing labels is legal from 5.2
export type { C };
```

##### `keyof`, `typeof`, indexed access

```ts
const LEVELS = { debug: 10, info: 20, warn: 30, error: 40 } as const;
type Level = keyof typeof LEVELS;
type LevelValue = (typeof LEVELS)[Level];

export function threshold(l: Level): LevelValue {
  return LEVELS[l];
}

type Dict = { [k: string]: unknown };
type DK = keyof Dict; // string | number
export type { DK };
```

`Object.keys(obj)` returns `string[]`, not `(keyof T)[]`, because objects may carry extra properties at run time. Casting it to `(keyof T)[]` is only safe for objects the code created itself.

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

export class Savings extends Account {
  override deposit(amount: number): void { // `override` required under noImplicitOverride
    super.deposit(amount);
  }
}
```

- Prefer `#private` for real encapsulation; `private` is only a compile-time check (`(obj as any).legacyFlag` works, and `JSON.stringify` serialises it).
- `#private` requires `target` ≥ `es2015` (TS18028 with the default `es5`, verified) and is down-levelled to `WeakMap`s below `es2022`.
- Under `strictPropertyInitialization` every field needs an initializer, a constructor assignment, or `!` (definite assignment). Review each `!`.
- Since 5.1, a getter and setter may have unrelated types (`get size(): number` / `set size(v: string | number)`); use it for normalising setters, not to hide inconsistent state.

##### Class-field semantics (`useDefineForClassFields`)

With `target` ≥ `es2022` (or `esnext`), `useDefineForClassFields` defaults to `true` and fields use ECMAScript `[[Define]]` semantics. Subclass field initializers run **after** the base constructor:

```ts
class Base {
  name = "base";
  constructor() {
    this.init();
  }
  init(): void {}
}
export class Child extends Base {
  items: string[] = [];
  override init(): void {
    this.items.push("x"); // runs during super(): items is still undefined → TypeError at run time
  }
}
```

Never call overridable methods from a base constructor. Re-declaring an inherited property without an initializer resets it to `undefined` under `[[Define]]`; 5.2 reports TS2612 for that case — use `declare name: Narrower;` to re-type an inherited field without emitting it:

```ts
class Animal { kind: string = "animal"; }
class Dog extends Animal {
  kind: "dog"; // error TS2612 (plus TS2564 under strictPropertyInitialization)
}
class Cat extends Animal {
  declare kind: "cat"; // OK: type-only redeclaration, nothing emitted
}
export { Dog, Cat };
```

Changing `target` from `es2019` to `es2022` silently switches the semantics of every class field in a code base; frameworks that rely on accessors defined by a base class (older MobX, Lit with legacy decorators) break unless `useDefineForClassFields: false` is set deliberately.

##### Standard decorators (5.0+)

Standard (TC39 Stage 3) decorators are the default in 5.2 — no flag:

```ts
export function logged<This, Args extends unknown[], R>(
  target: (this: This, ...args: Args) => R,
  context: ClassMethodDecoratorContext<This, (this: This, ...args: Args) => R>,
) {
  const name = String(context.name);
  return function (this: This, ...args: Args): R {
    console.debug(`-> ${name}`);
    return target.call(this, ...args);
  };
}

export class Service {
  @logged
  greet(name: string): string {
    return `hi ${name}`;
  }
}
```

- Contexts are typed: `ClassMethodDecoratorContext`, `ClassFieldDecoratorContext`, `ClassAccessorDecoratorContext`, `ClassGetterDecoratorContext`, `ClassSetterDecoratorContext`, `ClassDecoratorContext`. A decorator whose signature does not match the decorated element is a compile error — good; do not "fix" it with `any`.
- `accessor` fields (5.0+) create a private backing field plus a getter/setter pair that decorators can wrap.
- **Parameter decorators do not exist** in standard decorators: `constructor(@Inject() repo: Repo)` is TS1206 "Decorators are not valid here." without `experimentalDecorators` (verified).
- **Emit**: with `target` ≤ `es2022` 5.2 down-levels decorators to the `__esDecorate`/`__runInitializers` helpers (tslib ≥ 2.5.0 with `importHelpers`; verified present in 2.5.3). With `target: "esnext"` (also implied by `module: "nodenext"`) the `@dec` syntax is emitted verbatim, and Node.js 26.12.1 throws `SyntaxError` on it (verified). No JavaScript engine shipped decorators by October 2026.

##### 5.2 bug: `addInitializer` on fields and accessors runs too early

In 5.2 (and 5.3), initializers registered with `context.addInitializer()` from a **field or `accessor`** decorator run at the start of construction, *before* the class's fields are initialised. Fixed in 5.4 (verified: 5.4.5, 5.5, 5.9 and 6.0 run them after the field is defined).

```ts
export class Settings {
  @((_v: ClassAccessorDecoratorTarget<Settings, number>, ctx: ClassAccessorDecoratorContext<Settings, number>) => {
    ctx.addInitializer(function () {
      console.log("retries =", this.retries); // reads the accessor before its storage exists
    });
  })
  accessor retries = 3;
}
new Settings();
```

Compiled by 5.2.2 for `es2022` and run on Node.js 18, 22 and 26, this throws `TypeError: Cannot read private member #retries_accessor_storage from an object whose class did not declare it`; for plain fields the initializer observes `undefined`. In a 5.2 project, do not read the decorated member (or later fields) inside a field/accessor `addInitializer` callback; do that work lazily or in a class decorator. Method decorators' `addInitializer` is unaffected.

##### Legacy decorators: `experimentalDecorators` and `emitDecoratorMetadata`

Angular, NestJS, TypeORM, InversifyJS, class-validator and many DI containers still use the legacy decorator system:

```json
{ "compilerOptions": { "experimentalDecorators": true, "emitDecoratorMetadata": true } }
```

```ts
// flags: --experimentalDecorators --emitDecoratorMetadata
function Injectable(): ClassDecorator {
  return () => {};
}
function Inject(token: string): ParameterDecorator {
  return () => {};
}

@Injectable()
export class UserService {
  constructor(@Inject("DB") private readonly db: string) {} // parameter decorators work only here
}
```

- The flag switches **every** decorator in the program to the legacy semantics; the two systems cannot be mixed in one compilation. Standard decorator signatures (`ClassMethodDecoratorContext`, …) fail to type-check under the flag and vice versa.
- `emitDecoratorMetadata` emits `design:type`/`design:paramtypes` through `Reflect.metadata`; the program must import `reflect-metadata` (or the framework's equivalent) **before** any decorated class is evaluated.
- A class imported with `import type { Repo }` and used as a constructor parameter type is emitted as `__metadata("design:paramtypes", [Function])` instead of `[repo_1.Repo]` (verified, no diagnostic — also not under `verbatimModuleSyntax` + `isolatedModules`). The DI container then cannot resolve the dependency at run time. Import classes used as injection tokens as values.
- `experimentalDecorators` is **not** deprecated in 5.2. Do not suggest migrating a framework project to standard decorators unless the framework supports them.

***

#### Enums and Namespaces

##### Prefer union types and `as const` objects

```ts
export const Status = {
  Pending: "pending",
  Active: "active",
  Closed: "closed",
} as const;
export type Status = (typeof Status)[keyof typeof Status];

export function isOpen(s: Status): boolean {
  return s !== Status.Closed;
}
```

They emit plain JavaScript, work with every single-file transpiler and Node.js type stripping, and accept string values from JSON after validation.

##### If enums are used

```ts
enum Color { Red, Green }
declare const fromApi: number;
const c: Color = fromApi;           // compiles: any `number` is assignable to a numeric enum
const lit: Color = 42;              // error TS2322
const name = Color[Color.Red];      // reverse mapping exists only for numeric members

enum Mode { Read = "r", Write = "w" }
const m: Mode = "r";                // error TS2322

enum Flags { None = 0, A = 1 << 0, B = 1 << 1, AB = A | B } // bit flags: the remaining good use of numeric enums
export { c, lit, name, m, Flags };
```

- Since 5.0 all enums are union enums and an out-of-range *literal* is rejected, but any computed `number` (from JSON, a database, arithmetic) is still accepted. A `switch` over an enum needs a `default` that throws.
- String enums are nominal: plain strings are not assignable, so parse external input explicitly.
- `const enum` is inlined by `tsc` but cannot be compiled by single-file transpilers; under `isolatedModules`, every access to a `const enum` declared in a `.d.ts` file is an error (TS2748 "Cannot access ambient const enums when 'isolatedModules' is enabled."). Do not publish `const enum`s in libraries.
- Enums, runtime namespaces, parameter properties and `import x = require()` are TypeScript-only runtime syntax: Node.js type stripping (`--experimental-strip-types`, default in Node.js 23.6+/22.18+) rejects them. 5.2 has no `erasableSyntaxOnly` to enforce this (5.8); lint for it (`@typescript-eslint/no-namespace`, a ban on `TSEnumDeclaration`/`TSParameterProperty`) if the code must run unbundled with type stripping.

##### Namespaces

ES modules replace namespaces for organising code. Legitimate uses left: type-only namespaces, `declare namespace` in `.d.ts` files, and merging with functions/classes in declarations. Write `namespace`, not the legacy `module Foo { }` keyword form (still silent in 5.2, an error from 6.0). `declare module "pkg" { … }` (ambient modules and augmentation) is a different construct and unaffected.

***

#### Error Handling

##### `catch` variables are `unknown`

`useUnknownInCatchVariables` (part of `strict`) types `catch (e)` as `unknown`:

```ts
export class HttpError extends Error {
  constructor(
    message: string,
    readonly status: number,
    options?: ErrorOptions,
  ) {
    super(message, options);
    this.name = "HttpError";
  }
}

export async function fetchJson(url: string): Promise<unknown> {
  let res: Response;
  try {
    res = await fetch(url);
  } catch (e) {
    throw new HttpError(`network error for ${url}`, 0, { cause: e }); // keep the original error
  }
  if (!res.ok) throw new HttpError(`HTTP ${res.status} for ${url}`, res.status);
  return res.json();
}

export function describe(e: unknown): string {
  if (e instanceof Error) return `${e.name}: ${e.message}`;
  return String(e);
}
```

- `new Error(msg, { cause })` and `ErrorOptions` are typed in `lib` `es2022` (runtime: Node.js 16.9+).
- `catch (e: any)` and `(e as Error).message` are unchecked; JavaScript can throw any value (strings, `null`, objects from other realms where `instanceof Error` is false).
- Never swallow errors (`catch {}`) without a comment explaining why; never `throw` strings.
- Errors from `using` scopes may be wrapped in `SuppressedError` (see Explicit Resource Management).
- Set `this.name` in custom error constructors; with `target: "es5"` subclassing `Error` breaks `instanceof` unless `Object.setPrototypeOf(this, new.target.prototype)` is called — one more reason to target `es2015`+.

##### Result types for expected failures

```ts
export type Result<T, E = Error> = { ok: true; value: T } | { ok: false; error: E };

export function parsePort(raw: string | undefined): Result<number, string> {
  const n = Number(raw);
  if (raw === undefined || raw.trim() === "" || !Number.isInteger(n) || n < 1 || n > 65535) {
    return { ok: false, error: `invalid port: ${String(raw)}` };
  }
  return { ok: true, value: n };
}
```

Note that `Number("")` is `0` and `Number(" ")` is `0`: `Number(raw) || 8080` and `Number(raw) ?? 8080` are both wrong defaults (the second never applies — 5.2 does not warn).

***

#### Async Code and Promises

```ts
// BAD: floating promise — rejection becomes an unhandledRejection and crashes Node.js
declare function save(): Promise<void>;
export function onClick(): void {
  save();
}

// GOOD
export async function onClickAsync(): Promise<void> {
  await save();
}
export function onClickFireAndForget(): void {
  void save().catch((e: unknown) => console.error("save failed", e));
}
```

- `tsc` does not report floating promises. Enable typescript-eslint's `no-floating-promises`, `no-misused-promises` and `await-thenable` in every 5.2 project.
- Run independent work concurrently with `Promise.all`; use `Promise.allSettled` when partial failure is acceptable. Sequential `await` inside a `for` loop is a performance smell unless ordering or rate-limiting requires it.
- `async` callbacks passed to `forEach`, `Array#map` without `Promise.all`, or event emitters are not awaited.
- `Promise.withResolvers` is not typed in 5.2 (5.4) and absent before Node.js 22; write the classic form:

```ts
export function deferred<T>() {
  let resolve!: (value: T | PromiseLike<T>) => void;
  let reject!: (reason?: unknown) => void;
  const promise = new Promise<T>((res, rej) => {
    resolve = res;
    reject = rej;
  });
  return { promise, resolve, reject };
}
```

- Cancellation: pass `AbortSignal` through every layer (`fetch(url, { signal })`, `setTimeout` from `node:timers/promises`, stream pipelines). `AbortSignal.timeout(ms)` is typed by the 5.2 DOM lib and by `@types/node`.
- `await using` scopes the lifetime of async resources; disposal of `await using` resources is awaited even on error paths.
- Top-level `await` requires `module` `es2022`/`esnext`/`system`/`node16`/`nodenext` and an ES-module output file (TS1309 in a CommonJS-format file under `nodenext`, verified).

***

#### Declaration Files and Library Publishing

- Publish `declaration: true` output (and `declarationMap` for go-to-source). `isolatedDeclarations` (5.5) does not exist in 5.2; write explicit return types on exported functions anyway — it keeps the public API stable and declaration emit fast.
- The `types` condition goes first in every `exports` entry; test the package with `@arethetypeswrong/cli` and with `moduleResolution` `nodenext` and `bundler` consumers.
- Declarations emitted by 5.2 can be consumed by newer compilers. The reverse is not guaranteed: `.d.ts` files emitted by 5.3+ may contain syntax 5.2 cannot parse (import attributes in `import()` types, `NoInfer`). Libraries that support 5.2 consumers must build with a compiler whose declaration output stays 5.2-compatible, or ship `typesVersions` fallbacks.
- Global augmentation needs a module context:

```ts
// file: globals.d.ts
// flags: --skipLibCheck false
export {};
declare global {
  interface Window {
    __APP_CONFIG__?: { apiUrl: string };
  }
}
```

- Module augmentation (`declare module "express-serve-static-core" { interface Request { user?: User } }`) must live in a file that is a module and is included by the tsconfig; otherwise it silently declares a new ambient module.
- `skipLibCheck: true` is standard practice for applications (faster, avoids errors in third-party `.d.ts`), but it also skips the project's **own** `.d.ts` files — including the 5.2 merged-export check (TS2395). Libraries should run at least one CI job with `skipLibCheck: false`.

***

#### JavaScript Checking (`checkJs`, JSDoc)

- `allowJs` + `checkJs` (or `// @ts-check` per file) type-checks JavaScript with JSDoc annotations.
- 5.0 added JSDoc `@satisfies` and `@overload`; 5.2 allows `.ts` extensions in JSDoc `import("…")` types:

```ts
// file: util.js
// flags: --allowJs --checkJs
/** @typedef {{ id: string; tags?: string[] }} Item */

/** @satisfies {Record<string, Item>} */
const items = { a: { id: "a" } };

/**
 * @overload
 * @param {string} v
 * @returns {string}
 */
/**
 * @overload
 * @param {number} v
 * @returns {number}
 */
/** @param {string | number} v */
function id(v) {
  return v;
}

export { items, id };
```

- JSDoc `@import` is 5.5+; in 5.2 write `/** @typedef {import("./types.js").User} User */`.
- A JavaScript file with `// @ts-nocheck` or excluded from `checkJs` gets no checking at all — review such files as untyped code.

***

#### Security-Relevant Patterns

TypeScript types provide **no** runtime security. Review the boundaries:

- **Validate all external input** (HTTP bodies, query strings, headers, env vars, files, queue messages, `postMessage` data) with a runtime schema before it gets a static type. `req.body as CreateUser` is not validation.
- **Prototype pollution**: merging untrusted JSON into objects (`Object.assign(target, JSON.parse(body))`, recursive "deep merge") can set `__proto__`/`constructor.prototype`. Use `Object.create(null)` dictionaries, `Map`, or validated copies; reject `__proto__`, `constructor` and `prototype` keys.

```ts
export function safeMerge(target: Record<string, unknown>, input: unknown): Record<string, unknown> {
  if (typeof input !== "object" || input === null || Array.isArray(input)) throw new TypeError("object expected");
  for (const [k, v] of Object.entries(input)) {
    if (k === "__proto__" || k === "constructor" || k === "prototype") continue;
    target[k] = v;
  }
  return target;
}
```

- **Excess data**: structural typing does not strip properties; returning a database row typed as `PublicUser` still serialises `passwordHash`. Map explicitly.
- **Injection**: build SQL with parameters (`db.query("select * from t where id = $1", [id])`), shell commands with `execFile`/`spawn` argument arrays (no `shell: true` with user input), HTML with escaping or a sanitiser. Template literal types do not make strings safe.
- **`any` leaks**: `JSON.parse`, `response.json()` (typed `Promise<any>` in 5.2's DOM lib), `catch` with `any`, untyped dependencies. Assign them to `unknown` immediately.
- **Assertions at boundaries** (`as`, `!`, `<T>` generic return casts) bypass checking exactly where attackers control the data.
- **`eval`, `new Function`, `vm.runInNewContext`** with any user-controlled string are code execution.
- **Regular expressions** from user input or with nested quantifiers can cause ReDoS; 5.2 does not check regex syntax at all (5.5), so invalid flags such as `/a/v` on an old runtime fail only at run time.
- **Secrets**: do not commit `.env` files; do not log full configuration objects or request headers; keep `sourcesContent` out of public production source maps if the source is confidential.
- Pin the compiler (`"typescript": "5.2.2"` or `~5.2.2`) and `@types/node`; an accidental major upgrade of either changes emit or typings.

***

#### Performance of the Compiler and of Emitted Code

##### Compiler

- 5.2 speeds up relation checks of deeply nested recursive types (the release notes measured a drizzle-based case at about 1.5× faster).[^1]
- Use `tsc --extendedDiagnostics` and `tsc --generateTrace <dir>` to find slow files and types; look for huge unions, deeply recursive conditional types and inferred types of enormous object literals.
- `incremental: true` (with `tsBuildInfoFile`) and project references (`composite: true`, `tsc -b`) cut rebuild times. In 5.2 `tsc -b` stops at the first project with errors (continuing past errors is 5.6).
- Prefer `interface extends` over large intersections, and explicit return types on exported functions — they reduce re-checking and declaration-emit work.
- `skipLibCheck: true` typically saves a large share of check time for applications.
- Type-check in CI with `tsc --noEmit`; transpile with esbuild/SWC/`tsx` for speed. 5.2 has no `--noCheck` (5.6) for emit-only builds.

##### Emitted code

- Down-levelling costs: `target: "es5"` turns `async`/`await` into state machines and classes into functions; `#private` below `es2022` becomes `WeakMap` lookups; `using` below `esnext` becomes `try`/`finally` with helper calls (cheap, but not free in hot loops). Choose the highest `target` the oldest runtime supports — `es2022` for every supported Node.js line.
- Duplicate helpers in every file inflate bundles; `importHelpers: true` + `tslib` (≥ 2.6.0 for `using`) deduplicates them.
- `const enum` inlining saves a lookup but breaks `isolatedModules` builds — not worth it.
- Prefer `toSorted()`/`toReversed()` for clarity, but in hot paths that own the array, in-place `sort()` avoids an allocation.

***

#### Testing and Type Tests

- Run the type checker as a test step (`tsc --noEmit -p tsconfig.json`); test runners that transpile with esbuild/SWC (Vitest, Jest with `@swc/jest`, `tsx --test`) do **not** type-check.
- Pin test tooling that still supports TypeScript 5.2 for type tests: `expect-type`/Vitest's `expectTypeOf` and `tsd` compile assertions with the project's compiler, and their own `.d.ts` files must parse under 5.2.
- Lightweight type assertions need no library:

```ts
type Equals<A, B> = (<T>() => T extends A ? 1 : 2) extends <T>() => T extends B ? 1 : 2 ? true : false;
type Expect<T extends true> = T;

type Sorted = ReturnType<number[]["toSorted"]>;
export type Tests = [
  Expect<Equals<Sorted, number[]>>,
  Expect<Equals<Awaited<Promise<string>>, string>>,
];

// @ts-expect-error — must fail: a string is not a number
export const wrong: number = "x";
```

- Prefer `@ts-expect-error` over `@ts-ignore`: it fails when the error disappears, so stale suppressions get removed. Require a reason in the comment.
- Test disposal paths explicitly: assert that `[Symbol.dispose]` ran when the body throws, and that disposal errors surface as `SuppressedError`.
- When a test relies on fake timers or module mocking, type the mocks (`vi.mocked(fn)`, `jest.mocked(fn)`) instead of casting to `any`.

***

#### Tooling

- **Compiler**: pin `typescript` to `5.2.2` (or `~5.2.2`); `^5.2.2` silently installs 5.9.x. The `tsc` 5.2 binary needs Node.js 14.17 or newer to run.
- **Linting**: typescript-eslint 8.x supports 5.2 (`peerDependencies.typescript: ">=4.8.4 <6.1.0"`). Use the type-aware presets (`strictTypeChecked`, `stylisticTypeChecked`) — they cover floating promises, unsafe `any` flows, unnecessary conditions and template-expression restrictions that 5.2's compiler does not check.
- **Formatting**: make sure the formatter's TypeScript parser understands 5.2 syntax (`using`, `await using`, `accessor`, mixed tuple labels); older formatter releases may not. Verified: Prettier 3.9.10 formats all of it.
- **Transpilers**: esbuild, SWC and Babel down-level `using` and standard decorators in current versions, but each has its own option for decorator flavour (esbuild follows `experimentalDecorators` from tsconfig; SWC needs `jsc.transform.decoratorVersion: "2022-03"` or legacy). Make sure the transpiler and `tsc` agree on decorator semantics and `useDefineForClassFields`.
- **Node.js type stripping** (Node.js 22.18+/23.6+ by default) runs `.ts` files directly but requires erasable syntax and real `.ts` extensions in relative imports — which 5.2 only allows for type-only imports or with `allowImportingTsExtensions` + `noEmit`. A 5.2 project usually type-checks with `tsc --noEmit` and runs through `tsx` or a build step instead.
- **Editors**: VS Code bundles a much newer TypeScript. Select the workspace version ("TypeScript: Select TypeScript Version" → "Use Workspace Version") so that editor diagnostics match the 5.2 build; otherwise the editor accepts `with` import attributes, `NoInfer` and narrowing that `tsc` 5.2 rejects.
- **Package checks**: `@arethetypeswrong/cli` for published packages, `publint` for `package.json` `exports`.

***

#### Removed / Changed in 5.2 — Migration Notes

##### Upgrading from 5.1 to 5.2

1. Fix `module`/`moduleResolution` pairs that mix Node.js and non-Node.js modes (TS5109/TS5110). Typical fix: `module: "nodenext"` alone, or `module: "esnext"` + `moduleResolution: "bundler"`.
2. Fix merged declarations that mix exported and local members (TS2395), including in hand-written `.d.ts` files and `declare module` blocks.
3. Expect DOM typing changes in browser code.
4. Tools on the compiler API: handle `undefined` in `TupleType.labeledElementDeclarations`.
5. Remove workarounds that 5.2 makes unnecessary: casts on `string[] | number[]` method calls, `[...arr].sort()` copies (use `toSorted` if the runtime is Node.js 20+), `WeakMap<object>` wrappers around symbol keys, artificial labels on every tuple element.
6. If you adopt `using`: add `esnext.disposable` to `lib`, keep `target` ≤ `es2022` for runtimes older than Node.js 24, upgrade `tslib` to ≥ 2.6.0 when `importHelpers` is on, and polyfill `DisposableStack`/`Symbol.dispose` where needed.

##### Upgrading beyond 5.2 (what will change)

- **5.3** adds `with` import attributes; replace `assert` (later deprecated). It also narrows more (`switch (true)`, boolean comparisons), which can surface new errors in dead branches.
- **5.4** fixes the field/accessor `addInitializer` ordering, preserves narrowing in closures, adds `NoInfer`, `Object.groupBy`/`Map.groupBy`.
- **5.5** removes the 5.0-deprecated options for good (configs relying on `ignoreDeprecations: "5.0"` stop compiling), adds inferred type predicates and regex checking (new errors in existing code), `isolatedDeclarations` and `${configDir}`.
- **5.6** reports always-truthy/nullish conditions (TS2872/TS2869) and changes built-in iterator types; `@types/node` 26.x already depends on those 5.6 lib types.
- **6.0** changes almost every default (`strict`, `module`, `target`, `types`, `rootDir`) and deprecates `baseUrl`, `moduleResolution: node10`, `module: amd/umd/system`, `target: es5`, `esModuleInterop: false` and the `module` keyword for namespaces; **7.0** removes them.
- Code written today for a 5.2 project should already avoid those: no `ignoreDeprecations`, no `baseUrl` for lookups, explicit `module`/`target`/`types`, `namespace` keyword, `strict: true`.

***

#### Production / Review Checklist

##### Configuration

- [ ] `typescript` pinned to 5.2.2 in the lockfile; editor uses the workspace TypeScript
- [ ] `strict: true`, plus `noUncheckedIndexedAccess`, `noImplicitOverride`, `noFallthroughCasesInSwitch` where feasible
- [ ] `target` set explicitly (never implied by `module: nodenext`); `lib` matches the oldest runtime (`es2023` only for Node.js 20+)
- [ ] `module` and `moduleResolution` set explicitly and consistent (`nodenext`, or `esnext` + `bundler`); never `esnext` with the implied `classic` resolution
- [ ] `verbatimModuleSyntax` + `isolatedModules` when any single-file transpiler is involved
- [ ] No `ignoreDeprecations: "5.0"` / deprecated options (`importsNotUsedAsValues`, `preserveValueImports`, `suppressImplicitAnyIndexErrors`, `target: es3`, `out`, …)
- [ ] `types` lists the needed global type packages; `@types/node` pinned to the runtime major and known to compile with 5.2 (not 26.x)
- [ ] `tslib` ≥ 2.6.0 when `importHelpers` is on and `using` is used

##### Language and types

- [ ] No `any` at boundaries; external data validated at run time; `unknown` + narrowing elsewhere
- [ ] Every `as`, `!`, `@ts-ignore` justified; `@ts-expect-error` preferred and commented
- [ ] Exhaustive `switch` with a `never` check; no silent `default` fallbacks
- [ ] No 5.3+ narrowing idioms (`switch (true)`, closure narrowing after reassignment, `obj[key]` narrowing, inferred `filter` predicates) — 5.2 idioms with locals and explicit predicates
- [ ] No 5.3+ syntax or APIs: import attributes `with`, `NoInfer`, `Object.groupBy`, `Promise.withResolvers`, Set methods, `Array.fromAsync`, iterator helpers
- [ ] Results of `filter`/`find` on `A[] | B[]` treated as `(A | B)[]`
- [ ] No truthiness checks on numbers/strings where `0`/`""` are valid; no `??` on non-nullable expressions; no `if (/regex/)`

##### Resources, async and errors

- [ ] Resources scoped with `using`/`await using` or `try`/`finally`; no `using` on Promises or `AsyncDisposable`s
- [ ] `using` not emitted verbatim for runtimes older than Node.js 24 (`target` ≤ `es2022`)
- [ ] `DisposableStack`/`AsyncDisposableStack`/`Symbol.metadata` polyfilled (loaded first) where the runtime lacks them
- [ ] No floating promises (typescript-eslint `no-floating-promises`); `Promise.all` for independent work; `AbortSignal` passed through
- [ ] `catch` treats errors as `unknown`; `cause` preserved; `SuppressedError` chains logged in full

##### Modules

- [ ] `.js` extensions on relative imports under `nodenext`; `import type` for types
- [ ] No static JSON imports in ESM that Node.js 22+ executes (5.2 can only write `assert`); use `createRequire` or `readFile` + `JSON.parse`
- [ ] `paths` aliases mirrored in bundler/test-runner config; `exports` with `types` first

##### Classes and decorators

- [ ] One decorator system per project; `experimentalDecorators` only where a framework needs it; `reflect-metadata` imported first with `emitDecoratorMetadata`
- [ ] No field/accessor `addInitializer` callbacks that read class members (5.2 runs them too early)
- [ ] No overridable method calls from base constructors; `declare` for type-only field redeclarations
- [ ] Decorator metadata: `Symbol.metadata` polyfilled; parent metadata not mutated from subclasses

##### Security and observability

- [ ] Prototype-pollution-safe merges; parameterised SQL; no `shell: true` with user input
- [ ] DTOs mapped explicitly before serialisation
- [ ] Errors logged with `name`, `message`, `cause` and stack; no secrets in logs or public source maps
