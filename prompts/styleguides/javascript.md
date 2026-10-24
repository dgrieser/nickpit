### JavaScript Style Guide

> **Version / scope**: Version-agnostic JavaScript — ECMAScript 2015 through **ECMAScript 2026** (ECMA-262 17th edition, approved by the Ecma General Assembly on **2026-06-30**) plus the Stage 4 proposals already scheduled for ES2027. This guide applies when the runtime is unknown or not pinned: browser code, code that runs in several runtimes (isomorphic libraries, edge/worker code, Deno, Bun), and Node.js projects whose pinned version is newer than 26 (the Node.js 27 alpha channel).[^1][^2]
> **Status (as of October 2026)**: Newest Node.js release is **26.12.1** (2026-10-21, "Current", scheduled for promotion to Active LTS on 2026-10-28). Node.js 24 "Krypton" (24.22.1) and 22 "Jod" (22.23.4) are in Maintenance LTS; Node.js 25, 20 and 18 are end-of-life. Node.js 27 has **no release yet** (only untested nightly builds `v27.0.0-nightly…`): its alpha channel is scheduled to start on 2026-10-28 and 27.0.0 is planned for 2027-04-22.[^3][^4] Browser availability is quoted as **Baseline** status from the `web-features` npm package **3.44.0** (published 2026-10-22).[^5]
> **Note**: This document is self-contained. Every version boundary it states ("ES2024, Node 22") was executed on Node.js 14.21.3, 16.20.2, 18.20.8, 20.20.2, 22.23.4, 24.22.1, 25.9.0 and 26.12.1; browser-only behaviour was executed in Chromium 141. Examples run unchanged as ES modules (`.mjs`) on Node.js 24 unless they say otherwise.

***

#### Overview

JavaScript code is reviewed against **two** questions: is it correct JavaScript, and does every feature it uses exist on every runtime the project targets? The language core (equality, scoping, closures, `this`, prototypes, promises, coercion) has not changed semantically since ES2015 and is where most real bugs live. The library surface grows every year, and that is where version incompatibilities live: `Object.groupBy` throws `TypeError: Object.groupBy is not a function` on Node 20 and Safari 17.3; `structuredClone` is missing on Node 16; `Temporal` is missing on every Safari release and on Node 24.

How to use this guide:

1. **Find the target first** (section "Version-aware review"). Every feature below names the ECMAScript edition, the first Node.js major that ships it, and its Baseline status for browsers.
2. Apply the **language rules** (equality, scoping, async, errors, security) unconditionally — they are the same on every runtime.
3. Recommend a version-tied feature only when the target supports it, or recommend it together with the fallback or polyfill the target needs.

***

#### Version-Aware Review

##### Determine the runtime target before judging an API

Check, in this order:

- **Node.js**: `engines.node` in `package.json`, `.nvmrc` / `.node-version` / `.tool-versions`, `volta.node`, `devEngines.runtime`, `FROM node:<version>` in Dockerfiles, `node-version` in CI (`actions/setup-node`), and the major of `@types/node` (it should match the runtime major; a newer `@types/node` type-checks APIs that do not exist at runtime).
- **Browsers**: `browserslist` (in `package.json` or `.browserslistrc`), a Baseline target (`baseline widely available`, or a year such as `baseline 2024`, supported by `browserslist` ≥ 4.26), the bundler/transpiler target (`build.target` in Vite/esbuild, `target` in SWC, `@babel/preset-env` `targets`), and `compilerOptions.target`/`lib` in `tsconfig.json`/`jsconfig.json`.[^6]
- **Other runtimes**: `deno.json`, Bun version pins, Cloudflare Workers `compatibility_date`, Electron version (Chromium + Node pair).

If nothing is pinned, assume the **intersection of maintained runtimes**: Node.js 22 (the oldest maintained LTS until 2027-04-30) and **Baseline Widely available** browsers. Under that assumption, ES2023 and most of ES2024 are safe (resizable `ArrayBuffer` and `Atomics.waitAsync` are only Newly available); ES2025 features need a check (`Promise.try`, `RegExp.escape` and RegExp modifiers are missing on Node 22, `Iterator` helpers need Safari 18.4); ES2026 features and `Temporal` need a fallback.

##### Rules

- **Verify before claiming something is unavailable.** Training data lags; `Iterator` helpers, `Set` algebra, `Promise.try`, `RegExp.escape`, `Error.isError`, `Uint8Array.prototype.toBase64` and `Temporal` all ship in current engines. Say "not available on X" only when the pinned target demonstrably lacks it (the tables below list verified boundaries).
- **Transpilers do not add APIs.** Babel/SWC/esbuild/TypeScript lower *syntax* (`?.`, `??`, class fields, `using`); they do not implement *built-ins* (`Object.groupBy`, `Array.prototype.at`, `structuredClone`, `Temporal`). A built-in needs a polyfill (`core-js` or a targeted package) or a runtime that has it. `tsc` with `"lib": ["ES2024"]` and `"target": "ES2020"` happily type-checks `Object.groupBy` and emits a call that throws on Node 20.
- **Libraries must not assume the newest runtime.** A package with `"engines": { "node": ">=20" }` must not call `Promise.withResolvers` or the `Set` methods (both Node 22) without a fallback. Application code pinned to one runtime may use everything that runtime has.
- **Feature-detect, do not version-sniff**: `typeof Iterator?.prototype?.map === 'function'`, `typeof globalThis.Temporal === 'object'`, `'toBase64' in Uint8Array.prototype`. Never parse `navigator.userAgent` or compare `process.version` strings with `<`.
- **Polyfills must be conditional.** Unconditionally replacing a native built-in (`globalThis.Temporal = polyfill.Temporal`) shadows the faster, spec-complete native one and can break `instanceof` checks between realms.
- **Experimental and flagged features are not available.** A feature behind `--harmony-*`, `--js-*`, `--experimental-*` or `chrome://flags` is not part of the target.

##### Feature detection pattern

```js
// Detect once, at module load, and pick an implementation
const groupBy = typeof Object.groupBy === 'function'
  ? Object.groupBy
  : (items, keyFn) => {
      const out = Object.create(null);           // same null-prototype shape as the native one
      let i = 0;
      for (const item of items) (out[keyFn(item, i++)] ??= []).push(item);
      return out;
    };

const byKind = groupBy([{ k: 'a' }, { k: 'b' }, { k: 'a' }], (x) => x.k);
console.log(Object.keys(byKind), Object.getPrototypeOf(byKind)); // [ 'a', 'b' ] null
```

***

#### ECMAScript Editions and Runtime Support

"Node" is the first Node.js **major** that ships the feature unflagged; where the boundary falls on an odd line that is no longer installable (17, 19, 21, 23), the verified pair is "absent on N−1, present on N+1". Browser versions and Baseline status come from `web-features` 3.44.0. **Baseline Widely available** = supported in all core browsers (Chrome, Edge, Firefox, Safari, desktop and mobile) for 30 months; **Newly available** = supported in all of them since the given date; **Limited** = at least one core browser lacks it.[^5]

##### Editions at a glance

| Edition | Headline features | Complete in Node | Browser status |
|---|---|---|---|
| ES2015 (ES6) | `let`/`const`, arrow functions, classes, modules, promises, `Map`/`Set`/`WeakMap`, symbols, iterators/generators, template literals, destructuring, default/rest parameters, `Proxy`/`Reflect` | ≤ 14 (ESM in Node ≥ 12.17) | Widely available |
| ES2016 | `**`, `Array.prototype.includes` | ≤ 14 | Widely available |
| ES2017 | `async`/`await`, `Object.entries`/`values`, `padStart`/`padEnd`, `SharedArrayBuffer`/`Atomics` | ≤ 14 | Widely available |
| ES2018 | async iteration (`for await`), object rest/spread, `Promise.prototype.finally`, RegExp named groups, lookbehind, `s` flag, `\p{…}` | ≤ 14 | Widely available |
| ES2019 | `flat`/`flatMap`, `Object.fromEntries`, `trimStart`/`trimEnd`, optional `catch` binding, stable `Array.prototype.sort` | ≤ 14 | Widely available |
| ES2020 | `?.`, `??`, `BigInt`, `Promise.allSettled`, `globalThis`, `matchAll`, dynamic `import()`, `import.meta` | 14 | Widely available |
| ES2021 | `replaceAll`, `Promise.any`/`AggregateError`, `&&=`/`\|\|=`/`??=`, numeric separators, `WeakRef`/`FinalizationRegistry` | 15 (absent on 14.21.3, present on 16.20.2) | Widely available |
| ES2022 | class fields, `#private` methods/accessors, `static {}` blocks, `#x in obj`, top-level `await`, `.at()`, `Object.hasOwn`, `Error` `cause`, RegExp `d` flag | 16.11 | Widely available (top-level `await`: Newly available, see table below) |
| ES2023 | `findLast`/`findLastIndex`, `toSorted`/`toReversed`/`toSpliced`/`with`, hashbang grammar, symbols as `WeakMap` keys | 20 | Widely available except symbols as `WeakMap` keys (Newly 2025-12-09, Firefox 146) |
| ES2024 | `Object.groupBy`/`Map.groupBy`, `Promise.withResolvers`, `isWellFormed`/`toWellFormed`, RegExp `v` flag, resizable `ArrayBuffer`, `ArrayBuffer.prototype.transfer`, `Atomics.waitAsync` | 22 | Widely available except resizable buffers (Newly 2024-07-09) and `Atomics.waitAsync` (Newly 2025-11-11) |
| ES2025 | `Iterator` helpers, `Set` methods, import attributes + JSON modules, RegExp modifiers, duplicate named groups, `Promise.try`, `Float16Array`/`Math.f16round`, `RegExp.escape` | 24 | Newly available (2024-06 … 2025-09) |
| ES2026 | `Error.isError`, `Array.fromAsync`, `JSON.parse` source text + `JSON.rawJSON`, `Uint8Array` base64/hex, `Map`/`WeakMap` `getOrInsert`/`getOrInsertComputed`, `Iterator.concat`, `Math.sumPrecise` | **none** (Node 26 lacks `Math.sumPrecise`) | Mixed: `Error.isError` Limited (no Safari); the rest Newly or Widely available |
| ES2027 (expected; Stage 4) | `Temporal`, `using`/`await using` + `DisposableStack`, `Atomics.pause`, `Iterator.zip`/`zipKeyed`, iterator `chunks`/`windows`, `Iterator.prototype.includes`/`join` | partial (Node 24: `using`, `Atomics.pause`; Node 26: `Temporal`; the iterator additions are in no Node release) | `Atomics.pause` Newly; everything else Limited (no Safari; the iterator additions only in recent Chrome/Firefox) |

##### Version-tied features — exact boundaries

