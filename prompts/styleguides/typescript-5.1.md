### TypeScript 5.1: The Complete Production Developer Guide

> **Version**: TypeScript 5.1 — beta 5.1.0-beta (2023-04-18), RC 5.1.1-rc (2023-05-18), first stable build **5.1.3 (2023-06-01)**; 5.1.5 (2023-06-27; 5.1.4 was skipped) and the final patch **5.1.6 (2023-06-28)**.
> **Status (October 2026)**: 5.1 is **long superseded and receives no fixes**. TypeScript publishes no LTS or end-of-life schedule: fixes ship only in new releases of the current line, so 5.1.6 has been frozen since June 2023. Since then 5.2 (2023-08-24) through 5.9, then 6.0 (6.0.3, 2026-04-16, the last JavaScript-based compiler) and the native **TypeScript 7.0** (7.0.2, 2026-07-08, the newest stable release) have shipped; 7.1.0-beta appeared on npm on 2026-10-20. DefinitelyTyped only tests `@types` packages against TypeScript versions younger than two years, so 5.1 dropped out of its support window in mid-2025: the newest `@types/*` releases no longer compile with 5.1 (see Version-Aware Review).
> **Scope**: The full TypeScript language as usable with the 5.1 compiler, every 5.1 change versus 5.0, the 5.0 features a 5.1 codebase relies on (standard decorators, `const` type parameters, `--moduleResolution bundler`, `--verbatimModuleSyntax`, union enums, deprecations), module resolution for Node.js and bundlers, tsconfig recommendations, reviewer-relevant compiler pitfalls and tooling. Every TypeScript example in this guide was compiled with `tsc` 5.1.6 (boundary claims additionally against 5.0.4 and 5.2.2, some against later releases); emitted JavaScript was run on Node.js 14.21.3, 18.20.8 and 20.20.2 where runtime behaviour matters. Lines marked `// @ts-expect-error` are lines that 5.1.6 rejects, with the error code named after the directive.
> **Note**: This document is self-contained. Sections marked **[New in 5.1]** cover behaviour specific to this version.

---

#### What's New in TypeScript 5.1

