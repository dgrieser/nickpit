### TypeScript 5.4: The Complete Production Developer Guide

> **Version**: TypeScript 5.4.2 (the first stable 5.4 release) was published on **March 6, 2024**; the latest and final patch is **5.4.5 (April 10, 2024)**.[^1][^2]
> **Support status (October 2026)**: unmaintained. The TypeScript team ships patches only for the newest minor line; 5.4 received none after 5.4.5. Newer lines are 5.5–5.9 (5.9.3, September 30, 2025), 6.0 (6.0.3, April 16, 2026, the last JavaScript-based compiler) and 7.0 (7.0.2, July 8, 2026, the native Go port, now `latest` on npm).[^2] DefinitelyTyped tests `@types/*` packages only against TypeScript versions younger than two years, so 5.4 dropped out of that window in March 2026 and current `@types` releases may no longer compile with it (see Tooling).[^3]
> **Scope**: the whole TypeScript language and type system as usable in 5.4, every 5.4 change against 5.3, `tsconfig` and module resolution, runtime/Node.js interplay, async, errors, security, performance, testing, tooling and declaration publishing — with examples that were all compiled with `tsc` 5.4.5 (and boundary examples also with 5.3.3 and 5.5.4).
> **Note**: this document is self-contained. Everything from earlier releases that still applies is carried forward. Sections covering 5.4-specific behaviour are marked **New in 5.4**.

Unless an example says otherwise, it compiles with `tsc --strict --target es2022 --module nodenext --noEmit` in a package whose `package.json` has `"type": "module"`, with `@types/node` from the `ts5.4` dist-tag. Lines marked `// error` are the lines `tsc` 5.4.5 rejects. A comment `// tsc: ...` names extra flags the example needs.

***

#### What's New in TypeScript 5.4

TypeScript 5.4 is a small, mostly type-system release. Its changes against 5.3:[^1][^4]

| Change | Kind | Section |
| --- | --- | --- |
| Narrowing of parameters and `let` variables is preserved inside closures created after the last assignment | Type checking | Preserved narrowing in closures |
| `NoInfer<T>` intrinsic utility type | Type system | NoInfer |
| `Object.groupBy` / `Map.groupBy` declarations (`lib` `esnext.object`, `esnext.collection`) | `lib.d.ts` | groupBy |
| `Promise.withResolvers` declarations (`lib` `esnext.promise`) — not in the release notes | `lib.d.ts` | groupBy |
| `--module preserve`; `import x = require()` allowed with `--moduleResolution bundler` under `preserve` | Modules | module preserve |
| Import attributes/assertions checked against the global `ImportAttributes` interface | Modules | Import attributes |
| Auto-imports understand `package.json` `"imports"` (subpath imports, `#...`) | Editor | Tooling |
| Quick fix "Add missing parameter(s)" | Editor | Tooling |
| Conditional types get a more accurate (union) constraint | Breaking | Behavioural changes |
| Intersections of type variables and primitives reduce more aggressively | Breaking | Behavioural changes |
| Template-literal placeholders check `keyof T & string` correctly | Breaking (relaxation) | Behavioural changes |
| `isolatedModules`: a type-only import that conflicts with a local value is an error (TS2865) | Breaking | Behavioural changes |
| Enums with the same name are only compatible when known member values are identical | Breaking | Behavioural changes |
| Mapped types over tuples with `any` rest elements keep element types (e.g. `Promise.all`) | Breaking | Behavioural changes |
| DOM `lib.d.ts` refresh; new `dom.asynciterable`, `webworker.asynciterable`, `es2016.intl` libs | `lib.d.ts` | Behavioural changes |
| Last release in which the options deprecated in 5.0 still *work* with `"ignoreDeprecations": "5.0"` | Deprecation | Deprecations |