| Feature | Edition | Node.js (verified pair) | Chrome / Firefox / Safari | Baseline (web-features 3.44.0) |
|---|---|---|---|---|
| `String.prototype.replaceAll`, `Promise.any`, logical assignment | ES2021 | 15 (14 ✗, 16 ✓) | 85 / 77–79 / 13.1–14 | Widely (2020) |
| `Array.prototype.at`, `String.prototype.at` | ES2022 | **16.6** (14 ✗, 16.20 ✓) | 92 / 90 / 15.4 | Widely (2022-03-14) |
| `Object.hasOwn`, `Error` `cause` | ES2022 | 16.9 (14 ✗, 16.20 ✓) | 93 / 91–92 / 15–15.4 | Widely |
| Top-level `await` (modules) | ES2022 | 14.8 | 89 / 89 / 27 | Newly (2026-09-14) — web-features records Safari support only from Safari 27 |
| `structuredClone()` (HTML, not ECMAScript) | — | **17** (16 ✗, 18 ✓) | 98 / 94 / 15.4 | Widely (2022-03-14) |
| `findLast`, `findLastIndex` | ES2023 | 18 (16 ✗, 18 ✓) | 97 / 104 / 15.4 | Widely (2022-08-23) |
| Symbols as `WeakMap` keys (non-registered only) | ES2023 | **20** (19.9 ✗; 18.20.x also accepts them through an undocumented V8 backport — do not rely on it on 18) | 109 / 146 / 16.4 | Newly (2025-12-09) |
| `toSorted`, `toReversed`, `toSpliced`, `with` | ES2023 | 20 (18 ✗, 20 ✓) | 110 / 115 / 16 | Widely (2023-07-04) |
| `isWellFormed`, `toWellFormed`; RegExp `v` flag | ES2024 | 20 (18 ✗, 20 ✓) | 111–112 / 116–119 / 16.4–17 | Widely |
| Resizable `ArrayBuffer` | ES2024 | 20 (18 ✗, 20 ✓) | 111 / 128 / 16.4 | Newly (2024-07-09) |
| `ArrayBuffer.prototype.transfer`, `JSON.rawJSON` / reviver `context.source` | ES2024 / ES2026 | 21 (20 ✗, 22 ✓) | 114 / 122, 135 / 17.4, 18.4 | Widely / Newly (2025-03-31) |
| `Object.groupBy`, `Map.groupBy` | ES2024 | **21** (20 ✗, 22 ✓) | 117 / 119 / 17.4 | Widely (2024-03-05) |
| `Promise.withResolvers` | ES2024 | **22** (20 ✗, 22 ✓) | 119 / 121 / 17.4 | Widely (2024-03-05) |
| `Array.fromAsync` | ES2026 | 22 (20 ✗, 22 ✓) | 121 / 115 / 16.4 | Widely (2024-01-25) |
| `Set` methods (`union`, `intersection`, …) | ES2025 | **22** (20 ✗, 22 ✓) | 122 / 127 / 17 | Newly (2024-06-11) |
| `Iterator` helpers (`Iterator.from`, `.map`, `.filter`, `.take`, …) | ES2025 | **22** (20 ✗, 22 ✓) | 122 / 131 / 18.4 | Newly (2025-03-31) |
| Import attributes `with { type: 'json' }` | ES2025 | 18.20 / 20.10 / 22 (`assert` syntax is a `SyntaxError` from 22) | 123 / 138 / 17.2 | Newly (2025-04-29) |
| `Promise.try` | ES2025 | 23 (22 ✗, 24 ✓) | 128 / 134 / 18.2 | Newly (2025-01-07) |
| RegExp modifiers `(?i:…)` | ES2025 | 23 (22 ✗, 24 ✓) | 125 / 132 / 26 | Newly (2025-09-15) |
| Duplicate named capture groups | ES2025 | 23 (22 ✗, 24 ✓) | 125 / 129 / 17 | Newly (2024-08-06) |
| `Intl.DurationFormat` (ECMA-402) | — | 23 (22 ✗, 24 ✓) | — | Newly (2025-03-04) |
| `RegExp.escape` | ES2025 | **24** (22 ✗, 24 ✓) | 136 / 134 / 18.2 | Newly (2025-05-01) |
| `Float16Array`, `Math.f16round` | ES2025 | 24 (22 ✗, 24 ✓) | 135 / 129 / 18.2 | Newly (2025-04-04) |
| `Error.isError` | ES2026 | **24** (22 ✗, 24 ✓) | 134 / 138 / — | **Limited** (no Safari) |
| `using` / `await using`, `DisposableStack`, `SuppressedError` | ES2027 (expected) | 24 (22 ✗, 24 ✓) | 134 / 141 / — | **Limited** (no Safari) |
| `Atomics.pause` | ES2027 (expected) | 24 (22 ✗, 24 ✓) | 133 / 137 / 18.4 | Newly (2025-04-01) |
| `Uint8Array.fromBase64`, `toBase64`, `fromHex`, `toHex` | ES2026 | **25** (24 ✗, 25 ✓) | 140 / 133 / 18.2 | Newly (2025-09-05) |
| `Map`/`WeakMap` `getOrInsert`, `getOrInsertComputed` | ES2026 | **26** (25 ✗, 26 ✓) | 145 / 144 / 26.2 | Newly (2026-02-14) |
| `Iterator.concat` | ES2026 | **26** (25 ✗, 26 ✓) | 146 / 147 / 26.4 | Newly (2026-03-24) |
| `Temporal` | ES2027 (expected) | **26** (25 ✗, 26 ✓) | 144 / 139 / — | **Limited** (no Safari) |
| `Math.sumPrecise` | ES2026 | **not in any Node release** (absent on 26.12.1) | 147 / 137 / 26.2 | Newly (2026-04-10) |
| `Iterator.zip`/`zipKeyed`, `Iterator.prototype.join` | ES2027 (expected) | **not in any Node release** (absent on 26.12.1) | 153 / 148 (`zip`, `zipKeyed`), 154 (`join`) / — | **Limited** (no Safari) |
| `Iterator.prototype.includes`, `chunks`, `windows` | ES2027 (expected) | **not in any Node release** | `includes` 154 / 154 / —; `chunks`/`windows` — / 154 / — | **Limited** |
| Decorators (`@dec`) | Stage 2.7 (since May 2026) | not shipped (`SyntaxError`) | — | not in any core browser |

Notes on the table:

- "16.6" for `.at()`, "16.9" for `Object.hasOwn`/`cause` and "16.11" for ES2022 completeness come from the Node 16 changelog (V8 9.2/9.3/9.4 upgrades); only 16.20.2 is installed, so the verified pair is 14 ✗ / 16.20 ✓.[^7]
- `Math.sumPrecise` is the rare case where every core browser is ahead of Node.js: Node 26 (V8 14.6) only has it behind `--js-sum-precise`.
- `Error.isError`, `using` and `Temporal` exist on Node ≥ 24/26 and in Chromium and Firefox, but **not in Safari**: browser code must polyfill or avoid them.
- The ES2027 iterator additions (`Iterator.zip`, `join`, `includes`, `chunks`, `windows`) already ship in current Chrome and/or Firefox but in no Node.js release and no Safari release: they are not usable in Node code or cross-browser code without a polyfill.[^10]
- Dates in the Baseline column are the "Newly available" (low) date; "Widely available" status follows 30 months later (for example `Object.groupBy`: Newly 2024-03-05, Widely 2026-09-05).

***

#### Node.js Release Lines (October 2026)

| Line | V8 | First release | Latest patch | Status on 2026-10-22 | End of life |
|---|---|---|---|---|---|
| 18 "Hydrogen" | 10.2 | 2022-04-19 | 18.20.8 (2025-03-27) | **EOL** | 2025-04-30 |
| 20 "Iron" | 11.3 | 2023-04-18 | 20.20.2 (2026-03-24) | **EOL** | 2026-04-30 |
| 22 "Jod" | 12.4 | 2024-04-24 | 22.23.4 (2026-10-21) | Maintenance LTS | 2027-04-30 |
| 24 "Krypton" | 13.6 | 2025-05-06 | 24.22.1 (2026-10-21) | Maintenance LTS (since 2026-10-20) | 2028-04-30 |
| 25 | 14.1 | 2025-10-15 | 25.9.0 (2026-04-01) | **EOL** (odd line, never LTS) | 2026-06-01 |
| 26 | 14.6 | 2026-05-05 | 26.12.1 (2026-10-21) | Current; Active LTS from 2026-10-28 | 2029-04-30 |
| 27 | — | alpha channel scheduled 2026-10-28; 27.0.0 planned 2027-04-22 | **none** (no alpha published yet) | unreleased | 2030-04-30 (planned) |

- The 2026-10-20/21 patches (22.23.4, 24.22.1, 26.12.1) are **security releases**. A project on an older patch of a maintained line runs with known vulnerabilities; flag pins such as `node:24.10` in Dockerfiles.
- Node 18 and 20 receive no security fixes. Flag `engines`, CI matrices and base images that still target them for production.
- Starting with 27, Node.js ships **one major per year**: a six-month alpha channel (prereleases such as `27.0.0-alpha.1`, semver-major changes still possible), a Current phase from April, then LTS for every line — the odd/even distinction disappears.[^4] Do not recommend alpha builds for production.

***

#### Do Not Use or Recommend

##### Not available in Node.js or across browsers (as of October 2026)

- **Decorators** (`@decorator class …`, TC39 Stage 2.7 since May 2026): a `SyntaxError` in every Node release and every core browser. TypeScript `experimentalDecorators` or Babel decorators are compile-time features; the reviewed code must go through that compiler.
- `Iterator.zip()`/`zipKeyed()`, `Iterator.prototype.chunks()`/`windows()`/`includes()`/`join()` (Stage 4 for ES2027): in **no Node.js release** (absent on 26.12.1) and no Safari release; only recent Chrome (153/154: `zip`, `zipKeyed`, `join`, `includes`) and Firefox (148/154) ship them. Server code and cross-browser code must not use them without a polyfill.
- `Promise.allKeyed` (Stage 3 since July 2026): Firefox 155 only; not in Node.js, Chrome or Safari.[^10][^11]
- `import defer`, source-phase imports for JavaScript modules, `ShadowRealm`, `String.dedent`, `Math.clamp`, immutable `ArrayBuffer`, the pipeline operator `|>`, records and tuples (`#{}`/`#[]`, withdrawn), `Array.prototype.group` (the shipped name is `Object.groupBy`).
- Import **assertions** (`import x from './a.json' assert { type: 'json' }`): replaced by import attributes (`with`). `assert` is a `SyntaxError` on Node ≥ 22 and current Chromium.
- `Math.sumPrecise` on **Node.js** (ships in browsers, not in Node 26).

##### Legacy features to flag in new code

| Feature | Problem | Use instead |
|---|---|---|
| `var` | function-scoped, hoisted, redeclarable, leaks out of blocks | `const`, `let` |
| `==` / `!=` (except `x == null`) | coercion rules (`'' == 0`, `[] == false`) | `===` / `!==` |
| `with` | forbidden in strict mode, defeats static scoping | destructuring |
| `arguments`, `arguments.callee` | not in arrow functions; `callee` throws in strict mode | rest parameters, named function expressions |
| `__proto__` accessor, `__defineGetter__` | legacy Annex B, prototype-pollution vector | `Object.getPrototypeOf`/`Object.create`/`Object.defineProperty` |
| `String.prototype.substr`, `escape`/`unescape`, `trimLeft`/`trimRight`, `Date.prototype.getYear` | Annex B / non-standard | `slice`/`substring`, `encodeURIComponent`, `trimStart`/`trimEnd`, `getFullYear` |
| Global `isNaN`, `isFinite` | coerce their argument (`isNaN('x')` is `true`) | `Number.isNaN`, `Number.isFinite` |
| `new Array(n)` / `Array(n)` for data | creates holes that `map` skips | `Array.from({ length: n }, fn)` |
| `obj.hasOwnProperty(k)` | breaks on null-prototype objects and shadowed keys | `Object.hasOwn(obj, k)` (ES2022) |
| `RegExp.$1` and other static properties, `RegExp.prototype.compile` | global mutable state, Limited | match results |
| `eval`, `new Function(string)`, `setTimeout('code')` | code injection, CSP violations | real functions, `JSON.parse` |
| `document.write`, `innerHTML` with untrusted data | XSS | `textContent`, DOM APIs, a sanitizer |
| CommonJS in new browser-facing packages | no tree shaking, not loadable natively in browsers | ESM |

***
#### Declarations, Scoping and Hoisting

##### `const` by default, `let` when reassigned, never `var`

```js
const MAX_RETRIES = 3;               // module-level constants: SCREAMING_SNAKE_CASE (convention)
let attempt = 0;                     // variables and functions: camelCase
class RetryPolicy {}                 // classes and constructors: PascalCase
const isExhausted = attempt >= MAX_RETRIES; // booleans read as predicates

// const prevents rebinding, not mutation
const settings = { debug: false };
settings.debug = true;               // allowed
console.log(settings.debug, isExhausted, RetryPolicy.name); // true false RetryPolicy
```

- `var` is function-scoped and hoisted with the value `undefined`; `let`/`const`/`class` are block-scoped and live in the **temporal dead zone** (TDZ) until their declaration runs.
- Top-level `var` and function declarations in a classic browser script become properties of `window`; in ES modules and CommonJS they do not.
- Shadowing an outer variable with the same name (`const user = …` inside a loop over `users` that also uses an outer `user`) is legal and a frequent source of bugs; lint with `no-shadow`.

```js
// BAD: TDZ — the inner `value` shadows the outer one before it is initialized
const value = 1;
function read() {
  const doubled = value * 2;           // refers to the inner `value` declared below
  const value = 2;
  return doubled;
}
try { read(); } catch (e) { console.log(e.name); } // ReferenceError

// BAD: var leaks out of the block and is shared by all iterations
var callbacks = [];
for (var i = 0; i < 3; i++) callbacks.push(() => i);
console.log(callbacks.map((f) => f())); // [ 3, 3, 3 ]

// GOOD: let creates a fresh binding per iteration
const fixed = [];
for (let j = 0; j < 3; j++) fixed.push(() => j);
console.log(fixed.map((f) => f())); // [ 0, 1, 2 ]
```

##### Strict mode

- ES modules and class bodies are always strict. CommonJS files and classic scripts are **sloppy** unless they start with `'use strict'`.
- In sloppy mode, assigning to an undeclared variable silently creates a global, writing to a read-only property fails silently, and `this` in a plain function call is the global object. In strict mode these throw `ReferenceError`/`TypeError` and `this` is `undefined`.
- Flag new CommonJS files without `'use strict'` and classic `<script>` code without it.

```js
// Modules are strict: writes to frozen objects throw instead of failing silently
const frozen = Object.freeze({ a: 1 });
try { frozen.a = 2; } catch (e) { console.log(e.name); } // TypeError
try { undeclaredVariable = 1; } catch (e) { console.log(e.name); } // ReferenceError
```

##### Functions declared in blocks

Function declarations are hoisted to the top of their scope. Inside blocks, sloppy-mode code follows legacy Annex B rules that differ between "the function exists in the enclosing scope" and "only in the block". Declare functions at module or function level, or use `const fn = () => {}` inside blocks.

***

#### Equality and Coercion

##### Use `===` everywhere; the only accepted loose form is `x == null`

```js
console.log(0 == '', '0' == false, [] == false, null == 0, null >= 0, 1 < 2 < 3, 3 > 2 > 1);
// true true true false true true false

console.log(null == undefined, null === undefined); // true false — `x == null` checks both
console.log(NaN === NaN, Object.is(NaN, NaN), Object.is(0, -0), 0 === -0); // false true false true
console.log([NaN].indexOf(NaN), [NaN].includes(NaN)); // -1 true — includes uses SameValueZero
```

