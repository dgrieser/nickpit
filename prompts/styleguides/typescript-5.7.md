### TypeScript 5.7: The Complete Production Developer Guide

> **Version**: TypeScript 5.7. The 5.7.2 stable release came out on 2024-11-22. The latest and final patch is **5.7.3 (2025-01-08)**.
> **Support status (October 2026)**: unmaintained. TypeScript has no LTS lines, and the team does not backport fixes to older minors. 5.7 has had no release since 5.7.3. Newer lines: 5.8 (2025-02-28), 5.9 (2025-07-31), 6.0 (2026-03-23, latest 6.0.3), and **7.0, the native Go compiler, which is the newest stable release (7.0.2, 2026-07-08)**. 7.1 is in beta (2026-10-20).
> **Scope**: the full TypeScript language and type system as usable in 5.7, the compiler options, module resolution for Node.js and bundlers, the 5.7-specific changes compared with 5.6, the bugs a reviewer must catch, tsconfig recommendations, tooling, and a review checklist.
> **Note**: This document is self-contained. Everything from TypeScript 4.x and 5.0–5.6 that matters is carried forward here. Sections that cover 5.7-specific behaviour are marked **New in 5.7**. Every example was compiled with `tsc` 5.7.3. Unless a block says otherwise, the settings were `strict`, `target`/`lib` `es2024`, `module` `nodenext`, `noUncheckedIndexedAccess`, `exactOptionalPropertyTypes`, `noImplicitOverride`, `verbatimModuleSyntax`, and `@types/node`. Each line marked `// error` fails with that error; every other line compiles.

#### Overview

TypeScript 5.7 is a small, mostly "plumbing" release. It adds:

- **Checks for never-initialized variables**: a variable that is never assigned is now reported even when it is read inside a nested function.
- **`--rewriteRelativeImportExtensions`**: write `import "./foo.ts"`, run the source directly, and still emit correct `.js` output.
- **`--target es2024` and `--lib es2024`**: `Object.groupBy`, `Map.groupBy`, `Promise.withResolvers`, resizable and transferable `ArrayBuffer`, growable `SharedArrayBuffer`, `String.prototype.isWellFormed`, the regex `v` flag, and `Atomics.waitAsync` (which moves out of `es2022`).
- **TypedArrays generic over their buffer**: `Uint8Array<TArrayBuffer extends ArrayBufferLike = ArrayBufferLike>`.
- **Validated JSON imports under `--module nodenext`**: `with { type: "json" }` is required and named imports are rejected.
- **Classes with non-literal computed method names** now get index signatures.
- **More implicit-`any` errors** for `noImplicitAny` code that does not use `strictNullChecks`.
- **Editor project-ownership changes**: the editor keeps walking up to ancestor `tsconfig.json` files, and composite projects get a faster probe.
- **V8 compile caching** when `tsc` runs on Node.js 22.1 or later.

Everything earlier remains available and should be used where it fits:

- 4.9: `satisfies`, `accessor` auto-accessors, `in` narrowing of unlisted properties.
- 5.0: standard ECMAScript decorators, `const` type parameters, all enums as union enums, `--moduleResolution bundler`, `--verbatimModuleSyntax`, `extends` arrays in tsconfig.
- 5.1: unrelated getter/setter types, `undefined`-returning functions.
- 5.2: `using` and `await using` declarations, decorator metadata, named and anonymous tuple elements mixed.
- 5.3: import attributes (`with { type: "json" }`), `switch (true)` narrowing, `--resolution-mode` in all module modes.
- 5.4: preserved narrowing in closures after the last assignment, `NoInfer<T>`, `Object.groupBy`/`Map.groupBy` types, `--module preserve`.
- 5.5: inferred type predicates, constant-index narrowing, the JSDoc `@import` tag, regex syntax checking, `Set` methods in `esnext`, `--isolatedDeclarations`, `${configDir}`.
- 5.6: errors for always-truthy and always-nullish checks, iterator helper types, `--strictBuiltinIteratorReturn`, arbitrary module identifiers, `--noUncheckedSideEffectImports`, `--noCheck`, `--build` that continues past errors.

**Do not use or recommend anything from TypeScript 5.8 or later.** The project's compiler does not have it. See [Do Not Use or Recommend](#do-not-use-or-recommend) for the exact list, including:

- 5.8: `--erasableSyntaxOnly`, `--module node18`, `require()` of ESM under `nodenext`, per-branch checks of conditional `return` expressions, and `--libReplacement`.
- 5.9: `import defer` and `--module node20`.
- 6.0: `es2025`, `Temporal`, `Map.prototype.getOrInsert`, `RegExp.escape`, and `#/` subpath imports.
- 7.0: the native compiler.

Global truths about TypeScript 5.7:

- **Types are erased.** Annotations, interfaces, type aliases and `as` casts produce no runtime checks. Data that crosses a trust boundary (JSON, HTTP, environment variables, files) is `unknown` until validated at runtime.
- **The type system is structural.** Compatibility is decided by shape, not by declared name. Two interfaces with the same members are interchangeable.
- **The type system is deliberately unsound in known places.** These include `any`, type assertions, non-null assertions (`!`), method-parameter bivariance, array covariance, index access without `noUncheckedIndexedAccess`, and definite-assignment assertions. Review these spots hardest.
- **`tsc` is a type checker and a (mostly) syntax-lowering transpiler.** It does not polyfill. `--lib` describes what the runtime provides; it adds nothing to the runtime. Calling `Promise.withResolvers` on Node.js 20 type-checks under `lib: ["es2024"]` and fails at runtime.
- **The module format of each output file is decided at compile time** by `module`, the file extension (`.mts`/`.cts`), and the nearest `package.json` `"type"` field (under `node16`/`nodenext`).
- **Every `strict` sub-flag matters.** In 5.7, `strict` turns on `noImplicitAny`, `strictNullChecks`, `strictFunctionTypes`, `strictBindCallApply`, `strictPropertyInitialization`, `noImplicitThis`, `useUnknownInCatchVariables`, `alwaysStrict` and `strictBuiltinIteratorReturn`.

***

#### What's New in TypeScript 5.7

| Change | Kind | Who notices |
|---|---|---|
| Never-initialized variables reported inside nested functions | new error (TS2454) | all `strictNullChecks` code |
| `--rewriteRelativeImportExtensions` | new option | run-in-place users (Node.js type stripping, Deno, Bun, tsx), library authors |
| `--target es2024`, `--lib es2024` (+ `es2024.*` sub-libs) | new option values | anyone targeting Node.js 22+ / modern browsers |
| `Atomics.waitAsync` moved from `es2022` to `es2024` lib | lib change | `lib: ["es2022"]` users of `waitAsync` get TS2550 |
| TypedArrays generic over `ArrayBufferLike`; `Buffer` interplay | lib change | code that passes `Buffer`/`Uint8Array` to APIs expecting `ArrayBuffer` |
| JSON imports under `--module nodenext` need `with { type: "json" }`; no named imports | new errors (TS1543, TS1544) | ESM code importing `.json` |
| Non-literal computed method names in classes create index signatures | type change | classes using `[sym]()` with `symbol`/`string`-typed keys |
| More implicit-`any` errors without `strictNullChecks` | new error (TS7011) | non-strict code bases |
| TSServer keeps searching ancestor `tsconfig.json` files; faster composite-project ownership checks | editor behaviour | monorepos, solution-style configs |
| `tsc` enables Node's V8 compile cache (`module.enableCompileCache()`) | startup performance | everybody on Node.js ≥ 22.1 |
| DOM `lib.d.ts` regenerated | lib change | browser code |

##### New in 5.7: Checks for Never-Initialized Variables

TypeScript has long reported reads of a variable that is not definitely assigned on every path *in the same function*. Reads inside a nested function (closure, callback, hoisted helper) were optimistically accepted, because the checker cannot know when the inner function runs. Since 5.7 the checker reports the read when the variable is **never assigned anywhere**. It is still lenient when the variable is assigned on *some* path.

```ts
declare function someCondition(): boolean;
declare function doSomeWork(): number;

export function never() {
  let result: number;
  // ... work that forgets to assign `result`
  function printResult() {
    console.log(result); // error TS2454: Variable 'result' is used before being assigned.
  }
  printResult();
}

export function sometimes() {
  let result: number;
  if (someCondition()) result = doSomeWork();
  function printResult() {
    console.log(result); // still accepted: assigned on one path, read in a closure
  }
  printResult();
}
```

Boundary behaviour (verified on 5.6.3, 5.7.3 and 5.8.3):

- 5.6 accepts both functions above; 5.7 reports `never`.
- The check needs `strictNullChecks`. Without it, nothing is reported.
- If the declared type includes `undefined` (`let x: number | undefined`), there is no error, because reading `undefined` is type-correct.
- A definite-assignment assertion (`let x!: number`) silences it. Treat `!` on a local as a code smell: it hides exactly this bug.
- **`var` declarations are not checked.** `var w: string; return () => w.length;` compiles silently on 5.7. This is one more reason to forbid `var` (ESLint `no-var`).
- Module-level `let` read in an exported arrow function is reported too.

Review note: an upgrade to 5.7 that produces new TS2454 errors in closures almost always reveals a real bug (a variable that is `undefined` at runtime). Do not "fix" it with `!` or `| undefined` without understanding the data flow.

##### New in 5.7: `--rewriteRelativeImportExtensions`

Tools that run TypeScript in place need `.ts` in relative import specifiers:

- Node.js type stripping (on by default since 22.18.0 and 23.6.0)
- Deno
- Bun
- tsx

`--allowImportingTsExtensions` (5.0) allowed such specifiers, but only with `noEmit` or `emitDeclarationOnly`, because the emitted `.js` would still say `./foo.ts`. With `--rewriteRelativeImportExtensions`, `tsc` rewrites **relative** specifiers that end in `.ts`, `.tsx`, `.mts` or `.cts` (and are not declaration files) to `.js`, `.jsx`, `.mjs` or `.cjs` on emit. The option also permits the `.ts` specifiers themselves, so `allowImportingTsExtensions` is not needed in addition.

```ts
// src/main.ts, compiled with --module nodenext --rewriteRelativeImportExtensions
import { foo } from "./foo.ts";          // emitted as "./foo.js"
const m = await import("./foo.ts");      // emitted as "./foo.js"
const p = "./foo.ts";
const m2 = await import(p);              // emitted as import(__rewriteRelativeImportExtension(p))
```

Verified emit (5.7.3, `--module nodenext`, `"type": "module"`): static and literal dynamic imports are rewritten in place. The release notes call dynamic imports the hard case for path rewriting; the emitted helper covers only `import()` calls in the compiled code, never paths built inside dependencies. A **non-literal** `import(expr)` is wrapped in an injected runtime helper, `__rewriteRelativeImportExtension(path, preserveJsx)`, which rewrites at runtime only strings that start with `./` or `../`. 5.7.3 inlines this helper even under `importHelpers` (verified). The same files run unchanged with `node src/main.ts` (Node.js 22.18+/24) and as compiled `dist/main.js`. On 5.6.3 the option is unknown (TS5023), and `.ts` specifiers fail with TS5097.

What is **not** rewritten:

- Extensionless specifiers (`./foo`), `.js` specifiers, and bare package specifiers (`pkg/file.ts`).
- `paths` aliases (`@/utils.ts`).
- `package.json` `imports` subpaths (`#src/foo.ts`).

5.7 reports an error when a non-relative specifier with a `.ts` extension resolves to an input file:

```text
error TS2877: This import uses a '.ts' extension to resolve to an input TypeScript
file, but will not be rewritten during emit because it is not a relative path.
```

For workspaces where packages import each other's sources, use conditional `exports` with a **scoped** custom condition (`"@my-org/source": "./src/index.ts"`) and run with `node --conditions=@my-org/source`, or set `customConditions` for type checking. An unscoped `"development"` condition collides with dependencies that ship their own.

`tsc` in 5.7 cannot reject TypeScript syntax that Node.js type stripping refuses. Node.js errors at runtime with `ERR_UNSUPPORTED_TYPESCRIPT_SYNTAX` on:

- `enum`
- `namespace` with runtime code
- parameter properties
- `import x = require()` / `import x = N.y` aliases
- `export =`

Node.js also does not apply `paths`, and decorators are a plain `SyntaxError` there. The `--erasableSyntaxOnly` check that catches these at compile time is **5.8**. On 5.7 such code type-checks and then crashes under `node file.ts`. In a project that runs `.ts` directly, a reviewer must flag those constructs by hand or with lint rules: typescript-eslint `no-namespace`, `parameter-properties` with `prefer: "class-property"`, `no-require-imports`, plus `no-restricted-syntax` for `TSEnumDeclaration`.

##### New in 5.7: `--target es2024` and `--lib es2024`

`es2024` is a new `target` and `lib` value (5.6.3 rejects both with TS6046). `lib.es2024.d.ts` pulls in these sub-libs:

- `es2024.arraybuffer`: resizable `ArrayBuffer` (`new ArrayBuffer(n, { maxByteLength })`, `resize`, `resizable`, `maxByteLength`, `detached`, `transfer`, `transferToFixedLength`)
- `es2024.collection`: `Map.groupBy`
- `es2024.object`: `Object.groupBy`
- `es2024.promise`: `Promise.withResolvers`
- `es2024.regexp`: `RegExp.prototype.unicodeSets` (the `v` flag accessor)
- `es2024.sharedmemory`: growable `SharedArrayBuffer` and `Atomics.waitAsync`
- `es2024.string`: `isWellFormed`, `toWellFormed`

```ts
interface Item { kind: "a" | "b"; n: number }
const items: Item[] = [{ kind: "a", n: 1 }, { kind: "b", n: 2 }];

const byKind = Object.groupBy(items, (i) => i.kind);   // Partial<Record<"a" | "b", Item[]>>
const firstA = byKind.a?.[0];                          // groups may be missing
const big = Map.groupBy(items, (i) => i.n > 1);        // Map<boolean, Item[]>

const { promise, resolve } = Promise.withResolvers<number>();
resolve(42);

const buf = new ArrayBuffer(8, { maxByteLength: 16 });
buf.resize(12);
const moved = buf.transfer();                          // `buf` is now detached

const wellFormed = "a\uD800".isWellFormed();           // false
const re = /[\p{L}--[a-z]]/v;                          // set subtraction, needs target es2024
export { firstA, big, promise, moved, wellFormed, re };
```

Verified details:

- `Object.groupBy` returns `Partial<Record<K, T[]>>`, so each group is possibly `undefined`. Do not write `byKind.a.length`.
- The regex `v` flag is gated on **`target`**, not `lib`. With `--target es2023 --lib es2024`, `/…/v` fails with `TS1501: This regular expression flag is only available when targeting 'es2024' or later.`
- `Atomics.waitAsync` was in `lib.es2022.sharedmemory.d.ts` in 5.6 and is in `lib.es2024.sharedmemory.d.ts` in 5.7. Code with `lib: ["es2022"]` that calls it now gets TS2550 ("Try changing the 'lib' compiler option to 'es2024' or later").
- `Object.groupBy`, `Map.groupBy` and `Promise.withResolvers` were already available in 5.6 via `esnext`. The resizable `ArrayBuffer` members (`resize`, `transfer`) were not: 5.6 `esnext` rejects `buf.resize()`.

**Runtime reality check.** `lib` is a promise about the runtime. Measured on Node.js releases:

| Runtime | `Object.groupBy` / `Promise.withResolvers` | `ArrayBuffer.prototype.transfer` | regex `v` flag | Suitable `target`/`lib` |
|---|---|---|---|---|
| Node.js 18.20 (EOL 2025-04-30) | no | no | no | `es2022` |
| Node.js 20.20 (EOL 2026-04-30) | no | no | yes | `es2023` |
| Node.js 22.23 (maintenance LTS until 2027-04-30) | yes | yes | yes | `es2024` |
| Node.js 24.22 (LTS, maintenance since 2026-10-20) | yes | yes | yes | `es2024` (`using` runs natively only with `target: esnext`, see below) |

A project that declares `"engines": { "node": ">=20" }` but compiles with `lib: ["es2024"]` type-checks `Object.groupBy` and crashes on Node.js 20 with `TypeError: Object.groupBy is not a function`. Flag that mismatch.

##### New in 5.7: TypedArrays Are Generic Over `ArrayBufferLike`

Every TypedArray now has a buffer type parameter: `interface Uint8Array<TArrayBuffer extends ArrayBufferLike = ArrayBufferLike>`. The same holds for `Int8Array` … `Float64Array`, `BigInt64Array` and `BigUint64Array`. The default keeps `Uint8Array` meaning "backed by any buffer". Constructors infer a precise type:

```ts
const u = new Uint8Array(4);                              // Uint8Array<ArrayBuffer>
const ab: ArrayBuffer = u.buffer;                         // OK: precise buffer type
const shared = new Uint8Array(new SharedArrayBuffer(4));  // Uint8Array<SharedArrayBuffer>
const sab: ArrayBuffer = shared.buffer;                   // error TS2739 under lib es2024

function needsOwned(x: Uint8Array<ArrayBuffer>): ArrayBuffer { return x.buffer; }
needsOwned(u);
const loose: Uint8Array = u;                              // widened to Uint8Array<ArrayBufferLike>
needsOwned(loose);                                        // error TS2345
export { ab, sab };
```

5.6.3 rejects `Uint8Array<ArrayBuffer>` (TS2315, "Type 'Uint8Array' is not generic").

How strict this is depends on `lib`, and the release notes do not say so. Under `lib` ≤ `es2023`, `SharedArrayBuffer` is still structurally assignable to `ArrayBuffer`, so 5.7 with `--target es2022` accepts both marked lines above. Only `es2024` (or `esnext`) adds the `ArrayBuffer`-only members (`resizable`, `resize`, `detached`, `transfer`, `transferToFixedLength`) that make the two incompatible. Upgrading `lib` to `es2024` can therefore surface new errors in buffer-handling code even without a TypeScript upgrade.

With `@types/node`, a plain `Buffer` is `Buffer<ArrayBufferLike>` and `buf.buffer` is `ArrayBufferLike`. Recent `@types/node` releases type the results of APIs that always allocate a fresh buffer (`fs.readFileSync`, `Buffer.alloc`) as `NonSharedBuffer` (`Buffer<ArrayBuffer>`), so the error appears only where a generic `Buffer` flows in:

```ts
declare const b: Buffer;                       // e.g. a Buffer received from a stream or socket
const asU8: Uint8Array = b;                    // fine
const owned: Uint8Array<ArrayBuffer> = b;      // error TS2322
const raw: ArrayBuffer = b.buffer;             // error TS2322: ArrayBufferLike is not ArrayBuffer
export { asU8, owned, raw };
```

Review guidance:

- Errors such as `Type 'Buffer' is not assignable to type 'Uint8Array<ArrayBufferLike>'` after a 5.7 upgrade mean `@types/node` is too old. Update it (the current `@types/node` ships a `ts5.7/` variant through `typesVersions`). Do not cast.
- `buf.buffer` may be larger than the view and may be shared. Small `Buffer.from(string)`/`Buffer.allocUnsafe` results are views into Node's shared pool (`Buffer.poolSize`: 8 KiB on Node.js 22, 64 KiB on Node.js 24 and 26). On Node.js 22, `Buffer.from("abc").buffer.byteLength` is 8192 (65536 on Node.js 24), and a small `readFileSync` result also sat on a pool-sized `ArrayBuffer`. Code that does `new Uint8Array(b.buffer)` or posts `b.buffer` reads or transfers unrelated bytes. Use `b.buffer.slice(b.byteOffset, b.byteOffset + b.byteLength)`, or copy with `Uint8Array.prototype.slice()`. The 5.7 types make this visible as `ArrayBufferLike`. Do not paper over it with `as ArrayBuffer`.
- Write `Uint8Array<ArrayBuffer>` in signatures that really need a non-shared, transferable buffer (Web Crypto `BufferSource`, `postMessage` transfer lists, `structuredClone` transfer).
- Combining `lib: ["dom"]` (or no `lib`, which includes DOM) with a recent `@types/node` (26.x tested) and `skipLibCheck: false` on 5.7.3 produces TS2430 conflicts inside `lib.dom.d.ts` (`TextDecoder`/`TextEncoder` vs `node:util`). Node-only projects should set `lib` explicitly without `dom`. Mixed projects need `skipLibCheck: true`.

##### New in 5.7: Validated JSON Imports Under `--module nodenext`

Node.js requires the `type: "json"` import attribute for JSON in ES modules, and JSON modules have only a default export. Under `--module nodenext`, 5.7 enforces both rules in ESM files:

```ts
// tsconfig: module nodenext, resolveJsonModule; package.json "type": "module"
import bad from "./config.json";                                   // error TS1543
import cfg from "./config.json" with { type: "json" };             // OK
import * as ns from "./config.json" with { type: "json" };
import { version } from "./config.json" with { type: "json" };     // error TS1544
const v1: string = cfg.version;
const v2: string = ns.default.version;                             // namespace holds only `default`
const v3 = ns.version;                                             // error TS2339
```

Verified boundaries:

- 5.6.3 reports none of the 5.7 errors above (only the `ns.version` TS2339, which every version reports). On Node.js 22/24 the attribute-less import throws `ERR_IMPORT_ATTRIBUTE_MISSING`, and the named import fails at link time.
- Under `--module node16`, 5.7 reports TS1544 for named JSON imports. It also rejects the `with { … }` attribute itself (TS2823: import attributes require `esnext`, `nodenext` or `preserve`). ESM projects that import JSON must use `nodenext` (or `preserve`/`esnext` with a bundler).
- The deprecated `assert { type: "json" }` syntax is **still accepted by 5.7 under `nodenext`**. Node.js 22+ rejects it with a `SyntaxError`, and Node.js 18/20 print a deprecation warning. 5.8 turns it into TS2880. On 5.7, a reviewer must flag `assert {` in import declarations manually.
- In CommonJS output files (`.cts`, or `.ts` without `"type": "module"`), `import x = require("./x.json")` and default imports keep working without attributes.

##### New in 5.7: Index Signatures From Non-Literal Computed Method Names

A class method whose computed name has a non-literal type (`symbol`, `string`, `number`, not a `unique symbol` or a literal) now contributes an index signature, matching object-literal behaviour. Previously the member was invisible to the type system.

```ts
declare const symbolMethodName: symbol;
export class A {
  [symbolMethodName]() { return 1; }
}
declare const anySym: symbol;
const m = new A()[anySym];          // 5.7: () => number   (5.6: TS7053 implicit any)
const wrong: number = m;            // error TS2322
export { wrong };
```

This can surface new errors when such a class is assigned to a type with a conflicting index signature. It can also *remove* errors (TS7053) on dynamic lookups. Prefer `unique symbol` constants (`const key: unique symbol = Symbol()`) or literal keys, so the member gets a real name.

##### New in 5.7: More Implicit-`any` Errors (`noImplicitAny` Without `strictNullChecks`)

When a function expression is contextually typed by a signature that returns a generic type and the function returns only `null`/`undefined`, 5.7 reports TS7011 under `noImplicitAny` when `strictNullChecks` is off:

```ts
// compiled with --noImplicitAny (no strictNullChecks)
declare const p: Promise<number>;
const p2 = p.catch(() => null);   // error TS7011: Function expression, which lacks return-type annotation, implicitly has an 'any' return type.
```

Fix: annotate (`(): null => null`) or, better, enable `strict`. With `strictNullChecks` on, the return type is `null` and nothing is reported.

##### New in 5.7: Editor Project Ownership

- **Ancestor `tsconfig.json` search.** When TSServer opens a file, it no longer stops at the first `tsconfig.json` that does not include the file. It keeps walking up the directory tree and can pick a solution-style root config that references the right project. Layouts such as `src/tsconfig.json` + `src/tsconfig.test.json` + a root `tsconfig.json` with `"files": []` and `references` no longer need the `tsconfig.src.json` renaming trick.
- **Composite probing.** When checking whether a referenced `composite` project owns a file, the editor looks only at the project's root file set and no longer loads the project's entire import graph.

Both affect editor behaviour only, not `tsc` output. A file that is not in any project's `include` is still not type-checked by `tsc -b`.

##### New in 5.7: V8 Compile Caching

`lib/tsc.js` in 5.7 is a shim that calls `require("node:module").enableCompileCache()` (when present, Node.js ≥ 22.1) before loading `_tsc.js`. The V8 code cache goes to `$TMPDIR/node-compile-cache` (verified: the directory is created on first run with 5.7.3 and not with 5.6.3). The team measured `tsc --version` dropping from about 122 ms to 48 ms. It needs no configuration. Set `NODE_DISABLE_COMPILE_CACHE=1` to opt out, for example on read-only or shared temp directories.

##### `lib.d.ts` (DOM) Updates

The DOM declarations were regenerated for 5.7 and can introduce new errors in browser code. Do not pin DOM types by vendoring `lib.dom.d.ts`. If a specific DOM version is needed, 5.7 still supports `@typescript/lib-dom` replacement packages (`"@typescript/lib-dom": "npm:@types/web@…"`). The `--libReplacement` switch to turn that lookup off is 5.8.

***

#### Do Not Use or Recommend

The project's compiler is 5.7.3. A reviewer must never suggest the following. They fail to compile, are silently ignored, or describe behaviour 5.7 does not have.

##### TypeScript 5.8 (2025-02-28)