The 5.4.0 release notes also announced that enum members may no longer be named `Infinity`, `-Infinity` or `NaN`. That change shipped in 5.4.2 only: it was **reverted in 5.4.3** (PR #57853), so 5.4.3–5.4.5 accept those names again, exactly like 5.3.[^5] Verified: 5.4.2 reports TS2452 on `enum E { Infinity = 0 }`; 5.4.5 compiles it.

##### Do not use or recommend (not available in 5.4)

The project's compiler is 5.4. These features exist only in later releases — never suggest them, and flag code that relies on them (it does not compile, or the option is rejected as unknown):[^6][^7][^8][^9][^10][^11]

- **5.5** — inferred type predicates (`arr.filter(x => x !== undefined)` still yields `(T | undefined)[]` in 5.4); control-flow narrowing of `obj[key]` with a non-literal `key`; regular-expression syntax checking (5.4 accepts `/(?<y>\d)\k<m>/`); `lib` declarations for the new `Set` methods (`union`, `intersection`, `difference`, `symmetricDifference`, `isSubsetOf`, `isSupersetOf`, `isDisjointFrom`), `Array.fromAsync`, `String.prototype.isWellFormed`/`toWellFormed`; `--isolatedDeclarations`; the `${configDir}` template variable in `tsconfig.json`; the JSDoc `@import` tag; `--target es2023`; the `transpileDeclaration` API.
- **5.6** — "this condition is always truthy/nullish" errors (TS2869/TS2872); iterator helper methods (`Iterator.prototype.map`, `lib esnext.iterator`); `--strictBuiltinIteratorReturn` and the `BuiltinIteratorReturn`/`IteratorObject` types; `--noUncheckedSideEffectImports`; `--noCheck`; arbitrary module namespace identifiers (`export { x as "a-b" }`); `--build` continuing past errors.
- **5.7** — "variable is never initialized" checks across closures; `--rewriteRelativeImportExtensions`; `--target es2024` / `--lib es2024`; generic `TypedArray<TArrayBuffer>` types; required `with { type: "json" }` on JSON imports under `nodenext`.
- **5.8** — `--module node18`; `require()` of ESM under `--module nodenext`; `--erasableSyntaxOnly`; `--libReplacement`; granular checks of each branch of a conditional `return` expression; the error for `assert` import assertions under `nodenext`.
- **5.9** — `import defer`; `--module node20`; the minimal `tsc --init`.
- **6.0 / 7.0** — `--target es2025`, `Temporal` types, `Map.prototype.getOrInsert`, `RegExp.escape`, `--stableTypeOrdering`, `#/` subpath-import shortcuts, `bundler` resolution with `--module commonjs`, the new defaults (`types: []`, `rootDir: "."`, `strict`), and the native compiler (`tsc` 7, `@typescript/native-preview`).

Runtime availability is a separate question from `lib` declarations: Node.js 22 *has* `Set.prototype.union` and `Array.fromAsync`, but 5.4's `lib` files do not declare them, so code calling them fails to compile unless the project adds its own declarations.

##### Deprecated and on the way out

- **Options deprecated in 5.0** — `charset`, `target: "ES3"`, `importsNotUsedAsValues`, `noImplicitUseStrict`, `noStrictGenericChecks`, `keyofStringsOnly`, `suppressExcessPropertyErrors`, `suppressImplicitAnyIndexErrors`, `out`, `preserveValueImports`, `prepend` in project references and an implicitly OS-specific `newLine`. In 5.4 each one is error TS5101/TS5107 unless `"ignoreDeprecations": "5.0"` is set, in which case it still **works** (verified: `keyofStringsOnly` still changes `keyof`). 5.4 is the last release where that is true: in 5.5 the options are accepted but have **no effect**, and from **5.6** on (not 6.0, as the 5.5 notes suggest) setting them is error TS5102 "has been removed".[^1][^7] Replace `importsNotUsedAsValues`/`preserveValueImports` with `verbatimModuleSyntax`, `out` with `outFile`, `suppressImplicitAnyIndexErrors` with real index signatures.
- **`moduleResolution: "node"`** (alias `node10`) models Node.js 10 and ignores `package.json` `"exports"`; use `node16`/`nodenext` for Node.js and `bundler` for bundlers. TypeScript 6.0 deprecates `node10`, `classic`, `baseUrl`, `target: es5`, `downlevelIteration`, `module` `amd`/`umd`/`system`, `outFile` and the `assert` import syntax.[^11]
- **Import assertions** (`import x from "./a.json" assert { type: "json" }`) still compile in 5.4, but Node.js 22 and later reject the `assert` keyword with a `SyntaxError`. Write `with`.
- **`experimentalDecorators`** still works and is still needed for `emitDecoratorMetadata`-based frameworks, but new code should use standard (TC39) decorators, available since 5.0.

##### Version-aware review

- Verify an API against the pinned compiler and `lib` before claiming it is unavailable: run `npx tsc -v` and check the `lib`/`target` in `tsconfig.json`. An API can be declared under `--lib esnext` but not under `es2022`.
- Say "not available in TypeScript 5.4" only for the features listed above (or ones you have checked). `NoInfer`, `Object.groupBy`, `Map.groupBy`, `Promise.withResolvers` and `--module preserve` **are** available in 5.4 (the `lib` ones need `esnext` libs).
- Distinguish the **compiler** version (`typescript` in `devDependencies`), the **`lib`/`target`** (which APIs are declared), and the **runtime** (which APIs exist). A 5.4 project targeting Node.js 18 must not call `Object.groupBy` (Node.js 21+) or `Promise.withResolvers` (Node.js 22+) even though `--lib esnext` declares them.
- `@types/*` packages track the runtime, not the compiler. With 5.4, pin them to versions that still support 5.4 (see Tooling) rather than suggesting "upgrade @types/node".
- Do not "fix" 5.4 code by adding 5.5+ idioms (for example, removing an explicit `x is T` predicate because "TypeScript infers it" — 5.4 does not).

##### Available in 5.4 from earlier releases (carried forward)

Everything below compiles on 5.4.5; do not report it as unavailable. Boundaries were checked against 5.0–5.3.

- **5.0** — standard TC39 decorators and `accessor` fields; `const` type parameters; `--moduleResolution bundler`, `--verbatimModuleSyntax`, `--allowImportingTsExtensions`, `customConditions`, `resolvePackageJsonExports`/`Imports`; `export type *`; `extends` with an array in `tsconfig.json`; all enums are union enums.
- **5.1** — functions with return type `undefined` need no `return` statement (5.0: TS2355); unrelated types for `get`/`set` accessors with explicit annotations.
- **5.2** — `using`/`await using` declarations; decorator metadata (`context.metadata`, needs a runtime `Symbol.metadata`); calling methods such as `filter`/`find` on a union of array types (`string[] | number[]`, 5.1: TS2349); `lib es2023` copying array methods (`toSorted`, `toReversed`, `toSpliced`, `with`; 5.1: TS2339); named and anonymous tuple elements may be mixed.
- **5.3** — import attributes (`with { type: "json" }`) and `resolution-mode` in all module modes; `switch (true)` narrowing; narrowing through comparisons with `true`/`false` (`typeof v === "string" === true`); `instanceof` narrowing through a user-defined `static [Symbol.hasInstance]` type predicate (5.2: no narrowing); errors for `super.x` accesses to instance fields.

***

#### New in 5.4: Preserved Narrowing in Closures After the Last Assignment

Before 5.4, a narrowing of a parameter or `let` variable was discarded inside any callback, because the variable *might* be reassigned before the callback runs. 5.4 keeps the narrowing when the closure is created **after the last assignment** to the variable and the variable is **not assigned inside any nested function**.[^1][^12]

```ts
function getUrls(url: string | URL, names: string[]) {
  if (typeof url === "string") {
    url = new URL(url);
  }
  // 5.3: error TS2339 "Property 'searchParams' does not exist on type 'string | URL'"
  // 5.4: OK — the arrow function is created after the last assignment to `url`
  return names.map((name) => {
    url.searchParams.set("name", name);
    return url.toString();
  });
}
```

The analysis is deliberately conservative. These cases still lose the narrowing in 5.4 (and 5.5):

```ts
function printValueLater(value: string | undefined) {
  if (value === undefined) {
    value = "missing!";
  }
  setTimeout(() => {
    value = value; // any assignment inside a nested function disables the feature
  }, 500);
  setTimeout(() => {
    console.log(value.toUpperCase()); // error TS18048: 'value' is possibly 'undefined'
  }, 1000);
}

function hoisted(value: string | undefined) {
  if (value === undefined) value = "x";
  return inner();
  function inner() {
    return value.length; // error TS18048 — function declarations are hoisted
  }
}

function assignedLater(value: string | undefined) {
  if (value === undefined) value = "x";
  const f = () => value.length; // error TS18048 — a later assignment exists
  value = undefined;
  return f;
}
```

Review notes:

- After upgrading from 5.3, non-null assertions (`url!`, `value as string`) and copies such as `const u = url;` that existed only to work around the old limitation can be removed. Do not remove them in code that also compiles with older compilers (for example a library that supports TypeScript ≥ 5.0 for its consumers' declaration builds — declaration files are unaffected, but shared source is not).
- The feature is about **types**, not runtime: a callback that runs later still sees the *current* value of the variable. The narrowing is sound only because no later assignment exists — if someone adds one, `tsc` reports the error again.
- Class properties, object properties (`this.x`, `obj.x`) and `var` declarations captured by closures are not covered; only parameters and `let`. `const` was already narrowed in closures before 5.4.

#### New in 5.4: The `NoInfer<T>` Utility Type

`NoInfer<T>` is an intrinsic type (declared as `type NoInfer<T> = intrinsic` in `lib.es5.d.ts`, so it exists for every `lib` setting). It evaluates to `T`, but blocks the occurrence from contributing inference candidates. Use it when one parameter is the "source of truth" for a type parameter and another must merely conform to it.[^1][^13]

```ts
function createStreetLight<C extends string>(colors: C[], defaultColor?: NoInfer<C>) {
  return { colors, defaultColor };
}
createStreetLight(["red", "yellow", "green"], "red");
createStreetLight(["red", "yellow", "green"], "blue"); // error TS2345: '"blue"' is not assignable to '"red" | "yellow" | "green" | undefined'

// Callback parameters: the event list decides E, the handler only receives it
declare function on<E extends string>(events: E[], handler: (e: NoInfer<E>) => void): void;
on(["click", "keydown"], (e) => {
  const kind: "click" | "keydown" = e;
  return kind;
});
```

Without `NoInfer`, `"blue"` becomes an additional candidate and `C` is inferred as `"red" | "yellow" | "green" | "blue"` — the call silently succeeds.

Pitfall — `NoInfer` makes the *other* argument the only inference site, so literal types flow from it:

```ts
function withDefault<T>(value: T, fallback: NoInfer<T>): T {
  return value ?? fallback;
}
withDefault(1, 2); // error TS2345: Argument of type '2' is not assignable to parameter of type '1'
withDefault<number>(1, 2); // OK — state T explicitly, or widen: withDefault(1 as number, 2)
```

Before 5.4, libraries emulated it with `[T][T extends any ? 0 : never]` or a second type parameter `D extends C`. On 5.4 prefer `NoInfer`, but a library whose *declaration files* must work for consumers on TypeScript < 5.4 cannot use it in its public `.d.ts` (5.3 reports TS2304 "Cannot find name 'NoInfer'"). A local `type NoInfer<T> = ...` alias inside a module shadows the global intrinsic — remove such aliases when upgrading.

#### New in 5.4: `Object.groupBy`, `Map.groupBy` and `Promise.withResolvers`

5.4 adds declarations for three ES2024 APIs. They live in `esnext` libraries, so they are visible only with `--target esnext`, `--lib esnext`, or the specific sub-libraries (`esnext.object`, `esnext.collection`, `esnext.promise`). There is no `es2024` target or lib in 5.4 (added in 5.7).[^1][^14]

```ts
// tsc: --lib es2022,esnext.object,esnext.collection,esnext.promise
interface Order { id: string; status: "open" | "paid"; total: number }
declare const orders: Order[];

const byStatus = Object.groupBy(orders, (o) => o.status);
//    ^? Partial<Record<"open" | "paid", Order[]>>
const open: Order[] = byStatus.open; // error TS2322 — every group is optional
const openSafe: Order[] = byStatus.open ?? [];

const bySize = Map.groupBy(orders, (o) => o.total > 100); // Map<boolean, Order[]>
const big = bySize.get(true) ?? [];

const { promise, resolve, reject } = Promise.withResolvers<number>();
setTimeout(() => resolve(42), 10);
export { open, openSafe, big, promise, reject };
```

Review notes:

- `Object.groupBy` returns a **null-prototype** object typed as `Partial<Record<K, T[]>>`; always handle missing groups. Keys go through `ToPropertyKey`, so numbers become strings — use `Map.groupBy` for non-string keys or object identity.
- **Runtime support:** `Object.groupBy`/`Map.groupBy` exist from Node.js 21 (not in Node.js 18.20 or 20.20); `Promise.withResolvers` from Node.js 22 (verified). Adding `esnext` to `lib` while running on Node.js 18/20 type-checks code that throws `TypeError: Object.groupBy is not a function`. Either target the runtime's actual `lib` or ship a polyfill (`core-js`) — do not accept "it compiles" as proof.
- `--lib esnext` is a moving target: it also pulls in `esnext.decorators` and `esnext.disposable` declarations whose runtime support differs per engine.

#### New in 5.4: `--module preserve` and `require()` with `--moduleResolution bundler`

`--module preserve` (new in 5.4) models bundlers and runtimes such as Bun that accept both ESM `import` and CommonJS `require()` in the same file. It implies `moduleResolution: "bundler"`, `esModuleInterop: true` and `resolveJsonModule: true` (shown by `tsc --module preserve --showConfig`). ESM imports are emitted unchanged and `import x = require("...")` is emitted as a `require()` call; the syntax chosen also selects the `"import"` or `"require"` condition of the dependency's `package.json` `"exports"`.[^1][^15]

```ts
// file: src/main.ts — tsc --module preserve
import * as foo from "some-package/foo"; // resolved through the "import" condition
import bar = require("some-package/bar"); // resolved through the "require" condition
```

Emitted by 5.4.5 with `--target es2022`:

```js
import * as foo from "some-package/foo";
const bar = require("some-package/bar");
```

Rules verified on 5.4.5:

- `import x = require()` is still error TS1202 with `--module esnext --moduleResolution bundler`; it is allowed only with `--module preserve` (or CommonJS module kinds).
- `--moduleResolution bundler` requires `--module preserve` or `es2015` and later (error TS5095 with `commonjs`; combining `bundler` with `commonjs` is a 6.0 feature).
- `preserve` is the right choice when a bundler (Vite, esbuild, webpack, Bun) does the emit and `tsc` only type-checks (`noEmit: true`). Pair it with `verbatimModuleSyntax` so type-only imports are explicit. It is wrong for code that Node.js runs directly — use `nodenext` there.

#### New in 5.4: Checked Import Attributes

Import attributes (`with { type: "json" }`, 5.3) and import assertions (`assert { ... }`) are now checked against a global `ImportAttributes` interface. Runtimes or projects can declare which attributes they accept:[^1][^16]

```ts
// file: src/attributes.d.ts
interface ImportAttributes {
  type: "json";
}
// file: src/main.ts
import * as ns from "./config.json" with { type: "not-json" };
// error TS2322: Type '{ type: "not-json"; }' is not assignable to type 'ImportAttributes'.
```

Without such a declaration, any attribute object is accepted. The attribute does **not** affect type checking of the imported JSON.

JSON imports under `--module nodenext` (verified with 5.4.5 and Node.js 18.20/20.20/22.23/24.22):

- `import cfg from "./config.json" with { type: "json" };` — compiles and runs on all four Node.js lines. Requires `resolveJsonModule`.
- `import cfg from "./config.json";` — **compiles in 5.4**, but Node.js throws `ERR_IMPORT_ATTRIBUTE_MISSING` (20: `ERR_IMPORT_ASSERTION_TYPE_MISSING`) at load time. 5.7 made this a compile error (TS1543); in 5.4 the reviewer must catch it.
- `import cfg from "./config.json" assert { type: "json" };` — compiles in 5.4, works on Node.js 18/20, but is a `SyntaxError: Unexpected identifier 'assert'` on Node.js 22 and 24.

***

#### New in 5.4: Behavioural Changes That Can Break a Build

These are correctness fixes; upgrading from 5.3 can surface new errors (or remove old ones).[^1]

##### More accurate conditional-type constraints

A conditional type over an unresolved type parameter now has the union of both branches as its constraint, instead of eagerly picking a branch from the type parameter's constraint.

```ts
type IsArray<T> = T extends any[] ? true : false;

function check<U extends object>(x: IsArray<U>) {
  let first: true = x; // error TS2322 (5.3 and 5.4)
  let second: false = x; // error TS2322 — new in 5.4; 5.3 accepted it because `object` is not an array
  return [first, second];
}
```

##### More aggressive reduction of intersections with primitives

```ts
declare function intersect<T, U>(x: T, y: U): T & U;

function reduce<T extends "abc" | "def">(x: T, str: string, num: number) {
  const a = intersect(x, str); // 5.3: T & string — 5.4: T
  const b = intersect(x, num); // 5.3: T & number — 5.4: never
  const ta: T = a;
  const tb: never = b;
  return [ta, tb];
}
```

Code that relied on the `never` not appearing (for example a branch now typed `never` and flagged as unreachable by lint rules) needs a look.

##### Template literal placeholders

```ts
function idKey<T extends { id: string }>() {
  let x: `-${keyof T & string}`;
  x = "-id"; // 5.3: error TS2322 — 5.4: OK
  return x;
}
```

##### Type-only imports that conflict with a local value (`isolatedModules`)

```ts
// file: use.ts — tsc --isolatedModules (or --verbatimModuleSyntax)
import { Something } from "./types.js"; // Something is an interface
let Something = 123;
// 5.4: error TS2865: Import 'Something' conflicts with local value, so must be
// declared with a type-only import when 'isolatedModules' is enabled.
```

Fix with `import type { Something }` or rename the local. Single-file transpilers (esbuild, swc, Babel, Node.js type stripping) cannot know whether the import is a type, which is why the rule exists.

##### Stricter enum compatibility

Two enums from different declarations with the same name used to be mutually assignable even when known member values differed. 5.4 requires identical values; a string-valued member is only compatible with the same string.

```ts
namespace First {
  export enum SomeEnum { A = 0, B = 1 }
}
namespace Second {
  export enum SomeEnum { A = 0, B = 2 }
}
function sync(x: First.SomeEnum, y: Second.SomeEnum) {
  x = y; // error TS2322: Each declaration of 'SomeEnum.B' differs in its value, where '1' was expected but '2' was given.
  y = x; // error TS2322
}
```

This typically surfaces when two copies of a package (different versions) are installed and their enums meet — fix the duplicate dependency instead of casting.

##### Mapped types over tuples with `any` rest elements

```ts
Promise.all(["", ...([] as any)]).then((result) => {
  const head: number = result[0]; // error TS2322 in 5.4 (head is string); 5.3 typed it any and accepted this
  return head;
});
```

##### `lib.d.ts`

The DOM declarations were regenerated, and 5.4 adds the `dom.asynciterable` / `webworker.asynciterable` libraries (async iteration of `FileSystemDirectoryHandle`), `es2016.intl` (`Intl.getCanonicalLocales`) and the `esnext.object`, `esnext.collection`, `esnext.promise` libraries described above. Code that declared these APIs itself may now see duplicate-declaration errors.[^17]

##### Emit changes

Not breaking per se, but visible in diffs of compiled output: type-parameter names are preserved more often when shadowed in declaration emit, complex parameter lists of downlevelled `async` functions move into the generator body, binding aliases in function declarations are kept, and import attributes in `import()` types go through the normal emit pipeline.[^1]

***

#### `tsconfig.json` for TypeScript 5.4

##### Recommended baseline: Node.js application or library run directly by Node.js

```jsonc
{
  "compilerOptions": {
    // Language and runtime
    "target": "es2022",              // highest *named* target in 5.4 (es2023/es2024 targets do not exist yet)
    "lib": ["es2023"],                // es2023 lib exists in 5.4: toSorted, findLast, ...
    "module": "nodenext",             // Node.js ESM/CJS semantics, honours package.json "type" and "exports"
    "moduleResolution": "nodenext",
    "types": ["node"],                // only load the @types you need

    // Type checking
    "strict": true,
    "noUncheckedIndexedAccess": true,
    "exactOptionalPropertyTypes": true,
    "noImplicitOverride": true,
    "noImplicitReturns": true,
    "noFallthroughCasesInSwitch": true,
    "noPropertyAccessFromIndexSignature": true,
    "useUnknownInCatchVariables": true, // already implied by strict

    // Module hygiene
    "verbatimModuleSyntax": true,     // import type / export type must be explicit; replaces importsNotUsedAsValues
    "isolatedModules": true,          // safe for esbuild/swc/tsx/Node.js type stripping
    "resolveJsonModule": true,
    "forceConsistentCasingInFileNames": true,

    // Output
    "outDir": "dist",
    "rootDir": "src",
    "declaration": true,
    "declarationMap": true,
    "sourceMap": true,
    "skipLibCheck": true
  },
  "include": ["src"]
}
```

Notes:

- `target` controls syntax downlevelling and the *default* `lib`. With `target: es2022`, class fields use `[[Define]]` semantics (`useDefineForClassFields` defaults to `true` for ES2022+), and `#private` fields, static blocks and `?.`/`??` are emitted unchanged. Top-level `await` additionally needs an ESM output format (`nodenext` ESM files, `es2022`/`esnext`/`preserve`).
- Valid `target` values in 5.4: `es3` (deprecated), `es5`, `es6`/`es2015` … `es2022`, `esnext`. `"target": "es2023"` is error TS6046 in 5.4 (added in 5.5).
- Valid `module` values in 5.4: `none`, `commonjs`, `amd`, `system`, `umd`, `es6`/`es2015`, `es2020`, `es2022`, `esnext`, `node16`, `nodenext`, `preserve`. There is no `node18`/`node20`.
- `strict` is **not** on by default in 5.4 (it becomes the default in 6.0). A missing `"strict": true` is a review finding in new projects.
- `skipLibCheck: true` is standard practice (faster builds, tolerates conflicting `@types`), but it also hides errors in the project's own `.d.ts` files, and it does not make `@types` packages written for newer compilers usable (see Tooling).
- `verbatimModuleSyntax` with `module: nodenext` in a file that Node.js treats as CommonJS (`.cts`, or `.ts` without `"type": "module"`) forbids ESM `import` syntax there (TS1286); use `import x = require()` in such files or make the package ESM.

##### Bundler-built application (Vite, esbuild, webpack, Bun)

```jsonc
{
  "compilerOptions": {
    "target": "es2022",
    "lib": ["es2023", "dom", "dom.iterable"],
    "module": "preserve",             // 5.4+: implies moduleResolution bundler, esModuleInterop, resolveJsonModule
    "noEmit": true,                   // the bundler emits; tsc only type-checks
    "allowImportingTsExtensions": true, // only legal with noEmit or emitDeclarationOnly
    "verbatimModuleSyntax": true,
    "isolatedModules": true,
    "strict": true,
    "noUncheckedIndexedAccess": true,
    "skipLibCheck": true,
    "jsx": "react-jsx"
  },
  "include": ["src"]
}
```

On TypeScript 5.0–5.3 the equivalent is `"module": "esnext", "moduleResolution": "bundler"`; that still works in 5.4 but rejects `import x = require()`.

##### Options that matter in review

| Option | What to check |
| --- | --- |
| `strict` | Must be `true` for new code. Turning off individual members (`strictNullChecks: false`) needs a reason. |
| `noUncheckedIndexedAccess` | Not part of `strict`. Without it, `arr[i]` and `record[key]` are typed as present. |
| `exactOptionalPropertyTypes` | Not part of `strict`. Distinguishes `{ a?: string }` from `{ a?: string \| undefined }`. |
| `useDefineForClassFields` | Defaults to `true` for `target >= es2022`/`esnext`. Changing `target` from `es2020` to `es2022` changes class-field runtime semantics. |
| `esModuleInterop` | Needed for `import x from "cjs-package"` to behave like Node.js/bundlers. Implied by `module: preserve`; `nodenext` handles it automatically. |
| `skipLibCheck` | Fine for dependencies; do not rely on it to hide errors in hand-written `.d.ts`. |
| `paths` / `baseUrl` | Only affect type resolution; `tsc` does **not** rewrite the emitted specifiers. The runtime or bundler needs the same mapping (prefer `package.json` `"imports"`). |
| `allowJs` / `checkJs` | `checkJs` type-checks JS files with JSDoc; unexpected errors in `.js` files appear when it is on. |
| `incremental` / `composite` | Speed up rebuilds; `composite` is required for project references and forces `declaration`. |
| `ignoreDeprecations` | `"5.0"` keeps 5.0-deprecated options working in 5.4 only. Treat it as a migration TODO. |

***

#### Modules and Module Resolution

##### Choosing `module` / `moduleResolution`

| Code is run by | `module` | `moduleResolution` | Notes |
| --- | --- | --- | --- |
| Node.js directly (after `tsc` emit, or `tsx`/`ts-node`) | `nodenext` (or `node16`) | same (implied) | ESM vs CJS per file, `"exports"`/`"imports"` honoured, `.js` extensions required in ESM |
| A bundler or Bun | `preserve` (5.4+) | `bundler` (implied) | Extensionless imports allowed; `noEmit` |
| A bundler, compiler ≤ 5.3 compatibility | `esnext` | `bundler` | No `import = require()` |
| Legacy CommonJS-only build | `commonjs` | `node10` (alias `node`) | Ignores `"exports"`; avoid for new code |

In 5.4, `node16` and `nodenext` resolve and emit modules identically; they differ only in the implied `target` when none is set (`node16` → `es2022`, `nodenext` → `esnext`; every other `module` value defaults to `es5`). Pick `nodenext` and set `target` explicitly.

##### File format under `nodenext`

- `.mts` → always ESM, emits `.mjs`; `.cts` → always CommonJS, emits `.cjs`.
- `.ts`/`.tsx` → ESM if the nearest `package.json` has `"type": "module"`, otherwise CommonJS.
- In ESM files, relative imports must carry the **output** extension: `import { x } from "./util.js"` even though the source is `util.ts`. Without it: error TS2835 "Relative import paths need explicit file extensions...". `tsc` 5.4 never rewrites import specifiers (`--rewriteRelativeImportExtensions` is 5.7), so `"./util.ts"` is legal only with `allowImportingTsExtensions` plus `noEmit` or `emitDeclarationOnly` — the setup for Node.js type stripping, `tsx` or a bundler.
- A CommonJS file cannot statically import an ES module: error TS1479 "...the referenced file is an ECMAScript module and cannot be imported with 'require'. Consider writing a dynamic 'import(...)' call instead." Node.js 22.12+ can `require()` ESM, but TypeScript models that only from 5.8 (`--module nodenext`/`node18`). In 5.4, use `await import()` or convert the importer to ESM.

```ts
// file: src/legacy.cts (without verbatimModuleSyntax — with it, a .cts file must use import = require() and export =)
import { esmOnly } from "./esm.js"; // error TS1479 in 5.4
// GOOD in 5.4: load ESM lazily
export async function load(): Promise<number> {
  const mod = await import("./esm.js");
  return mod.esmOnly;
}
```

- `__dirname`, `__filename`, `require` and `module` are declared **globally** by `@types/node`, so `tsc` does not flag them in ESM files — but they throw `ReferenceError` at runtime in ESM. Use `import.meta.dirname`/`import.meta.filename`[^21] (Node.js 20.11+/21.2+; `undefined` on Node.js 18 although `@types/node` types them as `string`), or `fileURLToPath(new URL(".", import.meta.url))`, and `createRequire(import.meta.url)`.

```ts
// BAD — compiles in an ESM file, ReferenceError at runtime
const configDir = __dirname;
// GOOD
import { fileURLToPath } from "node:url";
import { dirname, join } from "node:path";
const here = dirname(fileURLToPath(import.meta.url));
export const configPath = join(here, "config.json");
export { configDir };
```

##### Importing CommonJS from ESM

Under `nodenext`, `import pkg from "cjs-pkg"` binds `module.exports` (like Node.js). Named imports from a CommonJS module work at runtime only when Node.js's static analysis (cjs-module-lexer) detects the export; TypeScript cannot see that. When a CJS package's types use `export =`, prefer the default import, and verify named imports by running the code.

##### Type-only imports and exports

```ts
// file: src/service.ts — tsc --verbatimModuleSyntax
import type { User } from "./types.js"; // erased entirely
import { type Role, ROLES } from "./types.js"; // only ROLES survives in the output
export type { User }; // re-exporting a type requires export type under isolatedModules
export type * from "./types.js"; // 5.0+
export type * as Types from "./types.js"; // 5.0+
```

With `verbatimModuleSyntax` (5.0+), anything without the `type` modifier is kept in the output — a plain `import { User }` of an interface is error TS1484. That is the predictable, single-file-transpiler-safe model; enable it in new projects (it supersedes `importsNotUsedAsValues`, `preserveValueImports` and most uses of `isolatedModules`).

Side-effect-only imports (`import "./polyfills.js";`) are **not** checked for existence in 5.4 if the path does not resolve; a typo goes unnoticed until runtime (`--noUncheckedSideEffectImports` is 5.6).

##### Subpath imports (`package.json` `"imports"`) instead of `paths`

```jsonc
// file: package.json
{
  "name": "app",
  "type": "module",
  "imports": {
    "#internal/*": "./src/internal/*"
  }
}
```

```ts
// file: src/main.ts — tsc --module nodenext
import { helper } from "#internal/helper.js"; // resolves to src/internal/helper.ts at type-check time
```

`"imports"` is understood by Node.js, bundlers and `tsc` (`nodenext`, `bundler`), so no tool needs a separate alias. Specifiers must start with `#` (`#/` alone is supported only from TypeScript 6.0 and recent Node.js). New in 5.4: auto-import completions in editors suggest these `#...` specifiers.[^1] For a compiled package, point the mapping at the build output (`"./dist/internal/*"`) or use conditions (`{"types": "./src/...", "default": "./dist/..."}`).

`compilerOptions.paths` is purely a type-checking alias; emitted JavaScript keeps `@/foo` and fails at runtime unless the bundler or a runtime loader also resolves it. Flag `paths` in packages run by plain Node.js.

##### Package `"exports"` and types for libraries

```jsonc
// file: package.json of a dual-format library
{
  "name": "my-lib",
  "type": "module",
  "exports": {
    ".": {
      "import": { "types": "./dist/index.d.ts", "default": "./dist/index.js" },
      "require": { "types": "./dist/index.d.cts", "default": "./dist/index.cjs" }
    },
    "./package.json": "./package.json"
  },
  "types": "./dist/index.d.ts"
}
```

- `"types"` must come **first** inside each condition object; conditions are matched in order.
- An ESM `.d.ts` must not describe a CommonJS file (and vice versa): each format needs its own declaration file (`.d.ts` beside `.js` in a `"type": "module"` package, `.d.cts` beside `.cjs`). Mismatches ("masquerading as ESM/CJS") produce runtime-only failures for consumers; check with `@arethetypeswrong/cli` (`attw --pack`).
- Top-level `"types"`/`"typings"` is used only by `node10` resolution; keep it for old consumers but `"exports"` wins for everyone else.
- A consumer on `moduleResolution: node10` cannot see subpath exports (`my-lib/sub`) at all; `typesVersions` is the workaround.

***

#### Types: Fundamentals

##### `any`, `unknown`, `never`

- `any` disables checking and **spreads**: properties, calls and assignments of an `any` are all `any`. Treat every `any` in new code as a finding unless justified (interop with untyped JS, deliberately escaped generics).
- `unknown` is the safe top type: nothing can be done with it until it is narrowed.
- `never` is the empty type: the return type of functions that always throw or loop, and the type left after exhaustive narrowing.

```ts
// BAD — any leaks into everything that touches the result
function parseBad(json: string): any {
  return JSON.parse(json);
}
const port: string = parseBad('{"port":8080}').port; // compiles, wrong at runtime

// GOOD — unknown forces validation at the boundary
interface Config { port: number; host: string }
function isConfig(v: unknown): v is Config {
  return (
    typeof v === "object" && v !== null &&
    "port" in v && typeof v.port === "number" &&
    "host" in v && typeof v.host === "string"
  );
}
function parseConfig(json: string): Config {
  const value: unknown = JSON.parse(json);
  if (!isConfig(value)) throw new TypeError("invalid config");
  return value;
}
export { port, parseConfig };
```

`JSON.parse`, `Response.json()` (DOM lib: `Promise<any>`), `structuredClone` of `any`, and many `@types` APIs return `any`. Assigning such a value directly to a typed variable is an unchecked cast — validate it (a type guard, or a schema library such as Zod/Valibot) at the trust boundary.

##### Type assertions and the non-null operator

`x as T` and `x!` are compile-time claims with no runtime check.

```ts
interface User { id: string; name: string }
declare const input: unknown;
declare const users: Map<string, User>;

const u1 = input as User; // BAD: no validation — a lie if input is not a User
const u2 = users.get("42")!; // BAD unless existence is proven just above
const u3 = users.get("42");
if (u3 === undefined) throw new Error("user 42 missing"); // GOOD: explicit handling
const n = 42 as unknown as string; // double assertion: almost always a bug
export { u1, u2, n };
```

`as const` is different — it is an assertion to the *narrowest* literal type and is safe:

```ts
const ROLES = ["admin", "editor", "viewer"] as const;
type Role = (typeof ROLES)[number]; // "admin" | "editor" | "viewer"
function isRole(s: string): s is Role {
  return (ROLES as readonly string[]).includes(s);
}
export { isRole };
```

##### `satisfies` (4.9+)

`satisfies` checks that an expression conforms to a type **without** widening the expression's own inferred type.

```ts
type Route = { path: string; auth: boolean };
const routes = {
  home: { path: "/", auth: false },
  admin: { path: "/admin", auth: true },
} satisfies Record<string, Route>;

routes.admin.path; // keys are known: routes.nope is an error
const r2: Record<string, Route> = routes;
r2.nope.path; // compiles: annotation widened the keys to string (would need noUncheckedIndexedAccess)

const bad = { home: { path: "/", auth: "no" } } satisfies Record<string, Route>; // error TS2322
export { bad };
```

Prefer `satisfies` over `as` for config objects and lookup tables; prefer an annotation when the variable should deliberately have the wider type.

##### Literal types, widening, and `const` type parameters (5.0+)

```ts
let mutable = "a"; // string (widened: let can change)
const fixed = "a"; // "a"

function tuple<const T extends readonly unknown[]>(...xs: T): T {
  return xs;
}
const t = tuple("a", 1); // readonly ["a", 1] — no `as const` needed at the call site
export { mutable, fixed, t };
```

##### Unions, intersections, and narrowing

Narrowing works through `typeof`, `instanceof`, `in`, equality checks (`===`, `==` with `null`), truthiness, discriminant properties, `Array.isArray`, user-defined type guards, assertion functions, and (5.3+) `switch (true)` and comparisons to boolean literals.

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
    case "rect":
      return s.w * s.h;
    default: {
      const unreachable: never = s; // compile-time exhaustiveness check
      throw new Error(`unknown shape ${JSON.stringify(unreachable)}`);
    }
  }
}