- `==` converts operands (`ToPrimitive`, `ToNumber`); results are rarely what the author meant. `x == null` is the single idiomatic exception (true for `null` and `undefined` only).
- Relational operators (`<`, `>=`) coerce too: `null >= 0` is `true` while `null == 0` is `false`. Validate types before comparing.
- `Object.is` distinguishes `NaN` and `-0`; `Array.prototype.includes`, `Map` and `Set` use SameValueZero (`NaN` equals `NaN`, `-0` equals `0`).
- Objects compare by identity: `{a: 1} === {a: 1}` is `false`. Use a deep-equality helper (`node:assert`'s `deepStrictEqual` in tests, `util.isDeepStrictEqual` in Node code) or compare a canonical key.

##### Coercion traps

```js
console.log('1' + 1, '3' * '4', [] + [], [] + {}, +[], +{});           // 11 12  [object Object] 0 NaN
console.log(Number(''), Number(' '), Number(null), Number(undefined));  // 0 0 0 NaN
console.log(Number('12px'), Number.parseFloat('12px'), Number('0x10'), Number('1_000')); // NaN 12 16 NaN
console.log([1, 2, 3].map(parseInt));                                   // [ 1, NaN, NaN ]
console.log(Number.isNaN('x'), isNaN('x'));                             // false true
```

- `+` concatenates as soon as either operand is a string: `total + input.value` with a form field is string concatenation. Convert explicitly with `Number(x)` and validate with `Number.isFinite`.
- `Number('')` and `Number(' ')` are `0`: an empty form field silently becomes zero. Check for empty strings before converting.
- `parseInt` stops at the first invalid character and needs a radix: `Number.parseInt(s, 10)`. Never pass it to `map` directly — the index becomes the radix.
- `Number('1_000')` is `NaN`: numeric separators exist only in literals.

##### Truthiness

`false`, `0`, `-0`, `0n`, `''`, `null`, `undefined` and `NaN` are falsy; everything else — including `'0'`, `'false'`, `[]` and `{}` — is truthy.

```js
function withDefaults(options) {
  const bad = { port: options.port || 3000, verbose: options.verbose || true };  // BAD
  const good = { port: options.port ?? 3000, verbose: options.verbose ?? true }; // GOOD
  return { bad, good };
}
console.log(withDefaults({ port: 0, verbose: false }));
// { bad: { port: 3000, verbose: true }, good: { port: 0, verbose: false } }
```

- Use `??` (ES2020) when `0`, `''` or `false` are valid values; `||` only when every falsy value should be replaced.
- `??=`, `||=`, `&&=` (ES2021) short-circuit: `cache[key] ??= compute()` calls `compute` only when the entry is `null`/`undefined`.
- `if (items.length)` and `if (isEnabled)` are fine for numbers and booleans. Do not use truthiness to test values that may legitimately be `0` or `''`.

##### `typeof` and type checks

```js
console.log(typeof null, typeof [], typeof function () {}, typeof class {}, typeof 1n, typeof Symbol()); // object object function function bigint symbol
console.log(Array.isArray([]), Number.isInteger(5.0), Number.isSafeInteger(2 ** 53));            // true true false
```

- `typeof null === 'object'`: check `value !== null && typeof value === 'object'` for plain objects.
- `Array.isArray` works across realms (iframes, `vm` contexts); `instanceof Array` does not. The same applies to errors: `Error.isError(value)` (ES2026, Node 24, Limited — not in Safari) is the cross-realm brand check.
- `typeof undeclaredName` returns `'undefined'` without throwing — the only safe way to probe a global that may not exist (prefer `globalThis.name`).

***

#### Functions, Closures and `this`

##### Parameters, defaults and destructuring

```js
// Options object with destructured defaults; `= {}` makes the argument optional
function createClient({ baseUrl, timeoutMs = 5_000, retries = 2 } = {}) {
  if (typeof baseUrl !== 'string') throw new TypeError('baseUrl must be a string');
  return { baseUrl, timeoutMs, retries };
}
console.log(createClient({ baseUrl: 'https://api.example' }));
// { baseUrl: 'https://api.example', timeoutMs: 5000, retries: 2 }

// Defaults apply only to undefined, not to null
function f(x = 1) { return x; }
console.log(f(undefined), f(null)); // 1 null

// Rest parameters instead of `arguments`
const sum = (...numbers) => numbers.reduce((total, n) => total + n, 0);
console.log(sum(1, 2, 3)); // 6
```

- More than three positional parameters, or any boolean parameter (`save(doc, true, false)`), should become an options object.
- Default parameter expressions are evaluated on every call (`function f(list = [])` gets a fresh array each time), unlike Python.
- Template literals over concatenation; object shorthand (`{ name, age }`) and method shorthand (`add(a, b) {}`) over repetition.

##### Closures

A closure keeps every variable it references alive. Review for:

- **Stale closures**: a callback created once captures the values of that moment (React hooks, timers, event handlers registered at startup).
- **Leaks**: listeners and timers that capture large objects and are never removed. Remove listeners (`removeEventListener`, `AbortController` with `{ signal }`), clear intervals, and avoid caching per-request data in module-level closures.
- **Private state**: closures or `#private` fields are the only real privacy; `_underscore` is a convention.

```js
function makeCounter() {
  let count = 0;                        // private to the closure
  return { next: () => ++count, peek: () => count };
}
const c = makeCounter();
c.next(); c.next();
console.log(c.peek()); // 2

// Remove listeners with an AbortController (EventTarget exists in browsers and Node 15+)
const target = new EventTarget();
const controller = new AbortController();
let hits = 0;
target.addEventListener('ping', () => hits++, { signal: controller.signal });
target.dispatchEvent(new Event('ping'));
controller.abort();                     // removes the listener
target.dispatchEvent(new Event('ping'));
console.log(hits); // 1
```

##### `this`

`this` is determined by **how a function is called**, except for arrow functions, which capture `this` lexically.

```js
class Greeter {
  constructor(name) { this.name = name; }
  greet() { return `hi ${this.name}`; }
  greetArrow = () => `hi ${this.name}`;   // class field: one function per instance, bound
}

const g = new Greeter('ann');
const detached = g.greet;
try { detached(); } catch (e) { console.log(e.name); } // TypeError — `this` is undefined in strict code
console.log(g.greet.bind(g)(), g.greetArrow());          // hi ann hi ann

// Arrow functions inherit `this`; regular callbacks do not
const timer = {
  ticks: 0,
  start() { [1, 2].forEach(() => this.ticks++); return this.ticks; },
};
console.log(timer.start()); // 2
```

- Passing a method as a callback (`button.addEventListener('click', this.onClick)`, `promise.then(obj.method)`, `arr.map(obj.method)`) loses `this`. Bind it, wrap it in an arrow, or use an arrow class field.
- Do not use arrow functions as object methods or prototype methods that need the object as `this`, and never as constructors (`new (() => {})` throws).
- In DOM event listeners declared with `function`, `this` is `event.currentTarget`; with arrows it is the enclosing `this`.
- Arrow class fields allocate one function per instance — fine for UI components, wasteful for classes with many instances (use prototype methods there).

##### Pure functions and composition

Prefer functions that return new values over functions that mutate their arguments or module state. Pure functions are trivially testable and safe to memoize.

```js
// BAD: mutates the caller's object
function markProcessedBad(options) { options.processed = true; return options; }

// GOOD: returns a new object
const markProcessed = (options) => ({ ...options, processed: true });

const pipe = (...fns) => (x) => fns.reduce((acc, fn) => fn(acc), x);
const normalize = pipe((s) => s.trim(), (s) => s.toLowerCase());

const input = { id: 1 };
console.log(markProcessed(input), input, normalize('  MiXeD ')); // { id: 1, processed: true } { id: 1 } mixed
console.log(markProcessedBad({ id: 2 }));                         // { id: 2, processed: true }
```

***

#### Objects, Prototypes and Classes

##### Classes

```js
class Account {
  static #instances = 0;               // private static field (ES2022)
  #balance = 0;                         // private instance field (ES2022)
  static ROLES = Object.freeze(['owner', 'viewer']); // public static field

  static {                              // static initialization block (ES2022)
    Account.created = new Date(0).toISOString();
  }

  constructor(owner) {
    this.owner = owner;
    Account.#instances += 1;
  }

  deposit(amount) {
    if (!Number.isFinite(amount) || amount <= 0) throw new RangeError(`invalid amount: ${amount}`);
    this.#balance += amount;
    return this;
  }

  get balance() { return this.#balance; }
  static isAccount(value) { return #balance in value; }     // ergonomic brand check (ES2022)
  static get instances() { return Account.#instances; }
  toJSON() { return { owner: this.owner, balance: this.#balance }; }
}

const acc = new Account('ann').deposit(10);
console.log(acc.balance, Account.isAccount(acc), Account.isAccount({}), JSON.stringify(acc), Account.instances);
// 10 true false {"owner":"ann","balance":10} 1
```

- `#private` fields are enforced by the engine; reading `#x` on an object without it throws `TypeError`. `_underscore` properties are public.
- Public class fields (Chrome 72, Firefox 69, Safari 16) and `#private` fields (Chrome 74, Firefox 90, Safari 14.1) are Baseline Widely available; `static {}` blocks need Node 16.11. Transpile only for legacy targets.
- `JSON.stringify` ignores `#private` fields and prototype getters — define `toJSON()` when an instance must serialize.
- Derived constructors must call `super()` before touching `this` (`ReferenceError` otherwise).
- Prefer composition over deep inheritance; keep hierarchies to one or two levels (errors are the main exception).
- **Decorators are not available** in any runtime (see "Do not use or recommend").

##### Inheritance and `instanceof`

```js
class Entity {
  constructor(id) { this.id = id; }
  equals(other) { return other instanceof Entity && this.id === other.id; }
}
class User extends Entity {
  constructor(id, name) { super(id); this.name = name; }
}
const u = new User(1, 'ann');
console.log(u.equals(new User(1, 'bob')), Object.getPrototypeOf(User.prototype) === Entity.prototype); // true true
```

`instanceof` fails across realms (iframes, workers after `postMessage`, `vm` contexts, duplicated package copies in `node_modules`). For data crossing such boundaries, check a brand (`#x in obj`), a discriminant property (`kind: 'user'`), or `Error.isError` for errors.

##### Prototypes and own properties

```js
const proto = { inherited: true };
const obj = Object.create(proto);
obj.own = 1;

console.log('inherited' in obj, Object.hasOwn(obj, 'inherited'), Object.keys(obj)); // true false [ 'own' ]
for (const key in obj) console.log('for-in sees', key);                              // own, inherited

const dict = Object.create(null);           // no prototype: no toString, no __proto__ setter
dict.__proto__ = 'just a key';
console.log(Object.keys(dict), typeof dict.toString);                                 // [ '__proto__' ] undefined
```

- `for...in` walks inherited enumerable properties; use `Object.keys`/`Object.entries` (own, enumerable, string keys).
- `Object.hasOwn(obj, key)` (ES2022, Node 16.9, Widely available) instead of `obj.hasOwnProperty(key)`, which breaks on null-prototype objects and when a key named `hasOwnProperty` exists.
- Never extend built-in prototypes (`Array.prototype.last = …`). It collides with future standard methods — this is why the ES2024 grouping proposal shipped as `Object.groupBy` instead of `Array.prototype.group` (the method name broke existing websites).
- Use `Map` for dictionaries with untrusted or dynamic keys; plain objects collide with `__proto__`, `constructor`, `toString`.

##### Getters, setters and `Object.defineProperty`

Getters must be cheap and side-effect free; reviewers should flag getters that do I/O, throw on normal paths or return a new object on every access (`obj.items !== obj.items` surprises callers). Setters that validate should throw `TypeError`/`RangeError`.

***

#### Immutability and Copying

```js
// const and Object.freeze are shallow
const config = Object.freeze({ db: { host: 'localhost' } });
config.db.host = 'changed';
console.log(config.db.host); // changed

// Spread copies are shallow too
const defaults = { headers: { accept: 'application/json' } };
const copy = { ...defaults };
copy.headers.accept = 'text/plain';
console.log(defaults.headers.accept); // text/plain — shared nested object

// structuredClone deep-copies data (HTML API: Node 17, Widely available)
const deep = structuredClone({ when: new Date(0), tags: new Set(['a']), nested: { n: 1 } });
console.log(deep.when instanceof Date, deep.tags.has('a'));   // true true
try { structuredClone({ fn() {} }); } catch (e) { console.log(e.name); } // DataCloneError
class Point { constructor(x) { this.x = x; } }
console.log(structuredClone(new Point(1)) instanceof Point);  // false — prototypes are not preserved
```

- `structuredClone` copies `Date`, `Map`, `Set`, `RegExp`, typed arrays, `Error` and cyclic graphs; it throws `DataCloneError` for functions, DOM nodes and symbols, and drops class prototypes and `#private` fields. `JSON.parse(JSON.stringify(x))` is worse: it turns `Date` into strings, drops `undefined`, `Map`, `Set`, and throws on `BigInt` and cycles.
- **In-place array methods** mutate: `sort`, `reverse`, `splice`, `fill`, `copyWithin`, `push`/`pop`/`shift`/`unshift`. Sorting a prop, a shared cache entry or a function argument in place is a bug. Use the ES2023 copying versions — `toSorted()`, `toReversed()`, `toSpliced()`, `with(index, value)` (Node 20; Baseline Newly 2023-07-04, Widely since 2026-01-04) — or copy first (`[...arr].sort(cmp)`) on older targets.
- Freeze configuration objects and enum-like constants (`Object.freeze`), and deep-freeze only when the object is small and created once.

```js
const scores = [30, 4, 100];
const ranked = scores.toSorted((a, b) => b - a);   // ES2023, Node 20
console.log(ranked, scores);                         // [ 100, 30, 4 ] [ 30, 4, 100 ]
console.log([10, 1, 2].sort());                      // [ 1, 10, 2 ] — default sort compares strings
console.log(scores.with(1, 5), scores.at(-1));       // [ 30, 5, 100 ] 100
```

***

#### Modules: ESM and CommonJS

##### Prefer ES modules

ES modules (ES2015; Node ≥ 12.17 unflagged; Baseline Widely available in browsers) are the standard module system. Use ESM for new code; keep CommonJS only where the toolchain or published package requires it.

```javascript
// math.js
export const PI_ROUNDED = 3.14;
export function area(r) { return PI_ROUNDED * r * r; }

// consumer.js
import { area } from './math.js';     // explicit extension: required by Node ESM and browsers
import * as math from './math.js';    // namespace import
const { heavy } = await import('./heavy.js'); // dynamic import (ES2020) for lazy loading
```

- **Prefer named exports.** Default exports invite inconsistent local names and make re-exports and refactoring harder.
- **Relative specifiers need the file extension** in Node ESM and browsers (`'./util.js'`, not `'./util'`); bundlers hide this, so code that is also run unbundled breaks.
- **ESM is strict and has no `require`, `module`, `exports`, `__filename`, `__dirname`.** Use `import.meta.url` (with `new URL('./file', import.meta.url)`), and on Node ≥ 20.11/21.2 `import.meta.dirname`/`import.meta.filename` (verified: present on 20.20.2, absent on 18.20.8).
- **Imports are hoisted and live bindings**; module bodies run once, in dependency order. Circular imports see uninitialized (`TDZ`) bindings — a `ReferenceError: Cannot access 'X' before initialization` at load time almost always means a cycle.
- **Avoid side effects at import time** (network calls, reading env-dependent config, starting servers). They make modules hard to test and break tree shaking. Export a function that does the work.
- **Barrel files** (`index.js` that re-exports everything) are convenient for public package entry points but slow down bundlers, test runners and cold starts when used inside an application; import from the defining module internally.
- **Top-level `await`** (ES2022) blocks every importer until it settles. Use it in entry points and scripts, not in widely imported library modules.

##### JSON and import attributes

```javascript
// ES2025 import attributes; Node 18.20 / 20.10 / 22+, Chrome 123, Firefox 138, Safari 17.2
import pkg from './package.json' with { type: 'json' };
// BAD (removed): import pkg from './package.json' assert { type: 'json' };  — SyntaxError on Node ≥ 22 and Chromium
```

Without the attribute, importing JSON is an error in Node and browsers. JSON modules expose only a default export, and the object is shared by every importer — do not mutate it.

##### CommonJS and interop

```js
// In an ES module: load CommonJS or builtins with createRequire when needed
import { createRequire } from 'node:module';
const require = createRequire(import.meta.url);
const path = require('node:path');
console.log(typeof path.join); // function
```

- ESM can `import` CommonJS (the default export is `module.exports`; named exports are detected heuristically and may be missing).
- CommonJS `require()` of an ES module (`require(esm)`) works without a flag on Node ≥ 22.12 and 20.19 (verified on 20.20.2 and 22.23.4; `ERR_REQUIRE_ESM` on 18.20.8), but not for modules that use top-level `await` (`ERR_REQUIRE_ASYNC_MODULE`).
- **Dual packages** (ESM + CJS builds) can load twice — two copies of module state, `instanceof` failures. Prefer ESM-only packages for new libraries targeting Node ≥ 20.19, or keep state in one format and wrap it from the other.
- `package.json`: set `"type": "module"` (or use `.mjs`), define `"exports"` (it blocks deep imports that are not listed), and use `"imports"` with `#` prefixes for internal aliases instead of `../../..` chains.
- Use the `node:` prefix for built-ins (`import fs from 'node:fs'`): it states that a built-in is meant, and recent built-ins exist only with the prefix (`'test'` resolves to an npm package, `'node:test'` to the test runner).

##### Dependency injection

Pass collaborators in (constructor or factory parameters) instead of importing singletons deep in the code; it keeps modules testable without module-mocking machinery.

```js
class UserService {
  constructor({ api, logger = console }) { this.api = api; this.logger = logger; }
  async getUser(id) {
    this.logger.info?.(`fetching user ${id}`);
    return this.api.get(`/users/${encodeURIComponent(id)}`);
  }
}
const fakeApi = { get: async (url) => ({ url }) };
console.log(await new UserService({ api: fakeApi, logger: {} }).getUser('a/b')); // { url: '/users/a%2Fb' }
```

***

#### Errors

##### Throw `Error` objects, attach causes, keep codes stable

```js
class AppError extends Error {
  constructor(message, { code = 'E_INTERNAL', status = 500, cause } = {}) {
    super(message, { cause });               // standard `cause` (ES2022, Node 16.9, Widely available)
    this.name = new.target.name;             // 'AppError' or the subclass name
    this.code = code;
    this.status = status;
  }
}

class NotFoundError extends AppError {
  constructor(resource, id, options = {}) {
    super(`${resource} ${id} not found`, { code: 'E_NOT_FOUND', status: 404, ...options });
    this.resource = resource;
  }
}

class ValidationError extends AppError {
  constructor(message, field, options = {}) {
    super(message, { code: 'E_VALIDATION', status: 400, ...options });
    this.field = field;
  }
}

function loadUser(id) {
  try {
    JSON.parse('{ not json');
  } catch (err) {
    throw new NotFoundError('user', id, { cause: err });
  }
}

try {
  loadUser(42);
} catch (err) {
  console.log(err instanceof NotFoundError, err instanceof AppError, err.name, err.code, err.status);
  // true true NotFoundError E_NOT_FOUND 404
  console.log(err.cause.name);  // SyntaxError
}
console.log(new ValidationError('too short', 'password').field); // password
```

- **Only throw `Error` instances** (or subclasses). `throw 'message'` and `throw { code }` carry no stack trace and break `instanceof`/`Error.isError` handling. Lint with `no-throw-literal` and `prefer-promise-reject-errors`.
- **Wrap with `{ cause }`** instead of concatenating messages; the original stack survives. Node and browser consoles print the cause chain.
- **Branch on `err.code` or class, never on `err.message`** — messages are not API and differ across engines.
- `this.name = new.target.name` keeps subclass names correct without repeating them. Set `name` — otherwise logs show `Error` for every subclass.
- `Error.captureStackTrace` and `Error.stackTraceLimit` are not ECMAScript (`captureStackTrace` is a Stage 2 proposal): V8 has both, Firefox has `captureStackTrace` since 138 and `stackTraceLimit` since 153, Safari has `captureStackTrace` since 17.2 and `stackTraceLimit` since 11.1. Guard them (`Error.captureStackTrace?.(this, new.target)`) in cross-engine code. Subclassing `Error` with `class … extends Error` already captures the stack.
- **Catch narrowly.** A `catch` that swallows every error (`catch {}` or `catch (e) { return null }`) hides bugs; catch, check the type/code you expect, and rethrow everything else.
- **Handle once, at a boundary** (request handler, job runner, UI error boundary). Logging and rethrowing at every layer duplicates logs.
- In a `catch (e)` the value can be anything (`unknown` in TypeScript) — code that reads `e.message` must tolerate non-errors: `Error.isError(e) ? e.message : String(e)` (ES2026; on Safari or Node < 24 use `e instanceof Error`).

```js
function classify(err) {
  if (err instanceof NotFoundError) return 404;
  if (err instanceof ValidationError) return 400;
  throw err;                          // unknown errors propagate
}
class NotFoundError extends Error {}
class ValidationError extends Error {}
console.log(classify(new NotFoundError('x')));       // 404
try { classify(new TypeError('bug')); } catch (e) { console.log(e.name); } // TypeError
```

##### Aggregates and suppressed errors

```js
const results = await Promise.allSettled([Promise.resolve(1), Promise.reject(new Error('boom'))]);
console.log(results.map((r) => r.status));                         // [ 'fulfilled', 'rejected' ]

try {
  await Promise.any([Promise.reject(new Error('a')), Promise.reject(new Error('b'))]);
} catch (e) {
  console.log(e.constructor.name, e.errors.map((x) => x.message)); // AggregateError [ 'a', 'b' ]
}
```

`AggregateError` (ES2021) carries `errors`; `SuppressedError` (with `using`, Node 24, Limited) carries `error` and `suppressed`. Error-reporting code should walk `cause`, `errors` and `suppressed`.

##### Error-tuple helpers

A `[error, value]` helper is acceptable for call sites that genuinely branch on failure, but it must not become a way to ignore errors:

```js
async function attempt(promise) {
  try {
    return [null, await promise];
  } catch (error) {
    return [error, null];
  }
}
const [error, data] = await attempt(Promise.reject(new Error('offline')));
console.log(error?.message, data); // offline null
```

Put the error first (callers cannot destructure the value without seeing it) and handle it on the next line.

***
#### Asynchronous Programming

##### Prefer `async`/`await` for sequential logic

```js
const delay = (ms, value) => new Promise((resolve) => setTimeout(resolve, ms, value));
const fetchUser = (id) => delay(10, { id, name: `user${id}` });
const fetchPosts = (userId) => delay(10, [{ userId, title: 'hello' }]);

// Harder to follow: multi-step .then() chain
function userPostsThen(id) {
  return fetchUser(id).then((user) => fetchPosts(user.id)).then((posts) => posts.length);
}

// GOOD: async/await reads top to bottom and works with try/catch
async function userPosts(id) {
  const user = await fetchUser(id);
  const posts = await fetchPosts(user.id);
  return posts.length;
}

console.log(await userPostsThen(1), await userPosts(1)); // 1 1
```

`.then()` stays fine for one-step transforms and for combinators (`Promise.all(items.map(load))`).

##### Run independent work concurrently

```js
const delay = (ms, value) => new Promise((resolve) => setTimeout(resolve, ms, value));

// BAD: sequential awaits for independent work (~60 ms)
async function sequential() {
  const users = await delay(20, 'users');
  const posts = await delay(20, 'posts');
  const comments = await delay(20, 'comments');
  return [users, posts, comments];
}

// GOOD: start everything, await together (~20 ms)
async function parallel() {
  return Promise.all([delay(20, 'users'), delay(20, 'posts'), delay(20, 'comments')]);
}

let t = performance.now();
await sequential();
const seqMs = performance.now() - t;
t = performance.now();
await parallel();
const parMs = performance.now() - t;
console.log(seqMs > 50, parMs < 45); // true true
```

| Combinator | Settles when | Use for |
|---|---|---|
| `Promise.all` (ES2015) | all fulfil, or the **first** rejects | all-or-nothing work |
| `Promise.allSettled` (ES2020) | all settle | best-effort batches, reporting each outcome |
| `Promise.any` (ES2021) | first fulfils, or all reject (`AggregateError`) | redundant sources, fallbacks |
| `Promise.race` (ES2015) | first settles | timeouts — prefer `AbortSignal.timeout` |

- `Promise.all` does **not cancel** the other operations when one rejects; they keep running and their later rejections are handled silently by `Promise.all`. Pass an `AbortSignal` to make cancellation real.
- `Promise.all(items.map(fn))` over thousands of items starts everything at once: sockets, file descriptors, rate limits and memory are exhausted. **Bound concurrency** (below).
- `Promise.withResolvers()` (ES2024, Node 22, Widely available) returns `{ promise, resolve, reject }` for bridging callback/event code; on Node 20 or older targets use `new Promise((resolve, reject) => …)`.
- `Promise.try(fn)` (ES2025, Node 23/24, Newly available 2025-01-07) runs `fn` and turns both a synchronous throw and a returned value into a promise; on Node 22 use `new Promise((resolve) => resolve(fn()))` or an `async` wrapper.

##### Every promise must be awaited, returned or handled

```js
const delay = (ms) => new Promise((resolve) => setTimeout(resolve, ms));
const log = [];
async function save(id) { await delay(5); log.push(id); }

// BAD: forEach ignores the returned promises; the function returns before the work is done
async function saveAllBad(ids) { ids.forEach(async (id) => { await save(id); }); }
await saveAllBad([1, 2]);
console.log(log.length); // 0

// GOOD: for...of for sequential work, Promise.all for concurrent work
async function saveAll(ids) { await Promise.all(ids.map((id) => save(id))); }
await saveAll([3, 4]);
console.log(log.length >= 2); // true
```

- **Floating promises** (a call to an async function whose result is not awaited, returned, or given a `.catch`) lose errors and ordering. In TypeScript lint with `@typescript-eslint/no-floating-promises` and `no-misused-promises` (catches `async` callbacks passed to `forEach`, event emitters and `if` conditions).
- `array.forEach`, `array.filter`, `array.some` and `array.reduce` do not await async callbacks. `filter(async …)` keeps every element (a promise is truthy).
- **`return await` inside `try`** is required for the `catch`/`finally` to see the rejection:

```js
async function failing() { throw new Error('nope'); }

async function withoutAwait() {
  try { return failing(); } catch { return 'caught'; }       // BAD: the rejection escapes the try
}
async function withAwait() {
  try { return await failing(); } catch { return 'caught'; } // GOOD
}
console.log(await withoutAwait().catch((e) => e.message), await withAwait()); // nope caught
```

- An `async` function **always** returns a promise. `if (isValid(x))` where `isValid` is async is always true.
- `await` in a non-async callback is a `SyntaxError`; `await` at module top level only works in ES modules.
- Mixing callbacks and promises: never call a callback inside `.then()` (an exception in the callback becomes a rejection of an unobserved promise). Promisify the callback API (`util.promisify` in Node, or `new Promise`).

##### Unhandled rejections

- **Node.js ≥ 15** terminates the process on an unhandled rejection (`--unhandled-rejections=throw` is the default; verified: Node 14 prints `UnhandledPromiseRejectionWarning` and keeps running, Node 16 exits with the error). Do not "fix" crashes with `--unhandled-rejections=warn`; fix the missing `await`/`.catch()`.
- **Browsers** report them on the `unhandledrejection` event and in the console but keep running; the failure is silent for users. Register a reporting handler (`window.addEventListener('unhandledrejection', …)`) that sends the error to monitoring.
- A rejection is "handled" only if a handler is attached **before the microtask checkpoint**. Creating a promise early and awaiting it later (after another `await`) can trigger a spurious unhandled rejection:

```js
const delay = (ms) => new Promise((resolve) => setTimeout(resolve, ms));
const events = [];
process.on('unhandledRejection', () => events.push('unhandled'));  // Node; in browsers: 'unhandledrejection'

async function bad() {
  const p = Promise.reject(new Error('early'));  // no handler yet
  await delay(10);                               // the rejection is reported as unhandled here
  try { await p; } catch { events.push('caught late'); }
}
await bad();
console.log(events); // [ 'unhandled', 'caught late' ]
```

Without the `unhandledRejection` listener, Node ≥ 15 would terminate at that point; with it, Node additionally prints `PromiseRejectionHandledWarning` when the late handler attaches. Start independent work together and await it with `Promise.all`/`allSettled`, so a handler is attached immediately.

##### Bounded concurrency

```js
async function mapLimit(items, limit, fn) {
  const results = new Array(items.length);
  let next = 0;
  async function worker() {
    while (next < items.length) {
      const i = next++;                     // safe: JS runs one callback at a time
      results[i] = await fn(items[i], i);
    }
  }
  await Promise.all(Array.from({ length: Math.min(limit, items.length) }, worker));
  return results;
}

let inFlight = 0;
let peak = 0;
const out = await mapLimit([1, 2, 3, 4, 5], 2, async (n) => {
  peak = Math.max(peak, ++inFlight);
  await new Promise((r) => setTimeout(r, 5));
  inFlight--;
  return n * 10;
});
console.log(out, peak); // [ 10, 20, 30, 40, 50 ] 2
```

Libraries such as `p-limit` do the same; the important review point is that *some* limit exists for fan-out over user-controlled or large collections.

##### Cancellation and timeouts with `AbortSignal`

```js
async function work(signal) {
  return new Promise((resolve, reject) => {
    const timer = setTimeout(resolve, 1_000, 'done');
    signal.addEventListener('abort', () => {
      clearTimeout(timer);
      reject(signal.reason);
    }, { once: true });
  });
}

const caller = new AbortController();
const signal = AbortSignal.any([caller.signal, AbortSignal.timeout(20)]);
try {
  await work(signal);
} catch (err) {
  console.log(err.name); // TimeoutError
}
```

- Accept an optional `signal` in long-running async APIs and pass it down (`fetch(url, { signal })`, timers, streams, event listeners).
- `AbortSignal.timeout(ms)` (Node 16.14/17.3; Baseline Newly 2024-04-18, Widely since 2026-10-18) rejects with a `TimeoutError` `DOMException`; `controller.abort()` uses `AbortError`. `AbortSignal.any()` (Node 20.3 and 18.17; Newly 2024-03-19, Widely since 2026-09-19) combines signals.
- Call `signal.throwIfAborted()` at loop boundaries; remove `abort` listeners from long-lived signals (`{ once: true }` or explicit removal), otherwise they accumulate.
- A `fetch` **timeout must cover the body**: the signal passed to `fetch` also aborts `response.json()`; a `setTimeout` + `Promise.race` around `fetch()` alone does not stop the download.

##### The event loop

- Microtasks (promise reactions, `queueMicrotask`) run to completion before the next task (timer, I/O callback, UI event). A loop of `await Promise.resolve()` or recursive microtasks starves rendering and I/O.
- CPU-heavy synchronous work blocks **everything**: UI rendering in browsers, every request in a server. Chunk it (yield with `await new Promise((r) => setTimeout(r))`, or `scheduler.yield()` where available — Chromium 129, Firefox 142, Limited), or move it to a Worker / `worker_threads`.
- `setTimeout(fn, 0)` is not immediate: browsers clamp timers nested more than five levels deep to ≥ 4 ms, and throttle timers in background tabs much further.
- Long `setInterval` callbacks can overlap with async work; prefer a self-rescheduling `setTimeout` after the async work finishes.

```js
const order = [];
setTimeout(() => order.push('timeout'), 0);
Promise.resolve().then(() => order.push('microtask'));
queueMicrotask(() => order.push('queueMicrotask'));
order.push('sync');
await new Promise((r) => setTimeout(r, 5));
console.log(order); // [ 'sync', 'microtask', 'queueMicrotask', 'timeout' ]
```

##### Debounce, throttle and memoization

```js
function debounce(fn, delayMs) {
  let timeoutId;
  return function debounced(...args) {
    clearTimeout(timeoutId);
    timeoutId = setTimeout(() => fn.apply(this, args), delayMs);
  };
}

function throttle(fn, intervalMs) {
  let last = -Infinity;
  return function throttled(...args) {
    const now = performance.now();
    if (now - last >= intervalMs) {
      last = now;
      return fn.apply(this, args);
    }
  };
}

let calls = 0;
const save = debounce(() => calls++, 10);
save(); save(); save();
await new Promise((r) => setTimeout(r, 30));
const ping = throttle(() => 'ran', 1_000);
console.log(calls, ping(), ping()); // 1 ran undefined

// Memoize with a bounded cache and an explicit key
function memoize(fn, { key = (x) => x, max = 1_000 } = {}) {
  const cache = new Map();
  return (arg) => {
    const k = key(arg);
    if (cache.has(k)) return cache.get(k);
    const value = fn(arg);
    if (cache.size >= max) cache.delete(cache.keys().next().value);   // evict the oldest entry
    cache.set(k, value);
    return value;
  };
}
const square = memoize((n) => n * n, { max: 2 });
console.log(square(3), square(3), square(4)); // 9 9 16
```

- Debounced/throttled functions must be created **once** (outside render functions/handlers), otherwise every call gets a fresh timer.
- Memoization review points: the cache must be **bounded** (or keyed by a `WeakMap` on objects), the key must be correct (`JSON.stringify` keys depend on property order and drop `undefined`, functions and `Map`s), and memoizing an async function must cache the *promise* and evict it on rejection.
- Memoize only pure functions.

***

#### Explicit Resource Management (`using`) — check the target

`using` / `await using` (Stage 4, expected in ES2027) dispose resources at the end of a block in reverse order, even on exceptions. Available on Node ≥ 24, Chrome 134, Firefox 141; **not in Safari** (Limited) — browser code needs a transpiler (TypeScript ≥ 5.2, Babel) **plus** a `Symbol.dispose`/`DisposableStack` polyfill.

```js
// Node 24+, Chrome 134+, Firefox 141+
const log = [];
function resource(name) {
  return { name, [Symbol.dispose]() { log.push(`dispose ${name}`); } };
}
{
  using a = resource('a');
  using b = resource('b');
  log.push('body');
}
console.log(log); // [ 'body', 'dispose b', 'dispose a' ]

// DisposableStack collects ad-hoc cleanups
{
  using stack = new DisposableStack();
  const interval = stack.adopt(setInterval(() => {}, 1_000), clearInterval);
  stack.defer(() => log.push('deferred'));
}
console.log(log.at(-1)); // deferred
```

Review rules: never `return` a value declared with `using` (it is already disposed); `using` is a `SyntaxError` directly in a `switch` case and at the top level of a classic script; dispose errors combine with the body error into a `SuppressedError`.

***

#### Iteration and Collections

##### Choosing the loop

```js
const users = [
  { id: 1, name: 'Alice', active: true },
  { id: 2, name: 'Bob', active: false },
  { id: 3, name: 'Charlie', active: true },
];

const names = users.map((u) => u.name);                        // transform
const active = users.filter((u) => u.active);                  // select
const bob = users.find((u) => u.id === 2);                     // first match (undefined if none)
const anyActive = users.some((u) => u.active);                 // boolean
const byId = new Map(users.map((u) => [u.id, u]));             // index for O(1) lookups
const activeNames = users.filter((u) => u.active).map((u) => u.name).toSorted(); // ES2023 toSorted

console.log(names, active.length, bob.name, anyActive, byId.get(3).name, activeNames);
// [ 'Alice', 'Bob', 'Charlie' ] 2 Bob true Charlie [ 'Alice', 'Charlie' ]
```

- `for...of` iterates values of any iterable (arrays, strings by code point, `Map`, `Set`, generators) and supports `break`, `continue` and `await`.
- **`for...in` is for object keys only**: on arrays it yields string indices, includes inherited enumerable properties and does not guarantee the order you expect for non-index keys. Flag `for (const i in array)`.
- `forEach` cannot `break` and does not await; use `for...of` when either matters.
- `reduce` without an initial value throws `TypeError` on an empty array; always pass the initial value. Prefer `Map`/`Object.groupBy`/a `for...of` loop over clever reductions that build objects (`reduce` with `{ ...acc, [k]: v }` is O(n²)).
- `find` returns `undefined`, `findIndex` returns `-1`, `indexOf` returns `-1`: check for those explicitly — `if (arr.indexOf(x))` is false for index 0.
- `array.at(-1)` (ES2022, Node 16.6, Widely available) replaces `array[array.length - 1]`; `array[-1]` is `undefined`.
- `findLast`/`findLastIndex` (ES2023, Node 18, Widely available) search from the end.

##### Sparse arrays and `Array` construction

```js
console.log(Array(3).map((_, i) => i));                   // [ <3 empty items> ] — holes are skipped
console.log(Array.from({ length: 3 }, (_, i) => i));      // [ 0, 1, 2 ]
console.log([1, , 3].indexOf(undefined), [1, , 3].includes(undefined)); // -1 true
console.log(new Array(3).fill().map((_, i) => i * 2));    // [ 0, 2, 4 ]
```

##### Mutation during iteration

Removing elements while iterating forward with an index skips elements; adding to a `Set`/`Map` while iterating it visits the new entries. Build a new collection (`filter`) or iterate a copy.

##### `Map`, `Set` and grouping

```js
// Map: any key type, insertion order, size, no prototype collisions
const counts = new Map();
for (const w of ['a', 'b', 'a']) counts.set(w, (counts.get(w) ?? 0) + 1);
console.log([...counts]); // [ [ 'a', 2 ], [ 'b', 1 ] ]

// Object.groupBy / Map.groupBy (ES2024, Node 21+; Baseline Newly 2024-03-05, Widely since 2026-09-05)
const people = [{ team: 'x', n: 1 }, { team: 'y', n: 2 }, { team: 'x', n: 3 }];
const byTeam = Object.groupBy(people, (p) => p.team);
console.log(Object.keys(byTeam), Object.getPrototypeOf(byTeam)); // [ 'x', 'y' ] null
console.log(Map.groupBy(people, (p) => p.n > 1).get(true).length); // 2

// Set algebra (ES2025, Node 22, Newly available since 2024-06-11)
const admins = new Set(['ann', 'bob']);
const online = new Set(['bob', 'cy']);
console.log([...admins.intersection(online)], [...admins.union(online)], admins.isDisjointFrom(new Set(['dan'])));
// [ 'bob' ] [ 'ann', 'bob', 'cy' ] true
```

- Use `Set` for membership tests and de-duplication (`[...new Set(arr)]`), `Map` for lookups by id — `array.includes`/`find` inside a loop is O(n²).
- `Object.groupBy` returns a **null-prototype** object: `byTeam.hasOwnProperty` is `undefined`; use `Object.hasOwn`.
- `Map`/`Set` compare object keys by identity: `map.get({ id: 1 })` never finds a previously stored `{ id: 1 }`. Use primitive keys.
- `JSON.stringify(new Map(...))` is `{}`; convert with `Object.fromEntries(map)` or `[...map]`.
- `WeakMap`/`WeakSet` for metadata keyed by objects you do not own (no leaks, not iterable). Symbols can be `WeakMap` keys since ES2023 (Node 20, Chrome 109, Firefox 146, Safari 16.4; Baseline Newly 2025-12-09) — except registered symbols from `Symbol.for()`, which throw `TypeError`.
- **Upsert** (`map.getOrInsert(key, value)`, `map.getOrInsertComputed(key, fn)`, ES2026) needs Node 26, Chrome 145, Firefox 144, Safari 26.2 (Newly available since 2026-02-14). On older targets keep the `get`/`?? `/`set` pattern above.

##### Iterators and generators

```js
function* naturals() { let n = 0; while (true) yield n++; }

// Iterator helpers (ES2025): lazy, work on any iterator — Node 22, Chrome 122, Firefox 131, Safari 18.4
console.log(naturals().filter((n) => n % 2).map((n) => n * n).take(3).toArray()); // [ 1, 9, 25 ]
console.log(new Map([['a', 1], ['b', 2]]).keys().map((k) => k.toUpperCase()).toArray()); // [ 'A', 'B' ]

// An iterator can be consumed only once
const it = [1, 2, 3].values();
console.log([...it], [...it]); // [ 1, 2, 3 ] []

// Array.fromAsync (ES2026, Node 22) collects async iterables sequentially
async function* pages() { yield 1; yield 2; }
console.log(await Array.fromAsync(pages())); // [ 1, 2 ]
```

- Iterator helpers return iterators, not arrays: call `.toArray()` or spread. `toArray()` on an infinite iterator never returns — `take(n)` first.
- `Iterator.concat(a, b)` (ES2026) needs Node 26 / Chrome 146 / Firefox 147 / Safari 26.4. `Iterator.zip`, `chunks`, `windows`, `includes`, `join` (ES2027) are in no Node.js or Safari release — only in recent Chrome/Firefox.
- Generators are lazy and stateful; returning early from `for...of` calls the generator's `return()` (its `finally` runs).
- `for await (const x of stream)` consumes async iterables (Node streams, `ReadableStream` in browsers with async iteration support — `ReadableStream` async iteration is Newly available only since 2026-09-14).

***

#### Numbers, BigInt and Floating Point

```js
console.log(0.1 + 0.2, 0.1 + 0.2 === 0.3);                      // 0.30000000000000004 false
console.log(Math.abs(0.1 + 0.2 - 0.3) < Number.EPSILON);        // true
console.log((1.005).toFixed(2), (1.45).toFixed(1), Math.round(-0.5)); // 1.00 1.4 -0
console.log(Number.MAX_SAFE_INTEGER, 2 ** 53 === 2 ** 53 + 1);   // 9007199254740991 true
console.log(9007199254740993);                                   // 9007199254740992 — literal already rounded
console.log(Number.parseInt(0.0000005), Math.max(), Math.min()); // 5 -Infinity Infinity
console.log(1 / 0, -1 / 0, 0 / 0, 1 / -0);                       // Infinity -Infinity NaN -Infinity
```

- **Money and exact decimals**: store integer minor units (cents) or use a decimal library; never accumulate currency in binary floats. Format with `Intl.NumberFormat` (`{ style: 'currency', currency: 'EUR' }`).
- `toFixed` rounds the binary value (`(1.005).toFixed(2)` is `'1.00'`); it is for display, not for rounding money.
- `Number.parseInt(0.0000005)` is `5` because the number is stringified as `5e-7` first — never `parseInt` a number; use `Math.trunc`.
- Validate numeric input with `Number.isFinite` / `Number.isInteger` / `Number.isSafeInteger`, never with `typeof x === 'number'` alone (`NaN` and `Infinity` are numbers).
- Integers above 2^53 − 1 (database `BIGINT`, Snowflake/Twitter IDs, nanosecond timestamps) lose precision as numbers. Keep them as **strings** or `BigInt`. `JSON.parse` turns them into rounded numbers; the ES2026 reviver `context.source` (Node 21+, Newly available since 2025-03-31) gives access to the original text.
- `Math.sumPrecise(iterable)` (ES2026) sums without intermediate rounding: Chrome 147, Firefox 137, Safari 26.2 — but **no Node.js release** (26.12.1 lacks it). On Node use a compensated (Kahan/Neumaier) sum or integer arithmetic.
- Sorting numbers needs a comparator: `[10, 1, 2].sort()` is `[1, 10, 2]`; use `(a, b) => a - b` (and not with values that may be `NaN`).

```js
// BigInt (ES2020, Widely available)
console.log(2n ** 64n, 5n / 2n, -5n / 2n, typeof 10n);             // 18446744073709551616n 2n -2n bigint
try { 1n + 1; } catch (e) { console.log(e.name); }                   // TypeError — no implicit mixing
try { BigInt(1.5); } catch (e) { console.log(e.name); }              // RangeError
try { JSON.stringify({ n: 1n }); } catch (e) { console.log(e.name); } // TypeError
console.log(JSON.stringify({ n: 1n }, (k, v) => (typeof v === 'bigint' ? v.toString() : v))); // {"n":"1"}

// Reading big integers from JSON without precision loss (ES2026 source text access, Node 21+)
const big = JSON.parse('{"id": 123456789012345678901}', (k, v, ctx) => (k === 'id' ? BigInt(ctx.source) : v));
console.log(big.id); // 123456789012345678901n
```

- BigInt division truncates toward zero; there is no fractional BigInt.
- `BigInt` and `Number` compare with `==`/`<` but never mix in arithmetic; `Math.*` functions reject BigInt.
- `JSON.stringify` throws on BigInt: serialize as strings deliberately (replacer, or `toJSON`), never by patching `BigInt.prototype.toJSON` in library code.

***

#### Dates, Times and Temporal

##### `Date` pitfalls

```js
// Run with TZ=UTC for the commented output
console.log(new Date('2026-10-22').toISOString());       // 2026-10-22T00:00:00.000Z — date-only strings are UTC
console.log(new Date('2026-10-22T10:00').getHours());     // 10 — date-time strings without offset are LOCAL time
console.log(new Date(2026, 0, 31).getMonth());            // 0 — months are 0-based

const d = new Date(2026, 0, 31);
d.setMonth(1);                                            // "one month later" overflows
console.log(d.getMonth(), d.getDate());                   // 2 3 — March 3rd

console.log(String(new Date('2026-02-30')));              // V8 rolls over to Mon Mar 02 2026 …
console.log(String(new Date('2026-02-32')), Number.isNaN(new Date('nope').getTime())); // Invalid Date true
try { new Date('nope').toISOString(); } catch (e) { console.log(e.name); } // RangeError
```

- `Date` is **mutable**: passing a `Date` into a function that calls `setX` changes the caller's value. Copy (`new Date(d)`) or use timestamps.
- Date parsing beyond the ISO format (`'10/22/2026'`, `'Oct 22 2026'`) is implementation-defined; out-of-range ISO values are rolled over by V8 (`2026-02-30` → March 2) rather than rejected. Never validate user dates with `new Date(str)`; parse explicitly.
- Invalid dates do not throw on creation; `toISOString()` throws `RangeError` and every getter returns `NaN`. Check `Number.isNaN(date.getTime())`.
- Store and transmit instants as ISO 8601 strings with `Z` or as epoch milliseconds; convert to a user's zone only for display (`Intl.DateTimeFormat` with `timeZone`).
- Day arithmetic with `24 * 60 * 60 * 1000` is wrong across DST changes in local time; add calendar days with `setDate(getDate() + n)` or use Temporal.
- `Date.now()` for timestamps; `performance.now()` for measuring durations (monotonic, not affected by clock changes).

##### Temporal — status and use

`Temporal` (Stage 4, expected in **ES2027**) replaces `Date` with immutable types for instants, zoned date-times, plain dates/times and durations.[^8]

| Runtime | Temporal |
|---|---|
| Node.js | **26** and later (enabled by default; absent on 25.9.0 and 24) |
| Chrome / Edge | 144 |
| Firefox | 139 |
| Safari | **not shipped** — Baseline **Limited** in web-features 3.44.0 |

Rules: use Temporal natively only when the target is Node ≥ 26 and/or Chromium/Firefox only. Browser code that must work in Safari, and Node code for ≤ 24, needs a polyfill (`temporal-polyfill` or `@js-temporal/polyfill`) loaded **conditionally** (`globalThis.Temporal ?? polyfill`), or stays with `Date` + `Intl`.

```js
// Node 26+, Chrome 144+, Firefox 139+ (not Node 24, not Safari)
if (typeof Temporal === 'undefined') {
  console.log('Temporal not available on this runtime');
} else {
  const instant = Temporal.Instant.from('2026-10-22T10:00:00Z');
  const berlin = instant.toZonedDateTimeISO('Europe/Berlin');
  console.log(berlin.toString());                      // 2026-10-22T12:00:00+02:00[Europe/Berlin]
  console.log(berlin.add({ days: 10 }).toString());    // 2026-11-01T12:00:00+01:00[Europe/Berlin] (DST-aware)

  const jan31 = Temporal.PlainDate.from('2026-01-31');
  console.log(jan31.add({ months: 1 }).toString());    // 2026-02-28 — clamps, unlike Date
  try { jan31.add({ months: 1 }, { overflow: 'reject' }); } catch (e) { console.log(e.name); } // RangeError
  try { Temporal.PlainDate.from('2026-02-30'); } catch (e) { console.log(e.name); }           // RangeError — strict parsing

  const a = Temporal.PlainDate.from('2026-01-01');
  const b = Temporal.PlainDate.from('2026-01-01');
  console.log(a === b, a.equals(b), Temporal.PlainDate.compare(a, b)); // false true 0
  try { a < b; } catch (e) { console.log(e.name); }   // TypeError — valueOf throws; use compare()
}
```

- Pick the type: `Instant` for timestamps, `ZonedDateTime` for events in a named zone, `PlainDate` for birthdays and due dates, `Duration` for spans. Storing a `PlainDateTime` for something that happened at a real moment loses the zone.
- Temporal objects are immutable: `date.add({ days: 1 });` without using the result is a bug.
- Compare with `equals`/`compare`, never `<`/`===`.

***

#### Strings and Unicode

```js
const s = 'naïve 👍';
console.log(s.length, [...s].length);                           // 8 7 — length counts UTF-16 code units
console.log('👍'.slice(0, 1).length, '👍'.slice(0, 1) === '\uD83D'); // 1 true — slicing can split a surrogate pair
const seg = new Intl.Segmenter('en', { granularity: 'grapheme' });
console.log([...seg.segment('👩‍👩‍👧')].length, [...'👩‍👩‍👧'].length);   // 1 5
console.log('\uD800'.isWellFormed(), '\uD800'.toWellFormed() === '\uFFFD'); // false true
console.log('\u00E9' === 'e\u0301', '\u00E9'.normalize() === 'e\u0301'.normalize()); // false true
console.log('İ'.toLowerCase().length, 'I'.toLocaleLowerCase('tr'), 'ß'.toUpperCase()); // 2 ı SS
console.log(['b', 'a', 'Ä'].sort(), ['b', 'a', 'Ä'].sort(new Intl.Collator('de').compare)); // [ 'a', 'b', 'Ä' ] [ 'a', 'Ä', 'b' ]
```

- `length`, `slice`, `substring` and indexing work on UTF-16 code units. For user-visible length, truncation and cursor movement use `Intl.Segmenter` (Node 16; Baseline Newly 2024-04-16, Widely since 2026-10-16); `[...str]` splits by code point, which still breaks emoji sequences and combining marks.
- Compare user-entered text after `normalize('NFC')`; usernames and identifiers additionally need case folding and a confusables policy.
- `toLowerCase()`/`toUpperCase()` are locale-independent but not length-preserving (`'ß'.toUpperCase()` is `'SS'`); use `toLocaleLowerCase(locale)` for user-visible text and plain `toLowerCase()` for protocol identifiers.
- Sort user-visible strings with `Intl.Collator`/`localeCompare`, not the default code-unit sort.
- Lone surrogates make `encodeURIComponent` throw `URIError` and `TextEncoder` emit U+FFFD; check with `isWellFormed()` (ES2024, Node 20, Widely available).
- `'abc'[-1]` is `undefined`; `'abc'.at(-1)` is `'c'`. `substr` is legacy; `substring` swaps its arguments when `start > end`, `slice` does not.

```js
// String#replace / replaceAll treat `$` in the replacement string specially — even for string patterns
const template = 'Total: AMOUNT';
const userText = '$& and $1';
console.log(template.replace('AMOUNT', userText));        // Total: AMOUNT and $1
console.log(template.replace('AMOUNT', () => userText));  // Total: $& and $1 — replacer function is literal
console.log('a-b-c'.replace('-', '+'), 'a-b-c'.replaceAll('-', '+')); // a+b-c a+b+c
```

- `replace` with a string pattern replaces only the **first** occurrence; `replaceAll` (ES2021) replaces all and throws `TypeError` for a non-global RegExp.
- Untrusted replacement text must go through a replacer function (or escape `$` as `$$`).

***

#### Regular Expressions

##### Correctness pitfalls

```js
// The g and y flags make test()/exec() stateful through lastIndex
const re = /a/g;
console.log(re.test('a'), re.test('a'), re.lastIndex);      // true false 0

// Anchors: ^ and $ match at line boundaries only with the m flag; . does not match \n without s
console.log(/^b$/.test('a\nb'), /^b$/m.test('a\nb'), /a.b/.test('a\nb'), /a.b/s.test('a\nb')); // false true false true

// matchAll needs g; replaceAll with a regex needs g
try { 'x'.matchAll(/x/); } catch (e) { console.log(e.name); } // TypeError

// Named groups (ES2018) instead of positional indices
const { groups } = /(?<year>\d{4})-(?<month>\d{2})/.exec('2026-10');
console.log(groups.year, groups.month);                    // 2026 10

// Unicode: use u or v so that code points and \p{…} work
console.log(/^.$/.test('👍'), /^.$/u.test('👍'), /\p{Lu}/u.test('Ä')); // false true true
```

- Never reuse a `/g` or `/y` regex across calls to `test`/`exec` on different strings (module-level constants!) — `lastIndex` carries over. Use a non-global regex for tests, or `String.prototype.match`/`matchAll`.
- Use the `u` flag (or `v`, ES2024, Node 20, Widely available) for any pattern that handles non-ASCII text.
- Validation patterns need both anchors (`^…$`); `/\d+/.test('abc1')` is `true`.
- `\d`, `\w` and `\b` are ASCII-only even with `u`; use `\p{Nd}`, `\p{L}` for Unicode.

##### Building patterns from input

```js
const userInput = 'a.b*c';

// BAD: metacharacters in input change the pattern (and can cause ReDoS)
console.log(new RegExp(userInput).test('aXbbbc'));          // true

// GOOD: escape — RegExp.escape (ES2025): Node 24, Chrome 136, Firefox 134, Safari 18.2 (Newly available 2025-05-01)
const escape = RegExp.escape ?? ((s) => s.replace(/[\\^$.*+?()[\]{}|/-]/g, '\\$&'));
console.log(new RegExp(escape(userInput)).test('aXbbbc'), new RegExp(`^${escape(userInput)}$`).test('a.b*c')); // false true
```

On Node 22 and older browsers `RegExp.escape` is missing: use a tested fallback (as above) or, better, avoid regexes for literal matching (`includes`, `startsWith`, `split`).

##### ReDoS (catastrophic backtracking)

JavaScript regex engines backtrack. Nested or overlapping quantifiers — `(a+)+`, `(\w+\s?)*`, `(a|aa)+`, `(.*a){10}` — take exponential time on crafted non-matching input and block the event loop (a denial of service for a server, a frozen tab in a browser).

```js
const evil = /^(a+)+$/;
const input = 'a'.repeat(22) + '!';

let t = performance.now();
evil.test(input);
const slowMs = performance.now() - t;

const safe = /^a+$/;                          // same language, no nested quantifier
t = performance.now();
safe.test(input);
const fastMs = performance.now() - t;
console.log(slowMs > 50 * fastMs, slowMs > 20);  // true true — tens of ms for 23 characters; each extra 'a' roughly doubles it
```

Review rules:

- Flag nested quantifiers and alternations whose branches can match the same text, applied to **untrusted input** (headers, query strings, uploaded text, user-provided patterns).
- Limit input length **before** matching (`if (s.length > 256) reject`).
- Never build a regex from user input without escaping; never accept user-supplied regexes server-side unless they run in a guarded engine (for example the linear-time `re2` binding) or a worker with a timeout.
- Lint with `eslint-plugin-regexp` (rules such as `regexp/no-super-linear-backtracking`) or a dedicated ReDoS checker in CI.
- Prefer simple parsing (`split`, `indexOf`, `URL`, `URLSearchParams`, a real parser for HTML/CSV/email) over complex regexes.

##### Modern regex features — check the target

- RegExp modifiers `(?i:…)` and duplicate named groups in alternatives (ES2025): Node 23/24, not Node 22 (`SyntaxError`); modifiers need Safari 26 (Newly available 2025-09-15).
- `v` flag with set operations `[\p{L}--[a-z]]` (ES2024): Node 20, Widely available.
- `d` flag (`hasIndices`, ES2022): Node 16, Widely available.

***

#### JSON

```js
console.log(JSON.stringify({ u: undefined, f() {}, n: NaN, i: Infinity, d: new Date(0), m: new Map([[1, 2]]) }));
// {"n":null,"i":null,"d":"1970-01-01T00:00:00.000Z","m":{}}
console.log(JSON.stringify([undefined, () => 1]));          // [null,null]

try { JSON.parse("{'single': 1}"); } catch (e) { console.log(e.name); } // SyntaxError
const parsed = JSON.parse('{"__proto__": {"polluted": true}}');
console.log(Object.hasOwn(parsed, '__proto__'), {}.polluted); // true undefined — own key, not yet pollution
```

- `JSON.parse` on untrusted input must be wrapped in `try/catch` and followed by **schema validation** (Zod, Valibot, Ajv, …). A successful parse says nothing about the shape.
- `JSON.parse` creates an own `__proto__` property instead of changing the prototype — harmless until code merges the result into another object (see "Prototype pollution").
- `JSON.stringify` drops `undefined`, functions and symbols, turns `NaN`/`Infinity` into `null`, `Map`/`Set` into `{}`, throws on `BigInt` and cycles. Round-trips of `Date` produce strings, not dates.
- Large `JSON.parse`/`JSON.stringify` calls are synchronous and block the event loop (tens of MB take hundreds of milliseconds); stream or move to a worker.

***
#### Security

##### Code injection: `eval` and friends

- Flag `eval(…)`, `new Function(string)`, `setTimeout(string)`/`setInterval(string)`, `vm.runInThisContext`/`vm.runInNewContext` on untrusted input (Node's `vm` is **not** a security boundary), and template engines compiled from user input.
- Parse data with `JSON.parse`, evaluate expressions with a dedicated parser, and pass functions (not strings) to timers.
- In browsers, a Content Security Policy without `'unsafe-eval'` blocks all of the above; code that needs `unsafe-eval` should be treated as a finding.

##### XSS sinks (browser)

| Sink | Risk | Safe alternative |
|---|---|---|
| `el.innerHTML = s`, `outerHTML`, `insertAdjacentHTML`, `document.write` | HTML injection; `<img onerror>` executes even though `<script>` inserted via `innerHTML` does not | `textContent`, `createElement` + properties, a sanitizer (DOMPurify) for real HTML |
| `el.setAttribute('on…', s)`, `el.onclick = 'string'` | inline handler injection | `addEventListener` with a function |
| `a.href`/`iframe.src`/`location = userUrl` | `javascript:` URLs execute | parse with `new URL()` and allow only `http:`/`https:` |
| `setHTMLUnsafe`, `Range.createContextualFragment`, `DOMParser` + insertion | HTML injection | as above |
| React `dangerouslySetInnerHTML`, Vue `v-html`, Angular `bypassSecurityTrust*` | framework escape hatches | render text; sanitize when HTML is required |
| `postMessage` handlers | any origin can send messages | check `event.origin` against an allow-list, validate `event.data` |

```js
// browser — verified in Chromium 141
const box = document.createElement('div');
document.body.append(box);
window.__xss = 0;
box.innerHTML = '<script>window.__xss += 1<\/script>';            // not executed
box.innerHTML = '<img src="data:," onerror="window.__xss += 10">'; // executed
await new Promise((r) => setTimeout(r, 100));

const safe = document.createElement('div');
safe.textContent = '<img src=x onerror=alert(1)>';                 // rendered as text
const link = document.createElement('a');
link.href = 'javascript:alert(1)';
const isSafeUrl = (u) => ['http:', 'https:'].includes(new URL(u, location.href).protocol);
console.log(window.__xss, safe.children.length, isSafeUrl(link.href)); // 10 0 false
```

- **Trusted Types** (`require-trusted-types-for 'script'` in CSP) turn DOM-XSS sinks into type errors; Baseline Newly available since 2026-02-24 (Chrome 83, Firefox 148, Safari 26). The **Sanitizer API** (`el.setHTML()`) is Limited (Chrome 146, Firefox 148, no Safari) — do not rely on it without a fallback.
- Escape for the right context: HTML text, HTML attribute, URL, CSS and JavaScript string contexts need different encoding. Template engines and JSX escape text by default; the escape hatches above disable it.
- Set cookies `HttpOnly`, `Secure`, `SameSite`; do not store session tokens in `localStorage` (readable by any XSS).

##### Prototype pollution

Merging untrusted objects into plain objects can write to `Object.prototype` through `__proto__`, `constructor.prototype` or crafted paths (`a.__proto__.isAdmin`), changing behaviour of every object in the process.

```js
// BAD: naive recursive merge
function mergeBad(target, source) {
  for (const key in source) {
    if (typeof source[key] === 'object' && source[key] !== null) {
      target[key] ??= {};
      mergeBad(target[key], source[key]);
    } else {
      target[key] = source[key];
    }
  }
  return target;
}
mergeBad({}, JSON.parse('{"__proto__": {"isAdmin": true}}'));
console.log({}.isAdmin); // true — every object is now "admin"
delete Object.prototype.isAdmin;

// GOOD: reject dangerous keys, copy only own keys, use null-prototype targets
const FORBIDDEN = new Set(['__proto__', 'constructor', 'prototype']);
function merge(target, source) {
  for (const key of Object.keys(source)) {
    if (FORBIDDEN.has(key)) continue;
    const value = source[key];
    if (value !== null && typeof value === 'object' && !Array.isArray(value)) {
      if (!Object.hasOwn(target, key)) target[key] = Object.create(null);
      merge(target[key], value);
    } else {
      target[key] = value;
    }
  }
  return target;
}
merge(Object.create(null), JSON.parse('{"__proto__": {"isAdmin": true}, "a": {"b": 1}}'));
console.log({}.isAdmin); // undefined
```

- Use `Map` for user-keyed data, `Object.create(null)` for dictionaries, and validate parsed input with a schema that rejects unknown keys.
- Look for vulnerable patterns in: deep merge/clone/extend helpers, `lodash.set`-style path setters, query-string parsers that build nested objects (`a[__proto__][x]=1`), and config loaders.
- Defensive reads: `if (options.isAdmin)` reads inherited properties — use `Object.hasOwn(options, 'isAdmin')` for security decisions on objects built from input.
- Node.js offers `--disable-proto=delete` (removes the `__proto__` accessor) and `--frozen-intrinsics` (experimental) as hardening.

##### Other injection classes

- **SQL/NoSQL**: parameterized queries only; never template literals into SQL (`` `… WHERE id = ${id}` ``). For MongoDB-style queries, reject objects where strings are expected (`{ "$ne": null }` login bypass).
- **Command injection** (Node): `child_process.execFile`/`spawn` with an argument array and no shell; never `exec` with interpolated strings.
- **Path traversal** (Node): resolve and check containment (`path.resolve(root, input)` must start with `root + path.sep`); reject `\0`.
- **SSRF**: server-side `fetch(userUrl)` needs an allow-list of hosts and protocols; redirects and DNS rebinding bypass naive checks.
- **Open redirects**: `location = params.get('next')` must only accept same-origin relative paths.
- **Regex injection / ReDoS**: see "Regular Expressions".

##### Randomness, secrets and comparisons

- `Math.random()` is **not** cryptographically secure. Tokens, IDs used for authorization, password reset codes and nonces need `crypto.getRandomValues()` or `crypto.randomUUID()` (Web Crypto global: browsers, and Node ≥ 19 — verified absent on 18.20.8 in a module, present on 20.20.2; use `node:crypto` on older Node).
- In browsers `crypto.randomUUID()` and `crypto.subtle` exist only in **secure contexts** (HTTPS, `localhost`); on plain-HTTP pages and `data:` URLs `crypto.randomUUID` is `undefined` (verified in Chromium 141), while `crypto.getRandomValues()` works everywhere.
- Compare secrets in constant time (`crypto.timingSafeEqual` in Node; HMAC-compare in browsers), never with `===`.
- Never ship secrets in browser bundles: anything in client JS (API keys, `process.env.*` inlined by bundlers) is public.

```js
const token = crypto.randomUUID();                          // global Web Crypto (Node 19+, browsers)
const bytes = crypto.getRandomValues(new Uint8Array(16));
console.log(/^[0-9a-f-]{36}$/.test(token), bytes.length);   // true 16
```

##### Dependency hygiene and supply chain

- Commit the lockfile (`package-lock.json`, `pnpm-lock.yaml`, `yarn.lock`, `bun.lock`) and install with `npm ci` / `pnpm install --frozen-lockfile` in CI.
- Review new dependencies: maintenance, download count, install scripts, transitive size. A 10-line helper is cheaper than a dependency tree. Many classic utilities are now built in: `structuredClone` (deep clone), `Object.groupBy`, `Array.prototype.at`/`flat`/`toSorted`, `crypto.randomUUID` (uuid v4), `fetch`, `AbortSignal.timeout`, `URL`/`URLSearchParams`, `Intl` (formatting, plural rules, relative time), `node:util.parseArgs`, `node:test` — check the target before removing a polyfill.
- Disable or allow-list lifecycle scripts for untrusted packages (`npm config set ignore-scripts true`, pnpm's allow-list of packages that may run build scripts) — install scripts are a common malware vector in npm supply-chain attacks.
- Run `npm audit` (or an SCA tool) in CI, keep dependencies updated with Renovate/Dependabot, and prefer packages with provenance attestations.
- Do not depend on Git URLs or `latest` tags; pin exact or caret ranges with a lockfile.
- Browser: load third-party scripts with Subresource Integrity (`integrity="sha384-…"`) or self-host them.

***

#### Performance

- **Measure first** (`performance.now()`, browser DevTools, `node --cpu-prof`). Do not accept micro-optimizations without numbers.
- **Algorithmic complexity dominates**: flag `includes`/`find`/`indexOf` inside loops over large collections (use `Set`/`Map`), `reduce` with object spread (`{ ...acc, [k]: v }` is O(n²)), and repeated `array.shift()` on large arrays (O(n) each).
- **Avoid blocking the main thread**: large synchronous JSON, regexes on big input, sync file/crypto/zlib APIs in request paths; split or move work to workers.
- **DOM**: batch reads and writes (layout thrashing: reading `offsetHeight` after each write forces layout), use event delegation for large lists instead of one listener per item, virtualize long lists, use `requestAnimationFrame` for visual updates and passive listeners for scroll/touch.
- **Memory**: unbounded caches, listeners never removed, timers never cleared, closures capturing large objects, and detached DOM nodes held in arrays are the usual leaks. Use `WeakMap`/`WeakRef` for caches keyed by objects (but never for correctness — GC timing is unspecified).
- **String building**: `+=` in a loop is fine in modern engines; `array.join` is not faster per se. Do not flag it without measurement.
- **Creating functions in loops** is cheap; do not flag it by itself. Flag it when it defeats memoization (a new callback prop on every render) or when the same handler could be shared through delegation.
- **Startup**: lazy-load rarely used modules with `import()`, avoid barrel files inside applications, and keep top-level module code cheap.
- **Bundle size**: prefer ESM packages (tree shaking), import specific functions (`import debounce from 'lodash-es/debounce.js'`), and check what polyfills the build injects for the browserslist target.

```js
// O(n*m) vs O(n+m)
const ids = Array.from({ length: 20_000 }, (_, i) => i);
const wanted = Array.from({ length: 20_000 }, (_, i) => i * 2);

let t = performance.now();
const slow = ids.filter((id) => wanted.includes(id)).length;
const slowMs = performance.now() - t;

t = performance.now();
const wantedSet = new Set(wanted);
const fast = ids.filter((id) => wantedSet.has(id)).length;
const fastMs = performance.now() - t;
console.log(slow === fast, slowMs > 5 * fastMs); // true true (typically 30–90× slower on Node 24)
```

***

#### Testing

- Use the runner the project already uses (Vitest, Jest, Mocha, `node:test`, Playwright for browsers). `node:test` is stable since Node 20 and needs no dependency.
- **Async tests must await** the code under test: a test that forgets `await` (or `return`) passes before the assertion runs. With Jest/Vitest, `expect(promise).rejects.toThrow()` must itself be awaited.
- Assert on **error types/codes**, not messages, and assert that code under test actually throws (`assert.rejects`, `expect(...).rejects`).
- Use **fake timers** for debounce/throttle/retry logic instead of real sleeps; reset them after each test.
- Do not depend on test order or shared mutable module state; restore mocks and stubs (`afterEach`). Mocks of `fetch`, `Date.now()` and `Math.random()` must be undone.
- Pin time zones and locales in CI (`TZ=UTC`, explicit `locale` arguments) — date and `Intl` output differ between developer machines and CI.
- Table-driven tests for rule-heavy functions (parsers, validators, money math), including edge cases: empty string, `0`, `-0`, `NaN`, very large numbers, Unicode (emoji, combining marks), `null`/`undefined`, `__proto__` keys.
- Test against the **lowest supported runtime** in CI (`engines.node` minimum, the oldest browser in `browserslist`), not only the newest — this is where version-tied built-ins fail.

```js
import { test } from 'node:test';
import assert from 'node:assert/strict';

async function parsePort(value) {
  const port = Number(value);
  if (!Number.isInteger(port) || port < 1 || port > 65_535) {
    throw Object.assign(new RangeError(`invalid port: ${value}`), { code: 'E_PORT' });
  }
  return port;
}

const cases = [['80', 80], ['65535', 65_535]];
for (const [input, expected] of cases) {
  test(`parsePort(${input})`, async () => assert.equal(await parsePort(input), expected));
}
for (const bad of ['', '0', '65536', '1.5', 'abc']) {
  test(`parsePort rejects ${JSON.stringify(bad)}`, async () => {
    await assert.rejects(parsePort(bad), { code: 'E_PORT' });
  });
}
```

***

#### Tooling and Linting

##### ESLint (flat config)

ESLint 10 (2026) supports only the flat config format (`eslint.config.js`); `.eslintrc.*` files are ignored.[^9] A baseline configuration:

```javascript
// eslint.config.js
import js from '@eslint/js';
import globals from 'globals';

export default [
  js.configs.recommended,
  {
    languageOptions: {
      ecmaVersion: 'latest',
      sourceType: 'module',
      globals: { ...globals.browser },          // or globals.node — match the runtime
    },
    rules: {
      eqeqeq: ['error', 'always', { null: 'ignore' }],
      'no-var': 'error',
      'prefer-const': 'error',
      'no-shadow': 'error',
      'no-throw-literal': 'error',
      'prefer-promise-reject-errors': 'error',
      'no-await-in-loop': 'warn',
      'no-promise-executor-return': 'error',
      'require-atomic-updates': 'warn',
      'no-implied-eval': 'error',
      'no-new-func': 'error',
      'no-param-reassign': ['error', { props: true }],
      'no-restricted-syntax': ['error', 'ForInStatement'],
      'object-shorthand': 'error',
      'prefer-template': 'error',
    },
  },
];
```

- `ecmaVersion: 'latest'` only controls **parsing**; it does not check whether APIs exist on the target. Use `eslint-plugin-compat` (browserslist-driven) or `eslint-plugin-n`'s `n/no-unsupported-features/*` rules (driven by `engines.node`) to flag built-ins the target lacks.
- For TypeScript or `// @ts-check` JavaScript, `typescript-eslint`'s type-aware rules `no-floating-promises`, `no-misused-promises` and `await-thenable` catch the most common async bugs.
- `eslint-plugin-regexp` catches ReDoS-prone and incorrect patterns; `eslint-plugin-security` flags some injection sinks (noisy — triage).
- Formatting belongs to Prettier (or the project's formatter), not to review comments; do not hand-align code.

##### Type checking JavaScript

`// @ts-check` or `"checkJs": true` in `jsconfig.json`/`tsconfig.json` with JSDoc types gives type checking without a build step. Set `compilerOptions.lib` to the **runtime's** edition (for example `["ES2023", "DOM"]` for a Node 20 / Safari 16 target), so that APIs the target lacks fail type checking instead of failing at runtime.

##### Build targets

- Set the transpiler target from the real runtime list (browserslist / `engines`) and check which polyfills are injected. Over-transpiling (ES5 output for evergreen browsers) costs size and speed; under-transpiling ships syntax the target cannot parse.
- Ship modern ESM to modern browsers; `<script type="module">` is Widely available.
- Do not commit generated bundles unless the project does so deliberately; never edit them by hand.

***

#### Common Review Findings (BAD → GOOD)

```js
// 1. Loose equality / truthiness bugs
const qty = 0;
const shown = qty || 'n/a';               // BAD: 0 becomes 'n/a'
const shownOk = qty ?? 'n/a';             // GOOD
console.log(shown, shownOk);              // n/a 0

// 2. Async callbacks in array methods
const ids = [1, 2, 3];
const results = [];
await Promise.all(ids.map(async (id) => results.push(id * 2))); // GOOD (BAD: ids.forEach(async …))
console.log(results.length);              // 3

// 3. Missing radix / unchecked number parsing
const page = Number.parseInt('08', 10);
console.log(Number.isInteger(page) ? page : 1); // 8

// 4. In-place sort of shared data
const shared = Object.freeze([3, 1, 2]);
console.log(shared.toSorted());           // [ 1, 2, 3 ] — shared.sort() would throw on a frozen array

// 5. Building URLs by concatenation
const q = 'a&b=c';
const url = new URL('https://api.example/search');
url.searchParams.set('q', q);             // GOOD: encoded
console.log(url.href);                    // https://api.example/search?q=a%26b%3Dc

// 6. Losing the original error
try {
  try { JSON.parse('{'); } catch (err) { throw new Error('config invalid', { cause: err }); }
} catch (e) {
  console.log(e.message, e.cause.name);   // config invalid SyntaxError
}

// 7. Object used as a dictionary for user keys
const counts = new Map();
for (const word of ['constructor', 'toString', 'constructor']) counts.set(word, (counts.get(word) ?? 0) + 1);
console.log(counts.get('constructor'));   // 2 — a plain {} would start from a function

// 8. Date arithmetic via milliseconds in local time — prefer calendar operations
const due = new Date(Date.UTC(2026, 9, 22));
due.setUTCDate(due.getUTCDate() + 10);
console.log(due.toISOString().slice(0, 10)); // 2026-11-01
```

Additional patterns to flag:

- `JSON.parse(JSON.stringify(x))` for cloning → `structuredClone(x)` (Node 17+, Widely available).
- `arr.indexOf(x) !== -1` → `arr.includes(x)` (also finds `NaN`).
- `Object.keys(obj).forEach(k => …obj[k]…)` → `for (const [k, v] of Object.entries(obj))`.
- `new Promise(async (resolve, reject) => …)` → an `async` function (errors thrown in an async executor are lost).
- `await` inside `for` loops over independent items → `Promise.all` with bounded concurrency.
- `catch (e) { console.log(e) }` that swallows the error in library code → rethrow or return a typed result.
- `setTimeout` used to "wait for" state → await the actual event/promise.
- `document.querySelector(...)` results used without a `null` check.
- `fetch()` without checking `response.ok` — `fetch` only rejects on network errors, not on HTTP 4xx/5xx.
- `localStorage.getItem` parsed with `JSON.parse` without `try/catch` (corrupt or foreign values, and `localStorage` throws in some privacy modes).
- `typeof x === 'object'` without excluding `null`.
- A feature newer than the target (see the tables) used without a fallback.

***

#### Production / Review Checklist

##### Target and compatibility

- [ ] Runtime target identified (`engines`, `.nvmrc`, Docker image, CI matrix, `browserslist`, transpiler target, `tsconfig` `lib`).
- [ ] Every built-in used exists on the **oldest** target (edition/Node/Baseline tables above) or has a conditional polyfill/fallback.
- [ ] Node.js version is a maintained line (22, 24, 26) on its latest patch; no Node 18/20/25 in production; no Node 27 alpha in production.
- [ ] No unshipped or removed features (decorators, import `assert`, `Iterator.zip`/`join`/`includes`/`chunks`/`windows` in Node or Safari-targeted code, `Math.sumPrecise` on Node, `Temporal`/`Error.isError`/`using` in Safari-targeted code without polyfill).

##### Correctness

- [ ] `const`/`let`, no `var`; strict mode (ESM or `'use strict'`).
- [ ] `===` everywhere except `== null`; `??` where `0`/`''`/`false` are valid.
- [ ] Explicit number parsing and validation (`Number.isFinite`, radix for `parseInt`); no money in floats; big IDs as strings/BigInt.
- [ ] No in-place mutation of arguments or shared arrays (`sort`, `reverse`, `splice`); shallow vs deep copy understood.
- [ ] `this` preserved for callbacks; no arrow methods relying on dynamic `this`.
- [ ] Dates: no ambiguous parsing, no local-time millisecond arithmetic, invalid dates detected; Temporal only on supported targets.
- [ ] Strings: Unicode-aware length/truncation; normalized comparisons; locale-aware sorting for UI.
- [ ] Regexes: anchored validation, no stateful `/g` reuse, escaped user input, no ReDoS-prone patterns on untrusted input.

##### Async and errors

- [ ] Every promise awaited, returned or explicitly handled; no async callbacks in `forEach`/`filter`.
- [ ] Independent work runs concurrently; fan-out bounded.
- [ ] Timeouts and cancellation via `AbortSignal` for network and long-running work.
- [ ] Only `Error` objects thrown; causes attached; errors classified by type/code; no swallowed errors.
- [ ] Unhandled rejections and uncaught errors reported (Node: process exits; browser: `unhandledrejection` handler sends to monitoring).

##### Security

- [ ] No `eval`/`new Function`/string timers; no untrusted HTML into DOM sinks; URLs validated for protocol.
- [ ] No prototype pollution paths (merges, path setters, query parsers); `Map`/null-prototype objects for user keys.
- [ ] Parameterized queries; no shell interpolation; path containment checks; SSRF allow-lists.
- [ ] Secure randomness for tokens; no secrets in client bundles.
- [ ] Lockfile committed, CI installs frozen, install scripts restricted, dependencies audited.

##### Quality

- [ ] ESLint (flat config) passes, with compat rules for the target; formatter applied.
- [ ] Tests await async code, cover edge cases (0, `NaN`, empty, Unicode, `__proto__`), pin `TZ`/locale, run on the oldest supported runtime.
- [ ] No unbounded caches, unremoved listeners or uncleared timers.