| Feature | What happens on 5.7 |
|---|---|
| `--erasableSyntaxOnly` | TS5023 unknown compiler option |
| `--module node18` | TS6046 invalid value |
| `require()` of an ES module from CommonJS under `--module nodenext` (Node.js 22 `require(esm)`) | 5.7 still rejects it: TS1479 for `import { x } from "./esm.mjs"` in a CommonJS file, TS1471 for `import x = require("./esm.mjs")` |
| Per-branch checking of `return cond ? a : b` against the declared return type | 5.7 checks the union of both branches. `return cache.has(k) ? cache.get(k) : k` with an `any` branch silently passes |
| `--libReplacement` | unknown option. 5.7 always looks up `@typescript/lib-*` |
| Computed property names preserved in declaration files (`[propName]: number` in `.d.ts`) | 5.7 emits an index signature or reports an error |
| TS2880 for `import … assert { … }` under `nodenext` | not reported. See the JSON import section |
| `lib` types for `Float16Array`, `Math.f16round`, `Promise.try` (5.8 `esnext`) | not declared even with `lib: ["esnext"]` (TS2552 for `Float16Array`, TS2551 for `Math.f16round`, TS2339 for `Promise.try`, verified) |

##### TypeScript 5.9 (2025-07-31)

| Feature | What happens on 5.7 |
|---|---|
| `import defer * as ns from "…"` | parse errors (TS1005/TS1434) |
| `--module node20` | TS6046 invalid value |
| the minimal `tsc --init` template | 5.7's `tsc --init` writes the old, fully commented template (`target: es2016`, `module: commonjs`) |
| `Error.isError`, `Atomics.pause` lib types (`esnext`) | not in 5.7 libs |
| `ArrayBuffer` no longer a supertype of TypedArrays / `Buffer` (5.9 `lib.d.ts`) | 5.7 still accepts some assignments 5.9 rejects. Do not "fix" 5.9 errors in a 5.7 project |

##### TypeScript 6.0 (2026-03-23)

- No `es2025` `target`/`lib`.
- No `Temporal` types.
- No `Map`/`WeakMap` `getOrInsert`/`getOrInsertComputed` types.
- No `RegExp.escape`, `Uint8Array.fromBase64`/`toHex`, or `Intl.DurationFormat` types. None of these are in 5.7's `esnext` lib (verified).
- No `#/` subpath imports. 5.7 requires at least one character after `#`.
- No `--moduleResolution bundler` together with `--module commonjs`. 5.7 reports TS5095: `bundler` needs `module` `preserve` or `es2015`+.
- No `--stableTypeOrdering` and no `ignoreDeprecations: "6.0"`.
- 6.0 changes defaults: `strict` on, `module` `esnext`, floating `target`, `types: []`, `rootDir` `.`, `noUncheckedSideEffectImports` on. 5.7 has the **old** defaults: `strict` off, `target` `es5` (verified: a default parameter compiles to `if (a === void 0)`), and all visible `@types/*` packages included. Never assume a 6.0 default. Write options explicitly.
- 6.0 deprecations do **not** apply to 5.7: `--moduleResolution node10`, `baseUrl`, `outFile`, `module amd/umd/system`, `target es5`, `downlevelIteration`, `esModuleInterop: false`. A 5.7 code base may still use them legitimately. When it does, recommend the migration path, not a 6.0 setting.
- `"lib": ["dom"]` alone does not include `dom.iterable` in 5.7. Iterating a `NodeList` with `for…of` needs `"dom.iterable"` in the `lib` list (the merge is 6.0).

##### TypeScript 7.0 (2026-07-08)

7.0 is the native (Go) compiler, shipped in the `typescript` npm package as `tsc`. It removes everything 6.0 deprecated and ships without a programmatic API: tools that need one keep using the 6.0 API (`@typescript/typescript6`), and a new API is expected in 7.1. A 5.7 project upgrades through 5.8 → 5.9 → 6.0 (fixing deprecations) before 7.0. Do not recommend `tsgo`/`@typescript/native-preview` commands for a 5.7 build.

##### Removed Before 5.7 (Do Not Accept in Configs)

The options deprecated in 5.0 stopped working in 5.5. `"ignoreDeprecations": "5.0"` no longer revives them. On 5.7 each one is a hard error (TS5102/TS5108, verified) when set to a non-default value. Set to its default (`"importsNotUsedAsValues": "remove"`, `"preserveValueImports": false`, `"keyofStringsOnly": false`), 5.7.3 accepts it silently, so such a leftover line is dead configuration, not proof the option still works:

- `importsNotUsedAsValues` and `preserveValueImports`: use `verbatimModuleSyntax`.
- `out`: use `outFile`.
- `target: "es3"`.
- `keyofStringsOnly`, `suppressExcessPropertyErrors`, `suppressImplicitAnyIndexErrors`, `noStrictGenericChecks`, `noImplicitUseStrict`, `charset`.
- `prepend` in project references.

Also obsolete for years: TSLint (deprecated since 2019; use typescript-eslint) and namespaces for code organisation (use ES modules).

##### Version-Aware Review

- Verify an API or option against **5.7.3** before claiming it is missing. Check the `lib` files in `node_modules/typescript/lib`, run `tsc --showConfig`, or compile a probe. Say "not available in TypeScript 5.7" only when the pinned compiler proves it.
- Distinguish **compiler version** from **`lib`/`target` choice**. `Object.groupBy` exists in 5.7 but only with `lib` ≥ `es2024` (or `esnext`). Missing `lib` entries are a config problem, not a version problem.
- Distinguish **types** from **runtime**. Even when the types exist, check the deployment runtime: Node.js version in `engines`, Dockerfile, CI, or the browserslist.
- `package.json` ranges such as `"typescript": "~5.7.2"` pin 5.7. `"^5.7.2"` allows 5.8 and 5.9 to install. Then the guidance of a newer version applies, and a lockfile shows which one is real.
- typescript-eslint officially supports TypeScript 5.7 from **8.16.0** (2024-11-25). Older 8.x releases print an "unsupported TypeScript version" warning and may misparse.
- DefinitelyTyped tests `@types/*` only on TypeScript versions younger than two years. 5.7 (November 2024) leaves that window in November 2026. New `@types/*` releases may start using syntax or lib types that 5.7 does not understand. Pin `@types/node` and other critical `@types` packages, and prefer `skipLibCheck: true` in applications.

***

#### Type System Fundamentals

##### `any`, `unknown`, `never`, `object`, `{}`

- `any` disables checking in both directions and spreads: `any.foo.bar()` is `any`, and `any` is assignable to everything. Treat each `any` in a diff as a finding unless it is in a well-commented interop shim.
- `unknown` is the safe top type. Nothing can be done with it until it is narrowed. Use it for parsed JSON, `catch` variables, message payloads and generic "anything" inputs.
- `never` is the empty type. Use it in exhaustiveness checks and as the return type of functions that always throw or never return.
- `object` means any non-primitive. `{}` means "any value except `null`/`undefined`", primitives included. `Object` (capital) is almost never right. typescript-eslint `no-empty-object-type`, `no-wrapper-object-types` and `no-unsafe-function-type` flag `{}`, `Object`, `String` and `Function`.

```ts
// BAD: any leaks through every expression
function parseBad(json: string): any {
  return JSON.parse(json);
}
const port: number = parseBad('{"port":"80"}').port;   // compiles; port is the string "80" at runtime

// GOOD: unknown forces validation
interface Config { port: number }
function isConfig(v: unknown): v is Config {
  return typeof v === "object" && v !== null && "port" in v && typeof v.port === "number";
}
function parseConfig(json: string): Config {
  const v: unknown = JSON.parse(json);
  if (!isConfig(v)) throw new Error("invalid config");
  return v;
}
export { port, parseConfig };
```

`"port" in v` narrows `v` to `object & Record<"port", unknown>` (4.9+), so `v.port` is accessible without a cast.

##### Literal Types, Widening and `as const`

```ts
let a = "GET";                   // string (mutable binding widens)
const b = "GET";                 // "GET"
const c = { method: "GET" };     // { method: string }  (properties widen)
const d = { method: "GET" } as const;          // { readonly method: "GET" }
const methods = ["GET", "POST"] as const;      // readonly ["GET", "POST"]
type Method = (typeof methods)[number];        // "GET" | "POST"

declare function request(m: "GET" | "POST"): void;
request(c.method);               // error TS2345: string is not assignable
request(d.method);
export { a, b, type Method };
```

##### `type` vs `interface`

Both describe object shapes. Differences that matter in review:

- `interface` declarations **merge**. Two `interface Window {}` blocks combine, which is useful for global augmentation and surprising by accident. `type` aliases cannot be reopened.
- `interface X extends Y` reports incompatible overrides at the declaration. `type X = Y & {...}` silently produces `never` members on conflicts.
- Unions, tuples, mapped and conditional types need `type`.
- Interfaces are cached by name and are usually cheaper for the checker in very large hierarchies.

```ts
interface A { id: string }
interface B extends A { id: number }           // error TS2430: incorrectly extends
type C = { id: string } & { id: number };     // no error, but C["id"] is never
declare const c: C;
const id: never = c.id;
export { id };
```

Pick one convention per code base (typescript-eslint `consistent-type-definitions`). Do not churn code between them.

##### Structural Typing and Excess Property Checks

Excess property checks apply only to **fresh object literals** assigned directly to a typed target. An intermediate variable bypasses them:

```ts
interface Options { timeoutMs?: number; retries?: number }
declare function connect(o: Options): void;

connect({ timeoutMs: 1000, retry: 3 });        // error TS2353: 'retry' does not exist in type 'Options'
const opts = { timeoutMs: 1000, retry: 3 };
connect(opts);                                 // compiles: the typo is silently ignored
```

Annotate the variable (`const opts: Options = …`) or use `satisfies Options` so the check applies where the literal is written.

##### Optional Properties and `exactOptionalPropertyTypes`

Without `exactOptionalPropertyTypes`, `prop?: T` accepts an explicit `undefined`. That differs at runtime from an absent key: `"prop" in o`, `Object.keys`, spreading over defaults, and JSON serialisation all see it.

```ts
interface Settings { theme?: "dark" | "light" }
const defaults = { theme: "light" as const };
const user: Settings = { theme: undefined };              // error TS2375 with exactOptionalPropertyTypes
const merged = { ...defaults, ...user };                  // { theme: undefined } at runtime without the flag
export { merged };
```

With the flag on, write `theme?: "dark" | "light" | undefined` where an explicit `undefined` is intended.

##### Index Signatures and `noUncheckedIndexedAccess`

Without `noUncheckedIndexedAccess`, `record[key]` and `array[i]` are typed as present. That is the most common source of "cannot read properties of undefined" in strict code bases. With the flag, they include `undefined`:

```ts
const counts: Record<string, number> = {};
const n = counts["missing"];            // number | undefined
const total = n + 1;                    // error TS18048: 'n' is possibly 'undefined'
const xs = [1, 2, 3];
const first = xs[0];                    // number | undefined
for (const x of xs) console.log(x + 1); // for-of yields number: no noise
export { total, first };
```

`noPropertyAccessFromIndexSignature` additionally forces `obj["key"]` instead of `obj.key` for index-signature access, making dynamic lookups visible in review.