function classify(x: unknown): string {
  switch (true) { // 5.3+: each case narrows
    case typeof x === "string":
      return x.toUpperCase();
    case Array.isArray(x):
      return `array of ${x.length}`;
    default:
      return "other";
  }
}
export { area, classify };
```

Narrowing pitfalls a reviewer must catch:

```ts
function len(s: string | null | undefined): number {
  if (!s) return 0; // BAD if "" should be distinguished from null: truthiness lumps them together
  return s.length;
}

function first(xs: number[] | undefined) {
  if (xs) return xs[0]; // without noUncheckedIndexedAccess this is typed number, but may be undefined
  return undefined;
}

// filter does not narrow in 5.4 — inferred type predicates are 5.5
const values = [1, undefined, 3];
const defined = values.filter((v) => v !== undefined); // (number | undefined)[]
const numbers: number[] = values.filter((v) => v !== undefined); // error TS2322
const ok = values.filter((v): v is number => v !== undefined); // number[]
const alsoOk = values.flatMap((v) => (v === undefined ? [] : [v])); // number[]
export { len, first, defined, numbers, ok, alsoOk };
```

`filter(Boolean)` also does not narrow (and drops `0`, `""`, `NaN`).

Narrowings are invalidated by function calls only for mutable *properties* in theory, but TypeScript keeps them optimistically: after `if (obj.x !== undefined) { mutate(obj); obj.x.foo }` the narrowing survives although `mutate` may have reset `obj.x`. Treat narrowed property accesses after calls that can mutate the object with suspicion.

##### Type guards and assertion functions

```ts
interface Cat { meow(): void }
interface Dog { bark(): void }

