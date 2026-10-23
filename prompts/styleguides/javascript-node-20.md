### JavaScript on Node.js 20: The Complete Production Developer Guide

#### Overview

**Node.js 20 ("Iron")** was released on **April 18, 2023**, entered Active LTS with 20.9.0 on October 24, 2023, moved to Maintenance LTS on October 22, 2024, and reached **end-of-life on April 30, 2026**.[^1][^2] The final release of the line is **20.20.2 (March 24, 2026)**, a security release; no 20.x release has been published since.[^3][^4] As of October 2026 the supported lines are Node.js 26 (Current, 26.12.1 of October 21, 2026, becoming LTS on October 28, 2026), Node.js 24 (Maintenance LTS since October 20, 2026, 24.22.1) and Node.js 22 (Maintenance LTS until April 30, 2027, 22.23.4). Node.js 27 has not been released: under the new one-major-per-year schedule it starts as an alpha channel (planned from October 28, 2026) ahead of an April 2027 release.[^2][^3][^5]

**Node.js 20 receives no more fixes.** The June, July and October 2026 security releases fixed vulnerabilities (several rated High, including TLS hostname-verification bypasses and a Brotli decoder out-of-bounds write) only in 22.x, 24.x and 26.x; the project states that end-of-life lines are always affected by such releases.[^6] A reviewer of a Node 20 project should therefore (1) review the code correctly *for Node 20* — never suggest APIs that 20.20.2 does not have — and (2) flag the runtime itself as an upgrade item: move to Node.js 24 (LTS until April 2028) or at least Node.js 22 (until April 2027).

Node.js 20 ships **V8 11.3** (Chromium 113), npm 10.8.2 (since 20.17.0), libuv 1.46, OpenSSL 3.0, undici 6.x as the `fetch()` implementation, Ada 2.x as the URL parser and Corepack 0.34.6; 20.20.2 bundles ICU 78.2 / Unicode 17.[^1][^7] JavaScript language features are fixed by the V8 version and do **not** change across 20.x minors; Node.js *runtime* APIs, on the other hand, kept arriving in semver-minor releases until 20.19.0 (March 2025), including large backports from Node.js 22 such as `require()` of ES modules. Every section that covers Node 20-specific behaviour is marked **New in 20** (vs. Node 18/19) or **New in 20.x** with the exact minor.

**Scope.** This guide covers JavaScript (CommonJS and ES modules) running on Node.js 20.x, the ECMAScript features its V8 supports, and the Node.js 20 runtime APIs. It is also selected for projects pinned to **Node.js 21.x** (an odd, non-LTS line released October 17, 2023 and end-of-life since June 1, 2024). For 21.x, treat everything below as the upper bound: none of the Node 22+ features exist, and several 20.13–20.19 backports are *missing* from 21.7.3 (see "Node.js 21.x projects"). The guide is self-contained; it does not assume any other style guide.

Global truths about JavaScript on Node.js 20:

- **One thread runs your JavaScript.** The event loop executes callbacks; CPU-bound work blocks every request. Use `worker_threads` for CPU work and keep I/O asynchronous.
- **Two module systems.** `.cjs` and `.js` under `"type": "commonjs"` (or no `type`) are CommonJS; `.mjs` and `.js` under `"type": "module"` are ES modules. Since **20.19.0**, `require()` can load synchronous ES modules and ambiguous files are syntax-detected (see Modules).[^8]
- **Unhandled promise rejections crash the process** (`--unhandled-rejections=throw` is the default since Node 15). A forgotten `await` or missing `.catch()` is a production outage, not a log line.
- **`fetch`, `Blob`, `File`, Web Streams, `AbortController`, `structuredClone`, `BroadcastChannel`, `CompressionStream` and Web Crypto (`globalThis.crypto`) are globals.** `WebSocket`, `EventSource` and `navigator` are not (flagged or absent in 20).
- **V8 11.3 = ES2023 plus a few ES2024 pieces.** You have `Array.prototype.toSorted()`/`with()`, `findLast()`, the RegExp `v` flag, `String.prototype.isWellFormed()`, resizable `ArrayBuffer`, `Atomics.waitAsync`. You do **not** have `Object.groupBy`, `Promise.withResolvers`, `Array.fromAsync`, Set methods, iterator helpers or `using` declarations.

***

#### Version-Aware Review

Before claiming that an API is missing, deprecated, or should be replaced, establish the exact runtime:

1. Read `engines.node` in `package.json`, `.nvmrc` / `.node-version`, `volta.node`, the `FROM node:20…` line of the Dockerfile, CI matrices (`actions/setup-node` `node-version`), and `@types/node` major. A range like `">=20"` does not pin a minor; `"^20.19.0"` does.
2. Many Node 20 runtime APIs depend on the **minor** version. `import.meta.dirname` needs ≥ 20.11.0, `process.loadEnvFile()` ≥ 20.12.0, `process.getBuiltinModule()` ≥ 20.16.0, `require(esm)` without a flag ≥ 20.19.0. If the project pins only `20.x` and the code uses such an API, recommend pinning the minimum minor (`"engines": { "node": ">=20.19.0" }`) rather than calling the API unavailable.
3. Language features do **not** vary by minor (V8 11.3.244 in every 20.x release). Do not flag `toSorted()` or the `v` flag as "too new" for Node 20; do flag `Object.groupBy()`, `Promise.withResolvers()`, `Set.prototype.union()`, `Iterator.prototype.map()` and `using` as unavailable.
4. Say "not available in Node 20" only when the pinned version proves it. The "Do not use or recommend" list below was verified by executing feature probes on 20.20.2 and the adjacent releases 18.20.8, 19.9.0, 21.7.3 and 22.23.4.
5. Remember the reviewer's second duty: Node 20 is end-of-life. Recommend the upgrade once, in a dedicated finding, instead of rewriting code to newer APIs that the project cannot run.

```js
// Runtime feature checks belong at startup, not scattered through the code base.
const [major, minor] = process.versions.node.split('.').map(Number);
const hasRequireEsm = process.features.require_module === true; // property exists from 20.19.0
if (major === 20 && minor < 19) {
  console.warn(`Node ${process.version}: upgrade to >= 20.19.0 (and plan the move to 22/24 LTS)`);
}
console.log({ major, minor, hasRequireEsm, v8: process.versions.v8 });
```

***

#### What's New in Node.js 20 (vs. 18 and 19)

Node.js 20.0.0 also contains all semver-major changes of the non-LTS 19.x line, so projects coming from Node 18 meet both sets. "18.x has it too" notes reflect backports into 18.x that a project pinned to an older 18 minor may not have.

##### Language (V8 11.3)

| Feature | Status in 20.20.2 | Boundary (tested) |
|---|---|---|
| Change-array-by-copy: `toSorted`, `toReversed`, `toSpliced`, `with` (Array and TypedArray) | ✅ | absent in 18.20.8 and 19.9.0 |
| RegExp `v` flag (set notation, string properties such as `\p{RGI_Emoji}`) | ✅ | `SyntaxError` in 18.20.8 / 19.9.0 |
| `String.prototype.isWellFormed()` / `toWellFormed()` | ✅ | absent in 18 / 19 |
| Resizable `ArrayBuffer`, growable `SharedArrayBuffer` | ✅ | absent in 18 / 19 |
| Symbols as `WeakMap` keys / `WeakRef` targets | ✅ | `TypeError` in 19.9.0 (works in 18.20.8) |
| `Intl.NumberFormat` v3 (`formatRange`, `roundingIncrement`, …) | ✅ | absent in 18.20.8, present in 19.9.0 |
| `Array.prototype.findLast()`, `Object.hasOwn()`, `.at()`, error `cause`, class static blocks, `#x in obj`, RegExp `d` flag, top-level `await` (ESM) | ✅ | already in 18 |

##### Runtime highlights of 20.0.0[^1][^9]

- **Stable test runner** (`node:test`, `node --test`).
- **Experimental Permission Model** (`--experimental-permission`, `--allow-fs-read`, `--allow-fs-write`, `--allow-child-process`, `--allow-worker`).
- **ESM loader hooks run off the main thread**; `import.meta.resolve()` returns a string synchronously.
- **Single executable applications** now require a prepared blob (`--experimental-sea-config`).
- `File` is a stable global; `util.parseArgs()` is stable; `crypto.DEFAULT_ENCODING` removed; `process.exit(code)` / `process.exitCode` reject non-integer values (DEP0164 End-of-Life); `url.parse()` warns on invalid ports (DEP0170); `net` connects with **Happy Eyeballs** (`autoSelectFamily: true`) by default; `new WASI()` requires `version`.
- From the 19.x line: `http.globalAgent`/`https.globalAgent` use **keep-alive** (5 s timeout) by default; `server.close()` also closes idle connections; Web Crypto `globalThis.crypto` and `CustomEvent` are unflagged globals; `--experimental-specifier-resolution=node` is gone (extensionless ESM imports fail); `fs.writeFile*` no longer coerce objects with a custom `toString()`; `process.config` is frozen; `performance.*` methods must be called with `performance` as receiver.[^10]

##### Runtime additions in 20.x minors (exact versions)[^7][^11]

| Minor (date) | Additions a reviewer will meet |
|---|---|
| 20.1.0 (2023-05-03) | `fs.readdir`/`opendir` `recursive: true`; `fs.cp` `mode`; `dns.getDefaultResultOrder()`; `assert.CallTracker` runtime-deprecated |
| 20.2.0 (2023-05-16) | `test.skip/todo/only` shorthands; `URLSearchParams#has(name, value)`/`delete(name, value)` |
| 20.3.0 (2023-06-08) | `AbortSignal.any()`; `readable.asIndexedPairs()` deprecated; libuv 1.45 |
| 20.4.0 (2023-07-05) | `node:test` `MockTimers`; `Symbol.dispose`/`Symbol.asyncDispose` and dispose methods on Node resources (experimental) |
| 20.5.0 (2023-07-18) | `events.addAbortListener()`; `--test-shard` |
| 20.6.0 (2023-09-04) | `--env-file`; `import.meta.resolve()` unflagged; `module.register()` + `initialize` hook; `load` hook can return CommonJS; `ReadableStream.from()` |
| 20.7.0 (2023-09-18) | multiple `--env-file`; npm 10; `process.sourceMapsEnabled` |
| 20.8.0 (2023-09-28) | `node:test` JUnit reporter; doc-deprecations of `fs.F_OK…`, `util.toUSVString`, promisifying promise-returning functions |
| 20.9.0 (2023-10-24) | **LTS** start, no new APIs |
| 20.10.0 (2023-11-22) | import **attributes** (`with`) replace assertions; `--experimental-detect-module`; `--experimental-default-type`; `--experimental-websocket`; `flush` option for `fs.writeFile*`/`appendFile*`; `--test-concurrency` |
| 20.11.0 (2024-01-09) | `import.meta.dirname` / `import.meta.filename`; `--disable-warning`; `--test-timeout`; mock `Date`, `timers.setTime()`; lcov reporter |
| 20.12.0 (2024-03-26) | `crypto.hash()`; `process.loadEnvFile()`; `util.parseEnv()`; multi-line `.env` values; `util.styleText()`; `dirent.parentPath` (`dirent.path` doc-deprecated); SEA assets (`node:sea`); `--allow-addons`; `require('node:timers').promises` |
| 20.13.0 (2024-05-07) | `suite()`; `process.availableMemory()`; `v8.queryObjects()`; `dns` `order: 'ipv6first'`; `--watch` **stable**; `CustomEvent` stable; streams accept `TypedArray`/`DataView` chunks |
| 20.14.0 (2024-05-28) | `--test-force-exit` |
| 20.15.0 (2024-06-20) | `t.plan()`; `zlib.crc32()`; `--inspect-wait` |
| 20.16.0 (2024-07-24) | `process.getBuiltinModule()`; `Blob#bytes()`; `t.fullName`; `--allow-wasi`; `--cpu-prof`/`--heap-prof` stable; `parseArgs` `--no-` options |
| 20.17.0 (2024-08-21) | `require(esm)` behind `--experimental-require-module`; `path.matchesGlob()` (experimental); `stream.duplexPair()` |
| 20.18.0 (2024-10-03) | `URL.parse()`; `mock.module()` (`--experimental-test-module-mocks`); `EventSource` (`--experimental-eventsource`); `vm.constants.DONT_CONTEXTIFY`; `util.styleText()` respects `isTTY`/`NO_COLOR`; `tls` `allowPartialTrustChain` |
| 20.18.3 (2025-02-10) | import attributes and JSON modules marked **stable** (no `ExperimentalWarning`) |
| 20.19.0 (2025-03-13) | **`require(esm)` on by default**; **ESM syntax detection on by default**; `"module-sync"` exports condition; `process.features.require_module`; `--env-file-if-exists`; `worker_threads.postMessageToThread()` |
| 20.19.1 – 20.20.2 | security and maintenance releases only (see Security) |

Version metadata gaps: the 20.x docs list `URL.parse()`, `Blob#bytes()` and `t.assert` as "added in v22", but all three exist in 20.20.2 (the release notes place `URL.parse()` in 20.18.0 and `Blob#bytes()` in 20.16.0). Check the runtime, not only the doc header.

***

#### Do Not Use or Recommend (Not in Node.js 20)

The project's runtime does not have the following. Each line was checked on 20.20.2 (absent) and on the first release line that has it.[^12][^13]

##### ECMAScript features newer than V8 11.3