`Record<string, T>` used as a dictionary keeps `Object.prototype` keys reachable (`counts["constructor"]`, `counts["__proto__"]`). For untrusted keys use `Map<string, T>` or `Object.create(null)`. See [Security](#security).

##### Readonly and Immutability

```ts
interface Point { readonly x: number; readonly y: number }
function move(p: Point): Point {
  p.x += 1;                                   // error TS2540: cannot assign to 'x' because it is a read-only property
  return { ...p, x: p.x + 1 };
}
function sum(xs: readonly number[]): number {
  xs.push(1);                                 // error TS2339: 'push' does not exist on readonly number[]
  return xs.reduce((a, b) => a + b, 0);
}
const frozen = Object.freeze({ a: [1, 2] });
frozen.a.push(3);                             // compiles: freeze and readonly are shallow
export { move, sum };
```

`readonly` is a compile-time property. A mutable alias to the same object can still change it (`const p: { x: number } = point` is allowed). Use `ReadonlyArray`/`readonly T[]` and `ReadonlyMap`/`ReadonlySet` in public signatures that should not mutate inputs.

##### Tuples

```ts
type Range = [start: number, end: number];             // labelled elements
type Args = [string, ...number[]];                     // leading fixed, rest variadic
type Opt = [id: string, name?: string];
function f(...args: Args) { return args[0].length; }
const r: Range = [1, 2];
const [s, e] = r;
const bad: Range = [1, 2, 3];                         // error TS2322: source has 3 elements
export { f, s, e, bad, type Opt };
```

`array.push` on a tuple type is still allowed (tuples are arrays). Use `readonly [number, number]` when the length must stay fixed.

***

#### Narrowing and Control-Flow Analysis

##### Built-in Guards

`typeof`, `instanceof`, `in`, `===`/`!==` against literals, truthiness, `Array.isArray`, and discriminant checks all narrow.

```ts
function describe(x: string | number | Date | string[] | null | undefined): string {
  if (x == null) return "nothing";                 // removes null and undefined
  if (typeof x === "string") return x.toUpperCase();
  if (typeof x === "number") return x.toFixed(2);
  if (x instanceof Date) return x.toISOString();
  return x.join(",");                              // string[]
}
export { describe };
```

Pitfalls:

- `typeof null === "object"`. `typeof x === "object"` does not exclude `null`. Add `x !== null`.
- Truthiness removes `0`, `""`, `NaN` and `false` as well as `null`/`undefined`. `if (count)` is wrong when `0` is valid, so use `count !== undefined` or `count != null`. typescript-eslint `strict-boolean-expressions` enforces this.
- `instanceof` fails across realms (iframes, `vm` contexts, duplicated package copies). Use `Array.isArray`, or a brand/discriminant for cross-realm values.
- Property narrowing survives function calls even when the call mutates the object. This is a deliberate unsoundness:

```ts
interface Box { value: string | undefined }
function reset(b: Box) { b.value = undefined; }
function use(b: Box) {
  if (b.value !== undefined) {
    reset(b);
    return b.value.length;     // compiles; throws at runtime because reset() cleared it
  }
  return 0;
}
export { use };
```

  Copy the narrowed value into a `const` before calling code that may mutate the object.

##### Discriminated Unions and Exhaustiveness

```ts
type Shape =
  | { kind: "circle"; radius: number }
  | { kind: "square"; side: number }
  | { kind: "rect"; w: number; h: number };

function assertNever(x: never): never {
  throw new Error(`unexpected: ${JSON.stringify(x)}`);
}

function area(s: Shape): number {
  switch (s.kind) {
    case "circle": return Math.PI * s.radius ** 2;
    case "square": return s.side ** 2;
    case "rect": return s.w * s.h;
    default: return assertNever(s);      // adding a new kind makes this line fail to compile
  }
}

function areaIncomplete(s: Shape): number {
  switch (s.kind) {
    case "circle": return 1;
    case "square": return 2;
    default: return assertNever(s);      // error TS2345: '{ kind: "rect"; ... }' is not assignable to 'never'
  }
}
export { area, areaIncomplete };
```

Without the `default: assertNever(…)` branch, a function with a declared return type still reports a missing case through TS2366 ("Function lacks ending return statement"), but a `void` function silently ignores the new member. typescript-eslint `switch-exhaustiveness-check` covers both.

Destructured discriminants narrow their siblings too (4.6+), as long as the destructuring is `const` or a parameter without defaults:

```ts
type Action = { kind: "n"; payload: number } | { kind: "s"; payload: string };
function handle({ kind, payload }: Action) {
  if (kind === "n") return payload.toFixed(1);   // payload: number
  return payload.toUpperCase();                  // payload: string
}
export { handle };
```

##### User-Defined Type Guards, Inferred Predicates (5.5) and Assertion Functions

A predicate `x is T` is trusted blindly. A wrong body creates unsoundness:

```ts
interface Cat { meow(): void }
// BAD: lies; any object passes
function isCatBad(x: unknown): x is Cat { return typeof x === "object"; }
// GOOD: checks what it claims
function isCat(x: unknown): x is Cat {
  return typeof x === "object" && x !== null && "meow" in x && typeof x.meow === "function";
}
export { isCatBad, isCat };
```

Since 5.5, TypeScript **infers** a type predicate for functions that return a boolean narrowing expression on a parameter and do not mutate it. The classic `filter` case therefore needs no annotation:

```ts
const xs = [1, undefined, 2].filter((x) => x !== undefined);   // number[]
const ys = [1, null, 2].filter(Boolean);                       // (number | null)[]: Boolean is not a predicate
const zs = [0, 1, null].filter((x) => !!x);                    // (number | null)[]: no inference for truthiness, 0 would be lost
export { xs, ys, zs };
```

Inference applies only when the predicate holds in both directions ("if it returns false, the value is *not* T"). Truthiness checks on `number | null` do not qualify because `0` is falsy.

Assertion functions narrow by throwing:

```ts
function assertDefined<T>(v: T, msg: string): asserts v is NonNullable<T> {
  if (v === undefined || v === null) throw new Error(msg);
}
declare const maybe: string | undefined;
assertDefined(maybe, "missing");
maybe.toUpperCase();
```

An assertion function must be called through an explicitly typed reference. Arrow functions assigned to an un-annotated `const` produce TS2775.

##### Narrowing in Closures (5.4) and Constant Indexed Access (5.5)

```ts
function format(input: string | number) {
  if (typeof input === "string") input = Number(input);
  // 5.4+: after the last assignment, closures created later keep the narrowing
  return () => input.toFixed(2);
}

function read(obj: Record<string, unknown>, key: string): string {
  // 5.5+: obj[key] narrows when both obj and key are effectively constant
  if (typeof obj[key] === "string") return obj[key];
  return "";
}
export { format, read };
```

If the variable is reassigned *after* the closure is created, the narrowing is dropped, correctly.

##### `switch (true)` Narrowing (5.3)

```ts
function kind(x: unknown): string {
  switch (true) {
    case typeof x === "string": return x.toUpperCase();
    case Array.isArray(x): return `array(${x.length})`;
    default: return "other";
  }
}
export { kind };
```

##### Always-Truthy and Always-Nullish Checks (5.6)

Since 5.6 the compiler rejects conditions it can prove syntactically constant:

```ts
declare const s: string;
declare const opts: { max?: number };
if (/^\d+$/) {}                                     // error TS2872: This kind of expression is always truthy.
if (() => s) {}                                     // error TS2872
const v = { a: 1 } ?? {};                           // error TS2869: Right operand of ?? is unreachable because the left operand is never nullish.
const bad = s.length < (opts.max ?? 100);           // GOOD: explicit parentheses
while (true) { break; }                             // `true`, `false`, `0`, `1` stay allowed
export { v, bad };
```

The check is syntactic. `if (someFunction)` (forgetting to call it) is caught by `strictNullChecks` "This condition will always return true since this function is always defined" (TS2774) only when the function is not called inside the block. typescript-eslint `no-unnecessary-condition` and `no-misused-promises` (for `if (promise)`) cover the rest.

##### Definite Assignment

```ts
class Conn {
  socket!: { write(s: string): void };        // `!`: "trust me, initialised elsewhere"
  constructor() { this.init(); }
  private init() { this.socket = { write() {} }; }
}
let token!: string;                            // same for locals; hides the 5.7 never-initialized check
export { Conn, token };
```

Every `!` definite-assignment and every non-null assertion `x!` is an unchecked claim. Ask for the invariant in a comment, or restructure (initialise in the constructor, use a factory, return the value).

***

#### Generics

##### Constraints and Inference

```ts
function getProp<T, K extends keyof T>(obj: T, key: K): T[K] {
  return obj[key];
}
const user = { id: 1, name: "a" };
const name = getProp(user, "name");          // string
getProp(user, "email");                      // error TS2345: '"email"' is not assignable to '"id" | "name"'

// BAD: unconstrained T with a cast; the signature lies
function parse<T>(json: string): T { return JSON.parse(json) as T; }
const n: number = parse<number>('"oops"');   // compiles; n is a string at runtime
export { name, n };
```

A type parameter that appears only in the return type (`parse<T>(): T`) is a disguised cast. typescript-eslint `no-unnecessary-type-parameters` flags it. Return `unknown` and validate instead.

##### `const` Type Parameters (5.0)

```ts
function routes<const T extends readonly string[]>(paths: T): T { return paths; }
const r = routes(["/a", "/b"]);              // readonly ["/a", "/b"], no `as const` at call site
function loose<T extends string[]>(paths: T): T { return paths; }
const l = loose(["/a", "/b"]);               // string[]
export { r, l };
```

The `const` modifier is what keeps the literals: `loose` (no `const`) infers `string[]`. Prefer a `readonly` constraint. With a mutable constraint (`<const T extends string[]>`), 5.3+ infers a mutable tuple (`["/a", "/b"]`, verified on 5.7.3); before 5.3 it fell back to `string[]`.

##### `NoInfer<T>` (5.4)

```ts
function createFSM<S extends string>(states: S[], initial: NoInfer<S>): S { return initial; }
createFSM(["idle", "running"], "idle");
createFSM(["idle", "running"], "stopped");   // error TS2345: without NoInfer, S would widen to include "stopped"
```

##### Defaults, Variance Annotations, Overloads

```ts
interface Box<T = string> { value: T }
const b: Box = { value: "x" };

interface Producer<out T> { get(): T }       // covariant (4.7 variance annotations)
interface Consumer<in T> { set(v: T): void } // contravariant

// Overloads: implementation signature is not callable from outside
function len(x: string): number;
function len(x: unknown[]): number;
function len(x: string | unknown[]): number { return x.length; }
len("ab");
len([1]);
declare const either: string | number[];
len(either);                                  // error TS2769: no overload matches a union; add a union overload or use a union parameter
export { b, type Producer, type Consumer };
```

Prefer a single union-typed signature over overloads when the return type does not depend on the argument type (typescript-eslint `unified-signatures`).

***

#### `satisfies`, Assertions and Non-Null Assertions

##### `satisfies` (4.9)

`satisfies` checks a value against a type **without** changing the inferred type:

```ts
type Route = { path: string; auth: boolean };
const routes = {
  home: { path: "/", auth: false },
  admin: { path: "/admin", auth: true },
} satisfies Record<string, Route>;
routes.admin.path;                         // keys stay precise: "home" | "admin"
routes.missing;                            // error TS2339

const routes2: Record<string, Route> = routes;
routes2.missing.path;                      // error TS18048 with noUncheckedIndexedAccess; without it, a runtime crash
const palette = { red: [255, 0, 0], green: "#0f0" } satisfies Record<string, string | number[]>;
palette.green.toUpperCase();               // still known to be a string
```

Use `satisfies` for config objects and lookup tables. Use an annotation when the variable should be widened on purpose (public API).

##### Type Assertions (`as`)

`x as T` is allowed when either type is assignable to the other. It performs **no runtime check**. Double assertions (`as unknown as T`) bypass even that. Every assertion in reviewed code needs a reason.

```ts
interface User { id: string; isAdmin: boolean }
const fromApi: unknown = JSON.parse('{"id":"1"}');
const u1 = fromApi as User;                    // compiles; u1.isAdmin is undefined at runtime
const u2 = "x" as unknown as User;             // compiles; nonsense
const n = "x" as number;                       // error TS2352: neither type sufficiently overlaps
const el = { tag: "div" } as const;            // `as const` is not an assertion of a different type: always fine
export { u1, u2, n, el };
```

Alternatives: a type guard, a schema validator (zod, valibot, ajv, typebox), `satisfies`, or a typed factory.

##### Non-Null Assertion `x!`

`x!` removes `null | undefined` without checking. Common real bugs:

- `map.get(key)!` where the key may be absent
- `document.querySelector(...)!` on an optional element
- `array.find(...)!`

Prefer an explicit check that throws a descriptive error, or `??` with a default. typescript-eslint `no-non-null-assertion` (strict config) and `no-non-null-asserted-optional-chain` (`a?.b!`) catch the worst cases.

***

#### Advanced Types

##### Mapped Types, Key Remapping and Modifiers

```ts
type Getters<T> = {
  [K in keyof T & string as `get${Capitalize<K>}`]: () => T[K];
};
interface Person { name: string; age: number }
type PersonGetters = Getters<Person>;   // { getName: () => string; getAge: () => number }

type Mutable<T> = { -readonly [K in keyof T]: T[K] };
type Concrete<T> = { [K in keyof T]-?: T[K] };
type OmitByValue<T, V> = { [K in keyof T as T[K] extends V ? never : K]: T[K] };
type NoNumbers = OmitByValue<Person, number>;  // { name: string }
const g: PersonGetters = { getName: () => "a", getAge: () => 1 };
export { g, type Mutable, type Concrete, type NoNumbers };
```

Homomorphic mapped types (`[K in keyof T]`) preserve `readonly`/`?` modifiers and map over array/tuple types element-wise. Non-homomorphic ones (`[K in Keys]`) do not.

##### Conditional Types and `infer`

```ts
type ElementOf<T> = T extends readonly (infer E)[] ? E : never;
type A1 = ElementOf<string[]>;                       // string
type Distrib<T> = T extends string ? "s" : "o";
type D1 = Distrib<string | number>;                  // "s" | "o"  (distributes over unions)
type NonDistrib<T> = [T] extends [string] ? "s" : "o";
type D2 = NonDistrib<string | number>;               // "o"
type D3 = Distrib<never>;                            // never  (empty union)
type Awaited2 = Awaited<Promise<Promise<number>>>;   // number
type NumFromString<T> = T extends `${infer N extends number}` ? N : never;
type N1 = NumFromString<"42">;                       // 42
const check: [A1, D1, D2, D3, Awaited2, N1] = ["x", "s", "o", undefined as never, 1, 42];
export { check };
```

Pitfalls: distribution surprises (`T extends …` on a naked type parameter distributes), `never` collapsing, and deep recursion. TS2589 ("Type instantiation is excessively deep and possibly infinite") fires at an instantiation depth of 100 or after 5,000,000 instantiations within a single checked expression or statement. Tail-recursive conditional types get up to 1000 iterations (limits read from the 5.7.3 checker source).

##### Template Literal Types

```ts
type HttpMethod = "GET" | "POST";
type Endpoint = `/${string}`;
type RouteKey = `${HttpMethod} ${Endpoint}`;
const k: RouteKey = "GET /users";
const bad: RouteKey = "PUT /users";                  // error TS2322
type EventName<T extends string> = `on${Capitalize<T>}`;
const e: EventName<"click"> = "onClick";
export { k, bad, e };
```

##### Built-in Utility Types

`Partial`, `Required`, `Readonly`, `Pick`, `Omit`, `Record`, `Exclude`, `Extract`, `NonNullable`, `ReturnType`, `Parameters`, `ConstructorParameters`, `InstanceType`, `Awaited`, `ThisType`, `NoInfer` (5.4), `Uppercase`/`Lowercase`/`Capitalize`/`Uncapitalize`.

- `Omit<T, K>` does not check that `K` is a key of `T` (`Omit<User, "nmae">` compiles). It also flattens unions: `Omit<A | B, "x">` is not a union of omissions. Write a distributive version (`T extends unknown ? Omit<T, K> : never`) for unions.
- `Partial<T>` is shallow.
- `ReturnType<typeof f>` of an overloaded function takes the **last** overload.
- `Record<K, V>` with `K = string` claims every key exists. Combine it with `noUncheckedIndexedAccess`, or use `Partial<Record<…>>`.

```ts
interface User { id: string; name: string }
type NoName = Omit<User, "nmae">;          // compiles: typo not detected, `name` still present
type Strict<T, K extends keyof T> = Omit<T, K>;
type NoName2 = Strict<User, "nmae">;       // error TS2344
const x: NoName = { id: "1", name: "n" };
export { x, type NoName2 };
```

##### Branded (Nominal) Types

```ts
type UserId = string & { readonly __brand: "UserId" };
type OrderId = string & { readonly __brand: "OrderId" };
const asUserId = (s: string): UserId => s as UserId;   // single validated entry point
function loadUser(id: UserId) { return id; }
const oid = "o-1" as OrderId;
loadUser(asUserId("u-1"));
loadUser(oid);                                          // error TS2345
loadUser("u-1");                                        // error TS2345
```

***

#### Functions

##### Parameter Bivariance: Method vs Property Syntax

`strictFunctionTypes` checks function-typed **properties** contravariantly, but **method** signatures stay bivariant:

```ts
interface Animal { name: string }
interface Dog extends Animal { bark(): void }
interface HandlerProp { handle: (a: Animal) => void }   // property syntax: checked
interface HandlerMethod { handle(a: Animal): void }     // method syntax: bivariant
const dogOnly = (d: Dog) => d.bark();
const h1: HandlerProp = { handle: dogOnly };            // error TS2322
const h2: HandlerMethod = { handle: dogOnly };          // compiles; crashes when called with a non-Dog
export { h1, h2 };
```

Prefer property syntax for callback members in interfaces (typescript-eslint `method-signature-style: property`).

##### `void` Return Types

A function type returning `void` accepts functions that return values. This is intentional, so `arr.forEach(x => list.push(x))` works, but it hides promises:

```ts
const results: number[] = [];
[1, 2].forEach((x) => results.push(x));              // fine: push returns a number, ignored
declare function save(x: number): Promise<void>;
[1, 2].forEach(async (x) => { await save(x); });      // compiles; promises are dropped, errors unhandled
```

Use `for…of` with `await`, or `await Promise.all(xs.map(save))`. typescript-eslint `no-misused-promises` (`checksVoidReturn`) flags async callbacks passed where `void` is expected.

##### `this` Parameters and Unbound Methods

```ts
class Counter {
  count = 0;
  inc() { this.count++; }
  incArrow = () => { this.count++; };
}
const c = new Counter();
const f = c.inc;
setTimeout(f, 0);           // compiles; `this` is undefined at runtime ("Cannot read properties of undefined")
setTimeout(c.incArrow, 0);  // fine
setTimeout(() => c.inc(), 0);
```

typescript-eslint `unbound-method` catches `const f = c.inc`. Declare `this: void` on functions that must not use `this`.

##### Optional Parameters and Defaults

- `f(x?: T)` and `f(x: T | undefined)` differ: the latter must be passed explicitly.
- Default parameters apply only for `undefined`, not `null`.
- A required parameter after an optional one is an error (TS1016).

***

#### Classes

##### Class Fields: `useDefineForClassFields` and Initialisation Order

`useDefineForClassFields` defaults to `true` when `target` is `es2022` or later (including `esnext`/`es2024`). Fields are then emitted as native class fields with `[[Define]]` semantics. Two consequences a reviewer must know:

```ts
interface Animal { sound: string }
interface Dog extends Animal { bark(): void }
class Base {
  animal: Animal;
  constructor(a: Animal) { this.animal = a; this.init(); }
  init(): void {}
}
class Derived extends Base {
  override animal!: Dog;     // error TS2612: would re-initialise to undefined after super()
}
class Derived2 extends Base {
  declare animal: Dog;       // GOOD: type-only redeclaration, nothing emitted
}
class Init extends Base {
  ready = false;
  override init(): void { this.ready = true; }   // runs inside super(), BEFORE `ready = false`
}
const dog: Dog = { sound: "woof", bark() {} };
console.log(new Derived2(dog).animal.sound, new Init(dog).ready);   // "woof" false
```

Verified on Node.js 22 with 5.7.3. With `target: es2022`, `Derived` (if forced to compile) prints `undefined` for `animal`, while `target: es2020` prints the object. `Init.ready` is `false` under both, because field initialisers always run after `super()` returns. TypeScript does **not** report the `Init` pattern (calling an overridable method from a base constructor). Flag it in review.

Changing `target` from `es2020` to `es2022` (or toggling `useDefineForClassFields`) is a runtime-behaviour change for classes whose fields shadow accessors or base properties. It also changes the emitted code that legacy decorators, MobX and older ORMs depend on. Review such config diffs as code changes.

##### Parameter Properties, `#private` vs `private`, `override`

```ts
class Account {
  #balance = 0;                         // runtime-private (ES private field)
  private audit: string[] = [];         // compile-time only; visible via (acct as any).audit and JSON.stringify
  constructor(public readonly id: string) {}   // parameter property: not erasable syntax
  deposit(n: number) { this.#balance += n; this.audit.push(`+${n}`); }
  get balance() { return this.#balance; }
}
class Savings extends Account {
  override deposit(n: number) { super.deposit(n * 1.01); }
  depsit(n: number) { super.deposit(n); }  // typo creates a new method; caught only when marked override
}
const a = new Savings("1");
a.deposit(10);
console.log(JSON.stringify(a));          // {"id":"1","audit":["+10.1"]}: `private` leaks, `#balance` does not
```

- Use `#private` for secrets and invariants that must hold at runtime. TypeScript `private` is a lint-level hint.
- `noImplicitOverride` (recommended) makes `override` mandatory, so renames in the base class surface as errors.
- Parameter properties, `enum`, runtime `namespace`, `import =` and `export =` are **not erasable**. Avoid them in code meant for Node.js type stripping (see the 5.7 section on `rewriteRelativeImportExtensions`).

##### `accessor` (Auto-Accessors, 4.9) and Getter/Setter Types (5.1)

```ts
class Temperature {
  accessor celsius = 0;                          // getter + setter over a private slot; decoratable
  #raw = "";
  get value(): string { return this.#raw; }
  set value(v: string | number) { this.#raw = String(v); }   // 5.1+: unrelated setter type allowed
}
const t = new Temperature();
t.value = 42;
const s: string = t.value;
export { s };
```

##### Abstract Classes and `implements`

`implements` checks shape only. It does not change the class's own member types (a parameter still needs its annotation). Abstract members must be implemented by concrete subclasses. Abstract constructors are typed `abstract new (...args) => T`.

***

#### Decorators

TypeScript 5.7 supports two incompatible decorator systems:

| | Standard (TC39 stage 3) | Legacy (`experimentalDecorators: true`) |
|---|---|---|
| Enabled by | default (5.0+) | `"experimentalDecorators": true` |
| Signature | `(value, context: ClassMethodDecoratorContext …)` | `(target, propertyKey, descriptor)` |
| Parameter decorators | not supported (TS1206) | supported |
| `emitDecoratorMetadata` (reflect-metadata) | not available (TS5052 without `experimentalDecorators`) | supported |
| Metadata | `context.metadata` / `Symbol.metadata` (5.2) | `reflect-metadata` |
| Used by | new code; libraries that document "standard" / "2022.3" decorator support | Angular, NestJS, TypeORM, InversifyJS, class-validator |

```ts
function logged<This, Args extends unknown[], R>(
  target: (this: This, ...args: Args) => R,
  context: ClassMethodDecoratorContext<This, (this: This, ...args: Args) => R>,
) {
  const name = String(context.name);
  return function (this: This, ...args: Args): R {
    console.log(`enter ${name}`);
    return target.call(this, ...args);
  };
}

function bound<This, F extends (this: This, ...args: any) => any>(   // same constraint as ClassMethodDecoratorContext
  _t: F, context: ClassMethodDecoratorContext<This, F>,
) {
  context.addInitializer(function (this: This) {
    const self = this as Record<string | symbol, unknown>;
    const fn = self[context.name] as F;
    self[context.name] = fn.bind(this);
  });
}

class Greeter {
  constructor(private readonly who: string) {}
  @logged @bound greet(): string { return `hi ${this.who}`; }
}
const greet = new Greeter("bob").greet;
console.log(greet());          // "enter greet", then "hi bob"
```

Review points:

- Mixing the systems is impossible per project. A legacy-style decorator (3 parameters) used without `experimentalDecorators` fails with TS1241/TS1270. Do not "fix" that by flipping the flag in a code base that uses standard decorators elsewhere.
- Standard decorators emit helper code (`__esDecorate`, `__runInitializers`) for every target below `esnext`. No JavaScript runtime ships native decorators as of October 2026, and Node.js type stripping rejects them. With `importHelpers`, they need `tslib` ≥ 2.5 (the first release with `__esDecorate`/`__runInitializers`).
- Decorator metadata (5.2) needs `Symbol.metadata` at runtime. Node.js 18–26 do not define it (verified: `typeof Symbol.metadata === "undefined"`), so polyfill it (`(Symbol as { metadata?: symbol }).metadata ??= Symbol("Symbol.metadata")`) before any decorated class is evaluated.
- 5.5 tightened decorator parsing to the proposal grammar: `@foo`, `@foo.bar`, `@foo.bar()` and `@(expr)` are fine, while forms such as `@super.decorate` now need parentheses (`@(super.decorate)`).

***

#### Enums and Their Alternatives

##### Numeric and String Enums

```ts
export enum Color { Red, Green }
const c: Color = 5;                  // error TS2322 (5.0+: all enums are union enums)
const c2: Color = 1;                 // OK: numeric literal matching a member
const label = Color[0];              // "Red": reverse mapping exists only for numeric members
export enum Status { Active = "active" }
const s: Status = "active";          // error TS2322: string enums are nominal
for (const k in Color) console.log(k);   // logs "0", "1", "Red", "Green": reverse keys included
export { c, c2, label, s };
```

Pitfalls:

- Iterating a numeric enum with `Object.keys`/`for…in` yields both names and numeric strings. Use `Object.values(E).filter((v) => typeof v === "number")`, or a const object.
- `Color[someNumber]` is typed `string` but is `undefined` for out-of-range values.
- Implicit numeric values renumber when a member is inserted. Persisted or wire values must be explicit: `Red = 0, Green = 1`, or use strings.
- Enums are not erasable syntax (Node.js type stripping rejects them).

##### `const enum` and `isolatedModules`

`const enum` members are inlined by `tsc`. Single-file transpilers (esbuild, swc, Babel, Node.js type stripping) cannot inline enums declared in other files. With `isolatedModules: true`, TypeScript:

- reports TS2748 when code accesses an **ambient** `declare const enum` (for example from a library's `.d.ts`): "Cannot access ambient const enums when 'isolatedModules' is enabled" (the message names `verbatimModuleSyntax` when that flag is the one set)
- treats local `const enum`s like regular enums, emitting the object, when `preserveConstEnums` is set or implied by `isolatedModules`

Never export `const enum` from a library's public `.d.ts`. Consumers using `isolatedModules` cannot use it.

##### Preferred Alternative: `as const` Objects

```ts
export const Role = { Admin: "admin", User: "user" } as const;
export type Role = (typeof Role)[keyof typeof Role];   // "admin" | "user"
function can(role: Role): boolean { return role === Role.Admin; }
can("user");                                            // plain literals accepted (unlike string enums)
can("root");                                            // error TS2345
```

This is erasable, tree-shakable, iterable without reverse-mapping noise, and interoperable with JSON.

***

#### Explicit Resource Management: `using` and `await using` (5.2)

```ts
class TempFile implements Disposable {
  constructor(readonly path: string) {}
  [Symbol.dispose]() { console.log("delete", this.path); }
}
class Pool implements AsyncDisposable {
  async [Symbol.asyncDispose]() { console.log("pool closed"); }
}

export async function job(): Promise<void> {
  await using pool = new Pool();
  using a = new TempFile("/tmp/a");
  using b = new TempFile("/tmp/b");
  console.log("work");
}   // disposes b, then a, then pool, in reverse order, even on throw
```

Requirements and pitfalls (verified with 5.7.3):

- **Types**: `Disposable`, `AsyncDisposable`, `Symbol.dispose`, `DisposableStack` and `SuppressedError` come from `lib` `esnext.disposable` (or `esnext`). They are **not** in `es2024`. `@types/node` also declares `Disposable`/`Symbol.dispose`, which is why Node projects often compile without adding the lib. A browser project with `lib: ["es2024", "dom"]` gets TS2304 "Cannot find name 'Disposable'".
- **Emit**: for any `target` below `esnext` (including `es2024`), `tsc` lowers `using` to `__addDisposableResource`/`__disposeResources` helpers. Those need `Symbol.dispose`/`Symbol.asyncDispose` at runtime: present in Node.js ≥ 18.18/20.4 and missing in older browsers, so polyfill there.
- With **`target: esnext`**, 5.7 emits `using` **verbatim**. Node.js 22 throws a `SyntaxError` on it. Node.js 24 and 26 run it natively. Do not combine `target: esnext` with a Node 22 runtime when the code uses `using`.
- `DisposableStack`/`AsyncDisposableStack` exist natively only from Node.js 24. On 22, polyfill them or avoid them.
- `using x = null` and `using x = undefined` are allowed (nothing to dispose). Any other non-disposable value throws `TypeError` at runtime.
- A `using` declaration must not be exported, and it is block-scoped. Returning the resource from the function disposes it before the caller can use it. That is a real bug pattern: return the resource without `using`, or transfer ownership with `DisposableStack.move()`.

***

#### Errors and Exceptions

##### `catch` Variables Are `unknown` (`useUnknownInCatchVariables`, part of `strict`)

```ts
async function load(url: string): Promise<string> {
  try {
    const res = await fetch(url);
    if (!res.ok) throw new Error(`HTTP ${res.status}`);
    return await res.text();
  } catch (err) {
    if (err instanceof Error) throw new Error(`load ${url} failed`, { cause: err });
    throw new Error(`load ${url} failed: ${String(err)}`);
  }
}
function bad() {
  try { JSON.parse("{"); } catch (e) { console.log(e.message); }   // error TS18046: 'e' is of type 'unknown'
}
export { load, bad };
```

- `Error` `cause` needs `lib` `es2022`.
- Never annotate `catch (e: any)` to silence the check. `catch (e: unknown)` is fine and explicit.
- JavaScript can throw anything. Libraries sometimes throw strings or plain objects, so `instanceof Error` is not exhaustive.
- typescript-eslint `only-throw-error` forbids `throw "string"`. `prefer-promise-reject-errors` does the same for `reject("x")`.
- `.catch((e) => …)` callbacks are typed `any` by `lib.d.ts`, so `useUnknownInCatchVariables` does not apply. typescript-eslint `use-unknown-in-catch-callback-variable` closes that gap.

##### Custom Error Classes

```ts
export class HttpError extends Error {
  override readonly name = "HttpError";
  constructor(readonly status: number, message: string, options?: ErrorOptions) {
    super(message, options);
  }
}
try { throw new HttpError(404, "not found"); }
catch (e) { if (e instanceof HttpError) console.log(e.status, e.name); }
```

With `target` ≥ `es2015`, `instanceof` works for `Error` subclasses. With `target: es5` it breaks (the prototype is not set) unless you call `Object.setPrototypeOf(this, new.target.prototype)`. A 5.7 project still on `es5` needs that call.

##### Result Types

For expected failures (validation, not-found), a discriminated result keeps error handling visible in types:

```ts
type Result<T, E = Error> = { ok: true; value: T } | { ok: false; error: E };
function parsePort(s: string): Result<number, string> {
  const n = Number(s);
  return Number.isInteger(n) && n > 0 && n < 65536 ? { ok: true, value: n } : { ok: false, error: `bad port ${s}` };
}
const r = parsePort("80");
if (r.ok) console.log(r.value); else console.error(r.error);
```

***

#### Async Code

##### Floating and Misused Promises

The compiler does not report unawaited promises. These are the highest-value async review findings:

```ts
declare function save(id: string): Promise<void>;
declare const ids: string[];

export async function bad() {
  save("a");                                   // floating: rejection becomes an unhandledRejection (process exit on Node >= 15)
  ids.forEach(async (id) => { await save(id); });   // forEach ignores the promises
  if (save("b")) { /* always true */ }         // error TS2801: This condition will always return true since this 'Promise<void>' is always defined.
}

export async function good() {
  await save("a");
  await Promise.all(ids.map((id) => save(id)));          // parallel, fail-fast
  const results = await Promise.allSettled(ids.map(save)); // parallel, collect failures
  for (const id of ids) await save(id);                  // sequential when order or rate matters
  void save("fire-and-forget").catch((e: unknown) => console.error(e));  // explicit and handled
  return results;
}
```

Enable typescript-eslint `no-floating-promises`, `no-misused-promises`, `await-thenable`, `require-await` and `return-await` (`in-try-catch`). They need type-aware linting (`parserOptions.projectService: true`).

##### `return await` Inside `try`

```ts
declare function fetchData(): Promise<string>;
async function withoutAwait(): Promise<string> {
  try { return fetchData(); }                 // rejection escapes the catch below
  catch { return "fallback"; }
}
async function withAwait(): Promise<string> {
  try { return await fetchData(); }           // rejection handled here
  catch { return "fallback"; }
}
export { withoutAwait, withAwait };
```

##### Concurrency Limits, Timeouts and Cancellation

- `Promise.all` over thousands of items starts them all at once. Use a pool (p-limit) or chunking for network and DB calls.
- Pass `AbortSignal` through async APIs. `AbortSignal.timeout(ms)` and `AbortSignal.any([...])` are typed in `@types/node` and DOM; check the runtime (`AbortSignal.any` needs Node.js ≥ 20.3 or ≥ 18.17).
- `Promise.withResolvers` needs `lib` `es2024`/`esnext` **and** Node.js ≥ 22.
- Top-level `await` needs `module` `es2022`/`esnext`/`nodenext`/`preserve` and an ES-module output file. Under `nodenext` in a CommonJS-format file it fails with TS1309 ("The current file is a CommonJS module and cannot use 'await' at the top level").

```ts
export async function fetchWithTimeout(url: string, ms: number): Promise<Response> {
  const res = await fetch(url, { signal: AbortSignal.timeout(ms) });
  if (!res.ok) throw new Error(`HTTP ${res.status}`);
  return res;
}
```

***

#### Iterators and Generators

##### `strictBuiltinIteratorReturn` (5.6, part of `strict`)

Built-in iterators (from `Array`, `Map`, `Set`, `String` and other `lib` APIs) now have `TReturn = undefined` instead of `any` under `strict`:

```ts
const it = new Map([["a", 1]]).values();
const r = it.next();
const v: number = r.value;          // error TS2322: 'number | undefined'
if (!r.done) { const ok: number = r.value; }
```

Code that called `.next().value` without checking `done` was always buggy. 5.6+ reports it.

##### Iterator Helpers (types since 5.6, `lib` `esnext`)

```ts
function* naturals() { let i = 0; while (true) yield i++; }
const firstEvens = naturals().filter((n) => n % 2 === 0).map((n) => n * 10).take(3).toArray();  // [0, 20, 40]
```

The types are in `esnext.iterator` (5.6+), not `es2024`, so `lib: ["es2024"]` rejects `.filter` on an iterator. At runtime, iterator helpers exist in Node.js ≥ 22 and current browsers. Do not use them in code that must run on Node.js 20.

##### Generators

`Generator<TYield, TReturn, TNext>`. An `async function*` returns `AsyncGenerator`. Consume async iterables with `for await…of`. Breaking out of a `for…of` calls the iterator's `return()`, which runs `finally` blocks in the generator. Use that for cleanup.

***

#### Modules and Module Resolution

##### Choosing `module` / `moduleResolution`

| Project kind | `module` | `moduleResolution` | Notes |
|---|---|---|---|
| Node.js app or library, run by Node | `nodenext` (or `node16`) | implied (`nodenext`/`node16`) | Mirrors Node's ESM/CJS rules exactly: extensions required in ESM, `exports`/`imports` honoured, format per file |
| Bundled web app (Vite, webpack, esbuild, Rollup, Bun) | `preserve` (5.4+) or `esnext` | `bundler` | Extensionless imports allowed; `exports` honoured; `import x = require()` kept under `preserve` |
| Legacy CommonJS project | `commonjs` | `node10` (`node`) | Ignores `package.json` `exports`; may resolve paths Node.js would reject. Plan a migration to `nodenext` |

Verified constraints in 5.7:

- `--module nodenext` with `--moduleResolution bundler` is rejected (TS5109). So is `bundler` with `commonjs` (TS5095), which is a 6.0 feature.
- `node16` vs `nodenext` in 5.7: both use the same resolution. `nodenext` additionally accepts import attributes (`with { type: "json" }`) and validates JSON imports. Under `node16`, attributes are TS2823.
- **The `classic` trap.** When `moduleResolution` is unset, 5.7 derives it from `module`: `commonjs` → `node10`, `node16`/`nodenext` → same, `preserve` → `bundler`, and **everything else (`es2015`…`esnext`, `amd`, `umd`, `system`) → `classic`**. `module` itself defaults to `es6` when `target` ≥ `es2015`. A tsconfig with only `"target": "es2022"` (or `"module": "esnext"` without `moduleResolution`) therefore resolves in `classic` mode and never looks in `node_modules`: `import { foo } from "foo"` fails with TS2792 (verified with `tsc --showConfig`: `"module": "es6", "moduleResolution": "classic"`). Always set `module`, and set `moduleResolution` unless `module` is `nodenext`/`node16`/`preserve`.

##### File Format Under `node16`/`nodenext`

For each input file, the output format is decided by:

1. the extension: `.mts` → ESM `.mjs`, `.cts` → CJS `.cjs`;
2. for `.ts`/`.tsx`, the nearest `package.json` `"type"` (`"module"` → ESM, otherwise CJS).

Pitfalls verified on 5.7:

- A `.ts` file without `"type": "module"` compiles to CommonJS. Copying the output into a directory whose `package.json` says `"type": "module"` produces `ReferenceError: exports is not defined in ES module scope`. Keep the `package.json` that governs `src/` and `dist/` consistent.
- In ESM files, relative imports need the **output** extension: `import { x } from "./util.js"` for `util.ts`. Extensionless relative imports fail with TS2835 ("Relative import paths need explicit file extensions… Did you mean './util.js'?").
- In CommonJS-format files, `import { x } from "./esm.mjs"` is TS1479. Use `await import("./esm.mjs")`. Synchronous `require(esm)` is only modelled from 5.8.

##### `verbatimModuleSyntax` (5.0)

With `verbatimModuleSyntax: true`, what you write is what is emitted. Imports without `type` are kept; `import type` and inline `type` specifiers are dropped.

```ts
// with verbatimModuleSyntax, in an ESM-format file
import type { IncomingMessage } from "node:http";        // erased
import { createServer, type Server } from "node:http";   // `Server` erased, `createServer` kept
import { ServerResponse } from "node:http";              // kept (a class is a value), although used only as a type
export function start(handler: (req: IncomingMessage, res: ServerResponse) => void): Server {
  return createServer(handler);
}
```

- A type-only symbol imported without `type` (an interface, for example) is TS1484 ("'X' is a type and must be imported using a type-only import when 'verbatimModuleSyntax' is enabled").
- In CommonJS-format files, `verbatimModuleSyntax` forbids ESM syntax (TS1286/TS1287). Write `import fs = require("node:fs")` and `export =`, or switch the package to `"type": "module"`.
- It supersedes `importsNotUsedAsValues`/`preserveValueImports` (removed) and implies the import-elision guarantees that `isolatedModules` single-file transpilers need.
- Inline `type` modifiers keep the statement: `import { type A, type B } from "m"` emits `import {} from "m"`, so the module is still loaded and its side effects still run (verified on 5.7.3). `import type { A, B } from "m"` emits nothing. Use the latter for pure type imports (typescript-eslint `no-import-type-side-effects`).

##### `isolatedModules` and `isolatedDeclarations` (5.5)

`isolatedModules: true` reports constructs that a per-file transpiler (esbuild, swc, Babel, tsx, Vite, Jest's ts-jest isolated mode, Node.js type stripping) cannot compile correctly: re-exporting types without `export type`, ambient `const enum` access, and so on. It belongs in every project that is not compiled solely by `tsc`.

`isolatedDeclarations: true` (requires `declaration` or `composite`) makes every exported API carry explicit types, so `.d.ts` files can be generated per file without the checker:

```ts
// compiled with --declaration --isolatedDeclarations
export function add(a: number, b: number) { return a + b; }        // error TS9007: needs an explicit return type
export const handler = (x: string) => x.length;                     // error TS9007
export const list = [1, 2].map((x) => x * 2);                       // error TS9010: needs an explicit type annotation
export const config = { retries: 3 };                               // OK: inferable from literals
export function ok(a: number, b: number): number { return a + b; }
```

Use it in libraries and monorepos that want parallel or third-party declaration emit. It is noisy for applications.

##### `esModuleInterop`, Default Imports and CommonJS Interop

- `esModuleInterop: true` (implies `allowSyntheticDefaultImports`) makes `import express from "express"` work against `module.exports = fn` packages. Without it, `import * as express from "express"` is the only correct form, and calling a namespace import is a runtime error under real ESM.
- Under `module: nodenext`, ESM files importing CommonJS get `module.exports` as the default import, matching Node. Named imports of CJS work only for the names Node's static analysis (cjs-module-lexer) can detect. TypeScript cannot fully verify that, so a named import that type-checks can still fail at runtime with "Named export 'x' not found". Prefer default import plus destructuring for CJS dependencies.
- Dual packages (ESM + CJS builds) risk the **dual package hazard**: two copies of module state, so `instanceof` and singletons break.

##### Publishing Libraries: `exports`, `types`, `.d.ts`

```json
{
  "name": "my-lib",
  "type": "module",
  "exports": {
    ".": {
      "types": "./dist/index.d.ts",
      "import": "./dist/index.js"
    },
    "./package.json": "./package.json"
  },
  "files": ["dist"],
  "sideEffects": false
}
```

- The `"types"` condition must come **first** in each `exports` object. Conditions are matched in order.
- For dual ESM/CJS publishing, each format needs its own declaration file (`index.d.ts` next to `.js` under `"type": "module"`, `index.d.cts` next to `.cjs`). One `.d.ts` for both formats is "masquerading", reported by `@arethetypeswrong/cli`. Run `attw --pack` and `publint` in CI for libraries.
- `declaration: true` + `declarationMap: true` for go-to-source in consumers. `stripInternal` removes `/** @internal */` members (best-effort; not a security boundary).
- Do not ship `paths` aliases in emitted output. `tsc` never rewrites `paths`, and that includes `rewriteRelativeImportExtensions`. Use `package.json` `imports` (`#internal/*`) or relative paths.

##### Path Aliases: `paths`, `baseUrl`, `imports`

`paths` only tells the **type checker** where modules are. The runtime (Node), bundler and test runner each need matching configuration. A reviewer seeing a new `paths` entry must check the bundler/Vite alias, the Jest/Vitest `moduleNameMapper`/alias, and the runtime (Node does not read tsconfig).

Since 4.1, `paths` works without `baseUrl` (entries resolve relative to the tsconfig). Since 5.5, `${configDir}` can make shared base configs resolve relative to the extending project (`"outDir": "${configDir}/dist"`).

For Node.js, prefer `package.json` `"imports"` (`"#db/*": "./src/db/*.js"`), which both Node and TypeScript (`node16`/`nodenext`/`bundler`) understand. In 5.7 the key must have at least one character after `#` (`#/` is a 6.0 feature).

##### `noUncheckedSideEffectImports` (5.6)

By default, `import "./polyfills"` is not resolved at all, so a typo is silently ignored. `noUncheckedSideEffectImports: true` reports unresolvable side-effect imports (TS2307 "Cannot find module …", verified on 5.7.3). CSS imports in bundler projects then need a `declare module "*.css";`.

***

#### tsconfig Recommendations for 5.7

5.7 has the old defaults: `strict: false`, `target: es5`, all `@types/*` visible in `node_modules/@types` included automatically, and `moduleResolution` derived from `module`. Always write the important options explicitly.

##### Node.js 22 Service (compiled with `tsc`)

```json
{
  "compilerOptions": {
    "target": "es2024",
    "lib": ["es2024"],
    "module": "nodenext",
    "types": ["node"],
    "rootDir": "src",
    "outDir": "dist",
    "sourceMap": true,

    "strict": true,
    "noUncheckedIndexedAccess": true,
    "exactOptionalPropertyTypes": true,
    "noImplicitOverride": true,
    "noImplicitReturns": true,
    "noFallthroughCasesInSwitch": true,
    "noPropertyAccessFromIndexSignature": true,
    "noUncheckedSideEffectImports": true,

    "verbatimModuleSyntax": true,
    "isolatedModules": true,
    "forceConsistentCasingInFileNames": true,
    "resolveJsonModule": true,
    "skipLibCheck": true,
    "incremental": true
  },
  "include": ["src"]
}
```

- `target`/`lib` `es2024` matches Node.js 22+. For Node.js 20 (EOL since 2026-04-30, but still deployed), use `es2023`. For Node.js 24+, `es2024` plus `"esnext.disposable"` (`using` runs natively only with `target: esnext`).
- `"types": ["node"]` stops every stray `@types/*` package (jest, mocha …) from leaking globals into production code. Test configs extend this one and add their own types.
- Add `"rewriteRelativeImportExtensions": true` if sources use `./x.ts` specifiers.

##### Library (published to npm)

Add to the above:

```json
{
  "compilerOptions": {
    "declaration": true,
    "declarationMap": true,
    "isolatedDeclarations": true,
    "composite": false,
    "stripInternal": true
  }
}
```

Libraries usually target an older baseline (`es2022`) so consumers on older runtimes can use them. Keep `lib` to the same baseline so you do not call APIs your consumers lack.

##### Bundled Front-End (Vite/webpack/esbuild)

```json
{
  "compilerOptions": {
    "target": "es2022",
    "lib": ["es2023", "dom", "dom.iterable"],
    "module": "preserve",
    "moduleResolution": "bundler",
    "jsx": "react-jsx",
    "noEmit": true,
    "allowImportingTsExtensions": true,
    "strict": true,
    "noUncheckedIndexedAccess": true,
    "verbatimModuleSyntax": true,
    "isolatedModules": true,
    "skipLibCheck": true,
    "types": ["vite/client"]
  },
  "include": ["src"]
}
```

`tsc` only type-checks here (`noEmit`). The bundler transpiles. `isolatedModules`/`verbatimModuleSyntax` guarantee the two agree.

##### Running `.ts` Directly with Node.js (Type Stripping) on 5.7

```json
{
  "compilerOptions": {
    "target": "es2024",
    "module": "nodenext",
    "rewriteRelativeImportExtensions": true,
    "verbatimModuleSyntax": true,
    "isolatedModules": true,
    "strict": true,
    "noEmit": true
  }
}
```

Node.js's own documentation recommends `erasableSyntaxOnly`, which **does not exist in 5.7**. Enforce the erasable subset by review and lint instead: no `enum`, runtime `namespace`, parameter properties, `import =`/`export =`, decorators, or `paths`.

##### Strictness Flags Reference

| Flag | In `strict` | Since | What it catches |
|---|---|---|---|
| `noImplicitAny` | yes | 1.x | untyped parameters and variables becoming `any` |
| `strictNullChecks` | yes | 2.0 | `null`/`undefined` not assignable to other types; needed for most narrowing and the 5.7 never-initialized check |
| `strictFunctionTypes` | yes | 2.6 | contravariant parameter checks (property syntax only) |
| `strictBindCallApply` | yes | 3.2 | typed `bind`/`call`/`apply` |
| `strictPropertyInitialization` | yes | 2.7 | class fields not initialised in the constructor |
| `noImplicitThis` | yes | 2.0 | `this` of type `any` |
| `alwaysStrict` | yes | 2.1 | emits `"use strict"`, parses in strict mode |
| `useUnknownInCatchVariables` | yes | 4.4 | `catch (e)` is `unknown` |
| `strictBuiltinIteratorReturn` | yes | 5.6 | built-in iterator `TReturn` is `undefined` |
| `noUncheckedIndexedAccess` | no | 4.1 | index access includes `undefined` |
| `exactOptionalPropertyTypes` | no | 4.4 | `prop?: T` rejects explicit `undefined` |
| `noImplicitOverride` | no | 4.3 | `override` required |
| `noImplicitReturns` | no | 1.8 | missing return on some path |
| `noFallthroughCasesInSwitch` | no | 1.8 | non-empty case fallthrough |
| `noPropertyAccessFromIndexSignature` | no | 4.2 | `obj.key` on index signatures |
| `noUnusedLocals` / `noUnusedParameters` | no | 2.0 | dead code (often left to ESLint) |
| `noUncheckedSideEffectImports` | no | 5.6 | unresolvable `import "x"` |
| `allowUnreachableCode: false` | no | 1.8 | turns unreachable-code suggestions into errors |

Turning off a `strict` sub-flag (`"strict": true, "strictNullChecks": false`) in a diff deserves a challenge. Turning one *on* in a large code base is best done per directory with project references, or with a temporary baseline tool, rather than with mass `!` and `as` edits.

***

#### Runtime Validation and Security

Types are erased. Every security property must hold at runtime.

##### Validate at Trust Boundaries

```ts
interface CreateUser { email: string; age: number }
function parseCreateUser(body: unknown): CreateUser {
  if (typeof body !== "object" || body === null) throw new TypeError("body must be an object");
  const { email, age } = body as Record<string, unknown>;
  if (typeof email !== "string" || !email.includes("@")) throw new TypeError("email");
  if (typeof age !== "number" || !Number.isInteger(age) || age < 0) throw new TypeError("age");
  return { email, age };            // build a new object: unknown extra keys are dropped
}
export { parseCreateUser };
```

- `req.body as CreateUser`, `JSON.parse(x) as T` and `res.json() as Promise<T>` are unchecked casts. `Response.json()` returns `Promise<any>` in the DOM/undici types, so the cast is not even needed to make the bug compile. Flag them at API boundaries. Use a schema library (zod, valibot, typebox+ajv, arktype) whose inferred type is the single source of truth (`type User = z.infer<typeof User>`).
- Returning the validated input object itself (instead of a freshly built one) passes extra properties through: mass assignment (`isAdmin: true`) and prototype keys.
- `process.env.X` is `string | undefined`. Parse configuration once at startup, fail fast, and export a typed object.

##### Prototype Pollution and Dictionary Objects

```ts
function setPath(target: Record<string, unknown>, path: string[], value: unknown): void {
  let cur: Record<string, unknown> = target;
  for (const key of path.slice(0, -1)) {
    if (key === "__proto__" || key === "constructor" || key === "prototype") throw new Error("forbidden key");
    const next = cur[key];
    cur = (typeof next === "object" && next !== null ? next : (cur[key] = {})) as Record<string, unknown>;
  }
  const last = path.at(-1);
  if (last === undefined || last === "__proto__") throw new Error("bad path");
  cur[last] = value;
}
const safeDict: Record<string, number> = Object.create(null) as Record<string, number>;
const counts = new Map<string, number>();     // preferred for untrusted keys
export { setPath, safeDict, counts };
```

`Record<string, T>` and index signatures say nothing about inherited keys: `"toString" in obj` is `true`, and `obj["__proto__"] = x` changes the prototype. Use `Map`, `Object.create(null)`, or `Object.hasOwn(obj, k)` (needs `lib` `es2022`) instead of `in`/truthiness checks.

##### `Object.keys`, `for…in` and `in` Narrowing

`Object.keys(obj)` is `string[]`, not `(keyof T)[]`, because objects may have extra runtime keys (structural typing). Casting with `as (keyof T)[]` and then indexing is a frequent source of `undefined` reads. When a closed key set matters, iterate a known list (`const KEYS = ["a", "b"] as const`).

`"key" in obj` narrows unions, but the property can come from the prototype chain and can hold `undefined`.

##### Injection and Dangerous APIs

- SQL: always parameterise (`db.query("… WHERE id = $1", [id])`). Template literals that build SQL are injection even when every interpolated value is typed `string`. Tagged-template query builders (``sql`… ${id}` ``) are fine because the tag parameterises.
- Shell: `child_process.execFile`/`spawn` with an argument array. Never `exec` with interpolated input.
- `eval`, `new Function`, and `setTimeout("code")` with strings (typescript-eslint `no-implied-eval`).
- Regex from user input: escape it (5.7 has no `RegExp.escape`; that is 6.0 types and a Node.js 24 runtime feature), and beware catastrophic backtracking. 5.5+ checks regex **syntax** in literals, not complexity.
- Path traversal: `path.resolve(base, userPath)` and then verify `startsWith(base + path.sep)`.
- Secrets: compare with `crypto.timingSafeEqual` on equal-length `Buffer`s. Generate tokens with `crypto.randomBytes`/`randomUUID`, never `Math.random`.
- HTML: avoid `innerHTML` with interpolated data, and in React avoid `dangerouslySetInnerHTML`.

##### Type-Level Escape Hatches to Search For

`any`, `as any`, `as unknown as`, `!`, `// @ts-ignore`, `// @ts-nocheck`, `// @ts-expect-error` without a reason, `Function`, `Object`, `{}` as a parameter type, `declare module "x"` shims with `any` exports, `skipLibCheck` hiding broken local `.d.ts` files, and `"strict": false`.

- `@ts-expect-error` is preferred over `@ts-ignore`, because it fails when the error disappears. Require a description (typescript-eslint `ban-ts-comment` with `allow-with-description`).
- `skipLibCheck: true` skips **all** `.d.ts` files, including your own hand-written ones. Keep project declaration files few, or check them in a separate config.

***

#### Performance

##### Compiler Performance

- `skipLibCheck: true` is the single biggest win in most applications.
- Restrict `types` to what is needed (`["node"]`). 5.7 loads every package in `node_modules/@types` by default.
- Use `incremental: true` (writes `.tsbuildinfo`), and use project references with `composite: true` plus `tsc -b` for monorepos. Since 5.6, `tsc -b` continues building downstream projects when an upstream project has errors (it still exits non-zero), and `.tsbuildinfo` is always written.
- `--noCheck` (5.6) emits without type-checking. Use it for a fast emit step in pipelines that type-check separately. Never ship a build that was only produced with `--noCheck` and never checked.
- Measure with `tsc --extendedDiagnostics` and `tsc --generateTrace <dir>` (analyse with `@typescript/analyze-trace`). Expensive patterns: huge unions (thousands of members), deeply recursive conditional types, large intersections of object types, and inference-heavy library generics. Annotate return types of exported functions to cut inference cost (and adopt `isolatedDeclarations`).
- On Node.js ≥ 22.1, 5.7 starts faster thanks to the V8 compile cache. It is not a type-checking speed-up.
- The native compiler (7.0) is 8–12× faster but requires migrating through 6.0. Do not suggest it as a drop-in for 5.7.

##### Runtime Performance Notes

- `enum` and namespaces emit objects and IIFEs. `as const` objects are lighter and tree-shake.
- Downlevelled features cost size and speed: `async`/generators to `es2015`, class fields to `es2020` (constructor assignments), `using` to below `esnext`, and decorators (helper calls). Pick the newest `target` your runtime supports.
- `importHelpers: true` + `tslib` dedupes helpers across files. `tslib` must be new enough for the helpers in use: ≥ 2.5 for standard decorators (`__esDecorate`) and ≥ 2.6 for `using` (`__addDisposableResource`).
- Spreading large objects or arrays in hot loops (`{ ...acc, [k]: v }` inside `reduce`) is O(n²). Mutate a local accumulator or use a `Map`.

***

#### Testing

##### Type-Level Tests

```ts
type Equal<A, B> = (<T>() => T extends A ? 1 : 2) extends (<T>() => T extends B ? 1 : 2) ? true : false;
type Expect<T extends true> = T;

declare function first<T>(xs: readonly T[]): T | undefined;
type _t1 = Expect<Equal<ReturnType<typeof first<number>>, number | undefined>>;

// @ts-expect-error: string is not assignable to number[]
first<number>("nope");
```

Libraries such as `expect-type` (`expectTypeOf(x).toEqualTypeOf<…>()`), Vitest's `expectTypeOf`/`--typecheck`, or `tsd` provide the same with better messages. Avoid hand-rolled `Assert<T, U extends T>` helpers: they only check assignability, so `any` passes.

##### Test Runners and TypeScript

- Vitest, and Jest via `ts-jest`/`@swc/jest`/`babel-jest`, mostly **transpile without type-checking**. A test suite that passes does not mean the code type-checks. CI must run `tsc --noEmit` (or `tsc -b`) separately.
- `node --test` can run `.test.ts` files directly on Node.js 22.18+/23.6+ via type stripping, with the erasable-syntax limits described above.
- Test-only globals (`describe`, `expect`) belong in a test tsconfig (`"types": ["node", "vitest/globals"]`), not in the production config.
- Mocks typed as `any` (`jest.fn() as any`) hide signature drift. Use `vi.fn<typeof impl>()`, `jest.mocked(fn)`, or `satisfies Partial<Service>`.

***

#### Tooling and Linting

- **typescript-eslint** ≥ **8.16.0** for TypeScript 5.7 (8.15 supports up to 5.6). Use the flat config with `tseslint.configs.strictTypeChecked` (or `recommendedTypeChecked`) and `parserOptions: { projectService: true }`. High-value rules: `no-floating-promises`, `no-misused-promises`, `await-thenable`, `switch-exhaustiveness-check`, `no-unnecessary-condition`, `strict-boolean-expressions`, `no-unsafe-*` (argument, assignment, call, member-access, return), `consistent-type-imports`, `no-import-type-side-effects`, `only-throw-error`, `prefer-promise-reject-errors`, `use-unknown-in-catch-callback-variable`, `restrict-template-expressions`, `no-deprecated` (reports uses of `@deprecated` APIs), `ban-ts-comment`, `no-explicit-any`, `unbound-method`, `method-signature-style`.
- **Formatting**: Prettier, dprint or Biome. If Biome also replaces ESLint, check that its rule set covers the type-aware rules the project relies on (floating promises, unsafe `any`). Do not debate formatting in review once a formatter is configured.
- **Runners**: `tsx` (esbuild-based, no type-check), `ts-node` (type-checks unless `transpileOnly`; ESM support is fragile), Node.js type stripping (22.18+/23.6+ by default, erasable syntax only), Bun and Deno (native). All of them skip type-checking or do it partially. `tsc --noEmit` stays the source of truth.
- **Library checks**: `@arethetypeswrong/cli`, `publint`, and `api-extractor` (for `.d.ts` rollups and API reports).
- **Pinning**: commit the lockfile and pin `"typescript": "~5.7.3"`. A caret range silently upgrades to 5.9 and changes diagnostics in CI.
- `tsc --init` in 5.7 generates the old verbose template with `target: es2016`, `module: commonjs`, `strict: true`, `esModuleInterop: true` and `skipLibCheck: true`. Review generated configs. `module: commonjs` with an ESM package is a common misconfiguration.

***

#### Observability and Debugging

- Emit source maps (`sourceMap: true`, or `inlineSources: true` for error trackers) and run Node.js with `--enable-source-maps` (or `NODE_OPTIONS=--enable-source-maps`), so stack traces point at `.ts` lines. Upload source maps to Sentry and similar services. Do not serve them publicly if the source is private.
- Preserve error chains with `new Error(msg, { cause })` (`lib` `es2022`). Log `err.cause` too, and check that the logger's error serialiser includes it.
- Name errors (`override readonly name = "HttpError"`) so logs and `instanceof`-free checks (`err.name === "HttpError"`) work across realms and duplicated packages.
- `#private` fields are invisible to `console.log`/`util.inspect` (verified on Node.js 22 and 24: `class A { #s = 1; p = 2 }` prints `A { p: 2 }`) and to `JSON.stringify`. Add an explicit `toJSON()` or `[util.inspect.custom]()` when diagnostics need them. TypeScript `private` fields are printed like any other property.
- Structured logging: type the log context (`logger.info({ userId, orderId }, "created")`), and never log secrets or whole request bodies.
- `tsc --listFiles`, `--explainFiles` and `--traceResolution` explain "why is this file included" and "why can't this module be found". Use them before changing `paths`/`types`.

***

#### Removed / Changed in 5.7 — Migration Notes (5.6 → 5.7)

1. **New TS2454 errors in closures.** A variable is read in a nested function but never assigned. Fix the bug: assign it or restructure. Do not add `!`.
2. **TypedArray/`Buffer` errors** (`Uint8Array<ArrayBufferLike>`, `ArrayBufferLike` not assignable to `ArrayBuffer`). Update `@types/node`. Then type APIs that need an owned buffer as `Uint8Array<ArrayBuffer>`, and copy data where a shared or pooled buffer could reach them.
3. **JSON imports under `nodenext`.** Add `with { type: "json" }`, replace named imports with a default import plus destructuring, and change `ns.key` to `ns.default.key`. Replace `assert { type: "json" }` with `with` (Node.js 22+ rejects `assert`).
4. **`lib: ["es2022"]` + `Atomics.waitAsync`.** Move to `lib: ["es2024"]`.
5. **Adopting `target`/`lib` `es2024`.** Confirm every runtime is Node.js ≥ 22 (or equivalent browsers). Expect stricter `SharedArrayBuffer` vs `ArrayBuffer` checks. The regex `v` flag needs `target` (not just `lib`) `es2024`.
6. **Classes with `symbol`/`string`-typed computed method names** now carry index signatures. Fix new assignability errors by using `unique symbol` keys.
7. **Non-strict code bases** (`noImplicitAny` without `strictNullChecks`) get TS7011 on `() => null` callbacks. Annotate them, or better, enable `strictNullChecks`.
8. **`rewriteRelativeImportExtensions` adoption.** Switch relative imports to `.ts` specifiers. Remove `allowImportingTsExtensions` if it was only there for type stripping. Make sure no `paths` or `#imports` specifiers carry `.ts` (TS2877). A non-literal `import()` gets an inlined runtime helper. 5.7.3 inlines it even with `importHelpers: true` (verified with tslib 2.8.0 installed), so no tslib upgrade is needed for it.
9. **Editor ownership**: solution-style configs no longer need the `tsconfig.src.json` rename. No build impact.
10. **Tooling**: bump typescript-eslint to ≥ 8.16. Check that other tools embedding TypeScript (ts-jest, ts-loader, fork-ts-checker, vue-tsc, Angular CLI) support 5.7. Angular supports TypeScript 5.7 only in 19.1.x and 19.2.x. Angular 20+ requires TypeScript ≥ 5.8, so an Angular upgrade forces a TypeScript upgrade.

Nothing was removed from the language in 5.7, and no compiler option was deprecated.

***

#### Production / Review Checklist

##### Version and Configuration

- [ ] `typescript` pinned to `~5.7.x` (lockfile committed). No 5.8+ options (`erasableSyntaxOnly`, `libReplacement`, `module: node18/node20`) or syntax (`import defer`)
- [ ] `strict: true`, plus `noUncheckedIndexedAccess`, `noImplicitOverride`, `noFallthroughCasesInSwitch`, and ideally `exactOptionalPropertyTypes`
- [ ] `module`/`moduleResolution` explicit: `nodenext` for Node, `preserve`+`bundler` for bundlers. No accidental `classic` resolution. No `node10` in new code
- [ ] `target`/`lib` match the **lowest** deployed runtime (Node.js 20 → `es2023`, Node.js 22 → `es2024`). No `dom` in Node-only projects. `types` restricted
- [ ] `verbatimModuleSyntax` + `isolatedModules` when anything other than `tsc` transpiles
- [ ] `package.json` `"type"` consistent with emitted format. `exports` has `types` first. Libraries pass `attw`/`publint`
- [ ] JSON imports use `with { type: "json" }` (never `assert`) and default imports
- [ ] Projects running `.ts` directly: `rewriteRelativeImportExtensions`, `.ts` specifiers, and no enums, namespaces, parameter properties, decorators, `import =`/`export =` or `paths`
- [ ] typescript-eslint ≥ 8.16 with type-aware rules. `@types/node` matches the Node.js major and is new enough for 5.7's generic TypedArrays

##### Type Safety

- [ ] No new `any`, `as any`, `as unknown as`, or unexplained `!`/`as`. `@ts-expect-error` with a reason instead of `@ts-ignore`
- [ ] External data (`JSON.parse`, `fetch().json()`, request bodies, env vars, message queues) typed `unknown` and validated
- [ ] Discriminated unions handled exhaustively (`assertNever` / `switch-exhaustiveness-check`)
- [ ] Type predicates and assertion functions actually check what they claim
- [ ] No unconstrained generic used as a cast (`function f<T>(): T`)
- [ ] Callback members declared with property syntax. No `Function`/`Object`/`{}` parameter types
- [ ] Never-initialized variables (5.7 TS2454) fixed, not silenced. No `var`

##### Runtime Correctness

- [ ] Every promise awaited, returned, or explicitly `void`ed with a `.catch`. No async callbacks in `forEach`/event emitters without handling
- [ ] `return await` inside `try` blocks. `Promise.all` vs `allSettled` chosen deliberately. Concurrency bounded
- [ ] Class fields that shadow base members use `declare`. No overridable method calls from base constructors
- [ ] `using` resources not returned or escaped. `target: esnext` + `using` only on Node.js ≥ 24
- [ ] `Uint8Array`/`Buffer` `.buffer` access respects `byteOffset`/`byteLength`
- [ ] Enum values that are persisted or sent over the wire have explicit initialisers. No `const enum` in public `.d.ts`
- [ ] `Object.groupBy` results treated as `Partial` (groups may be missing)

##### Security

- [ ] SQL/shell/HTML built with parameterisation or escaping, never string interpolation
- [ ] Untrusted keys use `Map`/`Object.create(null)`. `__proto__`/`constructor` rejected in merge/set-path helpers
- [ ] No `eval`/`new Function`/string timers. Regexes from input escaped and bounded
- [ ] Secrets compared in constant time, generated with `crypto`. Not logged

##### Build, Test and Observability

- [ ] CI runs `tsc --noEmit`/`tsc -b` in addition to tests (test runners do not type-check)
- [ ] Type tests for public generic APIs (`expect-type`/`tsd`)
- [ ] Source maps emitted and enabled at runtime (`--enable-source-maps`). Errors carry `cause` and a `name`
- [ ] Build performance watched (`--extendedDiagnostics`). `skipLibCheck`/`incremental`/project references used appropriately