// A user-defined guard is trusted blindly — a wrong body is an unsound cast
function isCat(a: Cat | Dog): a is Cat {
  return "meow" in a;
}

function assertIsDefined<T>(v: T, msg: string): asserts v is NonNullable<T> {
  if (v === undefined || v === null) throw new Error(msg);
}

function demo(a: Cat | Dog, maybe: string | undefined) {
  if (isCat(a)) a.meow();
  else a.bark();
  assertIsDefined(maybe, "maybe is required");
  return maybe.length;
}

// Assertion functions must be called through a reference with an explicit type annotation
const assertArrow = (v: unknown): asserts v is string => {
  if (typeof v !== "string") throw new TypeError("not a string");
};
type AssertString = (v: unknown) => asserts v is string;
const assertString: AssertString = (v) => {
  if (typeof v !== "string") throw new TypeError("not a string");
};
function useArrow(v: unknown, w: unknown) {
  assertArrow(v); // error TS2775: Assertions require every name in the call target to be declared with an explicit type annotation
  assertString(w); // OK: the variable itself is annotated
  return w.length;
}
export { demo, useArrow };
```

In 5.4 type predicates are never inferred; a helper like `const isString = (x: unknown) => typeof x === "string"` returns plain `boolean` and does not narrow at call sites (5.5 infers `x is string`). Do not tell 5.4 authors to remove explicit predicates.

***

#### Objects, Interfaces and Type Aliases

##### `interface` vs `type`

- Use `interface` for object shapes that are extended or implemented, and for public library shapes that consumers may augment (declaration merging). `extends` on interfaces reports conflicting members as errors, whereas `&` silently produces `never` members.
- Use `type` for unions, tuples, mapped and conditional types, function types, and aliases of primitives.
- Be consistent within a codebase; both are fine for plain object shapes.

```ts
interface Base { id: string }
interface Conflicting extends Base { id: number } // error TS2430: Interface 'Conflicting' incorrectly extends interface 'Base'

type Merged = Base & { id: number };
declare const m: Merged;
const idNever: never = m.id; // the intersection silently became never
export { idNever };
```

##### Excess property checks apply only to fresh object literals

```ts
interface Options { host: string; timeoutMs?: number }
declare function connect(o: Options): void;

connect({ host: "db", timeoutMS: 1000 }); // error TS2561: Did you mean to write 'timeoutMs'?
const opts = { host: "db", timeoutMS: 1000 };
connect(opts); // compiles: not fresh, timeoutMS is just an extra property — the typo is lost

interface Weak { timeoutMs?: number; retries?: number } // all-optional ("weak") type
declare function configure(o: Weak): void;
const w = { timeoutMS: 1000 };
configure(w); // error TS2559: no properties in common — weak types require one overlapping property
```

The weak-type check is the only safety net for non-fresh objects, and a single overlapping property disables it. Prefer passing literals directly or annotating the variable (`const opts: Options = {...}`).

##### Optional properties, `undefined` and `exactOptionalPropertyTypes`

```ts
// tsc: --exactOptionalPropertyTypes
interface Patch { name?: string }
const a: Patch = {}; // OK
const b: Patch = { name: undefined }; // error TS2375 with exactOptionalPropertyTypes
interface PatchExplicit { name?: string | undefined }
const c: PatchExplicit = { name: undefined }; // OK: undefined explicitly allowed
export { a, b, c };
```

The difference matters at runtime: `"name" in obj`, `Object.keys`, spread (`{...defaults, ...patch}` overwrites with `undefined`) and JSON serialization all treat a present-but-undefined key differently from an absent one.

```ts
const defaults = { host: "localhost", port: 80 };
const patch: { host?: string } = { host: undefined }; // allowed without exactOptionalPropertyTypes
const merged = { ...defaults, ...patch };
const host: string = merged.host; // typed string, but undefined at runtime
export { host };
```

##### Index signatures, `Record` and `noUncheckedIndexedAccess`

```ts
// tsc: --noUncheckedIndexedAccess
const counts: Record<string, number> = {};
const n: number = counts["missing"]; // error TS2322 with noUncheckedIndexedAccess (number | undefined)
const safe = counts["missing"] ?? 0;

const list = [1, 2, 3];
for (let i = 0; i < list.length; i++) {
  const v = list[i]; // number | undefined even here
  if (v !== undefined) console.log(v);
}
for (const v of list) console.log(v); // number — prefer for...of / array methods
export { n, safe };
```

Without the flag, every index access is assumed to hit. Enable it in new code; for existing code, at least review index accesses on dictionaries built from external input.

`Record<K, V>` with a finite union key requires all keys; with `string` it is a dictionary. Plain objects used as dictionaries inherit from `Object.prototype` — `"constructor" in counts` is `true` and `counts["__proto__"] = ...` mutates the prototype. Use `Map` (or `Object.create(null)`) for keys that come from users.

`Object.keys(obj)` returns `string[]`, not `(keyof T)[]`, because objects may have extra keys at runtime. Do not "fix" that with `as (keyof T)[]` without knowing the object is exact; iterate a known key list instead.

##### `readonly`

```ts
interface Point { readonly x: number; readonly y: number }
function sum(xs: readonly number[]): number {
  return xs.reduce((a, b) => a + b, 0);
}
const p: Point = { x: 1, y: 2 };
p.x = 3; // error TS2540
const mutableView: { x: number } = p; // compiles: readonly is not enforced through assignability
mutableView.x = 3; // mutates p
export { sum };
```

`readonly` is shallow and compile-time only; use `Object.freeze` for runtime immutability. Prefer `readonly T[]`/`ReadonlyArray<T>` for parameters that are not mutated — callers can then pass readonly tuples and `as const` arrays.

***

#### Functions

##### Parameter and return types

Annotate the parameters and return types of exported functions: the return annotation catches accidental changes of the public contract and speeds up type checking (especially in declaration emit). Local helpers and callbacks can rely on inference.

##### Overloads

```ts
function parse(input: string): number;
function parse(input: string[]): number[];
function parse(input: string | string[]): number | number[] {
  return Array.isArray(input) ? input.map(Number) : Number(input);
}
const one: number = parse("1");
const many: number[] = parse(["1", "2"]);
declare const either: string | string[];
parse(either); // error TS2769: No overload matches this call
export { one, many };
```

The implementation signature is not visible to callers. Prefer a union parameter or a generic over overloads when the return type does not depend on the input type; add an overload taking the union when callers hold unions.

##### Method vs function property bivariance

`strictFunctionTypes` (part of `strict`) checks function-typed **properties** contravariantly, but **method** declarations stay bivariant:

```ts
interface Animal { name: string }
interface Dog extends Animal { bark(): void }

interface HandlerMethod { handle(a: Animal): void }
interface HandlerProp { handle: (a: Animal) => void }

const dogOnly = (d: Dog) => d.bark();
const m: HandlerMethod = { handle: dogOnly }; // compiles (method bivariance) — unsound
const f: HandlerProp = { handle: dogOnly }; // error TS2322
export { m, f };
```

Declare callbacks as properties (`handle: (a: Animal) => void`) in new interfaces.

##### Array covariance

```ts
interface Animal { name: string }
interface Cat extends Animal { meow(): void }
const cats: Cat[] = [];
const animals: Animal[] = cats; // allowed — arrays are covariant
animals.push({ name: "rex" }); // now cats contains a non-Cat
cats.forEach((c) => c.meow()); // runtime TypeError
```

Accept `readonly Animal[]` when a function only reads.

##### `this` parameters and callbacks

```ts
class Counter {
  count = 0;
  increment() {
    this.count++;
  }
  incrementBound = () => {
    this.count++;
  };
}
const c = new Counter();
const detached = c.increment;
detached(); // compiles, TypeError at runtime: this is undefined
setTimeout(c.incrementBound, 10); // OK: arrow function property captures this

function onClick(this: HTMLButtonElement, ev: MouseEvent) {
  console.log(this.id, ev.button);
}
export { onClick };
```

typescript-eslint's `unbound-method` catches the detached-method bug; TypeScript does not, unless the method declares a `this` parameter.

##### Default and optional parameters

```ts
function greet(name: string, greeting = "Hello"): string {
  return `${greeting}, ${name}`;
}
greet("Ann", undefined); // undefined triggers the default
greet("Ann", null); // error TS2345 under strictNullChecks — null does not trigger defaults at runtime either
```

***

#### Generics

```ts
function firstOrUndefined<T>(items: readonly T[]): T | undefined {
  return items[0];
}

function pluck<T, K extends keyof T>(items: readonly T[], key: K): T[K][] {
  return items.map((i) => i[key]);
}

interface Repository<T extends { id: string }> {
  find(id: string): Promise<T | undefined>;
  save(item: T): Promise<void>;
}
export { firstOrUndefined, pluck, type Repository };
```

Guidelines:

- A type parameter should appear at least twice (relating inputs to outputs). `function f<T>(x: T): void` is just `(x: unknown) => void`. `function get<T>(): T` (return-only generic) is a disguised cast — flag it, e.g. `const user = cache.get<User>("u1")`.
- Constrain with `extends`; default with `= ...`. Avoid `<T extends any>`.
- Use `NoInfer<T>` (5.4) rather than a phantom second type parameter to stop unwanted inference.
- `const` type parameters (5.0) infer literal/readonly types without `as const` at call sites.
- Variance annotations `in`/`out` (4.7) are for library authors to speed up checks and document variance; a wrong annotation is unsound and is only partially verified.

Generic narrowing limitation (unchanged in 5.4): narrowing a value of type `T` does not narrow `T` itself, so a function returning `T[K]`-dependent types often needs a cast or overloads.

```ts
interface Payloads { text: string; count: number }
function defaultFor<K extends keyof Payloads>(kind: K): Payloads[K] {
  if (kind === "text") return ""; // error TS2322: '""' is not assignable to type 'Payloads[K]'
  return 0; // error TS2322
}
const defaults: { [K in keyof Payloads]: Payloads[K] } = { text: "", count: 0 };
function defaultFor2<K extends keyof Payloads>(kind: K): Payloads[K] {
  return defaults[kind]; // GOOD: index into a typed table instead of branching
}
export { defaultFor, defaultFor2 };
```

***

#### Advanced Types

##### Conditional types and `infer`

```ts
type ElementOf<T> = T extends readonly (infer E)[] ? E : never;
type A = ElementOf<string[]>; // string