- `Object.groupBy()`, `Map.groupBy()` — Node 21
- `ArrayBuffer.prototype.transfer()` / `transferToFixedLength()` / `detached` — Node 21
- `JSON.rawJSON()` / `JSON.isRawJSON()` — Node 21
- `Promise.withResolvers()` — Node 22
- `Array.fromAsync()` — Node 22
- Set methods: `union`, `intersection`, `difference`, `symmetricDifference`, `isSubsetOf`, `isSupersetOf`, `isDisjointFrom` — Node 22
- Iterator helpers: global `Iterator`, `Iterator.from()`, `iterator.map/filter/take/drop/flatMap/reduce/toArray/forEach/some/every/find` — Node 22
- `Promise.try()`, `Intl.DurationFormat`, RegExp duplicate named groups (`/(?<y>a)|(?<y>b)/`), RegExp modifiers (`/(?i:a)b/`) — Node 23 (not in 22.23.4)
- `using` / `await using` declarations, `DisposableStack`, `AsyncDisposableStack`, `SuppressedError` — Node 24 (`Symbol.dispose` itself exists in 20; see Resource Management)
- `RegExp.escape()`, `Float16Array`, `Math.f16round()`, `Error.isError()`, `Atomics.pause()`, `Intl.Locale.prototype.getWeekInfo()` — Node 24
- `Uint8Array.fromBase64()` / `toBase64()` / `fromHex()` / `toHex()` — Node 25
- `Temporal`, `Iterator.concat()`, `Map.prototype.getOrInsert()` — Node 26
- Decorators (`@dec class …`), `import defer`, source-phase imports — not supported natively in any release line through 26; they need a transpiler.

##### Node.js runtime APIs and flags newer than 20

- Global `navigator` (`navigator.hardwareConcurrency`, `navigator.userAgent`) — Node 21. Use `os.availableParallelism()`.
- Unflagged global `WebSocket` — Node 22 (flagged by `--experimental-websocket` in 20.10+). Global `EventSource` needs `--experimental-eventsource` (20.18+, still flagged in 22).
- `node --test 'glob/**/*.test.js'` (glob arguments) — Node 21; on 20 a glob is taken as a literal path ("Could not find …").
- `node --run <script>` — Node 22. `--experimental-strip-types` (Node 22.6) / `--experimental-transform-types` (Node 22.7) — running `.ts` directly.
- `fs.glob()` / `fs.globSync()` / `fsPromises.glob()` — Node 22. (`path.matchesGlob()` exists from 20.17 as experimental.)
- `node:sqlite` — Node 22.5. `module.enableCompileCache()` / `NODE_COMPILE_CACHE` — Node 22.1 (env var) / 22.8 (API). `module.registerHooks()` (synchronous hooks) — Node 22.15. `module.findPackageJSON()` — Node 22.14. `module.stripTypeScriptTypes()` — Node 22.13.
- `util.getCallSite()` / `util.getCallSites()` — Node 22.9; `util.diff()` — Node 22.15. `assert.partialDeepStrictEqual()` — Node 22.13.
- `process.features.typescript`, `process.finalization`, `process.execve()` — Node 22.
- `worker_threads.markAsUncloneable()`, `net.BlockList.isBlockList()`, `tls.getCACertificates()`, `zlib` Zstandard (`createZstdCompress`) — Node 22.
- `node:test` snapshot testing (`t.assert.snapshot`), `--test-coverage-include`/`-exclude` (22.5), coverage thresholds `--test-coverage-lines|branches|functions` (22.8), `--experimental-test-isolation` — Node 22.
- `--permission` (the stable spelling, Node 22.13+) and `--allow-net` (Node 25) — on 20 the flag is `--experimental-permission` (`--permission` is a "bad option") and the network is not restricted at all.
- Global `URLPattern` — Node 24; `localStorage`/`sessionStorage` globals — Node 25 (flagged in 22).

##### Node.js 21.x projects

A project on Node 21 (EOL June 1, 2024) has the Node 21 additions above (`Object.groupBy`, `Map.groupBy`, `ArrayBuffer#transfer`, `JSON.rawJSON`, global `navigator`, glob arguments to `--test`, `fetch`/Web Streams marked stable) but **lacks** these Node 20.x backports, verified absent on 21.7.3: `require(esm)` (throws `ERR_REQUIRE_ESM`), syntax detection by default, `"module-sync"` condition, `process.features.require_module`, `process.getBuiltinModule()`, `process.availableMemory()`, `URL.parse()`, `Blob#bytes()`, `node:test` `suite()`, `path.matchesGlob()`, `stream.duplexPair()`, `zlib.crc32()`, `v8.queryObjects()`, `vm.constants.DONT_CONTEXTIFY`, `worker_threads.postMessageToThread()`, the `export { x as 'module.exports' }` interop, `node:test` `t.plan()`, `t.assert` and `t.fullName`, `util.parseArgs` `allowNegative`, and the flags `--test-force-exit`, `--allow-wasi`, `--inspect-wait`, `--report-exclude-network`, `--experimental-eventsource` and `--experimental-test-module-mocks` (all "bad option" on 21.7.3). `--env-file-if-exists` is not supported either: 21.7.3 rejects it as a "bad option" when the file exists and exits with "<file>: not found" when it is missing. 21.0.0 also **removed** `readable.asIndexedPairs()`, which still exists (deprecated) on 20. JSON imports still print an `ExperimentalWarning` on 21. Code written against 20.19 can therefore fail on 21.7.3; the fix is to upgrade to 22/24, not to "downgrade" the code.

##### Deprecated or removed APIs (older features)

| API | Status in Node 20 | Replacement |
|---|---|---|
| `new Buffer()` / `Buffer()` (DEP0005) | runtime warning (outside `node_modules`) | `Buffer.alloc()`, `Buffer.from()` |
| `crypto.createCipher()` / `createDecipher()` (DEP0106) | runtime warning; **removed in 22** | `createCipheriv()` with a random IV, or AES-GCM |
| `crypto.DEFAULT_ENCODING` (DEP0091) | **removed in 20.0.0** | pass encodings explicitly |
| `process.exit('1.5')`, `process.exitCode = true` (DEP0164) | **throws** `ERR_OUT_OF_RANGE` / `ERR_INVALID_ARG_TYPE` since 20.0.0 | integer codes |
| `url.parse()` (DEP0169) | doc-only (runtime with `--pending-deprecation`); invalid ports warn (DEP0170) | `new URL()`, `URL.canParse()`, `URL.parse()` (20.18+) |
| `fs.rmdir(path, { recursive: true })` (DEP0147) | runtime warning | `fs.rm(path, { recursive: true, force: true })` |
| `assert.CallTracker` (DEP0173) | runtime warning since 20.1.0 | `mock.fn()` from `node:test` |
| `readable.asIndexedPairs()` | runtime warning (no code) since 20.3.0; **removed in 21.0.0** | `readable.map()` with a counter |
| Import assertions `assert { type: 'json' }` | works, V8 prints a deprecation warning; **SyntaxError in 22** | import attributes `with { type: 'json' }` |
| `util.isArray()`, `util.isDate()`, `util._extend()` … | doc-only in 20; runtime warnings from 22; all `util.is*` except `isArray`/`isDeepStrictEqual` removed in 23 | `Array.isArray()`, `instanceof Date`, `Object.assign()` |
| `punycode` built-in (DEP0040) | doc-only in 20; runtime warning from 21 | `url.domainToASCII()` or userland `punycode` |
| `require('sys')` (DEP0025) | runtime warning | `node:util` |
| `fs.F_OK`, `fs.R_OK`, … (DEP0176) | doc-only since 20.8; removed in 25 | `fs.constants.F_OK` |
| `dirent.path` (DEP0178) | doc-only since 20.12 | `dirent.parentPath` |
| `Hash`/`Hmac` constructors without `crypto.createHash()` (DEP0179/DEP0181) | doc-only | `crypto.createHash()`, `crypto.createHmac()` |
| `--experimental-policy` | deprecated in 20.13 docs; removed in 22 | Permission Model, lockfiles, `npm audit signatures` |
| `--experimental-loader` / `globalPreload` hook | both work on 20 with `ExperimentalWarning`s; `globalPreload` is removed in 21 (ignored with `UnsupportedWarning`) | `--import` + `module.register()` with an `initialize` hook |
| `--experimental-specifier-resolution=node` | silently ignored since 19; extensionless ESM imports throw `ERR_MODULE_NOT_FOUND` | full relative specifiers with extensions |

Runtime deprecation warnings were captured on 20.20.2 with a probe script; e.g. `[DEP0005] DeprecationWarning: Buffer() is deprecated …`, `[DEP0106] DeprecationWarning: crypto.createCipher is deprecated.`, `[DEP0173] DeprecationWarning: assert.CallTracker is deprecated.`[^14]

***

#### Code Style and Language Basics

##### Naming and Formatting

- `camelCase` for variables and functions, `PascalCase` for classes, `UPPER_SNAKE_CASE` for module-level constants, `#private` for class-private state. File names: `kebab-case.js`, with `.mjs`/`.cjs` when the package `type` would otherwise be wrong.
- Let a formatter (Prettier, or ESLint stylistic rules) own formatting; review logic, not whitespace.
- Always import built-ins with the `node:` prefix (`import fs from 'node:fs'`, `require('node:fs')`). It cannot be shadowed by an npm package named `fs`, it documents intent, and some modules exist **only** with the prefix (`node:test`, `node:sea`).

##### Declarations and Equality

```js
// const by default, let when reassigned, never var.
const limit = 10;
let count = 0;
for (const item of [1, 2, 3]) count += item;

// Strict equality everywhere; `== null` is the single accepted exception
// (it matches both null and undefined).
const value = undefined;
console.log(value == null, value === null); // true false

// Nullish coalescing keeps 0, '' and false; || does not.
const port = Number.parseInt(process.env.PORT ?? '', 10) || 3000;
const retries = 0;
console.log(retries ?? 3, retries || 3, port); // 0 3 3000 (when PORT is unset)
console.log(limit);
```

`??=`, `||=` and `&&=` are available (ES2021). Use optional chaining (`a?.b?.()`) only where `undefined` is a legitimate state; do not use it to hide programming errors.

##### Numbers and BigInt

```js
// Parse numbers explicitly; Number('') is 0, parseInt stops at the first bad char.
console.log(Number(''), Number('12px'), Number.parseInt('12px', 10)); // 0 NaN 12

// Floating point: compare with a tolerance or work in integer minor units (cents).
console.log(0.1 + 0.2 === 0.3, Math.abs(0.1 + 0.2 - 0.3) < Number.EPSILON); // false true

// Integers above 2**53 - 1 lose precision; use BigInt for IDs and counters that can exceed it.
console.log(Number.MAX_SAFE_INTEGER + 2, 9007199254740993n);
// JSON.stringify throws on BigInt — convert explicitly.
try { JSON.stringify({ id: 1n }); } catch (e) { console.log(e.name); } // TypeError
console.log(JSON.stringify({ id: 1n }, (_k, v) => (typeof v === 'bigint' ? v.toString() : v)));
```

Review flags: `parseInt(x)` without radix, `isNaN(x)` instead of `Number.isNaN(x)`, money in floats, 64-bit database IDs parsed into `Number`.

***

#### New in 20: Change Array by Copy

`toSorted()`, `toReversed()`, `toSpliced()` and `with()` return a new array and leave the original untouched; the same methods (except `toSpliced`) exist on typed arrays.[^15] They replace the error-prone `[...arr].sort()` copy idiom and are the fix for the common bug of sorting a shared array in place.

```js
const prices = [10, 1, 3];

// BAD: sort() mutates the caller's array and sorts lexicographically by default.
function cheapestBad(list) {
  return list.sort()[0];
}
const shared = [10, 1, 3];
console.log(cheapestBad(shared), shared); // 1 [ 1, 10, 3 ]  <- caller's data reordered, '10' < '3'

// GOOD: non-mutating, numeric comparator.
const sorted = prices.toSorted((a, b) => a - b);
console.log(prices, sorted); // [ 10, 1, 3 ] [ 1, 3, 10 ]
console.log(prices.toReversed(), prices.with(1, 99), prices.toSpliced(0, 1)); // [ 3, 1, 10 ] [ 10, 99, 3 ] [ 1, 3 ]

// with() validates the index (negative counts from the end); out of range throws.
console.log([1, 2].with(-1, 9)); // [ 1, 9 ]
try { [1, 2].with(5, 0); } catch (e) { console.log(e.name); } // RangeError

// Holes become undefined in the copy; undefined still sorts last.
console.log([, 1].toSorted(), [3, undefined, 1].toSorted()); // [ 1, undefined ] [ 1, 3, undefined ]
```

`findLast()` / `findLastIndex()` (already in 18) search from the end without reversing a copy. `Array.prototype.at(-1)` reads the last element. `Array.prototype.group()`/`groupToMap()` never shipped unflagged (renamed during standardization); the final API `Object.groupBy()` is **not** in Node 20 — see "Missing ES2024+ APIs".

##### New in 20: The RegExp `v` Flag

The `v` (unicodeSets) flag is a stricter superset of `u` that adds set subtraction (`--`), intersection (`&&`), nested classes and properties of strings such as `\p{RGI_Emoji}`.[^16]

```js
const upperOrNonAscii = /[\p{L}--[a-z]]/v;              // letters except a-z
console.log(upperOrNonAscii.test('a'), upperOrNonAscii.test('Ä')); // false true
console.log(/^\p{RGI_Emoji}$/v.test('👍🏽'));             // true (emoji + skin tone as one string)
console.log(/[\p{ASCII}&&\p{L}]/v.test('é'));           // false: intersection
console.log(/[[a-z]--[aeiou]]/v.test('e'));              // false: consonants only

// v is stricter: some characters must be escaped inside classes.
try { new RegExp('[(]', 'v'); } catch (e) { console.log(e.name); } // SyntaxError
console.log(/[(]/u.test('('));                          // true with u
```

Reviewer notes: `u` and `v` cannot be combined; `\p{RGI_Emoji}` with `u` is a `SyntaxError`. Do not suggest RegExp duplicate named groups or inline modifiers (`(?i:…)`) — both need Node 23+.

##### New in 20: Well-Formed Strings

JavaScript strings are UTF-16 and can contain lone surrogates (from slicing emoji, broken user input, or binary-to-string conversions). `encodeURIComponent()`, `TextEncoder` and many protocols reject or silently replace them.