TypeScript 5.1 is a small release focused on ergonomics, JSX and resolution. Its language-level changes are narrow but they change which code compiles; its breaking changes are about the environment (the compiler's own Node.js requirement) and about `typeRoots`.

| Area | Change in 5.1 | Verified against 5.0.4 |
|---|---|---|
| Return types | Functions declared to return `undefined` need no `return` statement; functions without return expressions are inferred to return `undefined` when contextually typed by an `undefined`-returning function type; `--noImplicitReturns` treats `undefined`-returning functions like `void` ones | 5.0 reports TS2355, TS2345 and TS7030 |
| Accessors | `get` and `set` may have completely **unrelated** types, as long as both are explicitly annotated | 5.0 reports TS2380 |
| JSX | `JSX.ElementType` decides which values are valid as a JSX tag (async components, components returning strings) | 5.0 reports TS2786 |
| JSX | Namespaced attribute and tag names (`a:b`, `a : b`) are parsed as one namespaced name | 5.0 parses `a : b` as two attributes |
| Resolution | Module resolution falls back to the configured `typeRoots` when a bare specifier cannot be resolved | 5.0 reports TS2307 |
| **Breaking** | An explicit `typeRoots` no longer walks up to parent `node_modules/@types` | 5.0 silently found the parent package |
| **Breaking** | The compiler itself targets ES2020 and requires **Node.js ≥ 14.17** (`engines` was `>=12.20` in 5.0) | — |
| Emit | Stacked field/accessor decorators apply their returned initializers in the corrected (spec) order | 5.0 applies them in the opposite order |
| Editor | Linked editing of JSX tag pairs, `@param` snippet completions, "Move to file" refactoring (preview) | 5.0 tsserver rejects `linkedEditingRange` |
| `lib.dom.d.ts` | New DOM typings: CSS Typed OM, Compression Streams, WebCodecs video, Screen Wake Lock, Reporting API, `UserActivation`, `FileSystemWritableFileStream` | absent in 5.0 |
| Performance | Fewer type instantiations, fast negative checks for literal unions, faster JSDoc scanning | — |
| API | `getOccurrencesAtPosition` / tsserver `occurrences` request removed (use document highlights) | present in 5.0 |

Patch releases: 5.1.5 fixed a 5.1 regression that crashed with "Maximum call stack size exceeded", an out-of-memory regression ("Type instantiation is excessively deep"), and a case where emitted JavaScript still contained JSX; 5.1.6 fixed a "Reached heap limit" regression in 5.1.3 and order-sensitive constraints of generic tuple types. There is no reason to run 5.1.3 instead of 5.1.6.

***

#### Version-Aware Review — Do Not Use or Recommend

##### How to judge code for a 5.1 project

- Determine the **compiler** version from `package.json` (`"typescript": "~5.1.6"`, `"5.1.x"`) and the lockfile, and the **runtime** separately (`engines.node`, `.nvmrc`, Dockerfile, browserslist). The compiler version decides which syntax, options and library typings type-check; the runtime decides which APIs exist at run time. TypeScript never polyfills — `lib` only declares types.
- Before claiming that an option, lib name or syntax "does not exist", check it against 5.1. Several things that look new are older: `satisfies` (4.9), `accessor` (4.9), `in` narrowing of unlisted properties (4.9), variance annotations `in`/`out` (4.7), `infer X extends C` (4.8), `node16`/`nodenext` (4.7), standard decorators, `const` type parameters, `--moduleResolution bundler`, `--verbatimModuleSyntax`, `export type *` and `extends: [...]` arrays (all 5.0) are available. Say "not available in 5.1" only for things this guide or the 5.1 compiler proves missing.
- `--target es2023` is **not** a valid target in 5.1 (TS6046; it arrives in 5.5), but `--lib es2023` is valid and contains only `findLast`/`findLastIndex`.
- A tsconfig that sets 5.0-deprecated options together with `"ignoreDeprecations": "5.0"` compiles on 5.1, is silently **ignored** on 5.5 (the options "no longer have any effect"), and **fails** on 5.6 and later (TS5102 "Option '…' has been removed. Please remove it from your configuration", verified on 5.6.3). Flag it as upgrade debt, not as a build break.

##### Do not use or recommend (not available in 5.1)

The compiler is TypeScript 5.1.6. Everything below was compiled on 5.1.6 and fails; the version in parentheses is the first release that accepts it (checked on that release).

- **5.2**: `using` / `await using` declarations (TS1434 "Unexpected keyword or identifier" on 5.1), `Symbol.dispose`, `Symbol.asyncDispose`, `Disposable`, `AsyncDisposable`, `DisposableStack`, `AsyncDisposableStack`, `SuppressedError` and the `esnext.disposable` lib (TS6046); **decorator metadata** (`context.metadata`, `Symbol.metadata`, the `esnext.decorators` lib); **mixed labeled/unlabeled tuple elements** such as `[first: string, ...number[]]` (TS5084 on 5.1); calling `filter`/`find`/`reduce` on a **union of array types** (`string[] | number[]`, TS2349 on 5.1); the ES2023 "change array by copy" methods `toSorted`, `toReversed`, `toSpliced`, `with` (TS2339 on 5.1); `WeakMap`/`WeakSet`/`WeakRef` with **symbol keys** (`lib.es2023.collection`); `import type` paths ending in `.ts` without `allowImportingTsExtensions` (TS5097 on 5.1); TS2729 for field initializers that read parameter properties under `target: es2022` (see Classes).
- **5.3**: import attributes `import x from "./a.json" with { type: "json" }` (TS1005 on 5.1 — 5.1 only knows the older `assert { … }` form); `resolution-mode` in all module modes; `switch (true)` narrowing (TS18046 on 5.1); narrowing on comparisons to booleans; `instanceof` narrowing via `Symbol.hasInstance`.
- **5.4**: preserved narrowing in closures after the last assignment (TS2339 on 5.1); the `NoInfer<T>` utility type (TS2304); `Object.groupBy`, `Map.groupBy` and `Promise.withResolvers` typings (TS2339); `--module preserve` (TS6046).
- **5.5**: inferred type predicates (`xs.filter(x => x !== undefined)` stays `(T | undefined)[]` on 5.1 — TS2322); `--isolatedDeclarations` (TS5023); regular-expression syntax checking; the `${configDir}` template in tsconfig paths; `Set` methods (`union`, `intersection`, … — TS2339); `--target es2023`; control-flow narrowing for constant indexed accesses (`obj[key]` with `const key`); JSDoc `@import`. 5.5 also stops honouring every option deprecated in 5.0.
- **5.6**: Iterator helper typings (`values().map(...)`, `IteratorObject`, `BuiltinIteratorReturn`; TS2339 on 5.1); `--noUncheckedSideEffectImports`, `--noCheck`, `--strictBuiltinIteratorReturn` (all TS5023); disallowed always-truthy/nullish checks (`if (/regex/)`).
- **5.7**: `--rewriteRelativeImportExtensions`, `--target es2024`/`--lib es2024`, never-initialized-variable checks, `TypedArray<TArrayBuffer>` generics.
- **5.8 / 5.9 / 6.0 / 7.0**: `--erasableSyntaxOnly`, `--libReplacement`, `--module node18`/`node20` (TS6046), `require()` of ESM under `nodenext`, `import defer`, the 6.0 default changes (`strict` on by default, `types: []`, …) and the native 7.0 compiler. None of them applies to a 5.1 build.
- **Library typings for newer runtime APIs**: anything that only exists in `lib.es2024`+ or newer `esnext.*` libs (`Array.fromAsync`, `Promise.withResolvers`, `Object.groupBy`, `Set` methods, Iterator helpers, `RegExp.escape`, `Float16Array`, `Temporal`). If the runtime has them and the project needs them, a local `declare global` augmentation is the only 5.1-compatible route — and the runtime must be checked.
- **The newest `@types` packages.** `@types/node` 26.6.5 (the current `latest`) produces 97 lines of errors on 5.1.6 (`Cannot find name 'Disposable'`, `Property 'dispose' does not exist on type 'SymbolConstructor'`, `Cannot find name 'IteratorObject'`, …). DefinitelyTyped publishes a `ts5.1` dist-tag that points to the last release tested with 5.1: in October 2026 that is **`@types/node@24.1.0`** and **`@types/react@19.1.9`** (`npm dist-tags @types/node`). Install with `npm i -D @types/node@ts5.1`, and pin with `~` so Renovate/Dependabot do not drift past it. `skipLibCheck: true` is not a fix: it silences the errors inside `@types/node`, but the declarations that reference missing 5.2+ types stay broken and fail at the use site — with `@types/node` 26.6.5 and `skipLibCheck`, `for await (const e of await opendir("."))` reports TS2504 "Type 'Dir' must have a '[Symbol.asyncIterator]()' method that returns an async iterator" on 5.1.6, while `@types/node@24.1.0` types `e` as `Dirent<string>` (verified).

##### Deprecated or removed older features (do not introduce in 5.1 code)

- Deprecated **in 5.0**, still accepted by 5.1 only with `"ignoreDeprecations": "5.0"` (otherwise TS5101/TS5107 "is deprecated and will stop functioning in TypeScript 5.5"), **without effect from 5.5 and rejected from 5.6** (the official notes announced hard errors for 5.5/6.0; 5.5.4 still accepts them with `ignoreDeprecations: "5.0"`, 5.6.3 rejects them with TS5102/TS5108): `target: ES3`, `out`, `noImplicitUseStrict`, `keyofStringsOnly`, `suppressExcessPropertyErrors`, `suppressImplicitAnyIndexErrors`, `noStrictGenericChecks`, `charset`, `importsNotUsedAsValues`, `preserveValueImports`, and `prepend` in project references. The only valid `ignoreDeprecations` value in 5.1 is `"5.0"`; anything else (for example `"5.5"`) is TS5103 "Invalid value for '--ignoreDeprecations'" (verified, also without any deprecated option set).
- 5.0 also changed two defaults that 5.1 keeps: `newLine` is `lf` (no OS-based inference) and `forceConsistentCasingInFileNames` is `true`.
- `experimentalDecorators` and `emitDecoratorMetadata` are **not** deprecated in 5.1 and remain required for Angular, NestJS, TypeORM, InversifyJS and other frameworks that depend on parameter decorators or `reflect-metadata`. New framework-independent decorators should use the standard form.
- `moduleResolution: "node"` is the old spelling of `node10` (both accepted by 5.1). It is not deprecated in 5.1 but it predates `exports` support; new projects should use `node16`/`nodenext` or `bundler`.

***

#### New in 5.1: Language and Checker Changes

##### Easier implicit returns for `undefined`-returning functions [New in 5.1]

Before 5.1, only `void`- and `any`-returning functions could omit every `return`. A function annotated `: undefined` needed `return undefined;` (or `return;` plus the annotation). 5.1 relaxes three rules:

1. A function whose declared return type is `undefined` needs no `return` statement.
2. A function expression or arrow function with no `return` expressions, contextually typed by a function type that returns `undefined`, is inferred to return `undefined` (not `void`).
3. Under `--noImplicitReturns`, a function returning only `undefined` may have code paths without an explicit `return`.

```ts
// tsconfig: {"noImplicitReturns": true}
// Compiles on 5.1 with --strict --noImplicitReturns; each function below is an error on 5.0.
function reset(): undefined {
  // no return statement needed
}

declare function onDone(cb: () => undefined): void;
onDone(() => {});               // inferred as () => undefined
onDone(function () { return; });

function maybeStop(flag: boolean): undefined {
  if (flag) {
    return;                     // other path falls off the end: fine under --noImplicitReturns
  }
}

export { reset, maybeStop };
```

What did **not** change (verified on 5.1.6):

```ts
// A union that merely contains undefined still needs a return:
// @ts-expect-error TS2355 A function whose declared type is neither 'undefined', 'void', nor 'any' must return a value.
function find(): string | undefined {}

// async functions declared Promise<undefined> are covered too:
async function flush(): Promise<undefined> {}

export { find, flush };
```

Review notes:

- Prefer `void` for callbacks whose result is ignored. Use `undefined` only when an API really distinguishes "returns `undefined`" from "returns anything" (some APIs check `=== undefined`). A `() => void` callback type accepts functions returning values; `() => undefined` does not.
- On a mixed-version monorepo (some packages on 5.0) a `(): undefined => {}` callback compiles in the 5.1 package and fails in the 5.0 one — check the compiler version per package.

##### Unrelated types for getters and setters [New in 5.1]

TypeScript 4.3 allowed different `get`/`set` types but required the getter type to be assignable to the setter type. 5.1 drops that requirement **when both accessors carry explicit type annotations**.

```ts
class SafeBox {
  #value: string | undefined;
  set value(next: string) {          // only strings may be written
    this.#value = next;
  }
  get value(): string | undefined {  // reads may be undefined (TS2380 on 5.0)
    return this.#value;
  }
}

interface StyleRule {
  get style(): { color: string };    // reads an object
  set style(css: string);            // writes a string (TS2380 on 5.0)
}

declare const rule: StyleRule;
rule.style = "color: red";
const color: string = rule.style.color;

const box = new SafeBox();
box.value = "x";
// @ts-expect-error TS2322 'undefined' is not assignable to the setter type 'string'
box.value = undefined;

export { color };
```

Without an annotation the getter's inferred type must still agree with the setter:

```ts
class Counter {
  #n = 0;
  // @ts-expect-error TS2322 the un-annotated getter is checked against the setter type
  get n() { return this.#n; }
  set n(v: string) { this.#n = Number(v); }
}
export { Counter };
```

Review notes:

- `obj.x = obj.x` is no longer guaranteed to type-check, and `obj.x += 1` may be legal for one accessor and not the other. Divergent accessor types are a sharp API; use them for faithful models of existing APIs (DOM `style`, `location.href`-like setters) or "accept only valid input, may read `undefined`" patterns — not as a general convenience.
- `Object.assign`, spreading, `structuredClone` and JSON serialisation only see the **getter** value. A setter-only type does not describe what the object holds.
- 5.1's `lib.dom.d.ts` does **not** yet use the new freedom (for example `CSSStyleRule.style` is still read-only `CSSStyleDeclaration`).

***

#### New in 5.1: JSX Changes

##### `JSX.ElementType` — decoupled checking of JSX tags [New in 5.1]

Before 5.1, a component was valid as a tag only if its return (or instance) type was assignable to `JSX.Element` (or `JSX.ElementClass`). So components returning `string`, `null` unions or `Promise`s (async server components) could not be typed without loosening `JSX.Element` for everyone. 5.1 looks up an optional `JSX.ElementType`; when present, it alone decides which values are valid as a tag, while `JSX.Element` stays the type *produced* by a JSX expression.

```tsx
// tsconfig: {"jsx": "preserve"}
declare global {
  namespace JSX {
    type Element = { tag: unknown };
    interface ElementClass { render(): unknown }
    interface IntrinsicElements {
      div: { id?: string; children?: unknown };
    }
    type ElementType =
      | keyof IntrinsicElements
      | ((props: any) => Element | Promise<Element> | string | null)
      | (new (props: any) => ElementClass);
  }
}

async function Profile() { return <div />; }   // TS2786 on 5.0: Promise<Element> is not a valid JSX element
function Label() { return "hello"; }            // TS2786 on 5.0: string is not a valid JSX element

const a = <Profile />;
const b = <Label />;
export { a, b };
```

Review notes:

- `ElementType` is defined by the **JSX library's type package**, not by applications. React's types declare it (`React.JSX.ElementType`): with `@types/react@19.1.9` (the `ts5.1` tag) and `jsx: react-jsx`, an `async` component and a component returning a `string` type-check as tags on 5.1, while 5.0 rejects both with TS2786 because it ignores `ElementType` (verified). Whether async components actually *work* depends on the runtime (React Server Components), not on TypeScript.
- When `ElementType` is declared, a lowercase tag that is not in `IntrinsicElements` produces an extra error (TS2786 "'svg:circle' cannot be used as a JSX component") besides the usual TS2339.

##### Namespaced JSX attribute and tag names [New in 5.1]

5.1 parses namespaced names (`xlink:href`, `svg:rect`) as a single `JsxNamespacedName`, including with whitespace around the colon. Attribute names are checked against props under the key `"a:b"`; namespaced tag names with a lowercase first segment are looked up in `JSX.IntrinsicElements["a:b"]`.

Tested difference: `<Foo a:b="x" />` (no spaces) already type-checked in 4.9 and 5.0 (it was scanned as one identifier), but `<Foo a : b="x" />` is a syntax error on 5.0 (TS1003) and its React emit turned into `{ a: true, b: "x" }`. On 5.1 both forms emit `React.createElement(Foo, { "a:b": "x" })`.

```tsx
// tsconfig: {"jsx": "preserve"}
declare global {
  namespace JSX {
    type Element = object;
    interface IntrinsicElements {
      "svg:rect": { width: number };
    }
  }
}
function Link(props: { "xlink:href": string }) { return { href: props["xlink:href"] }; }

const ok = <Link xlink:href="#top" />;
// @ts-expect-error TS2322 number is not assignable to string
const bad = <Link xlink:href={1} />;
const rect = <svg:rect width={10} />;
export { ok, bad, rect };
```

React DOM does **not** accept namespaced attribute names on DOM elements (it uses camelCase `xlinkHref`); namespaced names are for JSX runtimes that support them (some SVG/XML-oriented JSX libraries). Flag `xlink:href` on React DOM elements.

***

#### New in 5.1: Module Resolution and `typeRoots`

##### `typeRoots` are consulted in module resolution [New in 5.1]

When the configured resolution strategy cannot resolve a bare specifier, 5.1 tries to resolve it relative to each `typeRoots` entry. Verified with a `typeRoots: ["./types"]` project containing `types/foo/index.d.ts` and `import { foo } from "foo"`: 5.0.4 reports TS2307 "Cannot find module 'foo'" under `node10`, `bundler` and `nodenext`; 5.1.6 resolves the import to `types/foo/index.d.ts` in all three modes (shown by `--traceResolution`: "Module name 'foo' was successfully resolved to …/types/foo/index.d.ts").

Review consequences:

- A repository with hand-written `types/<pkg>/index.d.ts` folders listed in `typeRoots` now satisfies imports of `<pkg>` even when the real package is **not installed** — the build passes and the import fails at run time. Check that every imported package is in `dependencies`.
- The reverse is also true: removing a folder from `typeRoots` can break imports that silently depended on this fallback.
- Prefer `declare module "pkg"` in an included `.d.ts` file or `paths` mapping for intentional shims; they are explicit and work the same on 5.0.

##### Breaking: an explicit `typeRoots` no longer walks up to parent `node_modules/@types` [New in 5.1]

Previously, if a `types` entry (or `/// <reference types>`) could not be found in any `typeRoots` directory, TypeScript still walked up parent directories looking in each `node_modules/@types`. 5.1 stops doing that.

Verified: `typeRoots: ["./types"]`, `types: ["bar"]`, with `bar` only in a **parent** directory's `node_modules/@types`: 5.0 compiles, 5.1 reports

```text
error TS2688: Cannot find type definition file for 'bar'.
  The file is in the program because:
    Entry point of type library 'bar' specified in compilerOptions
```

Fix by listing every `@types` directory explicitly — including the hoisted monorepo root:

```jsonc
{
  "compilerOptions": {
    "types": ["node", "jest"],
    "typeRoots": [
      "./types",
      "./node_modules/@types",
      "../../node_modules/@types"   // monorepo root where npm/pnpm/yarn hoist @types
    ]
  }
}
```

Most projects should not set `typeRoots` at all — the default (`node_modules/@types` in every ancestor directory) is right. Set `types` instead to limit which global type packages are loaded.

***

#### New in 5.1: Editor Features, Performance, Runtime Requirement, `lib.d.ts`

##### Editor features [New in 5.1]

- **Linked editing for JSX tags**: tsserver answers `linkedEditingRange` (5.0 replies "Unrecognized JSON command: linkedEditingRange"). VS Code needs `"editor.linkedEditing": true`.
- **`@param` snippet completions**: typing `@par` inside a JSDoc comment above `function add(a: number, b: number)` offers `param a ` / `param b ` snippets (5.0 only offers the bare `param` tag). Verified through the tsserver protocol.
- **Move to file** (move declarations to an *existing* file) shipped as a preview refactoring.
- The tsserver `occurrences` request and `LanguageService.getOccurrencesAtPosition` were removed; editor plugins must use `documentHighlights`/`getDocumentHighlights`.

##### Performance [New in 5.1]

5.1 skips instantiation inside object types that cannot reference outer type parameters (the TypeScript team measured >50 % less check time on material-ui's docs), short-circuits literal-to-literal-union relations (one reported case went from ~45 s to ~0.4 s), and scans JSDoc in larger chunks. Projects that were slow on 5.0 because of large literal unions or heavy generic UI libraries should measure again before restructuring types.

##### The compiler requires Node.js 14.17+ [New in 5.1]

The `typescript` package now targets ES2020 and declares `"engines": { "node": ">=14.17" }` (5.0.4 declared `>=12.20`). On Node 10/12, `tsc` dies with `SyntaxError: Unexpected token '?'`, and npm/yarn print `EBADENGINE`/"engine incompatible" errors. 5.1.6 runs fine on Node 14.21.3 (verified). This only concerns the machine that **runs the compiler** — emitted code targets whatever `target` says. Node 14, 16 and 18 are themselves end-of-life (2023-04-30, 2023-09-11 and 2025-04-30), and Node 20 reached end-of-life on 2026-04-30; build machines should run Node 22 or 24.

##### `lib.dom.d.ts` additions [New in 5.1]

Comparing the declarations in 5.0.4 and 5.1.6 shows 136 new DOM declarations and none removed: CSS Typed OM (`CSSStyleValue`, `CSSNumericValue`, `CSSUnitValue`, `CSSTransformValue`, `StylePropertyMap`, …), `CompressionStream`/`DecompressionStream`, WebCodecs video (`VideoEncoder`, `VideoDecoder`, `VideoFrame`, `EncodedVideoChunk`), Screen Wake Lock (`WakeLock`, `WakeLockSentinel`), the Reporting API (`ReportingObserver`, `Report`), `UserActivation`, `FileSystemWritableFileStream`, `CSSPropertyRule`. A local polyfill or hand-written declaration of any of these names can now collide with the built-in one (verified: `declare global { var CompressionStream: any }` compiles on 5.0 and is TS2403 "Subsequent variable declarations must have the same type" on 5.1) — delete the local copy when upgrading from 5.0. The types say nothing about browser support: `CompressionStream` and WebCodecs need recent browsers.

##### Decorator initializer order [New in 5.1]

The standard-decorator emit changed so that initializers returned by **stacked** field (and accessor) decorators run in the order the updated decorators specification requires. The emitted `__esDecorate` helper uses `initializers.unshift(...)` where 5.0 used `push(...)`:

```ts
const tag = (s: string) =>
  (_t: undefined, _c: ClassFieldDecoratorContext<unknown, string>) =>
  (v: string) => v + s;

class C {
  @tag("A") @tag("B") x = "0";
}
console.log(new C().x); // 5.1.6 (and 5.2+): "0AB"   5.0.4: "0BA"
export { C };
```

With `importHelpers: true` the helper comes from **tslib**, and the order depends on the installed tslib, not on `tsc`: tslib 2.5.0 still produces `"0BA"`, tslib ≥ 2.5.1 (2023-05-17, "Reverse order of decorator-injected initializers") produces `"0AB"` (verified). A 5.1 project with standard decorators and `importHelpers` must use tslib ≥ 2.5.1.

***

#### The 5.0 Foundation a 5.1 Codebase Builds On

TypeScript 5.0 (2023-03-16) was a large release; 5.1 projects use its features constantly, and many 5.1 upgrades are really 4.9 → 5.1 upgrades. Each item below compiles on 5.1.6 and fails (or behaves differently) on 4.9.5.

##### Standard (TC39) decorators (5.0)

Without `--experimentalDecorators`, decorators follow the Stage 3 ECMAScript proposal: a decorator receives the decorated value and a typed **context object** (`ClassMethodDecoratorContext`, `ClassFieldDecoratorContext`, `ClassAccessorDecoratorContext`, `ClassGetterDecoratorContext`, `ClassSetterDecoratorContext`, `ClassDecoratorContext`) and may return a replacement. Details, pitfalls and the comparison with legacy decorators are in the Decorators section.

##### `const` type parameters (5.0)

`<const T>` infers literal/readonly types as if the caller had written `as const`. The constraint must be `readonly` for arrays, otherwise inference silently falls back to the constraint:

```ts
declare function routes<const T extends readonly string[]>(paths: T): T;
const r = routes(["/a", "/b"]);           // readonly ["/a", "/b"]

declare function bad<const T extends string[]>(paths: T): T;
const b = bad(["/a", "/b"]);              // string[] — the readonly tuple did not fit the mutable constraint

const list = ["/a", "/b"];
const c = routes(list);                   // string[] — const only affects literals written in the call

export { r, b, c };
```

##### All enums are union enums (5.0)

Every enum member — including computed ones — has its own literal type, so enums narrow and members can be used as types. Assigning an out-of-range literal to a numeric enum is now an error:

```ts
enum Digit { Zero = 0, Two = 2, Four = 4 }
// @ts-expect-error TS2322 Type '1' is not assignable to type 'Digit'
const d: Digit = 1;
const ok: Digit = 2;                      // a literal that matches a member is accepted
export { d, ok };
```

##### Forbidden implicit coercions in relational operators (5.0)

```ts
function over(limit: number | string) {
  // @ts-expect-error TS2365 Operator '>' cannot be applied to types 'string | number' and 'number'
  return limit > 4;
}
function overFixed(limit: number | string) {
  return Number(limit) > 4;               // explicit conversion; reject NaN if input is untrusted
}
export { over, overFixed };
```

##### Module and resolution features (5.0)

- `--moduleResolution bundler` (hybrid: `exports`/`imports` conditions, extensionless relative imports allowed). Requires `module` ≥ `es2015` (TS5095 "Option 'bundler' can only be used when 'module' is set to 'es2015' or later" on 5.1 — `module: commonjs` + `bundler` only became legal in 6.0).
- `--verbatimModuleSyntax`: imports/exports without `type` are kept verbatim; with `type` they are erased; replaces `importsNotUsedAsValues`/`preserveValueImports`.
- `--allowImportingTsExtensions` (only with `noEmit` or `emitDeclarationOnly`, otherwise TS5096), `--resolvePackageJsonExports`, `--resolvePackageJsonImports`, `--allowArbitraryExtensions` (`app.d.css.ts` declares `./app.css`), `--customConditions`.
- `export type * from "./mod"` and `export type * as ns from "./mod"`.
- `"extends": ["@tsconfig/strictest/tsconfig.json", "./tsconfig.base.json"]` — later entries win.
- `--build` accepts emit flags such as `--declaration`, `--sourceMap`, `--emitDeclarationOnly`, `--declarationMap`, `--inlineSourceMap`.

##### JSDoc (5.0)

`@satisfies` and `@overload` work in `checkJs` files (see JavaScript Checking).

##### Breaking changes in 5.0 that surface in 5.1 upgrades

- Under `--experimentalDecorators`, constructor-parameter decorators are checked with `propertyKey: undefined`; decorator factories typed `(target, key: string | symbol, index?) => void` fail under `strictNullChecks` with TS1239 "Unable to resolve signature of parameter decorator … Argument of type 'undefined' is not assignable to parameter of type 'string | symbol'" (4.9 accepted them) and need `key: string | symbol | undefined`.
- Enum overhaul (above), relational coercion errors (above), `lib.d.ts` DOM changes (some properties became numeric literal types; clipboard event members moved).
- Deprecated options listed in Version-Aware Review.

***

#### Recommended tsconfig Profiles

Both profiles below were compiled with 5.1.6 against a small `src/` tree (the bundler profile with `types: ["node"]` substituted for `vite/client`). None uses an option that 5.1 lacks.

##### Node.js application or library, ESM or CommonJS, emitted by `tsc`

```jsonc
{
  "compilerOptions": {
    // Language and environment
    "target": "es2022",                 // Node 16.11+ runs all of ES2022; there is no es2023 target in 5.1
    "lib": ["es2023"],                  // es2023 in 5.1 = es2022 + findLast/findLastIndex
    "types": ["node"],                  // pin @types/node to the ts5.1 tag (24.1.0)

    // Modules
    "module": "nodenext",               // implies moduleResolution nodenext; "type" in package.json decides ESM vs CJS per file
    "verbatimModuleSyntax": true,       // what you write is what is emitted; type-only imports need `type`
    "resolveJsonModule": true,

    // Strictness
    "strict": true,
    "noUncheckedIndexedAccess": true,
    "exactOptionalPropertyTypes": true,
    "noImplicitOverride": true,
    "noImplicitReturns": true,
    "noFallthroughCasesInSwitch": true,
    "noPropertyAccessFromIndexSignature": true,
    "useUnknownInCatchVariables": true, // already part of strict; listed for visibility

    // Emit
    "outDir": "dist",
    "rootDir": "src",
    "declaration": true,
    "declarationMap": true,
    "sourceMap": true,

    // Hygiene
    "forceConsistentCasingInFileNames": true, // default true since 5.0
    "skipLibCheck": true,
    "isolatedModules": true
  },
  "include": ["src"]
}
```

Notes:

- `module: "nodenext"` in 5.1 means "Node 16+ semantics": ESM files need explicit `.js` extensions on relative imports (TS2835 "Relative import paths need explicit file extensions … Did you mean './util.js'?"), CommonJS files may not use `import.meta`, and `import x = require("…")` in an ESM file is emitted through `createRequire(import.meta.url)`.
- `verbatimModuleSyntax` in a file that Node treats as **CommonJS** (no `"type": "module"`, or a `.cts` file) rejects ESM syntax: `export const a = 1` gives TS1287 "A top-level 'export' modifier cannot be used on value declarations in a CommonJS module when 'verbatimModuleSyntax' is enabled". Use `"type": "module"` (or `.mts`) or write `export =`/`import x = require()`. This is the right behaviour — it stops accidental CommonJS output — but it surprises teams turning the flag on.
- `useDefineForClassFields` defaults to `true` for `target` ≥ `es2022`; see Classes for the semantic consequences.
- `skipLibCheck: true` is standard practice, but it **does not** make incompatible `@types` packages safe on 5.1 — it only stops reporting errors inside them; declarations that depend on missing 5.2+ types still break where your code uses them (see Version-Aware Review).

##### Bundled application (Vite, esbuild, webpack, Rollup, Parcel)

```jsonc
{
  "compilerOptions": {
    "target": "es2022",
    "lib": ["es2023", "dom", "dom.iterable"],
    "module": "esnext",
    "moduleResolution": "bundler",
    "allowImportingTsExtensions": true,   // only legal with noEmit / emitDeclarationOnly
    "noEmit": true,                       // the bundler emits; tsc only type-checks
    "verbatimModuleSyntax": true,         // esbuild/swc compile one file at a time
    "isolatedModules": true,
    "jsx": "react-jsx",
    "strict": true,
    "noUncheckedIndexedAccess": true,
    "skipLibCheck": true,
    "types": ["vite/client"]
  },
  "include": ["src"]
}
```

`bundler` hides problems for **published libraries**: a package type-checked with `bundler` may import extensionless paths or rely on conditions that fail for Node.js consumers. Libraries should be checked with `nodenext`.

##### Library published to npm (dual consumers)

- Type-check and emit with `module: "nodenext"`; ship `.d.ts` next to the JavaScript; describe entry points with `exports` and a `types` condition first in each block.
- Declaration files carry your syntax to consumers. A `.d.ts` containing a `const` type parameter does not even parse on TypeScript 4.9 (TS1139 "Type parameter declaration expected", verified), so it breaks every consumer that has not upgraded, even with `skipLibCheck`. State the minimum TypeScript version in the README, or use `typesVersions` to ship downlevelled declarations.
- `"stripInternal": true` together with `/** @internal */` keeps internal APIs out of `.d.ts` files.

##### Strictness flags in 5.1

`strict: true` enables `alwaysStrict`, `noImplicitAny`, `noImplicitThis`, `strictNullChecks`, `strictFunctionTypes`, `strictBindCallApply`, `strictPropertyInitialization` and `useUnknownInCatchVariables`. Flags worth enabling on top (all exist in 5.1): `noUncheckedIndexedAccess` (4.1), `exactOptionalPropertyTypes` (4.4; requires `strictNullChecks`, otherwise TS5052), `noImplicitOverride` (4.3), `noPropertyAccessFromIndexSignature` (4.2), `noImplicitReturns`, `noFallthroughCasesInSwitch`, `noUnusedLocals`, `noUnusedParameters`, `allowUnreachableCode: false`, `allowUnusedLabels: false`. Not available in 5.1: `strictBuiltinIteratorReturn` (5.6), `noUncheckedSideEffectImports` (5.6), `isolatedDeclarations` (5.5), `erasableSyntaxOnly` (5.8).

***

#### Modules and Module Resolution

##### Choosing `module` / `moduleResolution` in 5.1

| Runtime / tool | `module` | `moduleResolution` |
|---|---|---|
| Node.js (ESM or CJS), code emitted by `tsc` | `nodenext` (or `node16`) | implied (`nodenext`/`node16`) |
| Bundler / Vite / esbuild / Bun | `esnext` (or `es2022`) | `bundler` |
| Legacy CommonJS-only project | `commonjs` | `node10` (alias `node`) |
| Deno | `esnext` | `bundler` (with `allowImportingTsExtensions` + `noEmit`) |

- In 5.1, `module` and `moduleResolution` **can disagree** without an error — `--module esnext --moduleResolution node16` and `--module nodenext --moduleResolution node10` both compile. Such combinations are misconfigurations (the emitted module format does not match the resolution rules), and 5.2 turns them into errors (TS5110/TS5109). Flag them in review so the 5.2+ upgrade does not break.
- `--module preserve` does not exist in 5.1 (5.4). For a bundler project that needs `require()` calls in TypeScript sources, 5.1 offers only `import x = require("x")` under `module: commonjs`.
- `module: node16` and `nodenext` model the same Node.js module rules in 5.1. `nodenext` is the floating option that later releases extend (for example `require()` of ESM in 5.8), `node16` stays fixed. `node18` and `node20` do not exist (5.8/5.9).

##### ESM rules under `node16` / `nodenext`

```text
// src/main.ts in a package with "type": "module", module: nodenext
import { helper } from "./util";
main.ts(1,24): error TS2835: Relative import paths need explicit file extensions in EcmaScript imports
when '--moduleResolution' is 'node16' or 'nodenext'. Did you mean './util.js'?
```

- Write the **output** extension: `./util.js` for `util.ts`, `./util.mjs` for `util.mts`, `./util.cjs` for `util.cts`. `.ts` extensions are rejected (TS5097) unless `allowImportingTsExtensions` (with `noEmit`) — and 5.1 rejects them even in `import type` (allowed from 5.2).
- Directory imports (`./lib` → `./lib/index.js`) do not work in Node ESM; TypeScript reports them under `nodenext` (TS2834 "Relative import paths need explicit file extensions …").
- Importing a CommonJS module from ESM: the default import is `module.exports`; named imports work only for names Node's static analysis (cjs-module-lexer) detects. TypeScript cannot see what cjs-module-lexer detects — when a named import of a CJS package fails at run time but type-checks, switch to a default import and destructure.
- `import.meta.url`, `import.meta.dirname` (runtime ≥ Node 20.11; not in `@types/node` typings older than 20.11) and top-level `await` are ESM-only. In a file that compiles to CommonJS, 5.1 reports TS1470 "The 'import.meta' meta-property is not allowed in files which will build into CommonJS output" and TS1309 "The current file is a CommonJS module and cannot use 'await' at the top level".
- JSON: `import data from "./data.json" assert { type: "json" }` is the only attribute syntax 5.1 parses; `with { … }` is a parse error until 5.3. Node.js 22 removed `assert` (verified: Node 20.20.2 loads the module, Node 22.23.4 throws `SyntaxError: Unexpected identifier 'assert'`) — on a 5.1 project running on Node ≥ 22, load JSON with `createRequire(import.meta.url)("./data.json")` or `JSON.parse(await readFile(…))`, or upgrade TypeScript to ≥ 5.3.

##### `verbatimModuleSyntax` and `isolatedModules`

```ts
// tsconfig: {"verbatimModuleSyntax": true, "module": "esnext", "moduleResolution": "bundler"}
import type { IncomingMessage } from "node:http";          // erased
import { createServer, type ServerResponse } from "node:http"; // kept as `import { createServer } from "node:http"`

export function handler(_req: IncomingMessage, res: ServerResponse) {
  res.end("ok");
}
export const server = createServer(handler);
```

- Every type-only import must use `type` (TS1484 "'X' is a type and must be imported using a type-only import when 'verbatimModuleSyntax' is enabled"). Auto-fix with `eslint --fix` (`@typescript-eslint/consistent-type-imports`) or the TypeScript quick fix.
- `import { type A } from "mod"` keeps `import {} from "mod"` — i.e. **the module still runs** for its side effects. Use `import type { A }` when the module must not be loaded.
- `isolatedModules` reports constructs that single-file transpilers (esbuild, swc, Babel, `ts.transpileModule`) cannot compile correctly: re-exporting a type without `export type` (TS1205), and referring to an ambient `declare const enum` (TS2748). Keep it on in every project that is not emitted by `tsc` alone.

##### Module detection and `.mts`/`.cts`

`moduleDetection: "auto"` (default) treats a file as a module if it has `import`/`export`, or if it is `.mjs`/`.mts`/`.cjs`/`.cts`, or if `"type": "module"` applies under `node16`/`nodenext`, or if `jsx` is `react-jsx`. `moduleDetection: "force"` treats every non-declaration file as a module — recommended so stray script files do not leak globals.

***

#### Types: Fundamentals

##### `any` vs `unknown`

`any` turns off checking in both directions and spreads through expressions; `unknown` forces narrowing before use. Untrusted input — `JSON.parse`, `res.json()`, `catch` variables, message events, `process.env` values parsed into structures — must be `unknown` and validated.

```ts
// BAD: any leaks; the typo compiles and returns undefined at run time
function userNameBad(raw: string): string {
  const data = JSON.parse(raw);        // any
  return data.usre.name;               // no error
}

// GOOD: unknown + a guard (or a schema library such as zod/valibot)
interface User { name: string; age: number }

function isUser(v: unknown): v is User {
  return (
    typeof v === "object" && v !== null &&
    "name" in v && typeof v.name === "string" &&   // `in` narrows unlisted properties (4.9+)
    "age" in v && typeof v.age === "number"
  );
}

function userName(raw: string): string {
  const data: unknown = JSON.parse(raw);
  if (!isUser(data)) throw new TypeError("invalid user payload");
  return data.name;
}

export { userNameBad, userName };
```

Review rules:

- Explicit `any` in new code needs a justification comment; prefer `unknown`, a generic, or a precise type. `@typescript-eslint/no-explicit-any` and the `no-unsafe-*` rules catch the leaks.
- `as SomeType` on parsed data is not validation. A type predicate whose body does not check what it claims (`return true`) is a lie the compiler trusts.
- `Function`, `Object` and `{}` are almost as loose as `any` for their purposes: `{}` accepts every non-nullish value including `0` and `""`; use `object` for "non-primitive" and `Record<string, unknown>` for dictionaries.

##### Type assertions and non-null assertions

```ts
declare const el: HTMLElement | null;

// BAD: the ! hides a real null case
const widthBad = el!.offsetWidth;

// GOOD: handle the null case
const width = el ? el.offsetWidth : 0;

// Double assertion through unknown is a red flag — it defeats checking entirely
const n = ("42" as unknown) as number;   // compiles; n is a string at run time

export { widthBad, width, n };
```

- `!` is acceptable only where an invariant is established nearby (e.g. right after a `Map.has` check — TypeScript does not correlate `has` and `get`).
- `as const` is not an assertion in the dangerous sense — it narrows literals and makes them readonly.
- Prefer `satisfies` (4.9) over `as` when you want to check a literal against a type without widening.

##### `interface` vs `type`

Both describe object shapes. Use `interface` for object types that are extended or implemented (better error messages, cached relations, declaration merging), `type` for unions, tuples, mapped and conditional types. Declaration merging is a hazard for accidental duplicates: two `interface Config` declarations in the same scope merge silently.

##### Optional properties and `exactOptionalPropertyTypes`

```ts
// tsconfig: {"exactOptionalPropertyTypes": true}
interface Options {
  timeoutMs?: number;            // may be absent, but not explicitly undefined
  label?: string | undefined;    // may be absent or undefined
}

// @ts-expect-error TS2375 'undefined' is not assignable to 'number' under exactOptionalPropertyTypes
const a: Options = { timeoutMs: undefined };
const b: Options = { label: undefined };
const c: Options = {};

// `in` and `hasOwn` distinguish "absent" from "present but undefined"
function hasTimeout(o: Options) {
  return "timeoutMs" in o;
}

export { a, b, c, hasTimeout };
```

With the flag, `{ ...defaults, ...overrides }` cannot overwrite a value with `undefined` by accident, which is a frequent production bug (a partial update that clears a field). Without it, `timeoutMs?: number` accepts `undefined`.

##### Index access and `noUncheckedIndexedAccess`

```ts
// tsconfig: {"noUncheckedIndexedAccess": true}
const counts: Record<string, number> = {};
const xs = [1, 2, 3];

// @ts-expect-error TS18048 'counts.missing' is possibly 'undefined'
counts.missing.toFixed();

const first = xs[0];                 // number | undefined
if (first !== undefined) first.toFixed();

for (const x of xs) x.toFixed();     // iteration values are not affected

const [head] = xs;                   // number | undefined (destructuring is indexed access)
export { head };
```

Without the flag, `xs[10]` is typed `number` — a classic source of `undefined is not a function` at run time. Enable it in new projects; in old ones, expect a large but mechanical diff.

##### Readonly data

```ts
interface Config { readonly hosts: readonly string[] }

function addHostBad(c: Config) {
  // @ts-expect-error TS2339 Property 'push' does not exist on type 'readonly string[]'
  c.hosts.push("x");
}

function addHost(c: Config, h: string): Config {
  return { ...c, hosts: [...c.hosts, h] };
}

// readonly is shallow and compile-time only:
const frozen = Object.freeze({ nested: { n: 1 } });
frozen.nested.n = 2;                 // compiles and runs: freeze is shallow too

export { addHostBad, addHost, frozen };
```

`readonly T[]` is assignable **from** `T[]`, so a function taking `readonly T[]` accepts mutable arrays; the reverse is an error. A `readonly` property in an interface does not prevent mutation through another alias that lacks the modifier.

##### Excess property checks and structural typing

Object literals assigned directly to a typed target get excess-property checks; the same object passed through a variable does not. (In 5.1 the excess-property error is reported as TS2322 with the elaboration "Object literal may only specify known properties"; 5.3+ reports TS2353 (verified: 5.2.2 TS2322, 5.3.3 TS2353) — do not treat the code difference as a different problem.)

```ts
interface Point { x: number; y: number }

// @ts-expect-error TS2322 ... Object literal may only specify known properties, and 'z' does not exist in type 'Point'
const p1: Point = { x: 1, y: 2, z: 3 };

const raw = { x: 1, y: 2, z: 3 };
const p2: Point = raw;               // OK — structural typing; z is still there at run time

// Danger: serialising p2 leaks z (think: password fields in a "public" view type)
const json = JSON.stringify(p2);     // {"x":1,"y":2,"z":3}

export { p1, p2, json };
```

To strip fields, copy explicitly (`const { password, ...pub } = user`) or validate with a schema that strips unknown keys.

##### Branded (nominal) types

```ts
type UserId = string & { readonly __brand: "UserId" };
type OrderId = string & { readonly __brand: "OrderId" };

function asUserId(s: string): UserId {
  if (!/^u_[a-z0-9]+$/.test(s)) throw new Error("bad user id");
  return s as UserId;
}

declare function loadUser(id: UserId): void;
declare const order: OrderId;

loadUser(asUserId("u_1"));
// @ts-expect-error TS2345 OrderId is not assignable to UserId
loadUser(order);
// @ts-expect-error TS2345 a plain string is not a UserId
loadUser("u_2");
```

##### Functions

```ts
// Overloads: implementation signature is not callable from outside
function parse(input: string): number;
function parse(input: string[]): number[];
function parse(input: string | string[]): number | number[] {
  return Array.isArray(input) ? input.map(Number) : Number(input);
}

// `this` parameters
function onClick(this: HTMLButtonElement, _ev: MouseEvent) {
  this.disabled = true;
}

// Optional vs default parameters
function retry(fn: () => Promise<void>, times = 3): Promise<void> {
  return fn().catch((e: unknown) => (times > 1 ? retry(fn, times - 1) : Promise.reject(e)));
}

// Callbacks returning void accept functions that return values (by design)
const pushed: number[] = [];
[1, 2].forEach((n) => pushed.push(n));   // push returns number; allowed for a void callback

export { parse, onClick, retry };
```

- `strictFunctionTypes` checks function-typed **properties** contravariantly but **methods** bivariantly. `interface Handler { handle(e: Event): void }` accepts an implementation taking `MouseEvent` — declare callbacks as properties (`handle: (e: Event) => void`) when soundness matters.
- `strictBindCallApply` types `fn.call/apply/bind` arguments; with it off they accept anything.
- Prefer union parameters over overloads when the return type does not depend on the argument type.

***

#### Narrowing and Control-Flow Analysis

##### Discriminated unions and exhaustiveness

```ts
type Shape =
  | { kind: "circle"; r: number }
  | { kind: "square"; side: number }
  | { kind: "rect"; w: number; h: number };

function assertNever(x: never, msg = "unexpected value"): never {
  throw new Error(`${msg}: ${JSON.stringify(x)}`);
}

function area(s: Shape): number {
  switch (s.kind) {
    case "circle": return Math.PI * s.r ** 2;
    case "square": return s.side ** 2;
    case "rect": return s.w * s.h;
    default: return assertNever(s);   // adding a new kind makes this a compile error
  }
}

// Destructured discriminants narrow too (4.6+), as long as the bindings are not reassigned
type Action = { kind: "n"; payload: number } | { kind: "s"; payload: string };
function run({ kind, payload }: Action) {
  if (kind === "n") return payload.toFixed();
  return payload.toUpperCase();
}
export { area, run };
```

`--noFallthroughCasesInSwitch` catches missing `break`; the `assertNever` default catches missing cases even when the function's return type would allow `undefined`. ESLint's `@typescript-eslint/switch-exhaustiveness-check` is an alternative.

##### Built-in narrowing

`typeof`, `instanceof`, `in`, `Array.isArray`, equality (`===`, `!=` null), truthiness, assignment, and type predicates all narrow. Pitfalls:

```ts
function len(x: string | null | undefined) {
  if (!x) return 0;                  // also catches "" — fine here, wrong for numbers (0)
  return x.length;
}

function price(p: number | undefined) {
  // BAD: 0 is a valid price but falsy
  // return p || 9.99;
  return p ?? 9.99;                  // GOOD: only null/undefined fall back
}

function isArrayOfStrings(v: unknown): v is string[] {
  return Array.isArray(v) && v.every((e) => typeof e === "string");
}

// typeof null === "object"
function keys(v: unknown) {
  if (typeof v === "object" && v !== null) return Object.keys(v);
  return [];
}

export { len, price, isArrayOfStrings, keys };
```

##### What 5.1 does **not** narrow (later releases do)

These are frequent sources of unnecessary casts in 5.1 code — the casts are correct for 5.1, do not ask to remove them:

```ts
// 1. Narrowing is lost inside closures created after the last assignment (fixed in 5.4)
function fmt(x: string | number) {
  if (typeof x === "string") x = Number(x);
  // @ts-expect-error TS2339 Property 'toFixed' does not exist on type 'string | number'
  return () => x.toFixed(2);
}
// 5.1 fix: copy to a const after narrowing
function fmtOk(x: string | number) {
  const n = typeof x === "string" ? Number(x) : x;
  return () => n.toFixed(2);
}

// 2. filter() does not infer a type predicate (fixed in 5.5)
const raw = ["a", undefined, "b"];
// @ts-expect-error TS2322 (string | undefined)[] is not assignable to string[]
const bad: string[] = raw.filter((s) => s !== undefined);
const good: string[] = raw.filter((s): s is string => s !== undefined);

// 3. switch (true) does not narrow (fixed in 5.3)
function describe(v: unknown) {
  switch (true) {
    // @ts-expect-error TS18046 'v' is of type 'unknown'
    case typeof v === "string": return v.toUpperCase();
    default: return "";
  }
}

// 4. obj[key] with a const key is not narrowed (fixed in 5.5)
function get(o: Record<string, string | undefined>, key: string) {
  if (o[key] !== undefined) {
    // @ts-expect-error TS2532 Object is possibly 'undefined'
    return o[key].length;
  }
  return 0;
}
function getOk(o: Record<string, string | undefined>, key: string) {
  const v = o[key];
  return v !== undefined ? v.length : 0;
}

export { fmt, fmtOk, bad, good, describe, get, getOk };
```

A `.filter(Boolean)` call also never narrows in 5.1 (nor in later versions) — `xs.filter(Boolean)` on `(T | undefined)[]` stays `(T | undefined)[]`.

##### User-defined type guards and assertion functions

```ts
function assert(cond: unknown, msg: string): asserts cond {
  if (!cond) throw new Error(msg);
}

function assertIsString(v: unknown): asserts v is string {
  if (typeof v !== "string") throw new TypeError("expected string");
}

function handle(input: unknown) {
  assertIsString(input);
  return input.toUpperCase();
}

// Assertion functions must be called through an explicitly typed reference:
const check: (v: unknown) => asserts v is string = assertIsString;
function handle2(input: unknown) {
  check(input);
  return input.length;
}

export { assert, handle, handle2 };
```

An assertion signature on an arrow function assigned to an un-annotated `const` gives TS2775 "Assertions require every name in the call target to be declared with an explicit type annotation".

##### `unknown` in `catch`

Under `strict` (`useUnknownInCatchVariables`, 4.4+) `catch (e)` is `unknown`. Narrow before use; do not annotate `catch (e: any)`.

```ts
function message(e: unknown): string {
  if (e instanceof Error) return e.message;
  if (typeof e === "string") return e;
  return JSON.stringify(e);
}
try {
  JSON.parse("{");
} catch (e) {
  console.error(message(e));
}
export { message };
```

***

#### Generics, Inference, `satisfies` and `const` Type Parameters

##### Constraints, defaults and when not to be generic

```ts
// GOOD: the type parameter links input and output
function firstOf<T>(xs: readonly T[]): T | undefined {
  return xs[0];
}

// BAD: T appears once — it is just a disguised `any`-ish cast for the caller
function parseBad<T>(json: string): T {
  return JSON.parse(json);              // unchecked; caller picks T freely
}
// GOOD: return unknown and let the caller validate
function parseJson(json: string): unknown {
  return JSON.parse(json);
}

// Constraints and keyof
function pluck<T, K extends keyof T>(obj: T, key: K): T[K] {
  return obj[key];
}

// Defaults
interface Page<T = unknown> { items: T[]; next?: string }

export { firstOf, parseBad, parseJson, pluck };
export type { Page };
```

##### `satisfies` (4.9)

`satisfies` checks an expression against a type but keeps the expression's own (narrower) type:

```ts
type Route = { path: string; auth: boolean };

const routes = {
  home: { path: "/", auth: false },
  admin: { path: "/admin", auth: true },
} satisfies Record<string, Route>;

routes.admin.path;                         // keys are known: "home" | "admin"
// @ts-expect-error TS2339 Property 'missing' does not exist
routes.missing;

const widened: Record<string, Route> = routes;   // still assignable where needed

// Catches typos in literal keys:
const bad = {
  home: { path: "/", auth: false },
  // @ts-expect-error TS2322 ... Object literal may only specify known properties, and 'auht' does not exist in type 'Route'
  admin: { path: "/admin", auht: true },
} satisfies Record<string, Route>;

export { widened, bad };
```

Combine with `as const` for deeply literal, readonly config: `{ … } as const satisfies Config`.

##### `const` type parameters (5.0)

See the 5.0 Foundation section: use `const T extends readonly unknown[]` (readonly constraint), and remember that `const` only affects literals written directly in the call.

##### Emulating `NoInfer` (the utility type is 5.4)

```ts
// Without NoInfer, T is inferred from both arguments:
function pickDefault<T extends string>(options: T[], fallback: T): T {
  return options.includes(fallback) ? fallback : options[0]!;
}
pickDefault(["a", "b"], "c");             // compiles: T = "a" | "b" | "c"

// 5.1-compatible workaround: block inference through a deferred conditional type
type NoInferCompat<T> = [T][T extends any ? 0 : never];
function pickDefaultStrict<T extends string>(options: T[], fallback: NoInferCompat<T>): T {
  return options.includes(fallback) ? fallback : options[0]!;
}
// @ts-expect-error TS2345 Argument of type '"c"' is not assignable to parameter of type '"a" | "b"'
pickDefaultStrict(["a", "b"], "c");

export { pickDefault, pickDefaultStrict };
```

##### Variance annotations and `infer … extends`

`in`/`out` on type parameters (4.7) document and enforce variance; use them only on hot generic types where measured check time matters, and never to silence errors (a wrong annotation makes the type unsound). `infer U extends string` (4.8) constrains an inferred type inline in conditional types.

```ts
interface Producer<out T> { get(): T }
interface Consumer<in T> { accept(v: T): void }

type FirstString<T> = T extends [infer S extends string, ...unknown[]] ? S : never;
type A = FirstString<["x", 1]>;            // "x"
type B = FirstString<[1, "x"]>;            // never

export type { Producer, Consumer, A, B };
```

***

#### Advanced Types: Conditional, Mapped, Template Literal

##### Utility types available in 5.1

`Partial`, `Required`, `Readonly`, `Record`, `Pick`, `Omit`, `Exclude`, `Extract`, `NonNullable`, `Parameters`, `ConstructorParameters`, `ReturnType`, `InstanceType`, `ThisParameterType`, `OmitThisParameter`, `ThisType`, `Awaited` (4.5), `Uppercase`, `Lowercase`, `Capitalize`, `Uncapitalize`. **Not** in 5.1: `NoInfer` (5.4).

##### `Omit`/`Pick` on unions are not distributive

```ts
type UiEvent =   // not `Event`: in a non-module file with lib "dom" that name collides with the DOM's global Event (TS2300)
  | { type: "click"; x: number; at: Date }
  | { type: "key"; key: string; at: Date };

type NoAtBad = Omit<UiEvent, "at">;              // { type: "click" | "key" } — x and key are gone
type DistributiveOmit<T, K extends PropertyKey> = T extends unknown ? Omit<T, K> : never;
type NoAt = DistributiveOmit<UiEvent, "at">;     // keeps the union

declare const e: NoAt;
if (e.type === "click") e.x.toFixed();
declare const eb: NoAtBad;
// @ts-expect-error TS2339 Property 'x' does not exist on type 'NoAtBad'
eb.x;

export type { NoAt };
```

`Omit<T, K>` also does not check that `K` is a key of `T` (`K extends keyof any`) — a typo in `K` silently omits nothing.

##### Conditional types and `infer`

```ts
type ElementOf<T> = T extends readonly (infer E)[] ? E : never;
type Unwrap<T> = T extends Promise<infer U> ? Unwrap<U> : T;

// Distribution happens over naked type parameters; wrap in [] to stop it
type IsString<T> = T extends string ? true : false;
type D = IsString<string | number>;            // boolean (distributed)
type IsStringStrict<T> = [T] extends [string] ? true : false;
type S = IsStringStrict<string | number>;      // false

export type { ElementOf, Unwrap, D, S };
```

##### Mapped types and key remapping

```ts
type Getters<T> = {
  [K in keyof T as `get${Capitalize<string & K>}`]: () => T[K];
};
type PersonGetters = Getters<{ name: string; age: number }>;
// { getName: () => string; getAge: () => number }

type Mutable<T> = { -readonly [K in keyof T]: T[K] };
type WithoutFunctions<T> = { [K in keyof T as T[K] extends Function ? never : K]: T[K] };

export type { PersonGetters, Mutable, WithoutFunctions };
```

##### Template literal types

```ts
type HttpMethod = "GET" | "POST";
type Endpoint = `${HttpMethod} /api/${string}`;
const ok: Endpoint = "GET /api/users";
// @ts-expect-error TS2322 not a valid Endpoint
const bad: Endpoint = "PATCH /api/users";

type ParamNames<P extends string> =
  P extends `${string}:${infer Name}/${infer Rest}` ? Name | ParamNames<`/${Rest}`> :
  P extends `${string}:${infer Name}` ? Name : never;
type Params = ParamNames<"/users/:userId/posts/:postId">;   // "userId" | "postId"

export { ok, bad };
export type { Params };
```

Deeply recursive template-literal and conditional types are the most common cause of slow type-checking and of TS2589 "Type instantiation is excessively deep and possibly infinite". Keep recursion shallow and cap inputs.

##### `keyof`, `typeof`, indexed access, `unique symbol`

```ts
const STATUS = { active: 1, banned: 2 } as const;
type StatusName = keyof typeof STATUS;              // "active" | "banned"
type StatusCode = (typeof STATUS)[StatusName];      // 1 | 2

// Object.keys returns string[] by design (objects may have extra keys at run time)
function names(o: typeof STATUS): StatusName[] {
  return Object.keys(o) as StatusName[];            // OK only for objects you created
}

const tokenKey: unique symbol = Symbol("token");
interface WithToken { [tokenKey]: string }

export { names, tokenKey };
export type { StatusCode, WithToken };
```

***

#### Classes and Class Fields

##### `useDefineForClassFields` and `target`

With `target` ≥ `es2022` (or `esnext`), `useDefineForClassFields` defaults to `true`: class fields are emitted as native ECMAScript fields with `[[Define]]` semantics. With older targets it defaults to `false` and fields are emitted as constructor assignments. The difference changes behaviour:

```ts
// tsconfig: {"target": "es2022"}
class Base {
  value?: string;
  constructor() { this.value = "from base"; }
}

class ChildBad extends Base {
  // @ts-expect-error TS2612 Property 'value' will overwrite the base property in 'Base'. ... add a 'declare' modifier ...
  value?: string;          // re-declared field is re-initialised to undefined AFTER Base's constructor
}

class ChildGood extends Base {
  declare value?: string;  // type-only redeclaration: no emit, keeps "from base"
}

export { ChildBad, ChildGood };
```

Verified by running the emitted code on Node 18: with `target: es2022`, `new ChildBad().value` is `undefined` (re-defined by the subclass field), with `target: es2020` it is `"from base"` — the same source behaves differently depending on `target`. Changing `target` from `es2020` to `es2022` is therefore a **behavioural** change, not just a syntax upgrade. 5.1 reports TS2612 for the redeclaration only when define semantics are active (no error with `target: es2020`). Libraries that rely on class fields being plain assignments that do not shadow prototype accessors (for example Lit reactive properties declared as fields) break under define semantics; their documentation prescribes `useDefineForClassFields: false`, `declare` or `accessor`.

##### Parameter properties read in field initializers — a bug 5.1 does not report under `es2022`

```ts
// tsconfig: {"target": "es2022"}
class Price {
  constructor(public net: number) {}
  gross = this.net * 1.19;   // NaN at run time with target es2022/esnext!
}
export { Price };
```

With define semantics, field initializers run **before** the constructor body assigns parameter properties, so `gross` is computed from `undefined` (`new Price(100).gross` is `NaN` on Node 20 with `target: es2022`; it is `119` with `target: es2020`). 5.1.6 reports TS2729 "Property 'net' is used before its initialization" only when `target` is `esnext`, **not** for `es2022` (even with an explicit `useDefineForClassFields: true`); 5.2.2 and later report it for `es2022` too (TypeScript issue #53286, fixed by PR #53885). Reviewers of 5.1 code must catch this by eye: any field initializer that reads `this.<parameterProperty>` is a bug under `target: es2022`. Fix by assigning in the constructor body:

```ts
class Price {
  readonly gross: number;
  constructor(public net: number) {
    this.gross = net * 1.19;
  }
}
export { Price };
```

##### Other class rules

```ts
abstract class Repo<T extends { id: string }> {
  protected readonly items = new Map<string, T>();
  abstract validate(item: T): void;
  save(item: T): void {
    this.validate(item);
    this.items.set(item.id, item);
  }
}

class UserRepo extends Repo<{ id: string; email: string }> {
  // `override` is required on overriding members with --noImplicitOverride (TS4114 otherwise)
  override validate(u: { id: string; email: string }): void {
    if (!u.email.includes("@")) throw new Error("invalid email");
  }
}

class Session {
  #token: string;                         // ECMAScript private: enforced at run time
  private legacy = 1;                     // TypeScript private: compile-time only, visible via (obj as any)
  static #count = 0;
  static { Session.#count = 0; }          // static blocks (4.4; Node 16.11+ at run time)
  constructor(token: string) {
    this.#token = token;
    Session.#count++;
  }
  matches(t: string) { return t === this.#token; }
  static get count() { return Session.#count; }
}

class Temperature {
  accessor celsius = 0;                   // auto-accessor (4.9; standard decorators can wrap it)
}

export { UserRepo, Session, Temperature };
```

Review notes:

- **Calling overridable methods from a constructor** sees subclass fields uninitialised (`undefined`). Avoid virtual calls in constructors.
- **Arrow-function properties** (`handle = () => {…}`) are per-instance closures: they bind `this` but cost memory per instance and are not on the prototype (not mockable via prototype, not callable via `super`). Methods plus explicit `bind` or a `@bound` decorator are alternatives.
- `#private` fields make `JSON.stringify`, `structuredClone` and proxies behave differently from TypeScript `private`; brand checks (`#token in obj`) are available (4.5).
- `strictPropertyInitialization` requires initialisation in the constructor; `!` (definite assignment, `token!: string`) opts out and should be justified (dependency injection, `init()` lifecycle).
- Classes are structural: `class A { x = 1 }` and `class B { x = 1 }` are interchangeable unless a private/protected/`#` member makes them nominal.

***

#### Decorators: Standard (TC39) vs `experimentalDecorators`

##### Which system is in use

| | Standard decorators (5.0+) | `experimentalDecorators: true` |
|---|---|---|
| Spec | Stage 3 TC39 proposal | old Stage 1/2 TypeScript design |
| Signature | `(value, context) => replacement \| void`; a decorator declaring a different parameter count is rejected (TS1241 on a method, TS1238 on a class: "The runtime will invoke the decorator with 2 arguments, but the decorator expects 1") | `(target, propertyKey, descriptor?)` |
| Parameter decorators | **not supported** (TS1206 "Decorators are not valid here") | supported |
| `emitDecoratorMetadata` | **not supported** (TS5052 without `experimentalDecorators`) | supported (`design:type`, `design:paramtypes`) |
| Metadata | `context.metadata` / `Symbol.metadata` only from 5.2 | via `reflect-metadata` |
| Fields | can return an initializer transform | receive no descriptor |
| `accessor` | supported (`ClassAccessorDecoratorContext`) | not applicable |
| Placement | before or after `export`/`export default`, not both (TS8038) | 5.1 accepts both positions here too, also not both at once (TS8038) |
| Runtime | emitted helpers (`__esDecorate`, `__runInitializers`), runs on any ES2015+ runtime | `__decorate`, `__param`, `__metadata` |

Frameworks built on parameter decorators or `reflect-metadata` — NestJS, TypeORM, InversifyJS, tsyringe, class-validator/class-transformer, Angular's constructor injection — require `experimentalDecorators`. Mixing the two systems in one compilation is impossible: the flag switches the whole program. Do not "modernise" a NestJS project by removing `experimentalDecorators`.

##### Writing standard decorators in 5.1

```ts
function logged<This, Args extends unknown[], Return>(
  target: (this: This, ...args: Args) => Return,
  context: ClassMethodDecoratorContext<This, (this: This, ...args: Args) => Return>,
) {
  const name = String(context.name);
  return function (this: This, ...args: Args): Return {
    console.log(`enter ${name}`);
    try {
      return target.call(this, ...args);
    } finally {
      console.log(`exit ${name}`);
    }
  };
}

function bound<This, T extends (this: This, ...args: any[]) => any>(
  _target: T,
  context: ClassMethodDecoratorContext<This, T>,
) {
  if (context.private) throw new Error("bound: private methods are not supported");
  const key = context.name as keyof This;
  context.addInitializer(function (this: This) {
    (this as any)[key] = (this as any)[key].bind(this);
  });
}

function defaultTo<T>(value: T) {
  return function (_t: undefined, _ctx: ClassFieldDecoratorContext<unknown, T | undefined>) {
    return (initial: T | undefined) => initial ?? value;   // field initializer transform
  };
}

function positive(
  target: ClassAccessorDecoratorTarget<unknown, number>,
  _ctx: ClassAccessorDecoratorContext<unknown, number>,
): ClassAccessorDecoratorResult<unknown, number> {
  return {
    set(v) {
      if (v <= 0) throw new RangeError("must be > 0");
      target.set.call(this, v);
    },
  };
}

const registry: string[] = [];
function register<C extends abstract new (...a: any[]) => unknown>(_target: C, ctx: ClassDecoratorContext<C>) {
  ctx.addInitializer(() => registry.push(String(ctx.name)));
}

@register
class Person {
  @defaultTo(3) retries: number | undefined;
  @positive accessor age = 1;
  constructor(public name?: string) {}
  @bound @logged greet() { return `hi ${this.name ?? "anonymous"}`; }
}

const greet = new Person("ray").greet;   // detached method still has `this` (bound)
console.log(greet());                    // enter greet / exit greet / hi ray
const p = new Person();
p.age = 5;
try { p.age = -1; } catch (e) { console.log((e as Error).message); }   // must be > 0
console.log(registry, p.retries);        // [ 'Person' ] 3

export { Person };
```

Compiled with 5.1.6 for `target: es2022` (run on Node 20) and `target: es2015` (run on Node 14): same output.

Review notes for standard decorators:

- Decorators apply bottom-up: in `@bound @logged greet()`, `logged` wraps first, `bound` binds the wrapped method.
- Method decorator `addInitializer` callbacks run at the start of construction, before field initializers; class decorator initializers run after the class is fully defined.
- `context.name` is a `string | symbol`; `String(...)` before interpolation (a `symbol` in a template literal throws `TypeError`).
- `context.private`/`context.static` must be checked before using `this[context.name]` — private names are not reachable that way; use `context.access.get/set`.
- There is no `context.metadata` in 5.1 (TS2339); metadata-based libraries need 5.2+.
- Field decorators receive `undefined` as the value; a returned function transforms the initial value. With `importHelpers`, tslib ≥ 2.5.1 is required for correct initializer ordering (see What's New).
- Type the decorator generically (`This`, `Args`, `Return`) — an `any`-typed decorator silently changes the decorated method's type to `any`.

##### Legacy decorators: `emitDecoratorMetadata` pitfalls

```ts
// tsconfig: {"experimentalDecorators": true, "emitDecoratorMetadata": true}
function Injectable(): ClassDecorator { return () => {}; }
function Inject(_token: string): ParameterDecorator { return () => {}; }

class Db { query() {} }

@Injectable()
class Repo {
  constructor(private readonly db: Db, @Inject("cfg") private readonly cfg: object) {}
}
export { Repo };
```

- Emitted metadata is `__metadata("design:paramtypes", [Db, Object])` (verified). Interfaces, type aliases and unions become `Object`; DI containers then cannot resolve them — use injection tokens.
- **`import type` breaks DI silently.** If `Db` comes from another module via `import type { Db } from "./db"`, 5.1 emits `design:paramtypes` as `[Function]` — no compile error, but the container injects the wrong thing or fails at run time. Enabling `@typescript-eslint/consistent-type-imports` or `verbatimModuleSyntax` auto-fixes can introduce exactly this regression in NestJS/Angular/TypeORM code. Keep value imports for classes used in decorated constructor signatures.
- Circular imports between decorated classes produce `undefined` in metadata at run time (`design:type` evaluated before the class exists); use `forwardRef`-style APIs.
- `reflect-metadata` must be imported once, before any decorated class is evaluated.

***

#### Enums, Namespaces and Other Emitting Syntax

##### Prefer union types and `as const` objects

```ts
const Role = { Admin: "admin", User: "user" } as const;
type Role = (typeof Role)[keyof typeof Role];   // "admin" | "user"

function can(role: Role) { return role === Role.Admin; }
can("user");                                    // plain strings work — friendly for JSON input

export { Role, can };
```

Union types erase completely, interoperate with JSON and with type-stripping runtimes, and do not have enum quirks.

##### If enums are used

```ts
enum Status { Active = "active", Banned = "banned" }     // string enums: no reverse mapping
enum Level { Low, Mid, High }                            // numeric: reverse mapping Level[0] === "Low"

// Numeric enums accept any number from arithmetic results:
declare const n: number;
const l: Level = n;                                      // compiles — no range check for non-literals

// String enums are nominal: a plain string is not assignable
// @ts-expect-error TS2322 Type '"active"' is not assignable to type 'Status'
const s: Status = "active";

// Iterating a numeric enum includes the reverse-mapping keys:
const keys = Object.keys(Level);                         // ["0","1","2","Low","Mid","High"]

export { l, s, keys };
```

- `const enum` is inlined by `tsc` but cannot be compiled by single-file transpilers (esbuild, swc, Babel, Node type stripping) when it comes from another file. Under `isolatedModules`, using an ambient `declare const enum` is an error (TS2748); prefer regular enums or unions. Never publish `const enum` in a library's `.d.ts`.
- Since 5.0 every enum is a union enum (see the 5.0 Foundation section).

##### Namespaces, parameter properties and type-stripping runtimes

`namespace X { export const … }`, `enum`, parameter properties (`constructor(public x)`), `import x = require()` and legacy decorators all emit JavaScript. Node.js 22.18+/23.6+ run `.ts` files directly by **stripping** types; they reject such syntax: Node 24.22.1 and 22.23.4 fail on an `enum` with `SyntaxError [ERR_UNSUPPORTED_TYPESCRIPT_SYNTAX]: TypeScript enum is not supported in strip-only mode`, and likewise on parameter properties (verified). TypeScript 5.1 has no flag to forbid this syntax (`--erasableSyntaxOnly` is 5.8) — if a 5.1 project runs its sources with Node's type stripping, review for these constructs by hand (or with a lint rule), and note that stripped imports must name real files (`./x.ts`), which 5.1 type-checks only with `allowImportingTsExtensions` + `noEmit`.

Use `namespace` only for declaration merging in `.d.ts` files (augmenting a function or class with static members). For code organisation, use modules. The `module Foo {}` keyword spelling for namespaces is legacy (an error in 6.0); write `namespace`.

***

#### Library Typings and Runtime Targets

##### What `lib` values exist in 5.1

Valid `target`s: `es3` (deprecated), `es5`, `es6`/`es2015` … `es2022`, `esnext`. Valid `lib` years: `es5` … `es2023`, `esnext`. In 5.1:

- `lib.es2023` adds only `Array.prototype.findLast` / `findLastIndex` (and on typed arrays). `toSorted`, `toReversed`, `toSpliced`, `with` and symbol-keyed `WeakMap`s arrive in 5.2's lib.
- `lib.es2022` covers `Array.prototype.at`, `Object.hasOwn`, `Error` `cause`, `String.prototype.replaceAll` (from es2021), RegExp match indices (`/d`), class fields/static blocks (syntax), top-level `await` (syntax).
- `esnext` in 5.1 = `es2023` + `esnext.intl`; nothing like `Disposable`, `Promise.withResolvers` or `Object.groupBy` exists.
- `decorators` and `decorators.legacy` libs contain the decorator context types (included automatically).

```ts
// tsconfig: {"lib": ["es2023"]}
const xs = [3, 1, 2];
const last = xs.findLast((x) => x > 1);          // OK in 5.1 (es2023)
const sorted = [...xs].sort((a, b) => a - b);    // 5.1-compatible copy-then-sort
// @ts-expect-error TS2339 Property 'toSorted' does not exist on type 'number[]' (typed from 5.2)
const s2 = xs.toSorted();
export { last, sorted, s2 };
```

If the runtime (Node 20+, evergreen browsers) has `toSorted` and the team wants to use it on 5.1, add a minimal augmentation instead of casting:

```ts
declare global {
  interface Array<T> {
    toSorted(compareFn?: (a: T, b: T) => number): T[];
  }
  interface ReadonlyArray<T> {
    toSorted(compareFn?: (a: T, b: T) => number): T[];
  }
}
export const sortedCopy = [3, 1, 2].toSorted((a, b) => a - b);
```

Delete such augmentations when upgrading to 5.2+, where they become redundant extra overloads (5.2.2 accepts them without error, so nothing forces the cleanup).

##### `target` vs runtime

`target` controls syntax downlevelling only; `lib` controls which APIs type-check. Neither polyfills. Common mistakes:

- `lib: ["esnext"]` with a Node 16/18 runtime: code type-checks against APIs the runtime lacks (`Array.prototype.findLast` needs Node 18+).
- `target: es5` for Node: emits `var` and helper-heavy code, and needs `downlevelIteration` for correct spread of iterables.
- `lib` without `"dom"` in a server project correctly hides `window`/`document`/`localStorage`; including `dom` in a Node project makes those browser-only globals type-check although they throw `ReferenceError` at run time. (`setTimeout` still resolves to the `@types/node` overload returning `NodeJS.Timeout` when both are loaded — verified.)

##### `@types/node` for a 5.1 project

As described in Version-Aware Review: install from the `ts5.1` tag (`@types/node@24.1.0`). Match the **major** to the runtime where possible — `@types/node@24` declares APIs that Node 20 lacks (`fs.glob` from Node 22.0, the `node:sqlite` module from Node 22.5, …; `process.getBuiltinModule` is *not* such a case — it was backported to Node 20.16), so on a Node 20 runtime prefer the newest `@types/node@20.x` that still compiles with 5.1, and verify with `tsc`.

***

#### Error Handling

##### `catch` variables are `unknown`

See Narrowing. `catch (e: unknown)` may be written explicitly; `catch (e: any)` and `catch (e: Error)` (TS1196 — only `any`/`unknown` annotations are allowed) are review findings.

##### Errors as values for expected failures

```ts
type Result<T, E = Error> = { ok: true; value: T } | { ok: false; error: E };

function parsePort(s: string): Result<number, string> {
  const n = Number(s);
  if (!Number.isInteger(n) || n < 1 || n > 65535) return { ok: false, error: `invalid port: ${s}` };
  return { ok: true, value: n };
}

const r = parsePort("8080");
if (r.ok) console.log(r.value);
else console.error(r.error);

export { parsePort };
```

##### Custom error classes and `cause`

```ts
class HttpError extends Error {
  override readonly name = "HttpError";
  constructor(readonly status: number, message: string, options?: { cause?: unknown }) {
    super(message, options);          // ErrorOptions.cause: lib es2022
  }
}

async function getJson(url: string): Promise<unknown> {
  let res: Response;
  try {
    res = await fetch(url);
  } catch (e) {
    throw new HttpError(0, `network error for ${url}`, { cause: e });  // keep the original error
  }
  if (!res.ok) throw new HttpError(res.status, `GET ${url} failed`);
  return res.json();                  // typed Promise<any> — returned as unknown
}

export { HttpError, getJson };
```

- With `target` ≥ ES2015, `instanceof HttpError` works; with `target: es5`, subclasses of `Error` lose their prototype (`Object.setPrototypeOf(this, new.target.prototype)` needed) — another reason not to target ES5.
- Never swallow errors (`catch {}`) without a comment; never `throw "string"`.
- Log `error.cause` chains; Node's `util.inspect` prints them.

***

#### Async Code and Promises

##### Floating promises

The compiler does not report un-awaited promises. Use `@typescript-eslint/no-floating-promises` and `no-misused-promises`.

```ts
declare function save(): Promise<void>;

async function handlerBad() {
  save();                    // BAD: rejection becomes an unhandledRejection — the process exits on Node 15+
}

async function handler() {
  await save();              // GOOD
  void save().catch((e: unknown) => console.error(e));   // GOOD: deliberate fire-and-forget
}

// BAD: async callback where the caller ignores the returned promise
[1, 2, 3].forEach(async (id) => { await save(); console.log(id); });   // not awaited, unordered

// GOOD
async function sequential(ids: number[]) {
  for (const id of ids) {
    await save();
    console.log(id);
  }
}

export { handlerBad, handler, sequential };
```

##### Concurrency patterns

```ts
declare function fetchUser(id: string): Promise<{ id: string }>;

// Parallel, fail-fast
async function loadAll(ids: string[]) {
  return Promise.all(ids.map(fetchUser));
}

// Parallel, collect failures
async function loadSettled(ids: string[]) {
  const results = await Promise.allSettled(ids.map(fetchUser));
  const ok = results.flatMap((r) => (r.status === "fulfilled" ? [r.value] : []));
  const failed = results.filter((r): r is PromiseRejectedResult => r.status === "rejected");
  return { ok, failed };
}

// Timeouts and cancellation (AbortSignal.timeout: Node 17.3+, modern browsers)
async function getWithTimeout(url: string): Promise<unknown> {
  const res = await fetch(url, { signal: AbortSignal.timeout(5_000) });
  return (await res.json()) as unknown;
}

// Deferred without Promise.withResolvers (typed only from 5.4)
function deferred<T>() {
  let resolve!: (v: T) => void;
  let reject!: (e: unknown) => void;
  const promise = new Promise<T>((res, rej) => { resolve = res; reject = rej; });
  return { promise, resolve, reject };
}

export { loadAll, loadSettled, getWithTimeout, deferred };
```

- Unbounded `Promise.all(huge.map(...))` exhausts sockets/DB pools — use a concurrency limit.
- `res.json()` and `JSON.parse` return `any`; treat as `unknown` and validate.
- `Promise.race` with a timer leaks the timer unless it is cleared; prefer `AbortSignal.timeout`.
- `async` functions always return promises; `Promise<void>`-returning callbacks passed where `() => void` is expected lose their errors.

##### Async iteration and resource cleanup without `using`

`using`/`await using` do not exist in 5.1. Use `try`/`finally`:

```ts
import { open } from "node:fs/promises";

async function firstLine(path: string): Promise<string | undefined> {
  const fh = await open(path, "r");
  try {
    for await (const line of fh.readLines()) {
      return line;             // return inside for-await closes the iterator; finally closes the file
    }
    return undefined;
  } finally {
    await fh.close();
  }
}
export { firstLine };
```

Breaking out of `for await` calls the iterator's `return()`, which runs generator `finally` blocks. (`FileHandle.readLines()` needs Node 18.11+; the example was run on Node 20 and 24 — calling it twice in a row returns the first line both times, so the handle is closed and reopened cleanly.) Note that `@types/node` ≥ 24 declares `[Symbol.asyncDispose]` members, but 5.1 cannot use them with `await using`.

***

#### JSX and React

- `jsx: "react-jsx"` (React 17+ automatic runtime) needs no `import React`; `jsx: "react"` (classic) needs `React` in scope. `jsxImportSource` selects Preact/Solid/Emotion runtimes; a per-file `/** @jsxImportSource preact */` pragma overrides it.
- With 5.1 and `@types/react@19.1.9` (`ts5.1` tag), async components and components returning strings type-check as tags (`JSX.ElementType`, see New in 5.1). That does not make them valid in client components — only React Server Components render async components.
- Type children explicitly (`children?: React.ReactNode`); `React.FC` no longer adds `children` implicitly since `@types/react` 18.
- Event handlers: `(e: React.ChangeEvent<HTMLInputElement>) => …`; avoid `any`.
- `useState<User | null>(null)` — annotate when the initial value does not describe all states.
- `useRef<HTMLDivElement>(null)` yields a `RefObject<HTMLDivElement | null>` in `@types/react` 19.
- Spreading `{...props}` onto DOM elements forwards unknown props (and with namespaced names, `a:b`) to the DOM; filter them.
- `dangerouslySetInnerHTML` must only receive sanitised HTML (DOMPurify); TypeScript cannot check this.

```tsx
import { useState, type ChangeEvent, type ReactNode } from "react";

interface FieldProps {
  label: string;
  children?: ReactNode;
  onValue(value: string): void;
}

export function Field({ label, children, onValue }: FieldProps) {
  const [value, setValue] = useState("");
  const onChange = (e: ChangeEvent<HTMLInputElement>) => {
    setValue(e.target.value);
    onValue(e.target.value);
  };
  return (
    <label>
      {label}
      <input value={value} onChange={onChange} />
      {children}
    </label>
  );
}

export async function UserName({ id }: { id: string }) {   // async server component: OK as a tag on 5.1
  const res = await fetch(`https://api.example.com/users/${encodeURIComponent(id)}`);
  const data = (await res.json()) as { name?: unknown };
  return <span>{typeof data.name === "string" ? data.name : "unknown"}</span>;
}

export const page = <UserName id="42" />;
```

***

#### Declaration Files and Library Publishing

- Ship `.d.ts` generated by `tsc` (`declaration: true`); hand-written `.d.ts` drift from the implementation. 5.1 has no `isolatedDeclarations` (5.5); explicit return types on exported functions still speed up declaration emit and make API changes visible in review.
- `package.json` `exports` with a `types` condition **first** in each block; for dual ESM/CJS packages, separate `.d.mts`/`.d.cts` (or per-format directories) — one `.d.ts` cannot correctly describe both formats under `node16`/`nodenext` consumers ("masquerading as ESM/CJS"). Check with `@arethetypeswrong/cli`.
- `declare module "x"` augmentations must be inside a module file (with `import`/`export`) to augment rather than replace.
- `declare global { … }` in a library leaks into every consumer; avoid it unless the library's purpose is a global.
- Avoid `const enum`, `export =` in ESM packages, and global `namespace` declarations in published types.
- `typesVersions` can redirect older TypeScript consumers to downlevelled declarations (`"typesVersions": { "<5.0": { "*": ["ts4.x/*"] } }`).

```ts
// Module augmentation: adds a property to an existing interface of a dependency
import "node:http";
declare module "node:http" {
  interface IncomingMessage {
    requestId?: string;
  }
}

import { createServer } from "node:http";
export const srv = createServer((req, res) => {
  req.requestId = crypto.randomUUID();     // global crypto: Node 19+ runtime, typed by @types/node
  res.end(req.requestId);
});
```

***

#### JavaScript Checking (`checkJs`, JSDoc)

`allowJs` + `checkJs` (or `// @ts-check` per file) type-check JavaScript using JSDoc. Since 5.0, `@satisfies` and `@overload` work in JSDoc; verified on 5.1.6 (4.9.5 resolves both `@overload` calls to `number | number[]` and reports TS2322):

```js
// @ts-check
/** @typedef {{ path: string, auth: boolean }} Route */

/** @satisfies {Record<string, Route>} */
export const routes = {
  home: { path: "/", auth: false },
};

/**
 * @overload
 * @param {string} input
 * @returns {number}
 */
/**
 * @overload
 * @param {string[]} input
 * @returns {number[]}
 */
/**
 * @param {string | string[]} input
 */
export function parse(input) {
  return Array.isArray(input) ? input.map(Number) : Number(input);
}

/** @type {number} */
export const n = parse("1");
/** @type {number[]} */
export const ns = parse(["1"]);
```

- Import types in JSDoc with `/** @type {import("./b.js").Opts} */` or `@typedef {import("./b.js").Opts} Opts`; the `@import` tag is 5.5.
- 5.1 adds `@param` snippet completions in editors (see New in 5.1) — an editor convenience, not a checking change.
- `checkJs` with `strict` on a large legacy codebase produces many errors; enable it file by file with `// @ts-check`.

***

#### Security-Relevant Patterns

TypeScript types disappear at run time. Every security boundary needs runtime validation.

- **Untrusted input is `unknown`.** HTTP bodies, query strings, headers, `JSON.parse`, `postMessage` data, environment variables, files, database rows from shared schemas, LLM output. Validate with a schema library (zod, valibot, ajv with JSON Schema, io-ts) and derive types from the schema, not the other way round. `as T` or a generic `parse<T>()` is not validation.
- **Prototype pollution**: merging untrusted objects (`Object.assign(target, JSON.parse(body))`, recursive deep-merge) can write `__proto__`/`constructor.prototype`. Use `Object.create(null)` maps or `Map`, reject `__proto__`, `constructor`, `prototype` keys, and prefer validated, explicit copies.
- **`Record<string, T>` lookups with user keys** hit inherited properties (`obj["toString"]`); use `Map` or `Object.hasOwn(obj, key)` (es2022 lib) before reading. Under `noUncheckedIndexedAccess` the result is at least `T | undefined`.
- **Injection**: TypeScript cannot detect SQL/shell/HTML injection. Parameterised queries (`db.query("… WHERE id = $1", [id])`), `execFile`/`spawn` with argument arrays (never `exec` with interpolated strings), output encoding/templating that escapes by default, DOMPurify before `innerHTML`/`dangerouslySetInnerHTML`.
- **Path traversal**: `path.join(base, userInput)` does not stay inside `base`; resolve and check the prefix (`path.resolve(base, p).startsWith(path.resolve(base) + path.sep)`).
- **Secrets in types/logs**: excess-property checks do not strip fields (see Types); explicitly map DTOs before logging or returning them.
- **Regular expressions**: 5.1 does not check regex syntax (5.5 does); review new regexes for catastrophic backtracking on user input (ReDoS).
- **`eval`, `new Function`, `vm`** on any input derived from users are critical findings regardless of types.
- **Dependency typings**: `typeRoots`-based shims can make an uninstalled package look present (New in 5.1); a typo-squatted `@types/*` package executes nothing (type packages have no runtime code) but a typo-squatted runtime package does — check `package.json` diffs.
- **Crypto**: use `node:crypto` `randomUUID()`/`randomBytes()` or Web Crypto, never `Math.random()` for tokens; compare secrets with `timingSafeEqual`.

```ts
import { timingSafeEqual, randomBytes } from "node:crypto";
import path from "node:path";

export function newToken(): string {
  return randomBytes(32).toString("base64url");
}

export function safeEqual(a: string, b: string): boolean {
  const ab = Buffer.from(a);
  const bb = Buffer.from(b);
  return ab.length === bb.length && timingSafeEqual(ab, bb);
}

export function resolveInside(base: string, userPath: string): string {
  const root = path.resolve(base);
  const full = path.resolve(root, userPath);
  if (full !== root && !full.startsWith(root + path.sep)) throw new Error("path escapes base directory");
  return full;
}

const FORBIDDEN = new Set(["__proto__", "constructor", "prototype"]);
export function safeAssign(target: Record<string, unknown>, src: unknown): void {
  if (typeof src !== "object" || src === null) return;
  for (const [k, v] of Object.entries(src)) {
    if (!FORBIDDEN.has(k)) target[k] = v;
  }
}
```

***

#### Performance of the Compiler and of Emitted Code

##### Compiler performance

- Measure with `tsc --extendedDiagnostics` (files, instantiations, check time, memory) and `tsc --generateTrace <dir>` (open in Perfetto/`@typescript/analyze-trace`).
- `incremental: true` (`.tsbuildinfo`) and project references with `composite: true` + `tsc --build` cut rebuild times in monorepos. Since 5.0, `--build` accepts emit flags (`--declaration`, `--sourceMap` …).
- `skipLibCheck: true` skips checking `.d.ts` files (big win, standard practice).
- Expensive patterns: deeply recursive conditional/template-literal types, huge unions (> ~10k members) of string literals, `Omit`/`Pick` chains over large intersections, overloaded generic UI component props. Prefer interfaces with `extends` over large intersections (`A & B & C` is re-computed; interfaces cache).
- 5.1's instantiation and literal-union optimisations make some formerly slow code fast; re-measure after upgrading before rewriting types.
- `types: [...]` limits automatically included global `@types` packages, which reduces program size and accidental globals.

##### Emitted code

- `target: es2022` emits native classes, private fields, `?.`, `??`, `async`/`await` unchanged — smaller and faster than downlevelled helpers. Choose the highest `target` the runtime supports (Node 18/20/22/24: `es2022` is safe; 5.1 has no `es2023` target).
- `importHelpers: true` + `tslib` avoids duplicated helpers in every file (check the tslib version — ≥ 2.5.1 for 5.1 decorator emit).
- `downlevelIteration` is needed for correct `for…of`/spread of non-arrays when targeting ES5; its emitted code is large and slow.
- `enum` emits an object and reverse mapping; unions emit nothing.
- `useDefineForClassFields: true` (default for ES2022+) uses native field definition — correct but different semantics (see Classes).

***

#### Testing and Type Tests

- Runners: Vitest and Jest (via `ts-jest` or `@swc/jest`/Babel) transpile TypeScript **without type-checking** (except `ts-jest` with diagnostics on). Run `tsc --noEmit` in CI as a separate step; a green test run does not mean the code type-checks.
- Type-level tests: `// @ts-expect-error` (fails the build if the line compiles), `expectTypeOf` (Vitest, `expect-type`), `tsd`. Prefer `@ts-expect-error` over `@ts-ignore` — `@ts-ignore` keeps silencing after the error disappears.

```ts
// A tiny, dependency-free type assertion kit that works on 5.1
type Equal<A, B> =
  (<T>() => T extends A ? 1 : 2) extends (<T>() => T extends B ? 1 : 2) ? true : false;
type Expect<T extends true> = T;

type Route = { path: string; auth: boolean };
const routes = { home: { path: "/", auth: false } } satisfies Record<string, Route>;

type _t1 = Expect<Equal<keyof typeof routes, "home">>;
// @ts-expect-error TS2344 Type 'false' does not satisfy the constraint 'true'
type _t2 = Expect<Equal<keyof typeof routes, string>>;

export type { _t1, _t2 };
```

- Mocks: typed fakes (`const repo: Pick<Repo, "find"> = { find: vi.fn() }`) catch signature drift; `as any` mocks hide it.
- Test data builders with `satisfies` keep fixtures in sync with types.
- When a test framework's newest `@types/jest`/`@types/mocha` no longer compiles on 5.1, use their `ts5.1` dist-tag like for `@types/node`.

***

#### Tooling: Linters, Runners, Bundlers, Editors

- **typescript-eslint**: v8 (8.72.0 in October 2026) declares support for TypeScript `>=4.8.4 <6.1.0`, so it works with 5.1; it requires Node `^18.18.0 || ^20.9.0 || >=21.1.0`. Recommended rules for 5.1 code: `no-floating-promises`, `no-misused-promises`, `await-thenable`, `no-unsafe-assignment`/`-member-access`/`-call`/`-return`/`-argument`, `no-explicit-any`, `consistent-type-imports` (careful with `emitDecoratorMetadata` — see Decorators), `switch-exhaustiveness-check`, `prefer-nullish-coalescing`, `strict-boolean-expressions` (opinionated), `no-unnecessary-condition`, `restrict-template-expressions`.
- **Transpile-only runners** (`tsx`, `esbuild`, `swc`, Vite, Bun, Node's type stripping): no type-checking, single-file semantics — keep `isolatedModules: true` and run `tsc --noEmit` separately. Node's built-in stripping also rejects enums, namespaces with values, parameter properties (see Enums).
- **`ts-node`** (10.9.2 is the latest release, from 2023): type-checks by default (slow), has ESM-loader caveats on modern Node; many teams moved to `tsx` + `tsc --noEmit`.
- **Bundlers**: use `moduleResolution: bundler`; bundlers ignore `paths` unless configured (`vite-tsconfig-paths`, webpack `TsconfigPathsPlugin`) — `paths` only affect type-checking and never rewrite emitted imports.
- **Editors**: the editor may use a different TypeScript than the project — in VS Code select "Use Workspace Version" so editor diagnostics match `tsc` 5.1. 5.1 editor features: linked JSX tag editing, `@param` snippets, Move to file (preview).
- **Formatting**: Prettier handles 5.1 syntax; `tsc` does not format.
- **Version pinning**: pin `"typescript": "~5.1.6"` (tilde) — a caret range `^5.1.6` resolves to the newest 5.x (5.9.3) on a fresh install, which changes errors and lib typings.

***

#### Upgrading Beyond 5.1

A 5.1 codebase is three years behind (October 2026). What breaks or changes first, verified by compiling with each release:

| Release | Change that can break a 5.1 build |
|---|---|
| 5.2 | `module` and `moduleResolution` must agree for `node16`/`nodenext` (TS5109/TS5110); TS2729 now reported for field initializers reading parameter properties under `target: es2022`; DOM lib changes; `using`/decorator metadata become available |
| 5.3 | Import attributes `with`; `assert` still accepted; excess-property errors change from TS2322 to TS2353 (update `@ts-expect-error` comments or tooling that matches codes) |
| 5.4 | More accurate conditional-type constraints, intersection reduction, stricter enum compatibility; `NoInfer`, `Object.groupBy` typings |
| 5.5 | Options deprecated in 5.0 stop working (ignored even with `ignoreDeprecations: "5.0"`); inferred type predicates change inferred types of `filter` results; regex syntax errors |
| 5.6 | Options deprecated in 5.0 are rejected (TS5102/TS5108); always-truthy/nullish checks (TS2872 for `if (/re/)`); `strictBuiltinIteratorReturn` under `strict` |
| 5.7 | `TypedArray`s generic over `ArrayBufferLike` (breaks some `Buffer`/`Uint8Array` code); never-initialized variable checks |
| 6.0 | New defaults (`strict`, `module: esnext`, `types: []`, `target: es2025`, …), many options deprecated (`moduleResolution: node10`, `baseUrl`, `target: es5`, …) |
| 7.0 | Native compiler, removes everything deprecated in 6.0, no stable programmatic API in 7.0 |

Practical path: 5.1 → 5.4 (fix `module`/`moduleResolution` mismatches, parameter-property initializers, deprecated options) → 5.9 (fix TypedArray and iterator typing changes, update `@types/*` to `latest`) → 6.0 (adopt new defaults explicitly) → 7.0.

***

#### Removed / Changed in 5.1 — Migration Notes

Upgrading 5.0 → 5.1 (all verified on 5.0.4 vs 5.1.6):

1. **Build machines**: Node.js ≥ 14.17 for the compiler (`engines` changed from `>=12.20`).
2. **`typeRoots`**: if set, list every `@types` directory explicitly, including parent/monorepo-root `node_modules/@types`; otherwise TS2688 for `types` entries that used to be found by walking up.
3. **`typeRoots` resolution fallback**: imports that previously failed (TS2307) may now resolve to stub declarations in `typeRoots` folders — verify the runtime package exists.
4. **Decorator initializer order**: stacked field/accessor decorators whose initializers have side effects or depend on each other run in the reversed (spec-correct) order; with `importHelpers`, upgrade tslib to ≥ 2.5.1.
5. **New DOM declarations** can conflict with local polyfill declarations (`CompressionStream`, `WakeLock`, `VideoEncoder`, CSS Typed OM …) — remove the local copies.
6. **New accepted code**: `undefined`-returning functions without `return`, unrelated accessor types, async/string JSX components (with an `ElementType`-aware JSX library), namespaced JSX names with spaces. Code using them no longer compiles with 5.0 — pin TypeScript in every package of a monorepo.
7. **Editor plugins** calling `getOccurrencesAtPosition` or the `occurrences` tsserver command must switch to document highlights.
8. **`@types/*`**: newest DefinitelyTyped releases do not support 5.1; pin to the `ts5.1` dist-tags (`@types/node@24.1.0`, `@types/react@19.1.9`).
9. **Patch level**: use 5.1.6; 5.1.3 has stack-overflow, out-of-memory and JSX-emit regressions fixed in 5.1.5/5.1.6.

***

#### Production / Review Checklist

##### Version and configuration

- [ ] `typescript` pinned to `~5.1.6` (not `^`), lockfile committed; same version in all workspace packages
- [ ] No syntax or options from 5.2+ (`using`, `with` import attributes, `NoInfer`, `--module preserve`, `--isolatedDeclarations`, `--noCheck`, `--erasableSyntaxOnly`, `--target es2023`, `lib es2024`)
- [ ] No 5.2+ lib APIs typed only by later versions (`toSorted`, `Object.groupBy`, `Promise.withResolvers`, Set methods, Iterator helpers) unless a local `declare global` augmentation exists and the runtime supports them
- [ ] `@types/*` pinned to `ts5.1` dist-tags; Renovate/Dependabot constrained
- [ ] `strict: true` plus `noUncheckedIndexedAccess`, `noImplicitOverride`, `noImplicitReturns`, `noFallthroughCasesInSwitch` (and `exactOptionalPropertyTypes` for new code)
- [ ] `module`/`moduleResolution` consistent (`nodenext`+`nodenext`, `esnext`+`bundler`) — mismatches compile on 5.1 but fail on 5.2+
- [ ] `ignoreDeprecations: "5.0"` and deprecated options tracked as upgrade debt
- [ ] `typeRoots` absent, or listing every needed `@types` directory
- [ ] `verbatimModuleSyntax` or `isolatedModules` on for projects built by esbuild/swc/Vite/Babel
- [ ] `tsc --noEmit` runs in CI separately from tests and bundling
- [ ] Compiler runs on a supported Node.js (22/24); emitted `target` matches the deployment runtime

##### Types and correctness

- [ ] No unjustified `any`, `as`, `!`, `as unknown as`; external data typed `unknown` and validated
- [ ] Discriminated unions with `assertNever` exhaustiveness
- [ ] `??` instead of `||` for numeric/string defaults
- [ ] Narrowing copied into `const` before closures (5.1 does not preserve narrowing in closures)
- [ ] Explicit type predicates on `filter` (5.1 does not infer them)
- [ ] `Omit` on unions uses a distributive helper
- [ ] Accessor pairs with unrelated types are intentional and documented
- [ ] `(): undefined` vs `(): void` chosen deliberately

##### Classes and decorators

- [ ] Field initializers do not read parameter properties (silent `NaN`/`undefined` under `target: es2022` on 5.1)
- [ ] Re-declared inherited fields use `declare` under `useDefineForClassFields`
- [ ] No overridable method calls in constructors
- [ ] Decorator system matches the framework (`experimentalDecorators` for NestJS/Angular/TypeORM); no `context.metadata` on 5.1
- [ ] Classes used in decorated constructor signatures imported as values (not `import type`) when `emitDecoratorMetadata` is on
- [ ] tslib ≥ 2.5.1 with `importHelpers` and standard decorators

##### Modules

- [ ] Relative ESM imports carry `.js` extensions under `nodenext`
- [ ] JSON imports work on the target Node (no `assert {}` on Node 22+)
- [ ] Type-only imports use `import type` where the module must not load
- [ ] No `const enum` across files with single-file transpilers; no enums/namespaces/parameter properties when running via Node type stripping
- [ ] No reliance on `typeRoots` stubs for packages that are not installed

##### Async, errors, security

- [ ] No floating promises; no `async` callbacks to `forEach`/event APIs without error handling
- [ ] `catch (e)` narrowed from `unknown`; errors wrapped with `{ cause }`
- [ ] Timeouts/cancellation via `AbortSignal`
- [ ] Untrusted input validated at runtime; no prototype pollution via merges
- [ ] Parameterised SQL, argument-array process spawning, sanitised HTML, path-prefix checks
- [ ] Secrets compared in constant time, tokens from `crypto`

##### Tooling and tests

- [ ] typescript-eslint type-aware rules enabled (`no-floating-promises`, `no-unsafe-*`)
- [ ] Type tests with `@ts-expect-error` (not `@ts-ignore`)
- [ ] Editor uses the workspace TypeScript 5.1.6
- [ ] Published packages: `exports` with `types` first, checked by `@arethetypeswrong/cli`; minimum TypeScript version documented