// infer with an extends constraint (4.7+)
type FirstNumber<T> = T extends [infer N extends number, ...unknown[]] ? N : never;
type B = FirstNumber<[42, "x"]>; // 42

// Distribution over unions happens only for a naked type parameter
type ToArray<T> = T extends unknown ? T[] : never;
type C = ToArray<string | number>; // string[] | number[]
type ToArrayNonDist<T> = [T] extends [unknown] ? T[] : never;
type D = ToArrayNonDist<string | number>; // (string | number)[]

// never distributes to never
type E = ToArray<never>; // never
export type { A, B, C, D, E };
```

Since 5.4 the constraint of an unresolved conditional type is the union of its branches (see Behavioural changes) — code that assigned such a value to one branch type now errors.

##### Mapped types and key remapping

```ts
type Getters<T> = {
  [K in keyof T & string as `get${Capitalize<K>}`]: () => T[K];
};
interface Person { name: string; age: number }
type PersonGetters = Getters<Person>; // { getName: () => string; getAge: () => number }

type Mutable<T> = { -readonly [K in keyof T]: T[K] };
type RequiredKeys<T> = { [K in keyof T]-?: undefined extends T[K] ? never : K }[keyof T];
type PickByValue<T, V> = { [K in keyof T as T[K] extends V ? K : never]: T[K] };
type Strings = PickByValue<{ a: string; b: number; c: string }, string>; // { a: string; c: string }
export type { PersonGetters, Mutable, RequiredKeys, Strings };
```

Homomorphic mapped types (`[K in keyof T]`) preserve optionality and `readonly`, and map arrays/tuples to arrays/tuples.

##### Template literal types

```ts
type HttpMethod = "GET" | "POST";
type Route = `/${string}`;
type Endpoint = `${HttpMethod} ${Route}`;
const e1: Endpoint = "GET /users";
const e2: Endpoint = "PATCH /users"; // error TS2322

type EventName<T extends string> = `on${Capitalize<T>}`;
type Click = EventName<"click">; // "onClick"
export { e1, e2, type Click };
```

Large cross-products of template literal unions explode combinatorially (TS2590 "Expression produces a union type that is too complex to represent" past 100,000 members); keep them small.

##### Tuples

```ts
type Pair = [key: string, value: number]; // labelled elements (4.0+)
type Rest = [first: string, ...rest: number[]];
type Concat<A extends unknown[], B extends unknown[]> = [...A, ...B]; // variadic (4.0+)
type AB = Concat<[1, 2], ["x"]>; // [1, 2, "x"]
type Opt = [a: string, b?: number];

function call<Args extends unknown[], R>(fn: (...args: Args) => R, ...args: Args): R {
  return fn(...args);
}
call((a: string, b: number) => a.repeat(b), "x", 3);
call((a: string, b: number) => a.repeat(b), "x"); // error TS2345
export type { Pair, Rest, AB, Opt };
```

##### Built-in utility types available in 5.4

`Partial`, `Required`, `Readonly`, `Pick`, `Omit`, `Record`, `Exclude`, `Extract`, `NonNullable`, `Parameters`, `ConstructorParameters`, `ReturnType`, `InstanceType`, `ThisParameterType`, `OmitThisParameter`, `ThisType`, `Awaited` (4.5), `Uppercase`/`Lowercase`/`Capitalize`/`Uncapitalize` (4.1), and **`NoInfer` (new in 5.4)**.

```ts
interface User { id: string; name: string; password: string }
type PublicUser = Omit<User, "password">;
type Loose = Omit<User, "passwrd">; // compiles: Omit's key is not constrained to keyof T — typos go unnoticed
type Strict = Pick<User, "passwrd">; // error TS2344: Pick is constrained
type R = Awaited<Promise<Promise<number>>>; // number
export type { PublicUser, Loose, Strict, R };
```

`Omit` over a union collapses it (it is not distributive): `Omit<A | B, "k">` keeps only common keys. Write a distributive version when needed:

```ts
type DistributiveOmit<T, K extends PropertyKey> = T extends unknown ? Omit<T, K> : never;
type Msg = { kind: "a"; x: number; ts: number } | { kind: "b"; y: string; ts: number };
type Plain = Omit<Msg, "ts">; // { kind: "a" | "b" } — x and y lost
type Kept = DistributiveOmit<Msg, "ts">; // { kind: "a"; x: number } | { kind: "b"; y: string }
export type { Plain, Kept };
```

##### Branded (nominal) types

```ts
type UserId = string & { readonly __brand: "UserId" };
type OrderId = string & { readonly __brand: "OrderId" };
const asUserId = (s: string): UserId => s as UserId; // the only place the cast happens, after validation
declare function loadUser(id: UserId): void;
declare const orderId: OrderId;
loadUser(asUserId("u-1"));
loadUser(orderId); // error TS2345
loadUser("u-1"); // error TS2345
```

##### `unique symbol` and `keyof` details

`keyof T` is `string | number | symbol` for objects with a `string` index signature (numeric keys are allowed). Use `keyof T & string` in template literals. `keyof any` is `string | number | symbol` (the deprecated `keyofStringsOnly` changed that; it stops working after 5.4).

***

#### Classes

##### Fields, `useDefineForClassFields` and initialization order

With `target: es2022`/`esnext` (or `useDefineForClassFields: true`), class fields are defined with `Object.defineProperty` semantics **after** `super()` returns. A field redeclared in a subclass therefore overwrites whatever the base constructor assigned, and a field initializer cannot see parameter properties declared later.

```ts
class Base {
  name: string;
  constructor() {
    this.name = "base";
    this.init();
  }
  init(): void {}
}
class Derived extends Base {
  name!: string; // error TS2612: Property 'name' will overwrite the base property in 'Base' (re-defined as undefined after super())
  label = "derived";
  override init(): void {
    console.log(this.label); // undefined: Derived's fields are not initialized yet when Base calls init()
  }
}
class DerivedOk extends Base {
  declare name: string; // GOOD: type-only redeclaration, emits nothing
}
console.log(new Derived().name, new DerivedOk().name); // prints undefined (from init), then: undefined base
```

With define semantics `tsc` reports the redeclaration (TS2612), but nothing reports the uninitialized `label` read from the base constructor — that compiles and prints `undefined` (verified on Node.js 22). Review rules: do not call overridable methods from constructors; use `declare` for type-only field redeclarations (never silence TS2612 by adding an initializer); check every `!` definite-assignment assertion (it silences `strictPropertyInitialization` with no runtime effect). Changing `target` from `es2020` to `es2022` changes this behaviour for existing code (and breaks frameworks that rely on accessors defined on the prototype, such as some MobX/Lit versions with legacy decorators).

##### Visibility: `private` vs `#private`

```ts
class Account {
  private balanceTs = 0; // compile-time only: (acc as any).balanceTs works, JSON.stringify includes it
  #balance = 0; // runtime-private (ES2022), not visible to JSON.stringify or Object.keys
  deposit(amount: number): void {
    if (amount <= 0) throw new RangeError("amount must be positive");
    this.#balance += amount;
    this.balanceTs += amount;
  }
  get balance(): number {
    return this.#balance;
  }
  static isAccount(x: unknown): x is Account {
    return typeof x === "object" && x !== null && #balance in x; // ergonomic brand check (4.5+)
  }
}
export { Account };
```

Use `#private` for real encapsulation (secrets, invariants); note that `#` fields do not work through `Proxy` and make objects fail structured cloning of the private state. `protected`/`private` are fine for ordinary encapsulation inside a codebase.

##### `override`, `abstract`, parameter properties, `accessor`

```ts
// tsc: --noImplicitOverride
abstract class Repo<T> {
  constructor(protected readonly table: string) {} // parameter property
  abstract find(id: string): Promise<T | undefined>;
  describe(): string {
    return `repo(${this.table})`;
  }
}
class UserRepo extends Repo<{ id: string }> {
  override async find(id: string) {
    return { id };
  }
  describe(): string { // error TS4114: must have an 'override' modifier
    return "users";
  }
}
class Box {
  accessor value = 0; // auto-accessor (5.0+): a private slot with get/set, decoratable
  static #instances = 0;
  static {
    Box.#instances = 0; // static initialization block (4.4+)
  }
}
export { UserRepo, Box };
```

Parameter properties, `enum`, `namespace` with values, and `import x = require()` are TypeScript-only *runtime* syntax: Node.js type stripping (strip-only mode) rejects them with `ERR_UNSUPPORTED_TYPESCRIPT_SYNTAX`, and 5.4 has no `--erasableSyntaxOnly` (5.8) to flag them at compile time.

***

#### Decorators

TypeScript 5.4 supports two incompatible decorator systems:

| | Standard (TC39 stage 3) | Legacy (`experimentalDecorators`) |
| --- | --- | --- |
| Since | 5.0 (metadata 5.2) | 1.5 |
| Enable | default (no flag) | `"experimentalDecorators": true` |
| Signature | `(value, context)` with `ClassMethodDecoratorContext` etc. | `(target, key, descriptor)` |
| Parameter decorators | no | yes |
| `emitDecoratorMetadata` (reflect-metadata, DI frameworks) | no | yes |
| Used by | new code, Lit 3 (standard mode) | Angular, NestJS, TypeORM, InversifyJS, class-validator |

Never mix them in one project: the flag switches the whole compilation. Check what the framework expects before "modernizing".

```ts
function logged<This, Args extends unknown[], Ret>(
  target: (this: This, ...args: Args) => Ret,
  context: ClassMethodDecoratorContext<This, (this: This, ...args: Args) => Ret>,
) {
  const name = String(context.name);
  return function (this: This, ...args: Args): Ret {
    console.log(`enter ${name}`);
    try {
      return target.call(this, ...args);
    } finally {
      console.log(`exit ${name}`);
    }
  };
}

class Greeter {
  @logged
  greet(who: string): string {
    return `hi ${who}`;
  }
}
console.log(new Greeter().greet("Ann"));
```

Decorator metadata (`context.metadata`, `Symbol.metadata`, 5.2+) needs a runtime `Symbol.metadata`. No Node.js release up to 24 provides it (verified: `Symbol.metadata` is `undefined` on Node.js 18–24), so `context.metadata` is `undefined` at runtime and writing to it throws `TypeError: Cannot set properties of undefined`. With `--lib esnext` (`esnext.decorators`) the type even claims it is always present. Polyfill it before any decorated class is evaluated:

```ts
// tsc: --lib es2022,esnext.decorators
(Symbol as { metadata?: symbol }).metadata ??= Symbol("Symbol.metadata");

function tag(_value: undefined, context: ClassFieldDecoratorContext) {
  context.metadata[context.name] = true; // safe only with the polyfill above
}
class Model {
  @tag id = 1;
}
export { Model };
```

The polyfill must run before any decorated class is evaluated. Within one file, top-to-bottom order suffices (as above); across files, remember that imported modules are evaluated *before* the importing module's body, so put the polyfill in its own module and import it first from the entry point (verified: `import "./polyfill.js"` followed by a decorated class yields `[Object: null prototype] { id: true }` on Node.js 22).

***

#### Enums and Alternatives

```ts
enum Direction { Up, Down } // numeric: reverse mapping Direction[0] === "Up"
const d: Direction = 42; // error TS2322 (5.0+: a literal outside the enum's values is rejected)
declare const n: number;
const d2: Direction = n; // compiles: any non-literal number is still assignable to a numeric enum
enum Status { Active = "active", Disabled = "disabled" } // string: no reverse mapping, nominal
const s: Status = "active"; // error TS2322: string literals are not assignable to string enums
export { d, d2, s };
```

Prefer a `const` object plus a union type for new code: it is plain JavaScript (works with type stripping, isolatedModules and every transpiler), structurally typed and iterable.

```ts
const Status = {
  Active: "active",
  Disabled: "disabled",
} as const;
type Status = (typeof Status)[keyof typeof Status]; // "active" | "disabled"

function label(s: Status): string {
  switch (s) {
    case Status.Active:
      return "Active";
    case Status.Disabled:
      return "Disabled";
  }
}
label("active");
export { label };
```