```js
const broken = 'ab\uD800';                    // lone high surrogate
console.log(broken.isWellFormed());           // false
console.log(broken.toWellFormed() === 'ab\uFFFD'); // true: replaced with U+FFFD
try { encodeURIComponent(broken); } catch (e) { console.log(e.name, e.message); } // URIError URI malformed

// GOOD: sanitize untrusted text before URL-encoding or hashing it.
const safe = encodeURIComponent(broken.toWellFormed());
console.log(safe); // ab%EF%BF%BD
```

`util.toUSVString()` is doc-deprecated since 20.8.0 (DEP0175) in favour of `toWellFormed()`.

##### New in 20: Resizable ArrayBuffer and Growable SharedArrayBuffer

```js
const buf = new ArrayBuffer(4, { maxByteLength: 16 });
const tracking = new Uint8Array(buf);        // length-tracking view (no length given)
const fixed = new Uint8Array(buf, 0, 4);     // fixed-length view
console.log(buf.resizable, buf.byteLength, tracking.length); // true 4 4

buf.resize(12);
console.log(buf.byteLength, tracking.length, fixed.length);  // 12 12 4

buf.resize(2);                                // shrink below the fixed view
console.log(tracking.length, fixed.length, fixed.byteLength); // 2 0 0  <- out-of-bounds view reads as empty

try { buf.resize(32); } catch (e) { console.log(e.name); }   // RangeError: above maxByteLength
```

Review flags: code that caches `view.length` across a `resize()`; fixed-length views silently becoming empty after a shrink. `ArrayBuffer.prototype.transfer()` is Node 21+, not 20.

##### Missing ES2024+ APIs: Correct Node 20 Equivalents

When reviewing Node 20 code, accept these idioms instead of suggesting the newer built-ins.

```js
// Object.groupBy (Node 21+) -> reduce into a null-prototype object or a Map.
const people = [{ name: 'a', team: 'x' }, { name: 'b', team: 'y' }, { name: 'c', team: 'x' }];
const byTeam = people.reduce((acc, p) => {
  (acc[p.team] ??= []).push(p.name);
  return acc;
}, Object.create(null));
console.log(byTeam); // [Object: null prototype] { x: [ 'a', 'c' ], y: [ 'b' ] }

const byTeamMap = new Map();
for (const p of people) {
  const list = byTeamMap.get(p.team) ?? [];
  list.push(p.name);
  byTeamMap.set(p.team, list);
}
console.log(byTeamMap); // Map(2) { 'x' => [ 'a', 'c' ], 'y' => [ 'b' ] }

// Promise.withResolvers (Node 22+) -> a tiny helper.
function withResolvers() {
  let resolve, reject;
  const promise = new Promise((res, rej) => { resolve = res; reject = rej; });
  return { promise, resolve, reject };
}
const d = withResolvers();
setTimeout(() => d.resolve('ready'), 10);
d.promise.then(console.log); // ready

// Set methods (Node 22+) -> explicit loops/filters.
const a = new Set([1, 2, 3]);
const b = new Set([2, 3, 4]);
const union = new Set([...a, ...b]);
const intersection = new Set([...a].filter((x) => b.has(x)));
const difference = new Set([...a].filter((x) => !b.has(x)));
console.log([...union], [...intersection], [...difference]); // [ 1, 2, 3, 4 ] [ 2, 3 ] [ 1 ]

// Array.fromAsync (Node 22+) -> for await.
async function collect(iterable) {
  const out = [];
  for await (const x of iterable) out.push(x);
  return out;
}
async function* gen() { yield 1; yield 2; }
collect(gen()).then((r) => console.log(r)); // [ 1, 2 ]
```

Iterator helpers (`iter.map()`, `.filter()`, `.take()`) are Node 22+. On Node 20 use generator functions or convert to arrays deliberately. Do not "polyfill" built-ins by assigning to `Object.groupBy` or `Promise.withResolvers` in application code; a polyfill library (core-js) may be acceptable for libraries that must run on several majors, but plain helpers are clearer.

##### Objects, Classes and Private State

```js
class Account {
  static #nextId = 1;           // private static field
  #balance = 0;                 // private instance field
  id = Account.#nextId++;       // public field initializer

  static {                      // class static block (ES2022)
    Object.freeze(Account.prototype);
  }

  deposit(amount) {
    if (!Number.isFinite(amount) || amount <= 0) throw new RangeError('amount must be > 0');
    this.#balance += amount;
    return this;
  }

  get balance() { return this.#balance; }

  static isAccount(obj) { return #balance in obj; } // ergonomic brand check (ES2022)
}

const acc = new Account().deposit(5);
console.log(acc.id, acc.balance, Account.isAccount(acc), Account.isAccount({})); // 1 5 true false
console.log(JSON.stringify(acc)); // {"id":1}  <- private fields are never serialized
```

- Prefer `#private` over `_underscore` conventions for real encapsulation. Note that `#fields` are not copied by `JSON.stringify`, `structuredClone` or spread, and a getter that reads `this.#field` throws `TypeError: Cannot read private member` when called through a `Proxy`.
- `Object.hasOwn(obj, key)` instead of `obj.hasOwnProperty(key)` (works on null-prototype objects).
- Use `Object.create(null)` or `Map` for dictionaries keyed by user input (see Prototype Pollution).
- Decorators are **not** supported by Node 20's V8; `@decorator` syntax only works through TypeScript/Babel transpilation.

##### Dates, Time and Intl

`Temporal` is not available (Node 26+). Use `Date` for instants, store and transmit ISO 8601 UTC strings, and format with `Intl`.

```js
const d = new Date(Date.UTC(2026, 0, 31, 12, 0, 0));
console.log(d.toISOString()); // 2026-01-31T12:00:00.000Z

// Month arithmetic overflows: Jan 31 + 1 month -> Mar 3 (2026 is not a leap year).
const next = new Date(d);
next.setUTCMonth(next.getUTCMonth() + 1);
console.log(next.toISOString().slice(0, 10)); // 2026-03-03

// Locale-aware formatting needs an explicit timeZone, or the server's TZ leaks in.
const fmt = new Intl.DateTimeFormat('de-DE', { dateStyle: 'medium', timeStyle: 'short', timeZone: 'Europe/Berlin' });
console.log(fmt.format(d)); // 31.01.2026, 13:00

// Intl.NumberFormat v3 (new vs 18): formatRange and rounding options.
const eur = new Intl.NumberFormat('en-US', { style: 'currency', currency: 'EUR' });
console.log(eur.formatRange(3, 5)); // €3.00 – €5.00
```

Node 20 builds ship full ICU (78.2 in 20.20.2), so all locales and time zones are available unless the project uses a `--with-intl=small-icu` custom build.

##### `structuredClone()` and Deep Copies

```js
class Point { constructor(x) { this.x = x; } }
const original = { when: new Date(0), tags: new Set(['a']), p: new Point(1), nested: { n: 1 } };
const copy = structuredClone(original);
console.log(copy.when instanceof Date, copy.tags.has('a'), copy.p instanceof Point, copy.p.x);
// true true false 1   <- class instances lose their prototype

try { structuredClone({ fn() {} }); } catch (e) { console.log(e.name); } // DataCloneError

// BAD: JSON round-trip drops undefined, turns Dates into strings, throws on BigInt and cycles.
console.log(JSON.parse(JSON.stringify({ when: new Date(0), u: undefined }))); // { when: '1970-01-01T00:00:00.000Z' }
```

***

#### Modules: ESM, CommonJS and Interop

##### Choosing the Module System

New code on Node 20 should be ES modules: set `"type": "module"` in `package.json` (or use `.mjs`), use `import`/`export`, and top-level `await` where it simplifies startup. CommonJS remains fully supported; mixed repositories should make every file's format explicit with `.mjs`/`.cjs` or a `type` field in each `package.json`.

```json
{
  "name": "@acme/orders",
  "version": "2.3.0",
  "type": "module",
  "engines": { "node": ">=20.19.0" },
  "exports": {
    ".": {
      "types": "./dist/index.d.ts",
      "import": "./dist/index.js",
      "require": "./dist/index.cjs"
    },
    "./package.json": "./package.json"
  },
  "files": ["dist"]
}
```

- `exports` encapsulates the package: deep imports not listed there fail with `ERR_PACKAGE_PATH_NOT_EXPORTED`. Order conditions from most to least specific; `"types"` first, `"default"` last.
- ESM relative imports need the full file name: `import { x } from './util.js'`, never `'./util'` (the old `--experimental-specifier-resolution=node` is gone since 19 and is silently ignored on 20, so the import fails with `ERR_MODULE_NOT_FOUND`).
- ESM has no `require`, `module`, `exports`, `__filename` or `__dirname`. Use `import.meta.dirname`/`import.meta.filename` (20.11+), or `fileURLToPath(import.meta.url)` for older 20 minors, and `createRequire(import.meta.url)` when a CommonJS-only API (e.g. `require.resolve`) is needed.

```js
import { createRequire } from 'node:module';
import { fileURLToPath } from 'node:url';
import path from 'node:path';

// 20.11+: import.meta.dirname / import.meta.filename (file: modules only).
const here = import.meta.dirname ?? path.dirname(fileURLToPath(import.meta.url));
console.log(here === path.dirname(fileURLToPath(import.meta.url))); // true

const require = createRequire(import.meta.url);
console.log(typeof require.resolve('node:fs')); // string
```

##### New in 20.19: `require()` of ES Modules (on by default)

From **20.19.0**, CommonJS can `require()` an ES module whose graph has no top-level `await`. The flag existed from 20.17.0 (`--experimental-require-module`); 20.19.0 enabled it by default and silenced the `ExperimentalWarning` (use `--trace-require-module` to see where it happens). `--no-experimental-require-module` turns it off. Node 18, 19 and **21** throw `ERR_REQUIRE_ESM` instead.[^8][^17]

```js
// main.cjs — requires an ESM-only dependency ("type": "module" package).
const pkg = require('esm-only-pkg');           // namespace object
console.log(Object.keys(pkg));                  // [ '__esModule', 'a', 'default' ]
console.log(typeof pkg.default);                // the default export lives on .default

try {
  require('pkg-with-top-level-await');
} catch (err) {
  console.log(err.code);                        // ERR_REQUIRE_ASYNC_MODULE -> use await import()
}
console.log(process.features.require_module);   // true on 20.19+
```

- The result is the **module namespace object**, not the default export. `const fn = require('esm-pkg')` where the package does `export default fn` gives an object; write `require('esm-pkg').default`, unless the ES module exports a binding named `"module.exports"` (`export { fn as 'module.exports' }`), which `require()` then returns directly (20.19+).
- Graphs with top-level `await` throw `ERR_REQUIRE_ASYNC_MODULE`; keep `await import()` for those.
- Package authors can ship one ESM build to both `import` and `require` via the **`"module-sync"`** exports condition, which matches whenever `require(esm)` is enabled (20.19+; Node 22 has it from 22.10.0). Old runtimes skip the condition and fall back to `"require"`.

```json
{
  "exports": {
    "module-sync": "./esm/index.js",
    "require": "./cjs/index.cjs",
    "import": "./esm/index.js"
  }
}
```

Reviewer notes: a dependency upgrade to an ESM-only major works in CommonJS on 20.19+ but **breaks** on 20.0–20.16 and on 21.x, so the `engines` floor must say `>=20.19.0`. Several popular tools already require it (ESLint 10, Vite 7/8 and Mocha 12 declare `^20.19.0` in `engines`; Vitest 5 and undici 8 dropped Node 20 entirely).[^18]

##### New in 20.19: ESM Syntax Detection (on by default)

Files ending in `.js` or without extension, whose nearest `package.json` has no `type` field (or that have no `package.json` at all), are first parsed as CommonJS; if that fails because of ESM syntax (`import`, `export`, `import.meta`, top-level `await`), Node re-runs them as ES modules. The flag `--experimental-detect-module` was added in 20.10.0 and became the default in 20.19.0.[^8]

- When a `package.json` without `type` exists, Node prints `[MODULE_TYPELESS_PACKAGE_JSON] Warning: … Reparsing as ES module because module syntax was detected. This incurs a performance overhead.` — the fix is to add `"type": "module"` (or `"type": "commonjs"`).
- Detection does not make `require` available in an ESM file: a `.js` file that mixes `require()` with `export` runs as ESM and throws `ReferenceError: require is not defined in ES module scope`.
- Library packages should always set `type` explicitly; never rely on detection for published code (consumers on 20.0–20.18 or 21.x do not detect by default).

##### Import Attributes and JSON Modules

```js
// GOOD (20.10+): import attributes. Stable and warning-free since 20.18.3.
import config from './config.json' with { type: 'json' };
const { default: data } = await import('./data.json', { with: { type: 'json' } });

// BAD: import assertions. Works on 20 with a V8 deprecation warning,
// but is a SyntaxError on Node 22+.
// import config from './config.json' assert { type: 'json' };

// ERROR: JSON without the attribute -> ERR_IMPORT_ASSERTION_TYPE_MISSING on 20.
// import config from './config.json';
```

In CommonJS, `require('./config.json')` still works and caches the parsed object (mutations are shared by every importer). For files that change at runtime, read with `fs.readFile` + `JSON.parse`.

##### Importing CommonJS from ESM

```js
import fs, { readFileSync } from 'node:fs';   // built-ins expose named exports
import cjsDefault from './legacy.cjs';          // always works: module.exports as default
import { helper } from './legacy.cjs';          // works only if cjs-module-lexer can see `exports.helper = …`
console.log(typeof fs.readFile, typeof readFileSync, typeof cjsDefault, typeof helper);
```

Named imports from CommonJS rely on static analysis of `exports.x =` / `module.exports = { x }` patterns. Dynamically built exports (`Object.assign(module.exports, compute())`) are only reachable through the default import; reviewers should flag `SyntaxError: Named export 'x' not found` risks when a CommonJS dependency is imported with named bindings.

##### `import.meta.resolve()` (Unflagged in 20.6)

`import.meta.resolve(specifier)` returns an absolute URL string **synchronously** (changed from a Promise in 20.0.0) and is available without a flag from 20.6.0. Since 20.6 it no longer throws for `file:` URLs that do not exist — check existence separately. The second `parent` argument still needs `--experimental-import-meta-resolve`.

```js
console.log(import.meta.resolve('node:fs'));        // node:fs
console.log(import.meta.resolve('./nope.js').startsWith('file://')); // true, even though the file is missing
```

##### Module Customization Hooks: `--import` + `module.register()`

Since 20.0.0 loader hooks run on a separate thread; 20.6.0 added `module.register()` and the `initialize` hook. Use `--import` to register hooks before the entry point; `--experimental-loader` still works but prints an `ExperimentalWarning` telling you to switch, and the `globalPreload` hook is deprecated on 20 and removed in 21.[^19]

```js
// register-hooks.mjs — loaded with: node --import ./register-hooks.mjs app.mjs
import { register } from 'node:module';
register('./hooks.mjs', import.meta.url, { data: { verbose: false } });
```

```js
// hooks.mjs — runs on the hooks thread; keep it small and side-effect free.
// data is undefined when the file is loaded via --experimental-loader: default it.
export async function initialize({ verbose } = {}) { /* receive data from register() */ }
export async function resolve(specifier, context, nextResolve) {
  return nextResolve(specifier, context);
}
export async function load(url, context, nextLoad) {
  return nextLoad(url, context); // 20.6+: may return format 'commonjs' with source
}
```

Synchronous, in-thread hooks (`module.registerHooks()`) are Node 22.15+; do not recommend them for Node 20. When the Permission Model is enabled, hooks need `--allow-worker` (since 20.18.2).

##### Built-in Module Access

- `process.getBuiltinModule('node:fs')` (20.16+) loads a built-in synchronously from code that may run in either module system, without making the code ESM-only.
- `module.isBuiltin('fs')` checks names.
- Some built-ins only exist with the prefix: `node:test`, `node:test/reporters`, `node:sea`. `require('test')` loads an npm package named `test`, not the test runner.

***

#### Error Handling

##### Throw Errors, Keep the Cause

```js
class AppError extends Error {
  constructor(message, { code, status = 500, cause } = {}) {
    super(message, { cause });            // ES2022 error cause; printed by util.inspect
    this.name = new.target.name;          // subclasses get their own name
    this.code = code;
    this.status = status;
  }
}
class NotFoundError extends AppError {
  constructor(resource, id) {
    super(`${resource} ${id} not found`, { code: 'NOT_FOUND', status: 404 });
  }
}

function loadUser(id) {
  try {
    JSON.parse('{bad json');
  } catch (err) {
    throw new AppError(`cannot load user ${id}`, { code: 'USER_LOAD', cause: err });
  }
}

try {
  loadUser(42);
} catch (err) {
  console.log(err.name, err.code, err.cause instanceof SyntaxError); // AppError USER_LOAD true
}
const nf = new NotFoundError('order', 7);
console.log(nf.name, nf.status, nf instanceof AppError); // NotFoundError 404 true
```

- Only throw `Error` instances (`throw 'x'` loses the stack). Wrap with `{ cause }` instead of string-concatenating messages.
- Check errors by `code`/`instanceof`, never by matching `message` text — Node core messages change between releases, `err.code` values (`ENOENT`, `ERR_INVALID_ARG_TYPE`) are stable.
- `AggregateError` (from `Promise.any()`) carries `errors`. `Error.isError()` and `SuppressedError` are Node 24+.
- Since 20.15.0, `assert.deepStrictEqual()` also compares `cause` and `errors` of Error objects; tests that compared errors with different causes start failing after a minor upgrade.

##### Errors in Async Code

```js
// BAD: fire-and-forget promise. A rejection crashes the process (default
// --unhandled-rejections=throw) and the stack points nowhere useful.
function handlerBad(job) {
  saveAudit(job);               // missing await / catch
  return 'ok';
}

// GOOD: await it, or attach an explicit catch when it is intentionally detached.
async function handlerGood(job, log) {
  await saveAudit(job);
  void notify(job).catch((err) => log.warn({ err }, 'notify failed')); // intentional background task
  return 'ok';
}

async function saveAudit() {}
async function notify() { throw new Error('smtp down'); }
handlerGood({}, { warn: (o, m) => console.log(m, o.err.message) }).then(console.log);
// notify failed smtp down
// ok
console.log(typeof handlerBad);
```

- `async` functions used as callbacks of APIs that ignore the returned promise (`EventEmitter.on`, `array.forEach`, `setTimeout`) turn every throw into an unhandled rejection. Use `for…of` with `await`, or `events.captureRejections`.
- Install `process.on('unhandledRejection')` / `process.on('uncaughtException')` only to log and **exit**; after an uncaught exception the process state is undefined.

```js
process.on('uncaughtException', (err, origin) => {
  console.error({ err, origin }, 'fatal');
  process.exitCode = 1;          // integer only: '1.5' or true throw since Node 20.0.0
  // flush logs, then let the process end (or call process.exit(1) after a timeout)
});
```

***

#### Asynchronous Programming

##### Promises and Concurrency Control

```js
const sleep = (ms, v) => new Promise((r) => setTimeout(r, ms, v));

async function main() {
  // Sequential awaits in a loop: total time = sum of all calls.
  // Parallel: Promise.all — rejects fast on the first failure.
  const [a, b] = await Promise.all([sleep(20, 'a'), sleep(10, 'b')]);

  // Promise.allSettled — collect every outcome (no fail-fast).
  const settled = await Promise.allSettled([sleep(5, 1), Promise.reject(new Error('x'))]);
  console.log(a, b, settled.map((s) => s.status)); // a b [ 'fulfilled', 'rejected' ]

  // Promise.any — first success; AggregateError if all fail.
  try { await Promise.any([Promise.reject(new Error('1')), Promise.reject(new Error('2'))]); }
  catch (e) { console.log(e.constructor.name, e.errors.length); } // AggregateError 2

  // Bounded concurrency without dependencies: a tiny worker pool.
  const ids = Array.from({ length: 10 }, (_, i) => i);
  const results = await mapLimit(ids, 3, async (id) => { await sleep(5); return id * 2; });
  console.log(results); // [ 0, 2, 4, ..., 18 ] in input order
}

async function mapLimit(items, limit, fn) {
  const out = new Array(items.length);
  let next = 0;
  async function worker() {
    while (next < items.length) {
      const i = next++;
      out[i] = await fn(items[i], i);
    }
  }
  await Promise.all(Array.from({ length: Math.min(limit, items.length) }, worker));
  return out;
}

main();
```