`const enum`s are inlined by `tsc` but cannot be inlined by single-file transpilers; with `isolatedModules` referencing an *ambient* const enum from a dependency's `.d.ts` is an error, and publishing `const enum` in a library's declarations breaks consumers who use `isolatedModules`. Avoid `const enum` in libraries.

5.4 tightened enum compatibility (see Behavioural changes). The announced ban on `Infinity`/`NaN` member names was reverted in 5.4.3 — do not report such names as compile errors on 5.4.5, but they are still a bad idea: the reverse mapping of numeric members can overwrite such members (TypeScript issue #48956, the bug the reverted change addressed).

***

#### Explicit Resource Management: `using` and `await using` (5.2+)

```ts
import { open } from "node:fs/promises";

class Lock implements Disposable {
  #held = true;
  [Symbol.dispose](): void {
    this.#held = false;
    console.log("lock released");
  }
}

function critical(): void {
  using lock = new Lock();
  console.log("working with", lock);
  throw new Error("boom"); // lock is still released
}

async function readFirstLine(path: string): Promise<string> {
  await using file = await open(path); // FileHandle implements Symbol.asyncDispose in @types/node
  for await (const line of file.readLines()) return line;
  return "";
}
export { critical, readFirstLine };
```

- Requires `Symbol.dispose`/`Symbol.asyncDispose` at runtime. Current Node.js 18, 20 and 22 releases define them (verified on 18.20.8, 20.20.2, 22.23.4; `fs/promises` `FileHandle` also implements `Symbol.asyncDispose` there); browsers and older runtimes need a polyfill. `SuppressedError` (thrown when both the block and a disposer throw) is native only from Node.js 24 — `tsc`'s downlevel helper supplies its own when `target` is below `esnext`.
- Types: `Disposable`/`AsyncDisposable` come from `lib esnext.disposable` (or `esnext`) or from `@types/node`. Without either, error TS2304 "Cannot find name 'Disposable'".
- `using` is block-scoped and disposes in reverse order; `DisposableStack`/`AsyncDisposableStack` (declared in `lib esnext.disposable`) combine resources, but they are native only from Node.js 24 (`undefined` on 22.23) — polyfill them elsewhere. Flag `using` on a value whose dispose method is not idempotent if it is also closed manually.

***

#### Error Handling

##### `catch` variables are `unknown`

Under `strict` (`useUnknownInCatchVariables`, 4.4+) `catch (e)` binds `unknown`. Narrow before use; never assume `e` is an `Error` — anything can be thrown, and errors from other realms (iframes, `vm` contexts) fail `instanceof Error`.

```ts
function message(e: unknown): string {
  if (e instanceof Error) return e.message;
  if (typeof e === "string") return e;
  return JSON.stringify(e) ?? String(e);
}

async function load(url: string): Promise<string> {
  try {
    const res = await fetch(url);
    if (!res.ok) throw new Error(`HTTP ${res.status} for ${url}`);
    return await res.text(); // `return await` inside try: rejections are caught here
  } catch (e) {
    throw new Error(`failed to load ${url}: ${message(e)}`, { cause: e }); // cause: lib es2022
  }
}
export { load };
```

`catch (e: Error)` is error TS1196; only `any` or `unknown` annotations are allowed.

##### Custom errors

```ts
class AppError extends Error {
  constructor(
    message: string,
    readonly code: string,
    options?: ErrorOptions,
  ) {
    super(message, options);
    this.name = new.target.name; // keeps subclass names
  }
}
class NotFoundError extends AppError {
  constructor(what: string, options?: ErrorOptions) {
    super(`${what} not found`, "NOT_FOUND", options);
  }
}
const err = new NotFoundError("user 1");
console.log(err instanceof NotFoundError, err.name, err.code); // true NotFoundError NOT_FOUND
```

`Object.setPrototypeOf(this, new.target.prototype)` is only needed when targeting ES5 (subclassing built-ins is broken there); with `target >= es2015` it is noise.

##### Typed results instead of exceptions (optional)

```ts
type Result<T, E = Error> = { ok: true; value: T } | { ok: false; error: E };

function parsePort(raw: string): Result<number, string> {
  const n = Number(raw);
  if (!Number.isInteger(n) || n < 1 || n > 65_535) return { ok: false, error: `invalid port ${raw}` };
  return { ok: true, value: n };
}
const r = parsePort("8080");
if (r.ok) console.log(r.value + 1);
else console.error(r.error);
```

Use them for expected, recoverable failures in domain code; keep exceptions for programmer errors and I/O failures. Do not mix styles within one module.

***

#### Async and Concurrency

##### Floating and misused promises

TypeScript does not report a promise that is never awaited. These compile cleanly in 5.4 and are among the most common production bugs:

```ts
declare function save(id: string): Promise<void>;
declare function isAllowed(id: string): Promise<boolean>;
declare const ids: string[];

async function bad(): Promise<void> {
  save("a"); // BAD: floating — rejection becomes an unhandledRejection (crashes Node.js 15+)
  ids.forEach(async (id) => {
    await save(id); // BAD: forEach ignores the returned promises; bad() resolves before saves finish
  });
  const ok = ids.every(async (id) => isAllowed(id)); // BAD: a Promise is always truthy — ok is always true
  console.log(ok);
}

async function good(): Promise<void> {
  await save("a");
  await Promise.all(ids.map((id) => save(id))); // parallel
  for (const id of ids) await save(id); // sequential, when order or rate matters
  const results = await Promise.all(ids.map((id) => isAllowed(id)));
  console.log(results.every(Boolean));
  void save("fire-and-forget").catch((e: unknown) => console.error(e)); // explicit and handled
}
export { bad, good };
```

TypeScript does catch a promise used directly as a condition:

```ts
declare function isAllowed(id: string): Promise<boolean>;
async function check(id: string) {
  const allowed = isAllowed(id);
  if (allowed) { // error TS2801: This condition will always return true since this 'Promise<boolean>' is always defined. Did you forget to use 'await'?
    return true;
  }
  return false;
}
export { check };
```

Enable typescript-eslint's `no-floating-promises`, `no-misused-promises`, `await-thenable` and `require-await`/`return-await` (type-aware rules) — the compiler alone does not cover these cases.

##### `return await` in `try`

```ts
declare function fetchUser(id: string): Promise<{ id: string }>;
async function loadBad(id: string) {
  try {
    return fetchUser(id); // BAD: a rejection bypasses this catch
  } catch {
    return undefined;
  }
}
async function loadGood(id: string) {
  try {
    return await fetchUser(id); // GOOD: the rejection is caught here
  } catch {
    return undefined;
  }
}
export { loadBad, loadGood };
```

##### `Promise.all`, `allSettled`, `any`, `race`

- `Promise.all` rejects on the first failure but does **not** cancel the others; they keep running. Pass an `AbortSignal` to cancel.
- `Promise.allSettled` (lib `es2020`) for "do all, report failures". Narrow on `status`.
- `Promise.any` (lib `es2021`) throws `AggregateError` when all fail.
- Unbounded `Promise.all(items.map(...))` over thousands of items opens thousands of sockets/file handles: limit concurrency (a worker pool or `p-limit`).

```ts
// tsc: --lib es2023
declare function fetchJson(url: string, signal: AbortSignal): Promise<unknown>;
async function fetchAll(urls: string[]): Promise<unknown[]> {
  const controller = new AbortController();
  const signal = AbortSignal.any([controller.signal, AbortSignal.timeout(5_000)]); // Node.js 20.3+
  try {
    return await Promise.all(urls.map((u) => fetchJson(u, signal)));
  } catch (e) {
    controller.abort(e); // cancel the rest on first failure
    throw e;
  }
}

async function summary(urls: string[], signal: AbortSignal) {
  const settled = await Promise.allSettled(urls.map((u) => fetchJson(u, signal)));
  return settled.map((r) => (r.status === "fulfilled" ? "ok" : String(r.reason)));
}
export { fetchAll, summary };
```

`AbortSignal.any` is declared by `@types/node`, but **not** by the 5.4 DOM library (it was added to `lib.dom.d.ts` in 5.5). If `lib` is unset (so the DOM lib is included by default) or contains `"dom"`, 5.4 reports TS2339 "Property 'any' does not exist" (verified) — another reason Node.js projects should set `"lib"` explicitly without `"dom"`.

##### Timers and sleeping

```ts
import { setTimeout as sleep } from "node:timers/promises";

async function retry<T>(fn: () => Promise<T>, attempts = 3, signal?: AbortSignal): Promise<T> {
  let lastError: unknown;
  for (let i = 0; i < attempts; i++) {
    try {
      return await fn();
    } catch (e) {
      lastError = e;
      await sleep(2 ** i * 100, undefined, { signal }); // exponential backoff, abortable
    }
  }
  throw new Error(`failed after ${attempts} attempts`, { cause: lastError });
}
export { retry };
```

Note: `setTimeout` returns `NodeJS.Timeout` under `@types/node` but `number` under the DOM lib; code shared between browser and Node.js should use `ReturnType<typeof setTimeout>`.

##### Async iteration and streams

```ts
import { createReadStream } from "node:fs";
import { createInterface } from "node:readline";

async function countLines(path: string): Promise<number> {
  const rl = createInterface({ input: createReadStream(path), crlfDelay: Infinity });
  let n = 0;
  for await (const _line of rl) n++;
  return n;
}

async function* paginate<T>(fetchPage: (cursor?: string) => Promise<{ items: T[]; next?: string }>) {
  let cursor: string | undefined;
  do {
    const page = await fetchPage(cursor);
    yield* page.items;
    cursor = page.next;
  } while (cursor !== undefined);
}
export { countLines, paginate };
```

Calling `.next()` by hand on a built-in iterator leaks `any` in 5.4: the iterators returned by `Array.prototype.values()`, `Map.prototype.entries()` and friends have an `any` return type, so `const s: string = [1].values().next().value;` compiles (the value is `number | undefined` at runtime). 5.6 types the done case as `undefined` under `--strictBuiltinIteratorReturn` (part of `strict` there). Prefer `for...of` or check `done` before using `value`.

##### Concurrency in Node.js

JavaScript runs on one thread per isolate; "concurrency bugs" are interleavings at every `await`. Check-then-act across an `await` is a race:

```ts
const cache = new Map<string, Promise<string>>();
declare function expensive(key: string): Promise<string>;

// BAD: two concurrent calls both miss and both compute
const plain = new Map<string, string>();
async function getBad(key: string): Promise<string> {
  const hit = plain.get(key);
  if (hit !== undefined) return hit;
  const value = await expensive(key);
  plain.set(key, value);
  return value;
}

// GOOD: cache the promise synchronously, evict on failure
function getGood(key: string): Promise<string> {
  let p = cache.get(key);
  if (p === undefined) {
    p = expensive(key);
    p.catch(() => cache.delete(key));
    cache.set(key, p);
  }
  return p;
}
export { getBad, getGood };
```

CPU-bound work blocks the event loop; move it to `worker_threads`. `SharedArrayBuffer`/`Atomics` (lib `es2017.sharedmemory`) are the only shared-memory primitives.

***

#### Runtime Validation and Data Boundaries

Types disappear at runtime. Every value that crosses a trust boundary — HTTP bodies, `JSON.parse`, environment variables, `process.argv`, database rows, message-queue payloads, `postMessage`, `localStorage` — must be validated before it is given a static type.

```ts
import { z } from "zod"; // or valibot, arktype, typebox, io-ts

const CreateUser = z.object({
  email: z.string().email(),
  name: z.string().min(1).max(100),
  age: z.number().int().nonnegative().optional(),
});
type CreateUser = z.infer<typeof CreateUser>;

function handle(body: unknown): CreateUser {
  return CreateUser.parse(body); // throws ZodError on invalid input
}
export { handle };
```

(Zod is a dependency, not part of TypeScript; the example was compiled against Zod 3. Check that the chosen library version still supports TypeScript 5.4 — current Zod documentation states it is tested against TypeScript 5.5 and later only.[^18])

Environment variables are `string | undefined` under `@types/node`'s `process.env` (index signature): validate and convert once at startup.

```ts
function requireEnv(name: string): string {
  const v = process.env[name];
  if (v === undefined || v === "") throw new Error(`missing env var ${name}`);
  return v;
}
const port = Number.parseInt(process.env["PORT"] ?? "8080", 10);
if (!Number.isInteger(port) || port <= 0) throw new Error("PORT must be a positive integer");
export const config = { dbUrl: requireEnv("DATABASE_URL"), port };
```

Do not write ambient declarations that claim more than is validated (e.g. `declare global { namespace NodeJS { interface ProcessEnv { DATABASE_URL: string } } }` without a startup check).

***

#### Security

TypeScript adds no runtime protection; the usual JavaScript vulnerabilities apply. Type-level issues that hide them:

- **Unchecked casts of input** (`req.body as User`, `JSON.parse(x) as Config`) — validate instead (previous section).
- **Prototype pollution** — merging untrusted objects into plain objects (`Object.assign(target, input)`, recursive deep-merge, `obj[key] = value` with user-controlled `key`). `Record<string, T>` typing does not prevent `__proto__`/`constructor` keys. Use `Map`, `Object.create(null)`, or reject those keys.
- **Injection** — template literals into SQL, shell commands, HTML. Use parameterized queries, `execFile`/`spawn` with argument arrays (never `exec` with interpolated strings), and context-aware HTML escaping. Template literal *types* (`` `SELECT ${string}` ``) are not sanitization.
- **`eval`, `new Function`, `vm`** on any input — `vm` is not a security boundary.
- **Path traversal** — `path.join(root, userPath)` does not stop `../`. Resolve and check the prefix:

```ts
import { resolve, sep } from "node:path";
function safeJoin(root: string, userPath: string): string {
  const base = resolve(root);
  const target = resolve(base, userPath);
  if (target !== base && !target.startsWith(base + sep)) throw new Error("path escapes root");
  return target;
}
export { safeJoin };
```

- **ReDoS** — 5.4 does not check regular expressions at all (syntax checking is 5.5 and still does not detect catastrophic backtracking). Review nested quantifiers on user input.
- **Secrets comparison** — use `crypto.timingSafeEqual` on equal-length `Buffer`s, not `===`.
- **Randomness** — `crypto.randomUUID()`/`crypto.getRandomValues()`, never `Math.random()` for tokens.
- **Dependency hygiene** — lock files committed, `npm ci` in CI, `npm audit`/`osv-scanner`. Typosquatted `@types/*` packages exist; install types only from the `@types` scope or the package itself.
- **`skipLibCheck`/`any` in `.d.ts`** — a hand-written declaration that types an untrusted value as trusted is a security bug in disguise.

***

#### Performance

##### Compile-time performance

- `skipLibCheck: true` (skips checking all `.d.ts`), `incremental: true` (writes `.tsbuildinfo`), and `types: [...]` to avoid loading every `@types` package.
- Split large repos with **project references** (`composite: true`, `tsc --build`); 5.4 stops a `--build` at the first project with errors (continuing past errors is 5.6).
- Annotate return types of exported functions; deeply inferred types and huge unions make declaration emit and checking slow.
- Prefer interfaces with `extends` over long intersection chains; avoid recursive conditional types with deep instantiation (TS2589 "Type instantiation is excessively deep and possibly infinite").
- Diagnose with `tsc --extendedDiagnostics`, `--generateTrace <dir>` (analyse with `@typescript/analyze-trace`), and `--explainFiles` for unexpected inclusions.
- Use a transpile-only tool (esbuild, swc, `tsx`) for dev loops and run `tsc --noEmit` separately in CI — they do **not** type-check. `isolatedModules` guarantees the code is transpilable file-by-file.

##### Runtime performance

- `target` matters: downlevelling `async`/generators, class fields or `?.` to old targets adds helpers and slows code. `es2022` output runs unchanged on Node.js 18 and later.
- With targets below the syntax level, use `importHelpers: true` with `tslib` to avoid duplicating helpers in every file.
- Numeric `enum`s and `namespace`s emit objects/IIFEs; `const` objects are equivalent; `const enum` inlines (with the caveats above).
- Types have no runtime cost; validation does — validate at boundaries, not in hot inner loops.
- Prefer `Map`/`Set` over objects for dynamic keys; avoid `delete` on hot objects (deoptimizes hidden classes); avoid creating closures in hot loops.

***

#### Testing

- Run `tsc --noEmit` (or `tsc -b`) in CI in addition to tests: Vitest, Jest with `ts-jest` `isolatedModules`/`babel-jest`/`@swc/jest`, `tsx` and `node --test` with type stripping all transpile without type checking.
- Pin test-runner versions compatible with TypeScript 5.4 and the project's Node.js version (current Vitest requires Node.js 22.12+, for example).
- Type-level tests: use `// @ts-expect-error` (fails if the line has *no* error, unlike `@ts-ignore`) and helper assertions.

```ts
type Equal<A, B> = (<T>() => T extends A ? 1 : 2) extends <T>() => T extends B ? 1 : 2 ? true : false;
type Expect<T extends true> = T;

declare function pick<T, K extends keyof T>(obj: T, keys: K[]): Pick<T, K>;
const picked = pick({ a: 1, b: "x" }, ["a"]);
type _t1 = Expect<Equal<typeof picked, { a: number }>>;

// @ts-expect-error — "c" is not a key
pick({ a: 1 }, ["c"]);

// @ts-expect-error — nothing below fails, so this directive itself is reported: TS2578 "Unused '@ts-expect-error' directive"
const fine: number = 1;
export { fine };
```

The last case is the protection `@ts-expect-error` gives: when the expected error disappears (because a bug was fixed or a type changed), the stale directive becomes a compile error. Prefer `@ts-expect-error` with a reason over `@ts-ignore`, and never use `@ts-nocheck` in production code.

- Test both branches of type guards with real inputs; a wrong guard body is unsound and the compiler trusts it.
- Mock with typed fakes (`satisfies Partial<Service>` plus explicit casts at one place) rather than `as any` sprinkled through tests.

***

#### Tooling

##### Compiler and editor

- Pin the compiler exactly (`"typescript": "5.4.5"` or `~5.4.5`); a caret range `^5.4.5` silently upgrades to 5.9 and changes diagnostics. Editors should use the workspace version ("TypeScript: Select TypeScript Version" → workspace) — a newer editor-bundled `tsserver` shows 5.5+ behaviour (e.g. inferred type predicates) that `tsc` 5.4 then rejects in CI.
- `tsc -v` must print `Version 5.4.5`. `npx tsc --showConfig` prints the effective configuration including implied options.
- 5.4 editor features: auto-imports of `package.json` `"imports"` subpaths and the "Add missing parameters" / "Add optional parameters" quick fixes for calls with too many arguments.[^1] Verified through the language-service API (`getCodeFixesAtPosition`): for an unresolved `helperFn` defined under an `"imports": {"#internal/*": ...}` mapping, 5.3.3 offers `Add import from "./internal/helper.js"` while 5.4.5 offers `Add import from "#internal/helper.js"`; the parameter quick fixes appear only in 5.4.5. These do not change what `tsc` accepts.
- `tsc --init` in 5.4 writes the long, commented template (the minimal template is 5.9).
- The `typescript` 5.4.5 package declares `engines: node >= 14.17`.

##### `@types` packages: pin versions that still support 5.4

DefinitelyTyped publishes each `@types` package with `typesVersions` and a minimum `typeScriptVersion`, and stops testing compiler versions older than two years.[^3] For TypeScript 5.4 this has practical consequences (verified):

- Current `@types/node` (26.x) declares `"typeScriptVersion": "5.6"` and uses `IteratorObject`/`BuiltinIteratorReturn`, types that do not exist in 5.4. With 5.4 it produces errors such as TS2304 "Cannot find name 'BuiltinIteratorReturn'" in `node_modules/@types/node`; `skipLibCheck` hides those but user code still breaks (`for await (const d of await opendir("."))` → TS2504 "Type 'Dir' must have a '[Symbol.asyncIterator]()' method").
- npm dist-tags name the last version tested with each compiler: `npm install -D @types/node@ts5.4` installs `@types/node` 25.9.3 (which declares `typeScriptVersion` 5.3) — the right choice for a 5.4 project. The same `tsX.Y` tags exist for every `@types` package.
- Transitive `@types/node` is not pinned by those tags: `@types/express` (and many other `@types` packages) depend on `@types/node: "*"`, so `npm install -D @types/express@ts5.4` alone still installs the newest `@types/node` and reproduces the errors above (verified). Add `@types/node` as a direct, pinned dev dependency (or an npm `overrides`/pnpm `overrides`/Yarn `resolutions` entry) so the whole tree uses one compatible version.
- Keep `@types/node`'s major version at or below the Node.js version the code runs on, or the types advertise APIs the runtime lacks.

Flag dependency bumps that pull in `@types/*` (or libraries with bundled types) requiring a newer compiler; the usual symptom is errors inside `node_modules/**/*.d.ts` or the sudden need for `skipLibCheck`.

##### Linting: typescript-eslint

typescript-eslint 8.x supports TypeScript `>=4.8.4 <6.1.0` (peer dependency of 8.72.0), so it works with 5.4.[^19] Use the type-aware presets (`strictTypeChecked`, `stylisticTypeChecked`) with `parserOptions.projectService: true`. Rules with the highest bug-catching value:

| Rule | Catches |
| --- | --- |
| `@typescript-eslint/no-floating-promises` | un-awaited promises |
| `@typescript-eslint/no-misused-promises` | promises in conditions, `forEach(async ...)`, void-returning callbacks |
| `@typescript-eslint/await-thenable`, `return-await` | awaiting non-promises; `return` without `await` in `try` |
| `@typescript-eslint/no-unnecessary-condition` | conditions that are always truthy/falsy per the types (partly what 5.6 added to the compiler) |
| `@typescript-eslint/switch-exhaustiveness-check` | missing union members in `switch` |
| `@typescript-eslint/no-unsafe-*` (`assignment`, `member-access`, `call`, `return`, `argument`) | `any` leaking from `JSON.parse`, untyped libraries |
| `@typescript-eslint/no-explicit-any`, `no-non-null-assertion` | escape hatches |
| `@typescript-eslint/consistent-type-imports` / `consistent-type-exports` | type-only import hygiene (or use `verbatimModuleSyntax`) |
| `@typescript-eslint/unbound-method` | detached methods losing `this` |
| `@typescript-eslint/strict-boolean-expressions` | truthiness checks on strings/numbers/nullable values |
| `@typescript-eslint/prefer-nullish-coalescing` | `||` where `0`/`""` are valid values |

Format with Prettier (or dprint/Biome); do not hand-format.

##### Running TypeScript without a build step

- **`tsx`** (esbuild-based) and **`ts-node --transpile-only`/`--swc`**: fast, no type checking.
- **Node.js type stripping**: Node.js 22.6 added `--experimental-strip-types`; it is on by default from Node.js 23.6 and 22.18 and marked stable from 24.12 and 25.2; `--experimental-transform-types` (22.7+) was removed in Node.js 26.[^20] It only erases types. Verified on Node.js 22.23 and 24.22:
  - `enum`, `namespace` with values, parameter properties and `import x = require()` throw `ERR_UNSUPPORTED_TYPESCRIPT_SYNTAX` (on Node.js 22–25 they need `--experimental-transform-types`; Node.js 26 has no transform mode at all). TypeScript 5.4 cannot warn about them (`--erasableSyntaxOnly` is 5.8).
  - A type imported without `type` (`import { User, make } from "./user.ts"` where `User` is an interface) fails at runtime: `SyntaxError: The requested module './user.ts' does not provide an export named 'User'`. Use `verbatimModuleSyntax` so `tsc` forces `import { type User, make }`.
  - Imports must name the `.ts` file. In 5.4 that requires `allowImportingTsExtensions` with `noEmit` (or `emitDeclarationOnly`); `tsc` cannot then emit runnable JavaScript (`--rewriteRelativeImportExtensions` is 5.7). Choose one model per package: "Node.js runs `.ts` directly" or "`tsc` emits `.js` with `.js` specifiers".
  - Type stripping ignores `tsconfig.json` (no `paths`, no `target` downlevelling) and does not type-check.

##### Build and bundle

- Libraries: `tsc` (declarations + JS), or `tsup`/`unbuild`/`rollup` with `tsc --emitDeclarationOnly`. Check output with `@arethetypeswrong/cli` and `publint`.
- Applications: Vite/esbuild/webpack with `module: preserve` + `noEmit` for type checking.
- Babel/esbuild/swc support for TypeScript syntax lags behind or differs in edge cases (`const enum`, `namespace` merging, `emitDecoratorMetadata` — esbuild does not implement it). For decorator-metadata frameworks (NestJS, TypeORM) use `tsc` or swc with `decoratorMetadata`.

***

#### Declaration Files and Publishing

- Emit declarations with `declaration: true` and ship `declarationMap: true` so "Go to definition" lands in source. 5.4 has no `--isolatedDeclarations` (5.5): declaration emit requires full type checking, so explicit return types on exported functions keep `.d.ts` stable and readable.
- A public `.d.ts` must not use features newer than the oldest TypeScript version consumers use. A library built with 5.4 that exports `NoInfer<T>` requires consumers on ≥ 5.4. Use `typesVersions` to ship downlevelled declarations for older consumers when needed.
- Do not reference `@types/node` types from a browser library's public API; do not publish `const enum`s; avoid `declare global` augmentations in libraries unless the library's purpose is to provide them.
- `stripInternal` removes `/** @internal */` declarations from `.d.ts` (still emitted in JS).
- Hand-written ambient declarations for untyped packages go in a `.d.ts` with `declare module "pkg" { ... }`; keep them minimal and accurate — `declare module "pkg";` (shorthand) types everything as `any`.
- Module augmentation must be in a module file (with at least one `import`/`export`), otherwise `declare module "x"` *replaces* the module instead of merging.

```ts
// file: src/types/express-augment.d.ts
import "express-serve-static-core";
declare module "express-serve-static-core" {
  interface Request {
    userId?: string; // optional: not every request is authenticated
  }
}
```

***

#### Observability and Debugging

- Emit source maps (`sourceMap: true`, or `inlineSourceMap` for tooling that cannot find files) and run Node.js with `--enable-source-maps` so stack traces point to `.ts` lines. `inlineSources: true` embeds the source for error trackers (Sentry etc.) — do not ship it publicly if the source is confidential.
- Preserve error causes (`new Error(msg, { cause })`, lib `es2022`) and log `err.cause` chains; structured loggers (pino, winston) need explicit serializers for `cause` and `AggregateError.errors`.
- Name custom errors (`this.name = new.target.name`) so logs and `instanceof`-free classification work.
- Type log payloads (`interface LogContext { requestId: string; userId?: string }`) instead of `Record<string, any>` to keep PII out by construction; never log raw request bodies, tokens or `process.env`.
- Use `AsyncLocalStorage` (`node:async_hooks`) for request-scoped context instead of passing `any` bags through every call.
- `tsc --extendedDiagnostics` and `--generateTrace` are the observability tools for the *compiler*.

***

#### Naming and Style

- `camelCase` for variables, functions and properties; `PascalCase` for types, interfaces, classes, enums and type parameters with meaning (`TItem` or `Item`; single letters `T`, `K`, `V` for simple generics); `UPPER_SNAKE_CASE` for true constants.
- No `I` prefix on interfaces (`User`, not `IUser`); no `Enum`/`Type` suffixes.
- Named exports over default exports (refactor-safe, consistent import names, better auto-import).
- One concept per module; `index.ts` barrels only at package boundaries (barrels slow down type checking and bundling and can create import cycles).
- Order imports: Node.js built-ins (`node:` prefix), external packages, internal aliases (`#internal/...`), relative imports.
- File names: kebab-case or camelCase consistently; match the output extension rules (`.mts`/`.cts` only when the format must differ from the package default).
- Prefer `unknown` over `any`, `readonly` arrays for inputs, union literal types over `string`, discriminated unions over optional-property bags, and `satisfies` over `as`.

***

#### Removed / Changed in 5.4 — Migration Notes (from 5.3)

1. **Upgrade the compiler**: `npm install -D typescript@5.4.5` (not 5.4.2: it contains the enum-name restriction that 5.4.3 reverted). Pin `@types/*` via `@ts5.4` dist-tags.
2. **Deprecated 5.0 options**: if `"ignoreDeprecations": "5.0"` is present, 5.4 is the last version where those options work. Migrate now: `importsNotUsedAsValues`/`preserveValueImports` → `verbatimModuleSyntax`; `out` → `outFile`; `target: es3` → `es5` or later; remove `keyofStringsOnly`, `suppressImplicitAnyIndexErrors`, `suppressExcessPropertyErrors`, `noStrictGenericChecks`, `noImplicitUseStrict`, `charset`, `prepend`; set `newLine` explicitly if needed. On 5.5 they silently do nothing; on 5.6+ they are errors.
3. **New errors to expect**: conditional-type constraint assignability (TS2322), enum compatibility across duplicate declarations (TS2322 "Each declaration of ... differs in its value"), TS2865 type-only import conflicting with a local value under `isolatedModules`, `Promise.all` results over `any` spreads now typed, intersection reductions to `never`.
4. **Errors that disappear**: closure narrowing after last assignment (remove now-redundant `!` and casts if you no longer support older compilers); template-literal placeholder checks.
5. **New capabilities to adopt**: `NoInfer<T>` instead of phantom type parameters; `module: preserve` for bundler projects; `Object.groupBy`/`Map.groupBy`/`Promise.withResolvers` *only if the runtime has them* (Node.js 21+/22+); an `ImportAttributes` declaration if the project wants to restrict import attributes.
6. **Emit diff**: expect small `.d.ts` and downlevel-`async` output changes; review snapshot tests of emitted code.
7. **Plan for 5.5+**: 5.5 removes the 5.0-deprecated options' effect and adds regex checking (new errors on invalid regexes); 5.6 adds always-truthy/nullish checks; 5.7 requires `with { type: "json" }` for JSON under `nodenext`. Writing code that already satisfies these (valid regexes, no tautological conditions, JSON import attributes) eases later upgrades.

***

#### Production / Review Checklist

##### Compiler and configuration

- [ ] `typescript` pinned to 5.4.5 (not a caret range that floats to 5.9; not 5.4.2)
- [ ] `strict: true`; plus `noUncheckedIndexedAccess`, `noImplicitOverride`, `exactOptionalPropertyTypes` in new code
- [ ] `module`/`moduleResolution` match the runtime: `nodenext` for Node.js, `preserve` (or `esnext`+`bundler`) with `noEmit` for bundlers
- [ ] `target` ≤ `es2022` or `esnext` (no `es2023`/`es2024` target in 5.4); `lib` matches the *runtime*, not just the compiler
- [ ] `verbatimModuleSyntax` (or at least `isolatedModules`) when any non-`tsc` transpiler or Node.js type stripping is involved
- [ ] No `ignoreDeprecations: "5.0"` left without a migration plan
- [ ] `@types/*` versions compatible with 5.4 (`@ts5.4` tags); `@types/node` major ≤ runtime major
- [ ] No 5.5+ syntax/options/APIs (inferred predicates relied on, `isolatedDeclarations`, `${configDir}`, Set methods, `Array.fromAsync`, iterator helpers, `--erasableSyntaxOnly`, `--rewriteRelativeImportExtensions`, `import defer`, `node18`/`node20` module)

##### Types and correctness

- [ ] No unjustified `any`; external data typed `unknown` and validated
- [ ] `as`, `!`, `as unknown as` and user-defined type guards reviewed for soundness
- [ ] Discriminated unions with `never` exhaustiveness checks; no optional-property soup
- [ ] Index access on records/arrays handles `undefined`
- [ ] `filter` callbacks that should narrow have explicit `x is T` predicates (5.4 does not infer them)
- [ ] Class fields: no subclass redeclaration without `declare`; no overridable calls from constructors; `!` assertions justified
- [ ] Callbacks declared as function properties, not methods, in new interfaces
- [ ] Enums: no duplicate-package enum mismatches; no `const enum` in published declarations

##### Modules and runtime

- [ ] Relative ESM imports carry `.js` (or `.ts` with `allowImportingTsExtensions` + no emit)
- [ ] No `__dirname`/`require` in ESM; `import.meta.dirname` only on Node.js 20.11+
- [ ] JSON imports use `with { type: "json" }` (not missing, not `assert`)
- [ ] No CommonJS `import` of ESM-only packages (TS1479) — dynamic `import()` instead
- [ ] `paths` aliases also resolved by the runtime/bundler, or replaced with `package.json` `"imports"`
- [ ] Library `exports` map: `types` first, separate `.d.ts`/`.d.cts` per format, checked with `attw`
- [ ] Runtime support verified for `Object.groupBy` (Node.js 21+), `Promise.withResolvers` (22+), `Symbol.metadata` (not in Node.js ≤ 24; polyfill), `DisposableStack` (24+), `Symbol.dispose` (current 18/20/22 releases)

##### Async

- [ ] No floating promises; no `forEach(async)`; no promises in boolean positions
- [ ] `return await` inside `try`; rejections handled; `AbortSignal` passed through and honoured
- [ ] Bounded concurrency for bulk operations; no check-then-act races across `await`

##### Security

- [ ] Parameterized SQL; `execFile`/`spawn` with argument arrays; HTML escaped by context
- [ ] No prototype pollution via merges/dynamic keys; `Map` for user-keyed data
- [ ] Path traversal checks with `resolve` + prefix test; `timingSafeEqual` for secrets; `crypto` randomness
- [ ] Ambient declarations (`ProcessEnv`, module augmentations) do not over-promise validated data

##### Tooling, testing and observability

- [ ] CI runs `tsc --noEmit` (or `tsc -b`) in addition to transpile-only test runners
- [ ] typescript-eslint type-aware rules enabled (`no-floating-promises`, `no-misused-promises`, `switch-exhaustiveness-check`, `no-unsafe-*`)
- [ ] `@ts-expect-error` with reasons instead of `@ts-ignore`; no `@ts-nocheck`
- [ ] Source maps enabled in production with `--enable-source-maps`; error `cause` chains logged; no secrets or PII in logs

***

[^1]: TypeScript 5.4 release notes. https://www.typescriptlang.org/docs/handbook/release-notes/typescript-5-4.html
[^2]: npm registry metadata for `typescript` (publish times of 5.4.2–5.4.5, 5.9.3, 6.0.3, 7.0.2; dist-tags). https://registry.npmjs.org/typescript
[^3]: DefinitelyTyped README — Support Window ("only tests packages on versions of TypeScript that are less than 2 years old"). https://github.com/DefinitelyTyped/DefinitelyTyped#support-window
[^4]: Announcing TypeScript 5.4 (March 6, 2024; includes auto-import support for subpath imports). https://devblogs.microsoft.com/typescript/announcing-typescript-5-4/
[^5]: microsoft/TypeScript PR #57853 "Revert PR 56161" (milestone TypeScript 5.4.3, cherry-picked to release-5.4) and PR #56161. https://github.com/microsoft/TypeScript/pull/57853 , https://github.com/microsoft/TypeScript/pull/56161
[^6]: TypeScript 5.5 release notes. https://www.typescriptlang.org/docs/handbook/release-notes/typescript-5-5.html
[^7]: TypeScript 5.5 release notes — Disabling Features Deprecated in TypeScript 5.0; TypeScript issue #51909 (deprecation plan). https://github.com/microsoft/TypeScript/issues/51909
[^8]: TypeScript 5.6 release notes. https://www.typescriptlang.org/docs/handbook/release-notes/typescript-5-6.html
[^9]: TypeScript 5.7 release notes. https://www.typescriptlang.org/docs/handbook/release-notes/typescript-5-7.html
[^10]: TypeScript 5.8 and 5.9 release notes. https://www.typescriptlang.org/docs/handbook/release-notes/typescript-5-8.html , https://www.typescriptlang.org/docs/handbook/release-notes/typescript-5-9.html
[^11]: TypeScript 6.0 release notes (new defaults, deprecations, preparing for 7.0). https://www.typescriptlang.org/docs/handbook/release-notes/typescript-6-0.html
[^12]: microsoft/TypeScript PR #56908 — Preserve narrowing in closures following last assignment. https://github.com/microsoft/TypeScript/pull/56908
[^13]: microsoft/TypeScript PR #56794 — NoInfer intrinsic type. https://github.com/microsoft/TypeScript/pull/56794
[^14]: microsoft/TypeScript PR #56805 — groupBy declarations; MDN Object.groupBy / Map.groupBy / Promise.withResolvers. https://github.com/microsoft/TypeScript/pull/56805 , https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Global_Objects/Object/groupBy
[^15]: microsoft/TypeScript PR #56785 — `--module preserve`. https://github.com/microsoft/TypeScript/pull/56785
[^16]: microsoft/TypeScript PR #56034 — Check import attributes against the global ImportAttributes type. https://github.com/microsoft/TypeScript/pull/56034
[^17]: microsoft/TypeScript PR #57027 — DOM lib updates for 5.4. https://github.com/microsoft/TypeScript/pull/57027
[^18]: Zod documentation — Requirements ("tested against TypeScript v5.5 and later"). https://zod.dev/
[^19]: typescript-eslint — supported TypeScript versions (peerDependencies of typescript-eslint 8.72.0 on npm). https://typescript-eslint.io/users/dependency-versions/
[^20]: Node.js 22.18.0 release notes (type stripping enabled by default) and Node.js TypeScript documentation. https://nodejs.org/en/blog/release/v22.18.0 , https://nodejs.org/api/typescript.html
[^21]: Node.js ECMAScript modules documentation — `import.meta.dirname` / `import.meta.filename` (added in v21.2.0, v20.11.0), import attributes, CommonJS interop. https://nodejs.org/api/esm.html