Review flags: `await` inside `for` loops over independent I/O (accidental serialization), unbounded `Promise.all(items.map(fetch))` over thousands of items (socket/FD exhaustion), `new Promise(async (resolve) => …)` (the executor's rejections are lost), and `Promise.withResolvers()` (Node 22+).

##### Cancellation: `AbortController`, `AbortSignal.timeout()` and `AbortSignal.any()` (20.3+)

```js
import { setTimeout as delay } from 'node:timers/promises';

async function fetchWithDeadline(task, { signal } = {}) {
  // Combine the caller's signal with a local timeout (AbortSignal.any: 20.3.0+).
  const timeout = AbortSignal.timeout(50);
  const combined = signal ? AbortSignal.any([signal, timeout]) : timeout;
  return task(combined);
}

try {
  await fetchWithDeadline((signal) => delay(1_000, 'late', { signal }));
} catch (err) {
  console.log(err.name, err.cause?.name); // AbortError TimeoutError
}

const ac = new AbortController();
const p = fetchWithDeadline((signal) => delay(1_000, 'late', { signal }), { signal: ac.signal });
ac.abort();
try { await p; } catch (err) { console.log(err.name); } // AbortError
```

- `AbortSignal.timeout()` aborts with a `TimeoutError` `DOMException`; `fetch()` rejects with that `TimeoutError` directly, while `node:timers/promises` and most `node:` APIs reject with an `AbortError` whose `cause` is the reason.
- Pass signals all the way down (`fetch`, `fs.readFile`, `events.once`, `stream.pipeline`, `child_process`). A timeout that only stops *waiting* while the work continues leaks sockets and memory.
- To listen for abort on a signal you do not own, use `events.addAbortListener(signal, listener)` (20.5+, experimental) — it handles `stopImmediatePropagation()` correctly and returns a disposable.

##### Timers and the Event Loop

```js
import { setTimeout as delay, setImmediate as immediate } from 'node:timers/promises';

const order = [];
process.nextTick(() => order.push('nextTick'));
queueMicrotask(() => order.push('microtask'));
setTimeout(() => order.push('timeout'), 0);
setImmediate(() => order.push('immediate'));
await delay(20);
// In an ES module the microtask runs first; the same code in a .cjs file prints
// [ 'nextTick', 'microtask' ]. timeout vs immediate order varies in the main module.
console.log(order.slice(0, 2)); // [ 'microtask', 'nextTick' ]

console.log(await immediate('done')); // done
```

- Never rely on the relative order of `process.nextTick()` and promise/microtask callbacks: it differs between CommonJS and ES module entry points (verified on 20.20.2).
- `process.nextTick()` is marked **Legacy** in the 20.18 docs; prefer `queueMicrotask()` for new code. Recursive `nextTick`/microtask loops starve I/O.
- CPU-bound loops block everything. Break long synchronous work into chunks (`await immediate()`) or move it to a worker thread.
- `timer.unref()` lets the process exit while a periodic timer is pending; use it for background intervals (metrics flush), never for work that must complete.

##### Worker Threads

```js
import { Worker, isMainThread, parentPort, workerData } from 'node:worker_threads';
import { availableParallelism } from 'node:os';

if (isMainThread) {
  const size = Math.max(1, Math.min(4, availableParallelism() - 1));
  const run = (n) => new Promise((resolve, reject) => {
    const w = new Worker(new URL(import.meta.url), { workerData: n });
    w.once('message', resolve);
    w.once('error', reject);
    w.once('exit', (code) => code !== 0 && reject(new Error(`worker exited ${code}`)));
  });
  console.log(size >= 1, await Promise.all([run(25), run(26)])); // true [ 75025, 121393 ]
} else {
  const fib = (n) => (n < 2 ? n : fib(n - 1) + fib(n - 2));
  parentPort.postMessage(fib(workerData));
}
```

- Use a pool (Piscina or a hand-written queue) instead of one worker per request; worker start-up costs milliseconds and memory.
- `os.availableParallelism()` respects CPU affinity; do not use `os.cpus().length`. The global `navigator.hardwareConcurrency` is **Node 21+**.
- Transfer large `ArrayBuffer`s via the `transferList` instead of copying. `SharedArrayBuffer` + `Atomics` (including `Atomics.waitAsync`) are available.
- `worker_threads.postMessageToThread(threadId, value)` (20.19+) sends to any thread by id; it does not exist on 21.x.

##### Async Context: `AsyncLocalStorage`

```js
import { AsyncLocalStorage } from 'node:async_hooks';

const requestContext = new AsyncLocalStorage();

function log(msg) {
  const ctx = requestContext.getStore();
  console.log(`[${ctx?.requestId ?? '-'}] ${msg}`);
}

async function handle(requestId) {
  return requestContext.run({ requestId }, async () => {
    await new Promise((r) => setTimeout(r, Math.random() * 10));
    log('done');
  });
}

await Promise.all([handle('a'), handle('b')]);
log('outside'); // [-] outside
```

Store per-request data (request id, user, trace context) in `AsyncLocalStorage` instead of module globals or parameter threading. Do not mutate a store object that is shared between requests; create a fresh object in `run()`.

##### EventEmitter

```js
import { EventEmitter, once, on } from 'node:events';

const jobs = new EventEmitter({ captureRejections: true });
jobs.on('job', async () => { throw new Error('async handler failed'); });
jobs.on('error', (err) => console.log('captured:', err.message)); // routed instead of unhandled
jobs.emit('job');
await new Promise((r) => setImmediate(r)); // captured: async handler failed

const ee = new EventEmitter();
// once() returns a promise; it also REJECTS if 'error' is emitted while waiting.
// Pass a signal so it cannot hang forever.
setTimeout(() => ee.emit('ready', 42), 5);
const [value] = await once(ee, 'ready', { signal: AbortSignal.timeout(1_000) });
console.log(value); // 42

// on() is an async iterator over events.
const ac = new AbortController();
setTimeout(() => { ee.emit('tick', 1); ee.emit('tick', 2); ac.abort(); }, 5);
try {
  for await (const [n] of on(ee, 'tick', { signal: ac.signal })) console.log('tick', n);
} catch (err) { console.log(err.name); } // AbortError
```

An `EventEmitter` without an `'error'` listener throws on `emit('error')`, crashing the process. The `MaxListenersExceededWarning` usually signals a listener leak (adding a listener per request), not a limit to raise.

***

#### Resource Management Without `using`

**New in 20.4/20.5:** Node 20 defines `Symbol.dispose` and `Symbol.asyncDispose` (as Node-provided symbols, e.g. `Symbol(nodejs.dispose)`) and implements them on `FileHandle`, `Readable`, `Writable` (20.16), `net`/`http` servers, sockets, child processes and timers (all marked experimental).[^20] The `using` / `await using` **syntax is not supported** by V8 11.3 (it is a `SyntaxError` until Node 24), and `DisposableStack` does not exist.

- TypeScript 5.2+ (or Babel) can down-level `using` to `try/finally` that calls `Symbol.dispose`; that output runs on Node 20 (verified with TypeScript 5.4 targeting ES2022) and on 18.20, but fails on 19.9 with `TypeError: Symbol.dispose is not defined`.
- In plain JavaScript on Node 20, write `try/finally` explicitly.

```js
import { open } from 'node:fs/promises';
import { writeFileSync, rmSync, mkdtempSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';

const dir = mkdtempSync(join(tmpdir(), 'res-'));
const file = join(dir, 'a.txt');
writeFileSync(file, 'hello');

// BAD: the FileHandle leaks if read() throws (and Node warns when it is GC-closed).
// const fh = await open(file); const { bytesRead } = await fh.read(); await fh.close();

// GOOD: deterministic cleanup on every path.
const fh = await open(file, 'r');
try {
  const { bytesRead } = await fh.read(Buffer.alloc(16), 0, 16, 0);
  console.log(bytesRead); // 5
} finally {
  await fh.close();
}
console.log(typeof Symbol.asyncDispose, typeof fh[Symbol.asyncDispose]); // symbol function
rmSync(dir, { recursive: true, force: true });
```

***

#### Streams

##### Always `pipeline()`, Never Bare `.pipe()` Chains

`readable.pipe(writable)` does not forward errors or destroy the other streams on failure, which leaks file descriptors and leaves half-written files. `stream/promises` `pipeline()` propagates errors, handles backpressure, destroys every stream on failure, and accepts async generator transforms and Web Streams (19.7+).

```js
import { pipeline } from 'node:stream/promises';
import { createReadStream, createWriteStream, writeFileSync, mkdtempSync, rmSync, readFileSync } from 'node:fs';
import { createGzip, gunzipSync } from 'node:zlib';
import { tmpdir } from 'node:os';
import { join } from 'node:path';

const dir = mkdtempSync(join(tmpdir(), 'pipe-'));
writeFileSync(join(dir, 'in.txt'), 'a\nb\nc\n');

// BAD: errors on any stream are not propagated; the gzip and file stay open on failure.
// createReadStream(src).pipe(createGzip()).pipe(createWriteStream(dst));

// GOOD
await pipeline(
  createReadStream(join(dir, 'in.txt'), { encoding: 'utf8' }),
  async function* upper(source, { signal }) {
    for await (const chunk of source) yield chunk.toUpperCase();
  },
  createGzip(),
  createWriteStream(join(dir, 'out.txt.gz')),
  { signal: AbortSignal.timeout(5_000) },
);
console.log(gunzipSync(readFileSync(join(dir, 'out.txt.gz'))).toString()); // A\nB\nC\n
rmSync(dir, { recursive: true, force: true });
```

##### Backpressure When Writing Manually

```js
import { Writable } from 'node:stream';
import { once } from 'node:events';

const slow = new Writable({
  highWaterMark: 4,
  write(chunk, _enc, cb) { setTimeout(cb, 1); },
});

// GOOD: respect write()'s return value and wait for 'drain'.
for (let i = 0; i < 20; i++) {
  if (!slow.write(`chunk-${i}`)) await once(slow, 'drain');
}
slow.end();
await once(slow, 'finish');
console.log('written with backpressure');
```

Review flags: `for (…) stream.write(x)` without checking the return value (unbounded memory), `readable.on('data')` with async handlers (no backpressure; use `for await`), `stream.Readable.prototype.map/filter/flatMap` treated as stable (they are **Experimental** in 20; the `concurrency` option is the reason to use them).

##### Web Streams and Interop

`ReadableStream`, `WritableStream`, `TransformStream`, `TextDecoderStream` and `CompressionStream` are globals (still marked Experimental in the Node 20 docs; stable from 21). Convert at the boundary with `Readable.toWeb()` / `Readable.fromWeb()`; `ReadableStream.from(iterable)` exists since 20.6.0.

```js
import { Readable } from 'node:stream';

const web = ReadableStream.from(['a', 'b']);               // 20.6+
const nodeReadable = Readable.fromWeb(web, { encoding: 'utf8' });
let text = '';
for await (const chunk of nodeReadable) text += chunk;
console.log(text); // ab

// Web stream bodies must carry Uint8Array chunks: a Node stream of strings fails with
// "TypeError: Received non-Uint8Array chunk" when the Response is consumed.
const res = new Response(Readable.toWeb(Readable.from([Buffer.from('x'), Buffer.from('y')])));
console.log(await res.text()); // xy
```

***

#### File System

```js
import { readFile, writeFile, rename, mkdir, readdir, rm, mkdtemp } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';

const dir = await mkdtemp(join(tmpdir(), 'fs-'));

// Atomic replace: write a temp file in the same directory, then rename.
async function writeJsonAtomic(file, data) {
  const tmp = `${file}.${process.pid}.tmp`;
  await writeFile(tmp, JSON.stringify(data), { encoding: 'utf8', flush: true }); // flush: 20.10+
  await rename(tmp, file);
}
await writeJsonAtomic(join(dir, 'state.json'), { ok: true });
console.log(JSON.parse(await readFile(join(dir, 'state.json'), 'utf8'))); // { ok: true }

// Recursive listing (20.1+); Dirent.parentPath (20.12+) replaces the deprecated dirent.path.
await mkdir(join(dir, 'a', 'b'), { recursive: true });
await writeFile(join(dir, 'a', 'b', 'f.txt'), '');
for (const d of await readdir(dir, { recursive: true, withFileTypes: true })) {
  if (d.isFile()) console.log(join(d.parentPath, d.name).slice(dir.length)); // /state.json, /a/b/f.txt (order not guaranteed)
}

// ENOENT is an expected condition: check the code, not the message.
try { await readFile(join(dir, 'missing')); } catch (err) { console.log(err.code); } // ENOENT
await rm(dir, { recursive: true, force: true });
```

- Never use `fs.existsSync()` followed by `open()` (time-of-check/time-of-use race); open directly and handle `ENOENT`/`EEXIST` (`{ flag: 'wx' }` for create-only).
- Sync APIs (`readFileSync`, `execSync`) are fine at startup and in CLIs, never inside request handlers.
- `fs.cp()` is still **Experimental** on 20 (stable in 22). `fs.glob()` does not exist on 20 — use a library (`fast-glob`, `tinyglobby`) or `readdir({ recursive: true })` with `path.matchesGlob()` (20.17+, experimental).
- `fs.rmdir(…, { recursive: true })` is deprecated (DEP0147): use `fs.rm(…, { recursive: true, force: true })`.
- `fs.watch` recursive works on Linux since 19.1, but is still prone to platform differences; use `chokidar` for cross-platform watchers.

##### Path Traversal

```js
import path from 'node:path';

const ROOT = path.resolve('/srv/uploads');

// BAD: path.join happily walks out of ROOT.
console.log(path.join(ROOT, '../../etc/passwd')); // /etc/passwd

// GOOD: resolve, then verify the result is still inside ROOT.
function safeResolve(userPath) {
  const target = path.resolve(ROOT, userPath);
  if (target !== ROOT && !target.startsWith(ROOT + path.sep)) {
    throw Object.assign(new Error('path escapes root'), { code: 'EACCES' });
  }
  return target;
}
console.log(safeResolve('a/b.txt')); // /srv/uploads/a/b.txt
try { safeResolve('../../etc/passwd'); } catch (e) { console.log(e.code); } // EACCES
try { safeResolve('/etc/passwd'); } catch (e) { console.log(e.code); } // EACCES
```

Symlinks inside `ROOT` can still point outside; for untrusted trees resolve with `fs.realpath()` before the prefix check. On Windows, `path.normalize()` had device-name traversal bugs fixed in 20.18.2 (CVE-2025-23084) and 20.19.4 (CVE-2025-27210) — another reason Node 20's lack of further fixes matters.[^21]

***

#### HTTP, `fetch()` and Networking

##### `fetch()` on Node 20

`fetch`, `Request`, `Response`, `Headers` and `FormData` are globals backed by undici (6.24.1 in 20.20.2). In the Node 20 documentation `fetch` is still **Stability 1 – Experimental** (it became stable in 21.0.0), but it is enabled by default and prints no warning.[^22] It is fine for production use on 20, with these rules:

```js
import http from 'node:http';

const server = http.createServer((req, res) => {
  if (req.url === '/slow') return; // never answers
  res.writeHead(req.url === '/missing' ? 404 : 200, { 'content-type': 'application/json' });
  res.end('{"ok":true}');
}).listen(0);
await new Promise((r) => server.once('listening', r));
const base = `http://127.0.0.1:${server.address().port}`;

// 1. fetch() does NOT reject on HTTP errors — check res.ok.
const res = await fetch(`${base}/missing`);
console.log(res.status, res.ok); // 404 false
await res.body?.cancel();         // 2. always consume or cancel the body, or the connection is held

// 3. No overall request timeout (undici's per-phase defaults are 300 s for headers and body): pass a signal.
try {
  await fetch(`${base}/slow`, { signal: AbortSignal.timeout(100) });
} catch (err) {
  console.log(err.name); // TimeoutError
}

// 4. Network failures are TypeError('fetch failed') with the real reason in err.cause.
try {
  await fetch('http://127.0.0.1:59999/');
} catch (err) {
  console.log(err.message, err.cause?.code); // fetch failed ECONNREFUSED
}

// 5. Streaming request bodies require duplex: 'half'.
try {
  await fetch(`${base}/`, { method: 'POST', body: ReadableStream.from([new TextEncoder().encode('x')]) });
} catch (err) {
  console.log(err.message); // RequestInit: duplex option is required when sending a body.
}
const ok = await fetch(`${base}/`, {
  method: 'POST',
  body: ReadableStream.from([new TextEncoder().encode('x')]),
  duplex: 'half',
});
console.log(await ok.json()); // { ok: true }

server.closeAllConnections();
server.close();
```

- `fetch()` ignores `HTTP_PROXY`/`HTTPS_PROXY` on Node 20. Proxy support needs undici's `ProxyAgent` via `dispatcher` (`NODE_USE_ENV_PROXY=1` exists only from Node 22.21).
- Use the `undici` npm package only when you need its extra APIs; pin a version that supports Node 20 (undici 7 requires `>=20.18.1`, undici 8 requires Node 22.19+).

##### HTTP Servers

```js
import http from 'node:http';

const server = http.createServer({ requestTimeout: 30_000, headersTimeout: 10_000 }, async (req, res) => {
  try {
    res.setHeader('content-type', 'application/json');
    res.end(JSON.stringify({ path: req.url }));
  } catch (err) {
    res.statusCode = 500;
    res.end();
  }
});
server.keepAliveTimeout = 5_000;   // must be lower than the load balancer's idle timeout
console.log(server.requestTimeout, server.headersTimeout, http.globalAgent.keepAlive); // 30000 10000 true

server.listen(0, () => {
  // Graceful shutdown: stop accepting, let in-flight requests finish, then exit.
  process.once('SIGTERM', () => server.close(() => process.exit(0)));
  server.close(); // closed right away so the example exits; close() also drops idle keep-alive sockets (Node 19+)
});
```

- Defaults on 20: `requestTimeout` 300 s, `headersTimeout` 60 s, `keepAliveTimeout` 5 s. `http.globalAgent` uses keep-alive by default (since 19); a client that disables keep-alive on purpose must pass `agent: false` or its own `Agent`.
- `net.connect`/`http.request` use Happy Eyeballs (`autoSelectFamily: true`, 250 ms attempt timeout) by default since 20.0.0; a host with broken IPv6 now fails over to IPv4 instead of timing out. `dns.lookup()` keeps the `verbatim` order; `--dns-result-order=ipv4first` remains available and `ipv6first` was added in 20.13.0.
- Validate `Content-Length`/body size limits in every handler that buffers bodies; Node itself does not cap request body size.
- `req.headersDistinct` / `trailersDistinct` use null-prototype objects since 20.20.2; a `__proto__` header crashed earlier releases (CVE-2026-21710).[^4]

***

#### Configuration and CLI

##### New in 20.6 / 20.12: `.env` Files

```js
// node --env-file=.env app.js      (20.6+; repeat the flag for several files, later files win, 20.7+)
// node --env-file-if-exists=.env.local app.js   (20.19+; no error when missing)
import { parseEnv } from 'node:util';
import { writeFileSync, rmSync } from 'node:fs';

writeFileSync('.env.test', 'A=fromfile\nB="multi\nline"\nC=x # comment\nexport D=1\nE=${A}\n');
process.env.A = 'shell';
process.loadEnvFile('.env.test');      // 20.12+
console.log(process.env.A, JSON.stringify(process.env.B), process.env.C, process.env.D, process.env.E);
// shell "multi\nline" x 1 ${A}
console.log(parseEnv('X=1\nY="a b"'));  // { X: '1', Y: 'a b' }  (20.12+)
rmSync('.env.test');
```

Verified semantics on 20.20.2:

- Variables already present in the environment **win** over the file (`A` stays `shell`).
- There is **no variable expansion** (`E` is the literal `${A}`); `export` prefixes, `#` comments and quoted multi-line values (20.12+) are supported.
- A missing file passed with `--env-file` is fatal (`node: missing.env: not found`, exit code 9); `process.loadEnvFile()` throws `ENOENT`.
- `NODE_OPTIONS` in the file is applied (e.g. `--max-old-space-size`), unless `NODE_OPTIONS` is already set in the real environment.
- The feature is still Experimental (Stability 1.1) on 20. Keep secrets out of the repository; `.env` is a development convenience, not a secret store.

##### Argument Parsing: `util.parseArgs()` (stable since 20.0)

```js
import { parseArgs } from 'node:util';

const { values, positionals } = parseArgs({
  args: ['--port', '8080', '--no-color', 'serve'],
  allowPositionals: true,
  allowNegative: true, // 20.16+: accept --no-<flag> for boolean options
  options: {
    port: { type: 'string', short: 'p', default: '3000' },
    color: { type: 'boolean', default: true },
  },
});
console.log(values, positionals); // [Object: null prototype] { port: '8080', color: false } [ 'serve' ]
```

Unknown options throw `ERR_PARSE_ARGS_UNKNOWN_OPTION` in strict mode (the default), which is what a CLI wants.

##### Terminal Output: `util.styleText()`

`util.styleText(format, text)` (20.12+) colours text with `util.inspect.colors` names. From 20.18.0 a single format string respects `NO_COLOR`, `FORCE_COLOR` and whether the stream is a TTY. **Caveat verified on 20.20.2:** an *array* of formats (`styleText(['bold', 'red'], s)`, 20.13+) still emits ANSI codes when stdout is not a TTY, whereas Node 22/24 strip them. Gate arrays on `process.stdout.isTTY` or use a single format.

```js
import { styleText } from 'node:util';
// Output piped to a file / CI log (not a TTY):
console.log(JSON.stringify(styleText('red', 'x')));            // "x"
console.log(JSON.stringify(styleText(['bold', 'red'], 'y')));  // "\u001b[1m\u001b[31my\u001b[39m\u001b[22m" on 20.20.2
```

***

#### Security

##### The Runtime Itself: End-of-Life

The most important security finding for a Node 20 project is the runtime. 20.20.2 (March 24, 2026) is the last release; it fixed, among others, a V8 HashDoS via integer-like strings (CVE-2026-21717, reachable through `JSON.parse` of attacker-controlled keys), a timing-unsafe HMAC comparison in Web Crypto (CVE-2026-21713), the `__proto__` header crash (CVE-2026-21710) and `SNICallback` exceptions bypassing TLS error handlers (CVE-2026-21637).[^4][^23] Vulnerabilities fixed later in 22.x/24.x/26.x (June, July and October 2026 releases) are not fixed for 20.[^6] Recommend, in this order: upgrade to Node 24 LTS (or 22 at minimum); if impossible right now, pin `20.20.2` exactly (`FROM node:20.20.2-…`, not `node:20`), keep request body and JSON size limits small, and track a commercial extended-support build.

Security behaviour that changed inside 20.x and that code may depend on:

- **20.11.1:** `crypto.privateDecrypt()` with `RSA_PKCS1_PADDING` throws `ERR_INVALID_ARG_VALUE` (Marvin attack, CVE-2023-46809) unless OpenSSL supports implicit rejection. Use OAEP (`oaepHash: 'sha256'`).
- **20.12.2:** on Windows, `child_process.spawn()`/`execFile()` of `.bat`/`.cmd` files without `shell: true` throws `EINVAL` (CVE-2024-27980, "BatBadBut"); passing user input with `shell: true` is still injection.[^24]
- **20.20.0:** `TLSSocket`s get a default error handler (CVE-2025-59465); callback exceptions in TLS are routed to error handlers (CVE-2026-21637); stack-overflow exceptions in `async_hooks` are rethrown (CVE-2025-59466); the Permission Model blocks `futimes` and requires full read+write for symlink APIs.[^25]

```js
const crypto = require('node:crypto');
const { publicKey, privateKey } = crypto.generateKeyPairSync('rsa', { modulusLength: 2048 });

// BAD: PKCS#1 v1.5 decryption — throws on Node 20.11.1+ (and is attackable where it does not).
const legacy = crypto.publicEncrypt({ key: publicKey, padding: crypto.constants.RSA_PKCS1_PADDING }, Buffer.from('hi'));
try {
  crypto.privateDecrypt({ key: privateKey, padding: crypto.constants.RSA_PKCS1_PADDING }, legacy);
} catch (err) { console.log(err.code); } // ERR_INVALID_ARG_VALUE

// GOOD: OAEP with SHA-256.
const enc = crypto.publicEncrypt({ key: publicKey, oaepHash: 'sha256' }, Buffer.from('hi'));
console.log(crypto.privateDecrypt({ key: privateKey, oaepHash: 'sha256' }, enc).toString()); // hi
```

##### Crypto Essentials

```js
const crypto = require('node:crypto');

// Random identifiers and tokens
console.log(crypto.randomUUID().length, crypto.randomBytes(32).toString('base64url').length); // 36 43

// One-shot hashing (20.12+): faster than createHash() for small inputs; not for passwords.
console.log(crypto.hash('sha256', 'abc'));
// ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad

// Constant-time comparison for secrets/MACs (lengths must match first).
function safeEqual(a, b) {
  const ba = Buffer.from(a), bb = Buffer.from(b);
  return ba.length === bb.length && crypto.timingSafeEqual(ba, bb);
}
console.log(safeEqual('token', 'token'), safeEqual('token', 'tokeN')); // true false

// Password hashing: scrypt (or argon2 from npm), never a plain hash.
const salt = crypto.randomBytes(16);
const hash = crypto.scryptSync('correct horse', salt, 64, { N: 16384, r: 8, p: 1 });
console.log(hash.length); // 64

// Authenticated encryption: AES-256-GCM with a fresh 12-byte IV per message.
const key = crypto.randomBytes(32);
const iv = crypto.randomBytes(12);
const cipher = crypto.createCipheriv('aes-256-gcm', key, iv);
const ct = Buffer.concat([cipher.update('secret', 'utf8'), cipher.final()]);
const tag = cipher.getAuthTag();
const decipher = crypto.createDecipheriv('aes-256-gcm', key, iv, { authTagLength: 16 });
decipher.setAuthTag(tag);
console.log(Buffer.concat([decipher.update(ct), decipher.final()]).toString()); // secret
```

- `crypto.createCipher()` (no IV, MD5 key derivation) is deprecated and **removed in Node 22** — flag every use.
- Pass `authTagLength` when decrypting GCM; accepting shorter tags without it is doc-deprecated since 20.13.0 (DEP0182; no runtime warning on 20).
- In an ES module or script file, the global `crypto` is **Web Crypto** (`crypto.subtle`, `crypto.randomUUID()`, `crypto.getRandomValues()`), not `node:crypto`: `crypto.createHash` is `undefined` there. Import `node:crypto` explicitly. (In `node -e` and the REPL the name refers to the `node:crypto` module, which hides the difference.)
- Web Crypto `Ed25519`/`X25519` became stable in 20.19.3; on older minors they were experimental.

##### Injection

```js
const { execFile } = require('node:child_process');
const { promisify } = require('node:util');
const execFileAsync = promisify(execFile);

async function listDir(userInput) {
  // BAD: exec(`ls ${userInput}`) — shell injection with "; rm -rf ~".
  // GOOD: no shell, arguments as an array, `--` ends option parsing, bounded runtime/output.
  const { stdout } = await execFileAsync('ls', ['--', userInput], { timeout: 5_000, maxBuffer: 1 << 20 });
  return stdout;
}
listDir('.').then((out) => console.log(out.length > 0)).catch((e) => console.log(e.code)); // true
```

- SQL: parameterized queries only (`client.query('SELECT … WHERE id = $1', [id])`); never template literals into SQL.
- `eval`, `new Function`, `vm.runInNewContext` on untrusted input: `node:vm` is **not** a security boundary.
- ReDoS: user-controlled input against backtracking regexes (`/(a+)+$/`) blocks the event loop. Bound input length, prefer linear patterns, or run matching in a worker with a timeout.
- Untrusted `JSON.parse` of huge payloads blocks the loop (and on Node 20 < 20.20.2 enables the integer-key HashDoS); enforce body size limits before parsing.

##### Prototype Pollution

```js
// BAD: recursive merge of user JSON into a plain object.
function mergeBad(target, src) {
  for (const k in src) {
    if (typeof src[k] === 'object' && src[k] !== null) mergeBad((target[k] ??= {}), src[k]);
    else target[k] = src[k];
  }
  return target;
}
mergeBad({}, JSON.parse('{"__proto__": {"isAdmin": true}}'));
console.log({}.isAdmin); // true  <- every object in the process is now "admin"
delete Object.prototype.isAdmin;

// GOOD: reject dangerous keys and use null-prototype targets or Map.
const FORBIDDEN = new Set(['__proto__', 'constructor', 'prototype']);
function mergeSafe(target, src) {
  for (const k of Object.keys(src)) {
    if (FORBIDDEN.has(k)) continue;
    const v = src[k];
    if (typeof v === 'object' && v !== null && !Array.isArray(v)) {
      target[k] = mergeSafe(Object.hasOwn(target, k) ? target[k] : Object.create(null), v);
    } else {
      target[k] = v;
    }
  }
  return target;
}
mergeSafe(Object.create(null), JSON.parse('{"__proto__": {"isAdmin": true}}'));
console.log({}.isAdmin); // undefined
```

`node --disable-proto=throw` (or `=delete`) removes the `__proto__` accessor process-wide as defence in depth.

##### New in 20: The Permission Model (Experimental)

`--experimental-permission` restricts file system, child processes, worker threads, native addons (`--allow-addons`, 20.12+) and WASI (`--allow-wasi`, 20.16+). It does **not** restrict the network on Node 20, and the stable `--permission` spelling does not exist (bad option).[^26]

```js
// Run with:
//   node --experimental-permission --allow-fs-read=/app --allow-fs-write=/app/tmp app.cjs
const fs = require('node:fs');
if (process.permission) {
  console.log(process.permission.has('fs.write', '/etc')); // false
}
try {
  fs.readFileSync('/etc/hostname');
} catch (err) {
  console.log(err.code, err.permission, err.resource); // ERR_ACCESS_DENIED FileSystemRead /etc/hostname
}
```

Observed on 20.20.2: without `--allow-fs-read` covering the entry file, the process cannot even load it; `child_process` and `new Worker()` throw `ERR_ACCESS_DENIED`; `net.createServer().listen()` still works; an `ExperimentalWarning` is printed. Since 20.7.0 multiple paths are given by repeating the flag; a comma-separated list is not split: `--allow-fs-read=/tmp,/etc` is treated as one literal path and grants neither directory (verified on 20.20.2). Many permission bypasses were fixed during the 20.x line (and later ones only in 22+), so treat the model as a seat belt against mistakes, not a sandbox for malicious code.

##### Dependency and Supply-Chain Hygiene

- Commit the lockfile; install with `npm ci` in CI and images. Use `npm audit --omit=dev` and `npm audit signatures` (npm 10).
- Review `postinstall` scripts of new dependencies; consider `npm ci --ignore-scripts` in CI where feasible.
- Check `engines` of new dependencies: in 2026 many majors dropped Node 20 or require `^20.19.0` (see Tooling).

***

#### Performance

- Measure first: `node --cpu-prof` / `--heap-prof` (stable since 20.16.0), `--prof`, Chrome DevTools via `--inspect`, `perf_hooks.monitorEventLoopDelay()`, `performance.eventLoopUtilization()`.
- `crypto.hash()` (20.12+) beats `createHash().update().digest()` for small, in-memory inputs; `fs.readFileSync(path, 'utf8')` and `fs.writeFileSync` with UTF-8 strings got fast paths in 20.5/20.11.[^7]
- Avoid `JSON.parse(JSON.stringify(x))` for cloning (`structuredClone`), `delete` on hot objects (changes hidden classes), and `Array.prototype.includes` inside loops over large arrays (use a `Set`).
- Stream large payloads instead of buffering (`pipeline`, `for await`).
- Container memory: set `--max-old-space-size` (in MiB) explicitly to ~75 % of the container limit; `process.constrainedMemory()` reports the cgroup limit and `process.availableMemory()` (20.13+) the free amount.
- Startup: the V8 compile cache (`module.enableCompileCache()`) is Node 22+; on 20 use single executable apps with `useCodeCache` (20.6+) or a bundler if startup time matters.

```js
import { monitorEventLoopDelay, performance } from 'node:perf_hooks';

const h = monitorEventLoopDelay({ resolution: 10 });
h.enable();
await new Promise((r) => setTimeout(r, 20)); // let the histogram's timer start
const start = performance.now();
while (performance.now() - start < 50) { /* simulate 50 ms of blocking work */ }
await new Promise((r) => setTimeout(r, 30));
h.disable();
console.log(h.max / 1e6 >= 40, performance.eventLoopUtilization().utilization > 0); // true true
console.log(process.constrainedMemory() >= 0, typeof process.availableMemory()); // true number
```

***

#### Testing with `node:test` (Stable in 20)

The built-in runner is stable since 20.0.0 and covers most unit-test needs without dependencies.[^27]

```js
// test/order.test.mjs — run with: node --test
import { describe, it, test, mock, beforeEach } from 'node:test';
import assert from 'node:assert/strict';

function total(items) { return items.reduce((s, i) => s + i.price * i.qty, 0); }
async function later(ms) { return new Promise((r) => setTimeout(() => r('done'), ms)); }

describe('total', () => {
  let items;
  beforeEach(() => { items = [{ price: 2, qty: 3 }]; });
  it('sums price * qty', () => { assert.equal(total(items), 6); });
  it('handles empty input', () => { assert.equal(total([]), 0); });
});

test('mock timers and Date (20.11+ option object)', (t) => {
  t.mock.timers.enable({ apis: ['setTimeout', 'Date'], now: 1_000 });
  const p = later(5_000);
  t.mock.timers.tick(5_000);
  assert.equal(Date.now(), 6_000);
  return p.then((v) => assert.equal(v, 'done'));
});

test('mock functions and methods', (t) => {
  const fn = mock.fn((x) => x * 2);
  fn(2);
  assert.equal(fn.mock.callCount(), 1);
  assert.deepEqual(fn.mock.calls[0].arguments, [2]);
  const obj = { now: () => 1 };
  t.mock.method(obj, 'now', () => 42);   // restored automatically after the test
  assert.equal(obj.now(), 42);
});

test('subtests must be awaited', async (t) => {
  t.plan(1);                              // 20.15+
  await t.test('child', () => { assert.ok(true); });
});
```

Verified behaviour on 20.20.2 that reviewers should know:

- `node --test` with no arguments finds `**/*.test.{js,mjs,cjs}`, `**/test/**/*.{js,mjs,cjs}` and similar patterns. **Glob arguments are not supported on 20** (`node --test 'test/**/*.test.js'` fails with "Could not find"); pass directories/files or let the shell expand the glob. Globs arrived in 21.
- The default reporter is `spec` on a TTY and `tap` otherwise; set `--test-reporter=spec` (or `junit`, 20.8+; `lcov`, 20.11+) in CI explicitly.
- A subtest that is not awaited fails with "test did not finish before its parent and was cancelled".
- `test.only()` runs everything unless `--test-only` is passed (the runner prints a hint).
- Coverage: `--experimental-test-coverage` works with `--test` (flag available since 19.7/18.15; still experimental on 20). Coverage include/exclude globs and thresholds are Node 22 features.
- `t.mock.timers.enable()` takes an options object since 20.11.0 (`{ apis, now }`); only that form can set the start time (`now`). The original 20.4 array form (`enable(['setTimeout'])`) still works on 20.20.2 (an array containing `'Date'` starts the clock at 0).
- `MockTimers` and `t.plan()` are still Experimental on 20 (using mock timers prints an `ExperimentalWarning`).
- `t.assert` exists (without `snapshot`); snapshot testing, `mock.module()` without `--experimental-test-module-mocks`, and `--experimental-test-isolation` are not available.
- Useful flags: `--test-name-pattern`, `--test-concurrency` (20.10+), `--test-timeout` (20.11+), `--test-force-exit` (20.14+), `--test-shard` (20.5+), `--watch` with `--test`.

Jest 30 and Vitest 4 still support Node 20 (`engines` include `^20.0.0`); Vitest 5 requires Node 22.12+, so a Vitest major upgrade is blocked on the runtime upgrade.[^18]

***

#### Tooling

- **Package manager:** npm 10.8.2 ships with 20.17+. Corepack 0.34.6 is bundled (experimental) and can pin pnpm/Yarn via `"packageManager"`; Corepack is no longer bundled from Node 25, so prefer installing the package manager explicitly in CI.
- **Watch mode:** `node --watch app.js` (stable since 20.13.0) restarts on changes; `--watch-path` limits the watched paths; use it instead of nodemon for simple cases.
- **TypeScript:** Node 20 cannot run `.ts` files itself. Compile with `tsc`, or use `tsx`/`ts-node`/a bundler. Use `@types/node@20` (latest 20.19.x types) so the type checker rejects Node 22+ APIs, and a config such as `@tsconfig/node20` (`target: es2022`, `lib: ["es2023"]`, `module: nodenext`). `lib: ["esnext"]` would type-check `Object.groupBy` and `Promise.withResolvers` that crash at runtime.
- **Linting:** ESLint with `eslint-plugin-n`'s `n/no-unsupported-features/es-syntax`, `es-builtins` and `node-builtins` rules reading `engines` catches newer APIs. ESLint 10 requires `^20.19.0`; ESLint 9 supports `^20.9.0`.
- **Engines:** state the real floor (`">=20.19.0"` if you rely on `require(esm)` or `--env-file-if-exists`, `">=20.12.0"` for `process.loadEnvFile`/`crypto.hash`/`styleText`). Add `engine-strict=true` in `.npmrc` if installs on older runtimes must fail.
- **Docker:** pin the full version and variant (`node:20.20.2-bookworm-slim`), run as the `node` user, `NODE_ENV=production`, `npm ci --omit=dev`, and use an init process (`docker run --init` or `tini`) so `SIGTERM` reaches Node.
- **Debugging:** `--inspect`, `--inspect-brk`, `--inspect-wait` (20.15+); `--enable-source-maps`; `--disable-warning=DEP0005` (20.11+) to silence one warning code instead of `--no-warnings`.

```json
{
  "compilerOptions": {
    "target": "es2022",
    "lib": ["es2023"],
    "module": "nodenext",
    "moduleResolution": "nodenext",
    "types": ["node"],
    "strict": true,
    "skipLibCheck": true
  }
}
```

***

#### Observability

- Structured logs (pino or a JSON logger) with request ids from `AsyncLocalStorage`; never log secrets, tokens or full request bodies.
- `node:diagnostics_channel` is stable; Node 20 publishes `http.client.request.start`, `http.server.request.start`, `http.client.request.error` (20.17+), `net.server.listen` tracing (20.16+) and undici's `undici:request:create`/`undici:request:headers` channels for `fetch()` (verified on 20.20.2) that OpenTelemetry instrumentations consume.
- `process.report.writeReport()` / `--report-on-fatalerror` produce JSON diagnostic reports; `--report-exclude-network` (20.13+) avoids slow DNS lookups in them.
- `v8.writeHeapSnapshot()`, `--heapsnapshot-near-heap-limit=N`, and `v8.queryObjects(Ctor)` (20.13+) for leak hunting.
- `performance.timerify()`, `PerformanceObserver` and `performance.mark()/measure()` — call them on `performance` (since 19 an unbound `const { now } = performance; now()` throws `ERR_INVALID_ARG_TYPE`).

```js
import diagnosticsChannel from 'node:diagnostics_channel';
import http from 'node:http';

diagnosticsChannel.subscribe('http.server.request.start', ({ request }) => {
  console.log('request', request.method, request.url);
});
const server = http.createServer((req, res) => res.end('ok')).listen(0);
await new Promise((r) => server.once('listening', r));
await (await fetch(`http://127.0.0.1:${server.address().port}/health`)).text(); // request GET /health
server.close();
```

***

#### Single Executable Applications (SEA)

Node 20 can embed one **CommonJS** script (bundle your app first) into a copy of the `node` binary. Since 20.0.0 the script must be prepared as a blob; assets (`sea.getAsset()`, `getAssetAsBlob()`, `getRawAsset()`, `isSea()`) arrive in 20.12.0, `useSnapshot`/`useCodeCache` in 20.6.0. The feature is Experimental (Stability 1.1).[^28]

```js
// app.cjs — the SEA entry (bundle dependencies into this file; require() in a SEA
// only loads built-in modules).
const sea = require('node:sea');
console.log('isSea', sea.isSea(), sea.isSea() ? sea.getAsset('greeting.txt', 'utf8').trim() : '-');
```

```json
{
  "main": "app.cjs",
  "output": "sea-prep.blob",
  "disableExperimentalSEAWarning": true,
  "assets": { "greeting.txt": "greeting.txt" }
}
```

Build (Linux, verified with 20.20.2 and `postject@1.0.0-alpha.6`): `node --experimental-sea-config sea-config.json`, `cp "$(command -v node)" app`, then `npx postject app NODE_SEA_BLOB sea-prep.blob --sentinel-fuse NODE_SEA_FUSE_fce680ab2cc467b6e072b8b5df1996b2`. On macOS/Windows remove and re-apply the code signature. The one-step `node --build-sea` (Node 25+) does not exist on 20; do not suggest it.

***

#### URLs and Query Strings

Use the WHATWG `URL` API (Ada parser in 20). `url.parse()` is legacy: it accepts malformed input that other parsers read differently, which has led to host-confusion bugs (DEP0169).

```js
// Parse-or-null without try/catch: URL.canParse (18.17+/19.9+) and URL.parse (20.18+).
console.log(URL.canParse('https://example.com/a?b=1'), URL.canParse('not a url')); // true false
console.log(URL.parse('relative', 'https://example.com/base/')?.href); // https://example.com/base/relative
console.log(URL.parse('::bad') ?? 'invalid');                         // invalid

// Build URLs with URLSearchParams — never by string concatenation.
const u = new URL('https://api.example.com/search');
u.searchParams.set('q', 'a&b=c');
u.searchParams.append('tag', 'x');
u.searchParams.append('tag', 'y');
console.log(u.href); // https://api.example.com/search?q=a%26b%3Dc&tag=x&tag=y
console.log(u.searchParams.size, u.searchParams.has('tag', 'y')); // 3 true  (has(name, value): 20.2+)
u.searchParams.delete('tag', 'x');                                // delete(name, value): 20.2+
console.log(u.searchParams.getAll('tag'));                        // [ 'y' ]

// SSRF guard: check the parsed protocol and hostname, not a string prefix.
function isAllowed(raw) {
  const url = URL.parse(raw);
  return url !== null && url.protocol === 'https:' && url.hostname === 'api.example.com';
}
console.log(isAllowed('https://api.example.com.evil.net/'), isAllowed('https://api.example.com/x')); // false true
```

- Use `fileURLToPath()`/`pathToFileURL()` to convert between paths and `file:` URLs; never strip `file://` by hand (breaks on Windows and with percent-encoding).
- Hostname allow-lists must also consider redirects (`fetch` follows up to 20 by default; pass `redirect: 'manual'` or re-validate) and DNS rebinding to private addresses.

***

#### Collections, Iteration and JSON

```js
// Map for dynamic keys (any key type, insertion order, size, no prototype keys).
const counts = new Map();
for (const word of 'a b a c b a'.split(' ')) counts.set(word, (counts.get(word) ?? 0) + 1);
console.log([...counts].toSorted((x, y) => y[1] - x[1])[0]); // [ 'a', 3 ]

// Plain objects keyed by user input inherit from Object.prototype:
const lookup = {};
console.log('constructor' in lookup, Object.hasOwn(lookup, 'constructor')); // true false

// WeakMap for per-object metadata without leaks; symbols can be WeakMap keys in 20.
const meta = new WeakMap();
const key = Symbol('session');
meta.set(key, { created: 0 });
console.log(meta.has(key)); // true

// Generators keep memory flat over large or infinite sequences (no iterator helpers on 20).
function* take(iterable, n) {
  if (n <= 0) return;
  for (const x of iterable) { yield x; if (--n === 0) return; }
}
function* naturals() { for (let i = 1; ; i++) yield i; }
console.log([...take(naturals(), 3)]); // [ 1, 2, 3 ]

// JSON: revive dates explicitly; JSON.parse never produces Date objects.
const parsed = JSON.parse('{"at":"2026-01-01T00:00:00.000Z"}', (k, v) =>
  k === 'at' ? new Date(v) : v);
console.log(parsed.at instanceof Date); // true
```

- `for…in` iterates inherited enumerable keys; use `Object.keys()`/`Object.entries()` or `for…of` over a `Map`.
- `Array.prototype.includes()` handles `NaN`, `indexOf()` does not. `Array.prototype.at(-1)` for the last element.
- `JSON.stringify` drops `undefined`, functions and symbols, turns `NaN`/`Infinity` into `null`, calls `toJSON()`, and throws on `BigInt` and cycles. `JSON.rawJSON()` is Node 21+.

***

#### Buffers and Binary Data

```js
const { Buffer, isUtf8, isAscii } = require('node:buffer');

// BAD: deprecated constructor (DEP0005) — and Buffer(number) was historically uninitialized memory.
// const b = new Buffer(10);

const zeroed = Buffer.alloc(8);                 // zero-filled
const fromText = Buffer.from('héllo', 'utf8');
console.log(zeroed.length, fromText.length, fromText.toString('base64')); // 8 6 aMOpbGxv

// Buffer.allocUnsafe() may contain old memory: only use it when you overwrite every byte.
const scratch = Buffer.allocUnsafe(4);
scratch.fill(0);

// Validate encodings of untrusted input (buffer.isUtf8: 18.14+/19.4+, buffer.isAscii: 18.15+/19.6+).
console.log(isUtf8(fromText), isAscii(fromText), isUtf8(Buffer.from([0xff]))); // true false false

// buf.subarray() (and the doc-deprecated buf.slice()) SHARE memory, unlike Uint8Array#slice().
const base = Buffer.from('abcdef');
const view = base.subarray(0, 3);
view[0] = 0x58; // 'X'
console.log(base.toString()); // Xbcdef

// Pooled small Buffers: .buffer is the whole 8 KiB pool, not just your bytes.
const small = Buffer.from('hi');
console.log(small.buffer.byteLength >= small.length, small.byteOffset >= 0); // true true
const exact = small.buffer.slice(small.byteOffset, small.byteOffset + small.length);
console.log(exact.byteLength); // 2
```

Review flags: `buf.buffer` passed to APIs expecting exactly the payload (leaks other pool data), string concatenation of binary chunks (`data += chunk` corrupts multi-byte characters; collect chunks and `Buffer.concat()`, or use `setEncoding('utf8')`/`TextDecoder` with `stream: true`). `Uint8Array.fromBase64()` and `toHex()` are Node 25+; use `Buffer.from(str, 'base64')` and `buf.toString('hex')`.

***

#### Child Processes and Process Lifecycle

```js
const { spawn } = require('node:child_process');

// Stream output, enforce a timeout, and always handle 'error' (ENOENT) and the exit code.
function run(cmd, args, { timeoutMs = 10_000 } = {}) {
  return new Promise((resolve, reject) => {
    const child = spawn(cmd, args, { stdio: ['ignore', 'pipe', 'pipe'], timeout: timeoutMs });
    let out = '';
    child.stdout.setEncoding('utf8').on('data', (d) => { out += d; });
    child.once('error', reject);                       // e.g. ENOENT: binary not found
    child.once('close', (code, signal) => {
      if (code === 0) resolve(out);
      else reject(Object.assign(new Error(`${cmd} failed`), { code, signal }));
    });
  });
}

run('node', ['-e', 'console.log(process.version)'])
  .then((v) => console.log(v.trim().startsWith('v')))   // true
  .then(() => run('definitely-not-a-binary', []))
  .catch((err) => console.log(err.code));                // ENOENT
```

Graceful shutdown: handle `SIGTERM` and `SIGINT`, stop accepting work (`server.close()`), finish in-flight requests with a deadline, close pools, then exit with `process.exitCode`. Do not call `process.exit()` while logs or responses are still being written. Remember that `npm start` may not forward signals in every environment; run `node` directly as PID 1's child (`--init`).

***

#### Moving From Node 18 to 20 — Migration Notes

Behaviour that changes when a project's runtime moves from 18.x to 20.x (verified by running the same probe on 18.20.8 and 20.20.2 where possible):

1. **`process.exit()` / `process.exitCode`** accept only integers or integer strings; `process.exitCode = true` throws `ERR_INVALID_ARG_TYPE`, `1.5` throws `ERR_OUT_OF_RANGE`.
2. **HTTP keep-alive by default** for `http.globalAgent`/`https.globalAgent` (since 19): clients now reuse sockets; servers behind proxies must set `keepAliveTimeout` correctly and clients must consume response bodies.
3. **Happy Eyeballs** (`autoSelectFamily: true`) is the default (`net.getDefaultAutoSelectFamily()` is `false` on 18.20.8, `true` on 20.20.2).
4. **`fs.writeFile*`** no longer stringify objects with a custom `toString()` — they throw `ERR_INVALID_ARG_TYPE`.
5. **`performance` methods** must be called on `performance` (destructured `now()` throws).
6. **`process.config`** is frozen: writes are silently ignored in sloppy-mode CommonJS and throw `TypeError` in strict mode / ES modules.
7. **`crypto.DEFAULT_ENCODING`** is gone; RSA-PSS key-generation option names `hash`/`mgf1Hash` are runtime-deprecated (DEP0154) in favour of `hashAlgorithm`/`mgf1HashAlgorithm`.
8. **`url.parse()`** warns on non-numeric ports (DEP0170).
9. **ESM loaders** run off-thread: hooks can no longer share state with the application through globals; use `register(…, { data, transferList })` and `MessageChannel`. `import.meta.resolve()` is synchronous.
10. **`--experimental-specifier-resolution=node`** is ignored; extensionless relative ESM imports fail.
11. **`new WASI()`** requires the `version` option (`'preview1'`).
12. **Web Crypto global**: `globalThis.crypto` exists without a flag and is a getter-only property. The old polyfill pattern `global.crypto = require('crypto')` is silently ignored in CommonJS and throws `TypeError: Cannot set property crypto … which has only a getter` in ES modules; code must import `node:crypto` explicitly.
13. **Native addons** must be rebuilt (`NODE_MODULE_VERSION` 115, N-API 9 on 20.3+).
14. **npm 9 → 10** (bundled from 20.7.0; npm 10.8.2 declares `engines` `^18.17.0 || >=20.5.0`): regenerate lockfiles deliberately and pin the npm version in CI if lockfile churn matters.

***

#### Moving Off Node 20 — What Will Break on 22/24

Because Node 20 is end-of-life, every review should keep the upgrade path open. Code that runs on 20 but breaks on newer lines:

- `import … assert { type: 'json' }` → `SyntaxError` on 22+ (use `with`).
- `crypto.createCipher()`/`createDecipher()` → removed in 22.
- `readable.asIndexedPairs()` → removed in 21.0.0 (absent on 21.7.3 and 22).
- `util.isBoolean()`, `isBuffer()`, `isDate()`, `isError()`, `isFunction()`, `isNull()`, `isNullOrUndefined()`, `isNumber()`, `isObject()`, `isPrimitive()`, `isRegExp()`, `isString()`, `isSymbol()`, `isUndefined()` → removed in 23 (only `util.isArray()` and `util.isDeepStrictEqual()` remain); `util.isArray()`/`util._extend()` print runtime warnings from 22.
- `fs.F_OK`/`R_OK`/`W_OK`/`X_OK` on `fs` and `assert.CallTracker` → removed in 25.
- `--experimental-policy` → removed in 22; `--experimental-permission` → `--permission` is the stable name from 22.13 (the old flag still works on 22).
- `--experimental-loader` + `globalPreload` → `globalPreload` removed in 21.
- Global `navigator` appears in 21: code that sniffed `typeof navigator !== 'undefined'` to detect browsers misdetects Node 21+.
- Global `localStorage` (Node 25) and `WebSocket` (22) can shadow npm polyfills of the same name.
- `require(esm)` (22.12) and syntax detection (22.7) are on by default on 22 as well; a project that only works with `--no-experimental-require-module` on 20.19 needs that flag on 22 too.

***

#### Production / Review Checklist

##### Runtime and Versions

- [ ] Node 20 is end-of-life (April 30, 2026; last release 20.20.2): an upgrade to Node 24 LTS (or 22) is planned; until then the exact `20.20.2` is pinned in Docker/CI
- [ ] `engines.node` states the real minimum minor (`>=20.19.0` for `require(esm)`, `>=20.12.0` for `loadEnvFile`/`crypto.hash`/`styleText`, `>=20.11.0` for `import.meta.dirname`)
- [ ] No Node 21+/22+ APIs: `Object.groupBy`, `Promise.withResolvers`, Set methods, iterator helpers, `Array.fromAsync`, `using`, `fs.glob`, `navigator`, unflagged `WebSocket`, `node --run`, `--experimental-strip-types`, `node:sqlite`
- [ ] For Node 21 projects: no 20.13–20.19 backports (`require(esm)`, `getBuiltinModule`, `URL.parse`, `suite()`, `zlib.crc32`, `path.matchesGlob`)
- [ ] `@types/node` major matches 20; TypeScript `lib` does not include `esnext`/`es2024`

##### Modules

- [ ] Every package sets `"type"`; files use full relative specifiers with extensions in ESM
- [ ] JSON imports use `with { type: 'json' }`, not `assert`
- [ ] `require()` of ESM accounts for the namespace object (`.default`) and top-level `await` (`ERR_REQUIRE_ASYNC_MODULE`)
- [ ] Loader hooks registered via `--import` + `module.register()`, not `--experimental-loader`/`globalPreload`
- [ ] Built-ins imported with the `node:` prefix

##### Correctness

- [ ] No floating promises; every async callback passed to an emitter/timer/`forEach` handles its errors
- [ ] `fetch()` callers check `res.ok`, consume or cancel bodies, and pass `AbortSignal.timeout()`
- [ ] Non-mutating array methods (`toSorted`, `toReversed`, `with`) where shared arrays were mutated; numeric comparators for numeric sorts
- [ ] Errors carry `cause`; checks use `err.code`/`instanceof`, not message text
- [ ] `process.exitCode` set to integers only
- [ ] No reliance on `nextTick` vs. microtask ordering

##### Async, Streams and Resources

- [ ] `stream/promises` `pipeline()` instead of `.pipe()` chains; backpressure honoured on manual writes
- [ ] File handles, sockets and child processes closed in `finally` (no `using` syntax on 20)
- [ ] Cancellation (`AbortSignal`) threaded through I/O; bounded concurrency for fan-out
- [ ] CPU-heavy work in worker threads (pooled), sized with `os.availableParallelism()`
- [ ] Graceful shutdown on `SIGTERM` with a deadline

##### Security

- [ ] No `exec`/`shell: true` with user input; `execFile`/`spawn` with argument arrays and timeouts
- [ ] Path inputs resolved and prefix-checked (`path.resolve` + `startsWith(root + sep)`), symlinks considered
- [ ] Prototype-pollution-safe merges (null-prototype objects or `Map`, forbidden keys filtered)
- [ ] Request body and JSON size limits enforced before parsing
- [ ] `crypto.timingSafeEqual` for secrets, scrypt/argon2 for passwords, AES-GCM with random IVs and `authTagLength`; no `createCipher`, no PKCS#1 v1.5 decryption
- [ ] `node:crypto` imported explicitly (global `crypto` is Web Crypto)
- [ ] Permission Model (if used) understood as experimental and not network-restricting on 20
- [ ] Lockfile committed, `npm ci`, `npm audit`, dependency `engines` compatible with Node 20

##### Testing and Tooling

- [ ] `node --test` invoked with paths (no glob arguments on 20); reporters set explicitly in CI
- [ ] Subtests awaited; mock timers via the 20.11+ options object
- [ ] Linting with `eslint-plugin-n` unsupported-features rules bound to `engines`
- [ ] Containers run as non-root with an init process, `NODE_ENV=production`, explicit `--max-old-space-size`

##### Observability

- [ ] Structured logging with request context from `AsyncLocalStorage`; no secrets in logs
- [ ] Event-loop delay and memory metrics exported; diagnostic reports/heap snapshots available on demand
- [ ] `--enable-source-maps` for transpiled code; specific warnings silenced with `--disable-warning=<code>` rather than `--no-warnings`

***

[^1]: Node.js 20 release announcement and v20.0.0 changelog (V8 11.3, Permission Model, stable test runner, SEA blob, semver-major changes). https://nodejs.org/en/blog/announcements/v20-release-announce , https://github.com/nodejs/node/blob/main/doc/changelogs/CHANGELOG_V20.md#20.0.0
[^2]: Node.js Release Working Group — release schedule (schedule.json: 20.x start 2023-04-18, LTS 2023-10-24, maintenance 2024-10-22, end 2026-04-30; 21.x end 2024-06-01; 22.x end 2027-04-30; 24.x maintenance 2026-10-20; 26.x LTS 2026-10-28; 27.x alpha 2026-10-28). https://github.com/nodejs/Release/blob/main/schedule.json
[^3]: Node.js distribution index (release dates and bundled versions: 20.20.2 on 2026-03-24, 26.12.1, 24.22.1, 22.23.4). https://nodejs.org/dist/index.json
[^4]: Node.js v20.20.2 release notes and the March 2026 security releases. https://nodejs.org/en/blog/release/v20.20.2 , https://nodejs.org/en/blog/vulnerability/march-2026-security-releases
[^5]: Evolving the Node.js Release Schedule (one major per year from 27.x, alpha channel). https://nodejs.org/en/blog/announcements/evolving-the-nodejs-release-schedule
[^6]: Node.js security releases of June, July and October 2026 (fixed only for 22.x/24.x/26.x; "End-of-Life versions are always affected"). https://nodejs.org/en/blog/vulnerability/june-2026-security-releases , https://nodejs.org/en/blog/vulnerability/july-2026-security-releases , https://nodejs.org/en/blog/vulnerability/october-2026-security-releases
[^7]: Node.js 20 changelog (all 20.x minors, notable changes and semver-minor commits). https://github.com/nodejs/node/blob/main/doc/changelogs/CHANGELOG_V20.md
[^8]: Node.js v20.19.0 release notes (require(esm) and module syntax detection enabled by default, module-sync condition). https://nodejs.org/en/blog/release/v20.19.0
[^9]: Node.js 20 documentation — API "added in"/"changes" metadata (all.json for v20.20.2). https://nodejs.org/docs/v20.20.2/api/all.json
[^10]: Node.js 19.0.0 changelog (keep-alive default, server.close idle connections, global Web Crypto, removal of --experimental-specifier-resolution). https://github.com/nodejs/node/blob/main/doc/changelogs/CHANGELOG_V19.md#19.0.0
[^11]: Node.js release posts for 20.6.0, 20.10.0, 20.11.0, 20.12.0, 20.13.0, 20.15.0, 20.16.0, 20.17.0, 20.18.0. https://nodejs.org/en/blog/release/v20.6.0 , https://nodejs.org/en/blog/release/v20.12.0 , https://nodejs.org/en/blog/release/v20.18.0
[^12]: Node.js 21 changelog (V8 11.8, stable fetch/WebStreams, navigator, test runner globs, globalPreload removal). https://github.com/nodejs/node/blob/main/doc/changelogs/CHANGELOG_V21.md
[^13]: Node.js 22 documentation (added-in versions of fs.glob, node:sqlite, module.registerHooks, util.getCallSites, --run, --permission, etc.). https://nodejs.org/docs/latest-v22.x/api/
[^14]: Node.js 20 deprecations list (DEP0005, DEP0040, DEP0106, DEP0147, DEP0164, DEP0169, DEP0170, DEP0173–DEP0185). https://nodejs.org/docs/latest-v20.x/api/deprecations.html
[^15]: TC39 finished proposals (Change Array by Copy, RegExp v flag, Well-Formed Unicode Strings, Resizable ArrayBuffer, Array Grouping, Promise.withResolvers, Set methods, Iterator helpers). https://github.com/tc39/proposals/blob/main/finished-proposals.md
[^16]: V8 — RegExp v flag with set notation and properties of strings. https://v8.dev/features/regexp-v-flag
[^17]: Node.js 20 Modules documentation — Loading ECMAScript modules using require(). https://nodejs.org/docs/latest-v20.x/api/modules.html#loading-ecmascript-modules-using-require
[^18]: npm registry metadata (engines) for eslint 10.13.0, vite 8.3.5/7.3.7, mocha 12.0.3, vitest 4.1.12/5.0.5, jest 30.5.2, undici 7.30.1/8.12.3, @types/node 20.19.43, @tsconfig/node20 20.1.10, as of October 2026. https://registry.npmjs.org/
[^19]: Node.js 20 ES modules documentation — customization hooks, import attributes, import.meta.dirname/filename, import.meta.resolve. https://nodejs.org/docs/latest-v20.x/api/esm.html , https://nodejs.org/docs/latest-v20.x/api/module.html#customization-hooks
[^20]: Node.js 20.4.0 changelog — explicit resource management symbols on Node resources. https://github.com/nodejs/node/blob/main/doc/changelogs/CHANGELOG_V20.md#20.4.0
[^21]: Node.js January 2025 and July 2025 security releases (CVE-2025-23084, CVE-2025-27210 path traversal on Windows). https://nodejs.org/en/blog/vulnerability/january-2025-security-releases , https://nodejs.org/en/blog/vulnerability/july-2025-security-releases
[^22]: Node.js 20 globals documentation (fetch Stability 1 – Experimental, Web Streams, WebSocket, EventSource). https://nodejs.org/docs/latest-v20.x/api/globals.html
[^23]: HashDoS in V8 array index strings (CVE-2026-21717). https://nodejs.org/en/blog/vulnerability/march-2026-hashdos
[^24]: Node.js April 2024 security releases (CVE-2024-27980, child_process on Windows). https://nodejs.org/en/blog/vulnerability/april-2024-security-releases-2
[^25]: Node.js v20.20.0 release notes (January 2026 security release). https://nodejs.org/en/blog/release/v20.20.0
[^26]: Node.js 20 Permissions documentation. https://nodejs.org/docs/latest-v20.x/api/permissions.html
[^27]: Node.js 20 test runner documentation. https://nodejs.org/docs/latest-v20.x/api/test.html
[^28]: Node.js 20 Single executable applications documentation. https://nodejs.org/docs/latest-v20.x/api/single-executable-applications.html
