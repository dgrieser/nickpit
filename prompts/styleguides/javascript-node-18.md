### JavaScript on Node.js 18: The Complete Production Developer Guide

#### Overview

Node.js 18.0.0 ("Hydrogen") was released on **April 19, 2022**, entered Active LTS with 18.12.0 on **October 25, 2022**, moved to Maintenance LTS on **October 18, 2023**, and reached **End-of-Life on April 30, 2025**.[^2] The final release of the line is **18.20.8 (March 27, 2025)**; there will be no 18.20.9.[^3] As of October 2026 Node 18 has been unsupported for eighteen months: the project does not assess security reports for EOL lines and treats every new CVE as affecting them by default, and several security releases since (for example the one published January 13, 2026, which fixed three high-severity issues) shipped only for supported lines.[^6][^7] The bundled **OpenSSL 3.0** branch has itself left upstream support.[^24]

Supported lines in October 2026: **Node 22** (Maintenance LTS until April 30, 2027; latest 22.23.4), **Node 24** (Maintenance LTS since October 20, 2026, until April 30, 2028; latest 24.22.1) and **Node 26** (Current; latest 26.12.1 of October 21, 2026; scheduled to become Active LTS on October 28, 2026, supported until April 30, 2029). Node 20 reached EOL on April 30, 2026. Node 27 has no release yet — only an alpha channel scheduled to start on October 28, 2026.[^2][^3] **Recommend upgrading to Node 24 or 26** whenever you review a Node 18 project — but review the code *as written for Node 18*: do not suggest APIs the pinned runtime does not have.

Exact component versions in **18.20.8** (verified with `process.versions`): V8 **10.2.154.26-node.39**, OpenSSL **3.0.16**, undici **5.29.0** (powers `fetch`), npm **10.8.2**, Corepack **0.32.0**, ICU **74.2** (Unicode 15.1, CLDR 44.1, tz 2024a), libuv **1.44.2**, `NODE_MODULE_VERSION` **108**, Node-API **9**. Prebuilt Linux binaries need glibc ≥ 2.28; macOS binaries need 10.15+.[^1][^4]

This guide also applies to **Node 19** projects (19.0.0 released October 18, 2022; EOL June 1, 2023; last release 19.9.0 with OpenSSL 3.0.8). Review Node 19 code against the 18 feature set documented here: 19 adds a few things 18 lacks (global `crypto`, global `CustomEvent`, `fs.openAsBlob`, `Intl.NumberFormat` v3 methods, recursive `fs.watch` on Linux, keep-alive default agent, `server.close()` closing idle sockets) and *lacks* several later 18.x backports (`AbortSignal.any`, `module.register`, `events.addAbortListener`, `Symbol.dispose`, mock timers, import attributes with `with`). Where this matters it is called out explicitly.[^23]

This guide is self-contained: it describes the JavaScript language as implemented by V8 10.2 (ES2022 plus `findLast`/`findLastIndex` from ES2023) and the Node.js 18 runtime APIs. Behavioral claims and examples were executed on Node **18.20.8** unless stated otherwise (Windows-only behavior and single executable applications are documented from release notes); "new in" and "not available" boundaries were checked against 16.20.2, 17.9.1, 19.9.0 and every newer major up to 26.12.1. Minor-version boundaries inside 18.x come from the release notes and API docs.

Global truths about JavaScript on Node 18:

- **Single-threaded event loop, asynchronous I/O.** JavaScript runs on one thread per isolate; blocking that thread (sync I/O, CPU loops, catastrophic regexes) blocks every request. Use `worker_threads` for CPU work.
- **Unhandled promise rejections crash the process** (default `--unhandled-rejections=throw` since Node 15).
- **Two module systems.** CommonJS (`require`) and ES modules (`import`) coexist; `require()` cannot load an ES module on Node 18 (`ERR_REQUIRE_ESM`).
- **Web platform APIs are partly global.** `fetch`, `Headers`, `Request`, `Response`, `FormData`, `Blob`, `BroadcastChannel`, `structuredClone`, `AbortController`, `EventTarget`, Web Streams and `TextEncoder`/`TextDecoder` are globals; `crypto` (Web Crypto), `File`, `CustomEvent`, `navigator` and `WebSocket` are **not** (see the "Do not use" list).
- **`fetch` is experimental in 18** (stability 1, built on undici 5.x). It works and no longer prints a warning since 18.13.0, but its behavior differs from browsers and from later Node lines in documented ways.[^8]
- **OpenSSL 3** is the crypto backend (since Node 17). Legacy algorithms (MD4, RC4, Blowfish, single DES, Whirlpool) throw `ERR_OSSL_EVP_UNSUPPORTED`.

***

#### Version-Aware Review

Node 18 spans three years of semver-minor releases. Many APIs reviewers think of as "Node 18" appeared in late minors (18.13–18.20). Pulling `node:18` or running `nvm install 18` today yields 18.20.8, but images, CI caches and lockfiles built earlier can still contain any older 18.x — check the version that actually runs.

Rules for reviewing Node 18 code:

1. **Find the pinned version first**: `engines.node` in `package.json`, `.nvmrc`/`.node-version`, `volta.node`, the Dockerfile `FROM node:18.x…` tag, CI matrices (`actions/setup-node` `node-version`). If only the major is pinned, assume the code may run on **any** 18.x unless `engines` says otherwise.
2. **Check the minor that introduced an API** before approving it (table below). If the code uses, for example, `AbortSignal.any()` (18.17.0) but `engines` says `>=18`, flag the mismatch and suggest `"engines": { "node": ">=18.17" }` or a fallback.
3. **Verify before claiming something is unavailable.** Say "not available in Node 18" only when the feature was introduced in a later major (see the "Do not use or recommend" list) or after the pinned minor. Many Node 20+ features were backported to 18.x (`--import`, `module.register`, `AbortSignal.any`, `node:test` mocks, `URL.canParse`, …); do not flag those as unavailable on 18.20.
4. **Verify against the runtime, not TypeScript `lib`.** `lib: ["es2023"]` (used by `@tsconfig/node18`) types `Array.prototype.toSorted()`, which **throws `TypeError` on Node 18** (see Tooling).[^25]
5. **Never suggest features from Node 20+** to fix a Node 18 problem; give the Node 18 idiom and, separately, recommend the upgrade.

Quick check commands a reviewer can quote:

```bash
node -p "process.version"                         # v18.20.8
node -p "typeof Array.prototype.findLast"         # function (18.0+)
node -p "typeof AbortSignal.any"                  # function only on 18.17+
node -p "typeof globalThis.crypto"                # 'object' in -p/-e (module object!), see Web Crypto section
```

##### Node 19 Projects

Node 19 is an odd-numbered, never-LTS line (October 18, 2022 – June 1, 2023; final 19.9.0 with V8 10.8, OpenSSL 3.0.8 and undici 5.21.0). It received no security fixes after June 2023 — every 2023–2026 Node and OpenSSL CVE applies. Review Node 19 code against this guide, with these verified differences from 18.20.8:

| Topic | 18.20.8 | 19.9.0 |
|---|---|---|
| `globalThis.crypto` (Web Crypto) | flag only | global |
| `CustomEvent`, `PerformanceObserver` globals | no (flag / import) | yes |
| `fs.openAsBlob()` | no | yes (19.8) |
| Recursive `fs.watch()` on Linux | throws | yes (19.1) |
| `Intl.NumberFormat` v3 (`formatRange`) | no | yes |
| `http.globalAgent.keepAlive` | false | true |
| `server.close()` closes idle sockets | no | yes |
| `fs.writeFileSync(path, objWithToString)` | DEP0162 warning | `ERR_INVALID_ARG_TYPE` |
| `--experimental-specifier-resolution` | works | removed (ignored) |
| Import attributes `with` | yes (18.20) | **SyntaxError** (only `assert`) |
| `AbortSignal.any`, `module.register`, `events.addAbortListener`, `Symbol.dispose`, `MockTimers`, `dns.getDefaultResultOrder()`, `test.skip/todo/only` shorthands | yes (18.17–18.19) | **no** |
| `require('node:wasi')` without a flag | yes (18.17) | needs `--experimental-wasi-unstable-preview1` |
| `--import` | yes (18.18) | yes (19.0) |
| Non-registered symbols as `WeakMap` keys | accepted (18.20) | `TypeError` |

The upgrade target for a Node 19 project is the same: Node 24 or 26.

***

#### What's New in Node 18 (vs. 16.20.2 and 17.9.1)

Node 17 was a short-lived odd line; most projects jump from 16 to 18, so the 17.0 changes (OpenSSL 3, DNS ordering, `structuredClone`) are new to them. Several 18.x features were later backported to 16.x (16.17–16.20), marked "also 16.x" below — on a 16 → 18 upgrade those are not new, but code that relies on them still needs ≥ that 16 minor.[^1][^5]

| Feature | Since | Also in 16.20.2? | Notes |
|---|---|---|---|
| OpenSSL 3.0 | 17.0.0 | no (1.1.1) | legacy algorithms throw `ERR_OSSL_EVP_UNSUPPORTED` |
| `dns.lookup()` returns addresses in resolver order (`verbatim`) | 17.0.0 | no | `localhost` may resolve to `::1` first |
| `structuredClone()` global | 17.0.0 | no | |
| `DOMException` global | 17.0.0 | no | |
| `readline/promises` (experimental) | 17.0.0 | no | |
| Fatal exception output prints the Node version | 17.0.0 | no | |
| V8 10.1 (18.0) → 10.2 (18.3): `Array.prototype.findLast`/`findLastIndex`, `Intl.supportedValuesOf`, `Intl.Locale` info getters | 18.0.0 | no | ES2023 array *find-from-last* only |
| Global `fetch`, `Headers`, `Request`, `Response`, `FormData` (experimental) | 18.0.0 | flag only | warning removed in 18.13.0 |
| Global Web Streams (`ReadableStream`, `TransformStream`, `CompressionStream`, `TextDecoderStream`, …) | 18.0.0 | no (module only) | |
| Global `Blob`, `BroadcastChannel` (stable) | 18.0.0 | no | |
| `node:test` test runner (experimental) | 18.0.0 | also 16.17 | `node:` prefix required |
| HTTP server `requestTimeout` 300 s, `headersTimeout` 60 s, `noDelay` true | 18.0.0 | no (`requestTimeout` 0) | |
| `node --test` CLI runner | 18.1.0 | also 16.17 | |
| `server.closeIdleConnections()` / `closeAllConnections()` | 18.2.0 | no | |
| `util.parseArgs()` | 18.3.0 | also 16.17 | |
| Loader chaining (`--experimental-loader` many times) | 18.6.0 | also 16.17 | |
| `CustomEvent` (global only with `--experimental-global-customevent`) | 18.7.0 | also 16.17 | unflagged global in 19 |
| `--build-snapshot` / `--snapshot-blob` (run-time user-land snapshots) | 18.8.0 | no (16.20.2 accepts `--build-snapshot` but exits with *"not yet supported in the node binary"*) | experimental |
| `os.machine()` | 18.9.0 | also 16.18 | |
| `--watch` / `--watch-path` (experimental) | 18.11.0 | also 16.19 | `--watch-path` unsupported on Linux in 18 |
| `FileHandle.readLines()` | 18.11.0 | no | |
| `--test-name-pattern` | 18.11.0 | no | |
| `util.MIMEType`, `buffer.File` (experimental), `node:test` `mock` | 18.13.0 | no | |
| `os.availableParallelism()`, `buffer.isUtf8()` | 18.14.0 | no | |
| `fs.statfs()`, test reporters, `--experimental-test-coverage`, `process.constrainedMemory()`, `buffer.isAscii()` | 18.15.0 | no | |
| Ada URL parser, `Buffer.copyBytesFrom()`, `AsyncLocalStorage.bind()/snapshot()`, `util.aborted()`, `URLSearchParams.size`, SEA (via `postject`) | 18.16.0 | no | |
| `AbortSignal.any()`, `URL.canParse()`, `readdir({ recursive })`, `stream.getDefaultHighWaterMark()`, `events.getMaxListeners()`, `test.skip/todo/only` shorthands, WASI unflagged | 18.17.0 | no | |
| `--import`, `events.addAbortListener()`, `Symbol.dispose`/`Symbol.asyncDispose` (symbols only), `URLSearchParams.has/delete(name, value)` | 18.18.0 | no | |
| `module.register()` + `initialize` hook, hooks off-thread, `import.meta.resolve()` unflagged, `--experimental-default-type`, `MockTimers`, `--test-concurrency`, `--test-shard`, junit reporter, `diagnostics_channel.tracingChannel()`, npm 10 | 18.19.0 | no | |
| Import attributes (`with { type: 'json' }`), `Dirent.parentPath` | 18.20.0 | no | `assert` still works with a V8 deprecation warning |
| JSON modules / import attributes marked stable | 18.20.5 | no | |

Breaking or behavior-changing items in **18.0.0** itself: `fs.write()`/`fs.writeFileSync()` coercion of objects with own `toString` became a runtime deprecation (DEP0162; an error in 19), `process.on('multipleResolves')` became a runtime deprecation (DEP0160), `dns.lookup()` option type coercion and `tls.parseCertString()` reached end-of-life, stream thenable support was removed (DEP0157), and `ERR_INVALID_CALLBACK` is gone (DEP0159; use `ERR_INVALID_ARG_TYPE`).[^1][^9]

##### Do Not Use or Recommend (Newer Than Node 18)

The project's runtime does not have these. Never suggest them as a fix or modernization for Node 18 code. (Major = first release line where the feature is available unflagged; verified by running every major from 19 to 26.)[^21][^22]

**JavaScript language / built-ins (V8 newer than 10.2):**

- `Array.prototype.toSorted()`, `toReversed()`, `toSpliced()`, `with()` and the `TypedArray` equivalents — **Node 20** (ES2023 change-array-by-copy). Use `[...arr].sort(cmp)`, `arr.slice().reverse()`.
- RegExp `v` flag (`/[\p{L}--[a-z]]/v`), `String.prototype.isWellFormed()`/`toWellFormed()`, resizable `ArrayBuffer` (`new ArrayBuffer(n, { maxByteLength })`, `.resize()`) — **Node 20**.
- `Object.groupBy()`, `Map.groupBy()`, `ArrayBuffer.prototype.transfer()`, `JSON.rawJSON()` — **Node 21**. (`Array.prototype.group` never shipped.)
- `Promise.withResolvers()`, `Array.fromAsync()`, Set methods (`union`, `intersection`, `difference`, `isSubsetOf`, …), iterator helpers (`Iterator.from`, `.map/.filter/.take/.drop/.toArray` on iterators) — **Node 22**.
- `Promise.try()`, RegExp modifiers `(?i:…)`, duplicate named capture groups, `Intl.DurationFormat` — **Node 23 / 24 LTS**.
- `using` / `await using` declarations, `DisposableStack`, `AsyncDisposableStack`, `SuppressedError`, `Float16Array`/`Math.f16round()`, `RegExp.escape()`, `Error.isError()`, `Atomics.pause()` — **Node 24**. (Node 18.18+ defines `Symbol.dispose`/`Symbol.asyncDispose`, but the `using` syntax is a `SyntaxError`.)
- `Uint8Array.fromBase64()`/`toBase64()`/`fromHex()`/`toHex()` — **Node 25**; `Temporal`, `Map.prototype.getOrInsert()`, `Iterator.concat()` — **Node 26**. `Math.sumPrecise()` is not in any Node line as of 26.12.1.
- `Intl.NumberFormat` v3 (`formatRange()`, `roundingMode`, …) — **Node 19+**. `Intl.Locale.prototype.getWeekInfo()` and the other `get*()` methods — **Node 24** (Node 18 only has the older getter properties `weekInfo`, `hourCycles`, `timeZones`, …).
- Do not enable in-progress V8 features in production with `--harmony-*` flags (`--harmony-array-grouping`, `--harmony-rab-gsab`, `--harmony-temporal`, `--harmony-shadow-realm`, `--harmony-intl-number-format-v3` exist in 18.20.8 but implement early drafts).

**Globals:**

- `crypto` as a global (Web Crypto) — unflagged in **Node 19**; on 18 it needs `--experimental-global-webcrypto`. Use `require('node:crypto').webcrypto` or the `node:crypto` functions.
- `File` global — **Node 20** (on 18.13+ use `require('node:buffer').File`, experimental). `CustomEvent` global — **Node 19** (flag on 18.7+). `PerformanceObserver`, `PerformanceEntry`, `PerformanceMark`, `PerformanceResourceTiming` globals — **Node 19** (import from `node:perf_hooks` on 18; `performance` itself is global).
- `navigator` — **Node 21**. `WebSocket` — **Node 22** (flag in 20.10/21). `EventSource` — 22.3 behind a flag.
- `process.getBuiltinModule()` — 22.3 / 20.16.

**Node APIs:**

- `require()` of ES modules (`require(esm)`) — 22.0 flagged, unflagged in 22.12 / 20.19; `process.features.require_module`. On 18 it throws `ERR_REQUIRE_ESM`.
- `import.meta.dirname` / `import.meta.filename` — 20.11 / 21.2.
- `--env-file` — 20.6; `process.loadEnvFile()` and `util.parseEnv()` — 21.7 / 20.12. Use `dotenv` on 18.
- `node --run` — 22.0. `fs.glob()`/`fsPromises.glob()` — 22.0. `path.matchesGlob()` — 22.5 / 20.17.
- `fs.openAsBlob()` — **19.8**. `fs.watch(dir, { recursive: true })` on **Linux** — 19.1.
- `util.styleText()` — 21.7 / 20.12. `util.getCallSites()` — 22.9. `util.diff()` — 23.11 / 22.15.
- `crypto.hash()` — 21.7 / 20.12. `URL.parse()` — 22.1 (also backported to late 20.x; present in 20.20.2). `module.findPackageJSON()` — 23.2 / 22.14. `module.registerHooks()` (synchronous hooks) — 23.5 / 22.15. `module.enableCompileCache()` — 22.8.
- `stream.duplexPair()` — 22.6 / 20.17. `process.availableMemory()` — 22.0 / 20.13. zlib Zstandard (`createZstdCompress`) — 23.8 / 22.15. `node:sqlite` — 22.5. `node:sea` module and `--experimental-sea-config` — 20.0.
- Permission model (`--permission` / `--experimental-permission`, `process.permission`) — 20.0. TypeScript type stripping (`--experimental-strip-types`) — 22.6.
- `--experimental-detect-module` — 21.1 / 20.10; `--experimental-require-module` — 22.0 / 20.17.
- `net.setDefaultAutoSelectFamily()` exists in 18.20.8 but the **default** of `autoSelectFamily` is `false` on 18 and `true` from 20.

**Test runner (`node:test`):** module mocking (`mock.module()`, 22.3 / 20.18), snapshot testing (`t.assert.snapshot`, 22.3), `assert.partialDeepStrictEqual()` (23.4 / 22.13), `--test-timeout` (21.2 / 20.11), `--test-force-exit` (22.0 / 20.14), `--test-skip-pattern` (22.1), glob patterns as `node --test` arguments (21), `MockTimers` with `Date` and the object form `enable({ apis, now })` (21.2 / 20.11) — on 18.19+ `enable()` takes only an array of timer names. Since 18.13.0, `node --test --watch` reruns tests on changes.

**Tooling majors that no longer support Node 18** (from their published `engines` fields):[^26] ESLint 10 (needs ^20.19 / ^22.13 / ≥ 24; ESLint 9 supports ^18.18), Mocha 12 (^20.19 / ≥ 22.12; Mocha 11 supports ^18.18), Vitest 4 (^20 / ^22 / ≥ 24) and Vitest 5 (^22.12 / ^24 / ≥ 26; Vitest 3 supports ^18), undici 7+ (≥ 20.18.1; undici 6 supports ≥ 18.17), npm 11 (^20.17 / ≥ 22.9; Node 18 ships npm 10.8.2). Recommending one of these on Node 18 is a breaking change, not a cleanup.

##### Deprecated or Removed by Node 18

- `new Buffer()` — runtime-deprecated (DEP0005). Use `Buffer.from()`, `Buffer.alloc()`, `Buffer.allocUnsafe()`.
- `buffer.slice()` — documentation-deprecated in 17.5 (DEP0158); use `buf.subarray()` (same shared-memory semantics, explicit name).
- `url.parse()` — documentation-deprecated in 18.13 (DEP0169, `--pending-deprecation` support in 18.17) because it parses non-standard input inconsistently; use `new URL()`.
- `fs.rmdir(path, { recursive: true })` — runtime-deprecated (DEP0147); use `fs.rm(path, { recursive: true, force: true })`.
- `process.on('multipleResolves')` — runtime-deprecated in 18.0 (DEP0160).
- `fs.write()`/`fs.writeFileSync()` object-to-string coercion — runtime-deprecated in 18.0 (DEP0162), `ERR_INVALID_ARG_TYPE` from 19.
- `process.exit(code)` / `process.exitCode` integer coercion of strings etc. — documentation-deprecated (DEP0164, 18.7/18.10).
- `dirent.path` — documentation-deprecated in 18.20 (DEP0178); use `dirent.parentPath`.
- `http` `.aborted` property / `'abort'` / `'aborted'` events — documentation-deprecated (DEP0156); use `'close'` plus `req.destroyed`/`res.writableFinished`, or an `AbortSignal`.
- `request.abort()` (DEP0140) — use `request.destroy()`.
- Weak Diffie-Hellman groups `modp1`, `modp2`, `modp5` — documentation-deprecated in 18.10 (DEP0167).
- `diagnostics_channel` `channel.subscribe(fn)` / `unsubscribe(fn)` — doc-deprecated (DEP0163); use `diagnostics_channel.subscribe(name, fn)`.
- `--trace-atomics-wait` — doc-deprecated (DEP0165).
- `tls.parseCertString()` and stream thenables — end-of-life in 18.0.
- `"exports"` folder mappings with trailing `/` — end-of-life in 17 (DEP0148); use `"./*"` patterns.
- `import … assert { type: 'json' }` — superseded by `with` in 18.20.0; V8 prints a deprecation warning, and `assert` was removed in Node 22.[^9][^12]

***

#### Language and Syntax (V8 10.2 / ES2022)

Everything up to ES2022 is available: classes with public, private and static fields, private methods and accessors, class static blocks, `#field in obj`, top-level `await` in ES modules, `Array.prototype.at()`, `Object.hasOwn()`, `Error` `cause`, RegExp match indices (`/d`), logical assignment (`??=`, `||=`, `&&=`), numeric separators, `String.prototype.replaceAll()`, `Promise.any()`/`AggregateError`, `WeakRef`/`FinalizationRegistry`, optional chaining and nullish coalescing. From ES2023 only `findLast()`/`findLastIndex()` are present.[^10]

##### Variables, Equality and Nullish Values

```js
'use strict';
const config = { retries: 0, name: '' };

// BAD: || treats 0 and '' as missing
const retriesBad = config.retries || 3;   // 3
// GOOD: ?? only replaces null/undefined
const retries = config.retries ?? 3;      // 0
const name = config.name ?? 'anonymous';  // ''

// Strict equality everywhere; `== null` is the one accepted idiom for "null or undefined"
function isMissing(v) { return v == null; }

let cache = null;
cache ??= new Map();                      // logical assignment (ES2021)
const total = 1_000_000;                  // numeric separator

console.log(retriesBad, retries, JSON.stringify(name), isMissing(undefined), cache.size, total);
```

Never use `var`; prefer `const`, then `let`. Do not rely on automatic semicolon insertion around lines that start with `(`, `[` or a template literal.

##### New in 18: `findLast()`; Copy-Then-Mutate Instead of `toSorted()`

```js
const items = [{ id: 1, ok: true }, { id: 2, ok: false }, { id: 3, ok: true }];

items.at(-1);                                   // { id: 3, ... }  (16.6+)
items.findLast((x) => x.ok);                    // { id: 3, ... }  (new in 18)
items.findLastIndex((x) => x.id > 10);          // -1             (new in 18)

// BAD on Node 18: TypeError: items.toSorted is not a function (Node 20+)
// const sorted = items.toSorted((a, b) => b.id - a.id);

// GOOD on Node 18: copy first, then mutate the copy
const sorted = [...items].sort((a, b) => b.id - a.id);
const reversed = items.slice().reverse();
const replaced = Object.assign([...items], { 1: { id: 2, ok: true } }); // instead of items.with(1, …)

console.log(sorted.map((x) => x.id), reversed[0].id, replaced[1].ok, items[1].ok);
```

`Array.prototype.sort()` is stable (guaranteed since ES2019) and sorts in place; a reviewer should flag `sort()` on an array the caller still owns (props, function arguments, module-level constants).

**Grouping without `Object.groupBy` (Node 21+):**

```js
function groupBy(iterable, keyFn) {
  const out = new Map();
  for (const item of iterable) {
    const key = keyFn(item);
    const bucket = out.get(key);
    if (bucket) bucket.push(item);
    else out.set(key, [item]);
  }
  return out;
}

const byParity = groupBy([1, 2, 3, 4], (n) => (n % 2 ? 'odd' : 'even'));
console.log(Object.fromEntries(byParity)); // { odd: [ 1, 3 ], even: [ 2, 4 ] }
```

Use a `Map` (or an object created with `Object.create(null)`) when keys come from input, so a key like `__proto__` cannot corrupt the result.

##### Objects: `Object.hasOwn()`, Prototype Safety, Immutability

```js
const fromInput = JSON.parse('{"__proto__": {"isAdmin": true}, "name": "x"}');
const target = {};
Object.assign(target, fromInput);           // [[Set]] of "__proto__" invokes the setter
console.log(target.isAdmin);                // true — the prototype of `target` was replaced

// GOOD: null-prototype dictionaries and explicit own-property checks
const dict = Object.create(null);
for (const [k, v] of Object.entries(fromInput)) {
  if (k === '__proto__' || k === 'constructor' || k === 'prototype') continue;
  dict[k] = v;
}
console.log(Object.hasOwn(dict, 'name'), 'hasOwnProperty' in dict); // true false

// Object.hasOwn works on null-prototype objects; obj.hasOwnProperty does not exist there
console.log(typeof dict.hasOwnProperty);    // 'undefined'
```

`JSON.parse()` creates an *own* property named `__proto__`; copying it with `Object.assign()` or spread-into-existing-object patterns, or a recursive "deep merge", invokes the `__proto__` setter and changes the prototype. Flag deep-merge helpers that do not skip `__proto__`, `constructor` and `prototype`. `Object.freeze()` is shallow.

##### Functions, `this` and Defaults

```js
class Ticker {
  constructor() { this.count = 0; }
  // BAD: a regular method loses `this` when passed as a callback
  tickBad() { this.count += 1; }
  // GOOD: an arrow-function class field binds `this` per instance
  tick = () => { this.count += 1; };
}

const t = new Ticker();
try { [1].forEach(t.tickBad); } catch (e) { console.log(e.constructor.name); } // TypeError: this is undefined
[1, 2].forEach(t.tick);
console.log(t.count);                                                         // 2

// Options objects: destructure with defaults, validate required fields
function createUser({ name, email, role = 'user' } = {}) {
  if (!name || !email) throw new TypeError('name and email are required');
  return { name, email, role, createdAt: new Date() };
}
console.log(createUser({ name: 'a', email: 'a@x' }).role);

// BAD: a shared module-level default object is mutated by every call
const defaults = { tags: [] };
function withTagsBad(opts = defaults) { opts.tags.push('x'); return opts; }
withTagsBad(); withTagsBad();
console.log(defaults.tags.length);                                             // 2
// GOOD: build fresh values per call
function withTags({ tags = [] } = {}) { return { tags: [...tags, 'x'] }; }
console.log(withTags().tags.length, withTags().tags.length);                   // 1 1
```

- Arrow-function class fields create one function per instance; fine for handlers, wasteful for thousands of objects — use `.bind` at the call site or prototype methods there.
- Do not mutate parameters; return new objects (`{ ...obj, changed: true }`).
- Prefer named functions for stack traces in long-lived callbacks.

##### Dates and Times (No `Temporal` on Node 18)

```js
const d = new Date('2024-03-10');            // date-only ISO string = UTC midnight
const local = new Date('2024-03-10T00:00');  // date-time without offset = LOCAL time
console.log(d.toISOString(), local instanceof Date);

// V8 does not validate day-of-month in ISO strings: it rolls over (days 30/31)
console.log(new Date('2024-02-30').toISOString().slice(0, 10));   // 2024-03-01, not Invalid Date
console.log(Number.isNaN(new Date('2024-02-32').getTime()));      // true (only > 31 is rejected)

// Month index is 0-based; month arithmetic overflows
const start = new Date(Date.UTC(2024, 0, 31));
const plusOneMonth = new Date(start);
plusOneMonth.setUTCMonth(start.getUTCMonth() + 1);
console.log(plusOneMonth.toISOString().slice(0, 10));             // 2024-03-02

// Format for users in an explicit zone; store and transmit UTC ISO strings or epoch ms
const fmt = new Intl.DateTimeFormat('de-DE', { timeZone: 'Europe/Berlin', dateStyle: 'medium', timeStyle: 'short' });
console.log(fmt.format(new Date(Date.UTC(2024, 6, 1, 12, 0))));   // 01.07.2024, 14:00
console.log(typeof globalThis.Temporal);                           // 'undefined'
```

Validate user-supplied dates by round-tripping the components (`d.getUTCFullYear()/getUTCMonth()/getUTCDate()` equal the parsed parts) or with a date library; `new Date(input)` accepts many implementation-defined formats and silently normalizes overflowing days.

##### Iteration and Generators (No Iterator Helpers)

```js
function* take(iterable, n) {
  if (n <= 0) return;
  for (const x of iterable) { yield x; if (--n <= 0) return; }
}
function* map(iterable, fn) { for (const x of iterable) yield fn(x); }
function* naturals() { for (let i = 1; ; i++) yield i; }

// Iterator.prototype.map/take (Node 22) do not exist; compose generators instead
console.log([...take(map(naturals(), (x) => x * x), 4)]);   // [ 1, 4, 9, 16 ]
```

`for…of` over a generator calls `return()` on `break`, running `finally` blocks — put cleanup there. Do not spread infinite iterators.

##### Classes, Private State and Static Blocks

```js
class Counter {
  static #instances = 0;
  static registry;
  #count = 0;

  static {
    // class static initialization block (ES2022)
    Counter.registry = new Set();
  }

  constructor() {
    Counter.#instances += 1;
    Counter.registry.add(this);
  }

  increment() { return ++this.#count; }

  static isCounter(value) {
    return #count in value;   // ergonomic brand check (ES2022)
  }

  static get instances() { return Counter.#instances; }
}

const c = new Counter();
c.increment();
console.log(c.increment(), Counter.isCounter(c), Counter.isCounter({}), Counter.instances);
```

Private fields (`#x`) are truly private (not visible to `Object.keys` or `JSON.stringify`, and not reachable through a `Proxy` wrapping the instance). Accessing `#x` on an object without it throws `TypeError`; use `#x in obj` for brand checks rather than `try/catch`.

##### Errors: `cause`, `AggregateError`, Custom Classes

```js
class AppError extends Error {
  constructor(message, { code, statusCode = 500, cause } = {}) {
    super(message, { cause });          // ES2022 error cause; shown by util.inspect as [cause]
    this.name = new.target.name;
    this.code = code;
    this.statusCode = statusCode;
  }
}

class NotFoundError extends AppError {
  constructor(resource, id, options) {
    super(`${resource} ${id} not found`, { code: 'NOT_FOUND', statusCode: 404, ...options });
  }
}

function loadUser(id) {
  try {
    throw new Error('ENOENT: no such row');
  } catch (err) {
    // GOOD: wrap with context, keep the original as cause
    throw new NotFoundError('user', id, { cause: err });
  }
}

try {
  loadUser(42);
} catch (err) {
  console.log(err instanceof NotFoundError, err.name, err.statusCode, err.cause.message);
}

Promise.any([Promise.reject(new Error('a')), Promise.reject(new Error('b'))])
  .catch((e) => console.log(e instanceof AggregateError, e.errors.map((x) => x.message)));
```

- `Error.captureStackTrace` is not needed in subclasses: V8 records the stack in the `Error` constructor.
- Do not throw strings or plain objects; they have no stack.
- `err.cause` is not part of `err.message`; loggers must serialize it (pino and `util.inspect` do).
- Check error *codes* (`err.code === 'ENOENT'`), never parse `err.message`.

##### New Since 16: `structuredClone()` (17.0)

```js
class Point { constructor(x) { this.x = x; } norm() { return Math.abs(this.x); } }

const original = { when: new Date(0), tags: new Set(['a']), map: new Map([[1, { deep: true }]]), point: new Point(-3) };
original.self = original;                                  // cycles are fine

const copy = structuredClone(original);
console.log(copy.when instanceof Date, copy.map.get(1), copy.self === copy);
console.log(copy.point instanceof Point, copy.point);      // false { x: -3 } — prototypes are dropped

try { structuredClone({ fn() {} }); } catch (e) { console.log(e.name); } // DataCloneError

const bytes = new Uint8Array([1, 2, 3]);
const moved = structuredClone(bytes, { transfer: [bytes.buffer] });
console.log(bytes.byteLength, moved.length);               // 0 3 — source buffer detached
```

Use `structuredClone` instead of `JSON.parse(JSON.stringify(x))` when values contain `Date`, `Map`, `Set`, typed arrays, `BigInt` or cycles. It drops class prototypes, getters and functions — do not use it to copy class instances.

##### Strings, Unicode and `Intl`

- No `isWellFormed()`/`toWellFormed()` (Node 20). To reject lone surrogates, use `/\p{Surrogate}/u.test(s)` or `Buffer.from(s, 'utf8')` (which replaces them with U+FFFD).
- Node 18 ships **full ICU** (74.2 in 18.20.8) — all locales work without `full-icu` packages.
- `Intl.supportedValuesOf('timeZone' | 'currency' | 'calendar' | …)` is new in 18.
- No `Temporal`. For time zones use `Intl.DateTimeFormat(locale, { timeZone })`; for arithmetic use UTC epoch milliseconds or a library.

```js
const s = 'a\uD800b';
console.log(/\p{Surrogate}/u.test(s));                       // true — malformed UTF-16
console.log(Buffer.from(s).toString());                       // 'a\uFFFDb'
console.log(Intl.supportedValuesOf('timeZone').includes('Europe/Berlin'));
console.log(new Intl.DateTimeFormat('en-GB', { timeZone: 'Asia/Tokyo', dateStyle: 'short', timeStyle: 'short' })
  .format(new Date(Date.UTC(2024, 0, 1, 0, 0))));            // 01/01/2024, 09:00
console.log([...new Intl.Segmenter('en', { granularity: 'grapheme' }).segment('👍🏽!')].length); // 2
```

##### Numbers and BigInt

- Use `Number.isInteger`, `Number.isSafeInteger`, `Number.parseInt(x, 10)` (always pass the radix).
- IDs from databases above 2^53 − 1 (for example Snowflake IDs, PostgreSQL `bigint`) must be strings or `BigInt`; `JSON.parse` silently rounds them. `JSON.stringify` throws on `BigInt` unless you provide a replacer.
- Money: integer minor units or a decimal library — never binary floating point.

```js
console.log(JSON.parse('{"id": 9007199254740993}').id);     // 9007199254740992 (precision lost)
const big = 9007199254740993n;
console.log(JSON.stringify({ id: big }, (k, v) => (typeof v === 'bigint' ? v.toString() : v)));
```

##### Buffers and Binary Data

```js
const { isUtf8, isAscii } = require('node:buffer');
const { StringDecoder } = require('node:string_decoder');

const buf = Buffer.from('héllo', 'utf8');
const view = buf.subarray(0, 2);        // shares memory with buf (buf.slice() does too; DEP0158)
view[0] = 0x48;                         // 'H'
console.log(buf.toString(), buf.length); // Héllo 6 — length is bytes, not characters

const copy = Buffer.from(buf);          // independent copy
const fromView = Buffer.copyBytesFrom(new Uint16Array([1, 2, 3]), 1); // 18.16+: copies 2 elements = 4 bytes
console.log(copy !== buf, fromView.length);

const raw = Buffer.from([0xfb, 0xff]);
console.log(raw.toString('base64'), raw.toString('base64url'));        // +/8= -_8 (URL-safe, unpadded)
console.log(isUtf8(Buffer.from([0xc3, 0xa9])), isUtf8(Buffer.from([0xff])), isAscii(Buffer.from('abc'))); // 18.14 / 18.15

// BAD: decoding chunk by chunk splits multi-byte characters
const euro = Buffer.from('€');          // 3 bytes
const chunks = [euro.subarray(0, 1), euro.subarray(1)];
console.log(chunks.map((c) => c.toString()).join('') === '€');           // false
// GOOD: StringDecoder (or readable.setEncoding('utf8')) buffers partial characters
const dec = new StringDecoder('utf8');
console.log(chunks.map((c) => dec.write(c)).join('') + dec.end() === '€'); // true

// Buffer.allocUnsafe returns uninitialized memory: use only when every byte is overwritten
const out = Buffer.allocUnsafe(4);
out.writeUInt32BE(0xdeadbeef, 0);
console.log(out.toString('hex'));
```

- `Uint8Array.prototype.slice()` copies but `Buffer.prototype.slice()` does not — a classic aliasing bug when code treats a `Buffer` as a plain `Uint8Array`. Prefer the explicit `subarray()` and copy with `Buffer.from(view)`.
- Never send `Buffer.allocUnsafe()` memory to a client without filling it; it can contain secrets from earlier allocations.
- `buf.toString('base64url')`/`Buffer.from(s, 'base64url')` (since 15.7) replace hand-written `+`/`/` replacement code.
- No `Uint8Array.fromBase64()`/`toHex()` on 18 (Node 25).

##### Symbols as WeakMap Keys — Do Not Rely On It

Non-registered symbols are accepted as `WeakMap` keys and `WeakRef` targets on **18.20.8** but rejected by 19.9.0 (`TypeError: Invalid value used as weak map key`) — observed, not documented. The 18.20.0 release backported V8 commits (nodejs/node#51004) so that Node's module loader could key a `WeakMap` by symbols (nodejs/node#48510); older 18.x minors lack those commits. The ES2023 feature (symbols as WeakMap keys) is officially available from Node 20. Do not depend on it in code that must run on any 18.x.[^1]

***

#### Modules: CommonJS, ES Modules and Interop

##### Choosing the Module System

- `"type": "module"` in `package.json` makes `.js` files ESM; `.mjs` is always ESM, `.cjs` always CommonJS.
- Always import built-ins with the `node:` prefix. `node:test` exists **only** with the prefix (`require('test')` loads a userland package).
- Use `"exports"` to define a package's public entry points; `"./*"` subpath patterns, not trailing-slash folder mappings (end-of-life since 17).
- Use `"imports"` (`#internal/*`) for private aliases instead of ad-hoc path hacks.

```json
{
  "name": "my-lib",
  "type": "module",
  "engines": { "node": ">=18.17" },
  "exports": {
    ".": { "import": "./dist/index.js", "require": "./dist/index.cjs" },
    "./package.json": "./package.json"
  },
  "imports": { "#config": "./src/config.js" }
}
```

##### `require()` Cannot Load ES Modules on Node 18

```js
// file: load.cjs
// BAD on Node 18: require() of an ES module throws ERR_REQUIRE_ESM (require(esm) is Node 22.12 / 20.19+)
try {
  require('./lib.mjs');
} catch (err) {
  console.log(err.code); // ERR_REQUIRE_ESM
}

// GOOD: dynamic import() works from CommonJS and returns a promise
import('./lib.mjs').then((mod) => console.log(mod.x));
```

Consequences reviewers must catch: upgrading a dependency to an ESM-only major (for example recent `chalk`, `execa`, `got`, `node-fetch@3`, `nanoid@4+`) breaks a CommonJS Node 18 app at `require()` time. Either keep the CJS-compatible major, use `await import()` inside an async function, or migrate the app to ESM. Do not suggest "Node now supports require(esm)" — not on 18.

##### Importing CommonJS from ESM

Named imports from CommonJS work only for exports that `cjs-module-lexer` can detect statically[^11] (`exports.a = …`, `module.exports.b = …`, `module.exports = { a, b }`). Computed assignments are not detected:

```js
// file: cjslib.cjs
exports.a = 1;
module.exports.b = 2;
const key = 'c';
exports[key] = 3;          // not statically detectable
```

```js
// file: use.mjs
import lib, { a, b } from './cjslib.cjs';    // default import is always module.exports
// import { c } from './cjslib.cjs';         // SyntaxError: Named export 'c' not found
console.log(a, b, lib.c);                     // 1 2 3
```

##### New in 18.20: JSON Modules — `with` vs `assert` Depends on the 18.x Minor

| Syntax | 18.0–18.19 | 18.20.x | 19.x | 22+ |
|---|---|---|---|---|
| `import data from './x.json' assert { type: 'json' }` | works (JSON modules experimental) | works; V8 prints *"'assert' is deprecated in import statements … use 'with' instead"* | works (experimental warnings) | **SyntaxError** |
| `import data from './x.json' with { type: 'json' }` | **SyntaxError** | works (stable since 18.20.5) | **SyntaxError** | works |
| no attribute | `ERR_IMPORT_ASSERTION_TYPE_MISSING` | `ERR_IMPORT_ASSERTION_TYPE_MISSING` | `ERR_IMPORT_ASSERTION_TYPE_MISSING` | `ERR_IMPORT_ATTRIBUTE_MISSING` |

(18.20.8, 19.9.0 and 22.23.4 were executed; the 18.0–18.19 column follows the 18.20.0 release notes, which introduced `with`.)[^12]

For code that must run on every 18.x (and on 22+), avoid both syntaxes and read JSON explicitly:

```js
// file: config.mjs — portable across all Node versions (reads the package.json next to this module)
import { readFile } from 'node:fs/promises';
import { createRequire } from 'node:module';

const require = createRequire(import.meta.url);
const pkg = require('./package.json');                       // cached, synchronous

const data = JSON.parse(await readFile(new URL('./package.json', import.meta.url), 'utf8'));
console.log(pkg.name === data.name);
```

##### `__dirname`, `__filename` and `import.meta` in ESM

`__dirname`, `__filename` and `require` do not exist in ES modules. `import.meta.dirname`/`filename` are Node 20.11+.

```js
// file: paths.mjs
import { fileURLToPath } from 'node:url';
import path from 'node:path';

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);
const assets = new URL('./assets/', import.meta.url);      // resolve files relative to the module

console.log(typeof import.meta.dirname);                      // 'undefined' on Node 18
console.log(path.basename(__dirname).length > 0, assets.protocol);
// import.meta.resolve is synchronous and unflagged since 18.19.0 (returns a URL string)
console.log(typeof import.meta.resolve === 'function' ? import.meta.resolve('./x.js').startsWith('file:') : 'needs 18.19+');
```

Use `fileURLToPath()` — never `new URL(import.meta.url).pathname`, which keeps percent-encoding (`%20`) and produces `/C:/…` on Windows.

##### Top-Level `await` and Exit Code 13

Top-level `await` works in ES modules only (a `SyntaxError` in CommonJS). If the awaited promise never settles and nothing else keeps the event loop alive, Node 18 exits **silently with code 13** (Node 22 additionally prints *"Warning: Detected unsettled top-level await"*). A common trigger: awaiting an `AbortSignal.timeout()` or other unref'd timer.

```js
// file: tla.mjs
import { once } from 'node:events';

// BAD: AbortSignal.timeout() uses an unref'd timer, the loop empties, process exits with code 13
// await once(AbortSignal.timeout(20), 'abort');

// GOOD: await work that holds a ref'd handle, or keep one explicitly
const keepAlive = setTimeout(() => {}, 1_000);
const signal = AbortSignal.timeout(20);
await once(signal, 'abort');
clearTimeout(keepAlive);
console.log(signal.reason.name); // TimeoutError
```

##### New in 18.18/18.19: `--import` and `module.register()` Customization Hooks

- `--experimental-loader` (alias `--loader`) still works on 18 but prints an `ExperimentalWarning` recommending `register()`.
- **18.19.0+**: `module.register(specifier, parentURL, { data, transferList })` plus the `initialize` hook; run it from a preload file with `--import` (18.18.0+). Hooks run on a **separate thread** (backported from Node 20): `console.log` inside a hook may never appear — use `fs.writeSync(2, …)` for debugging, and communicate with the main thread via `data`/`MessagePort`.
- `globalPreload` is replaced by `initialize`.[^13][^14]
- In Node 25.9+/26 `module.register()` is deprecated in favor of `module.registerHooks()` (DEP0205; 26.12.1 prints a runtime `DeprecationWarning`, 25.9.0 does not) — that newer API does not exist on 18, so `register()` is correct for Node 18 code.

```js
// file: hooks.mjs
export async function resolve(specifier, context, nextResolve) {
  if (specifier === 'virtual:hello') return { url: 'virtual:hello', shortCircuit: true };
  return nextResolve(specifier, context);
}
export async function load(url, context, nextLoad) {
  if (url === 'virtual:hello') {
    return { format: 'module', source: 'export default "hi from hook"', shortCircuit: true };
  }
  return nextLoad(url, context);
}
```

```js
// file: register.mjs — run as: node --import ./register.mjs app.mjs
import { register } from 'node:module';
register('./hooks.mjs', import.meta.url);
```

Do not use `--experimental-specifier-resolution=node` (extensionless ESM imports): it works on 18 with a warning but was **removed in 19** — the flag is silently ignored afterwards and imports fail with `ERR_MODULE_NOT_FOUND`. Write full relative specifiers with extensions (`./util.js`). `--experimental-default-type=module` (18.19.0) is experimental; prefer explicit `"type"` and file extensions.

##### Dual Packages

A package that ships both ESM and CJS builds can be loaded twice (once per system), producing two copies of module state and failing `instanceof` checks across them. Prefer one format with a thin wrapper (for example a CJS implementation plus an ESM wrapper that re-exports it), and always test `require()` and `import` entry points on Node 18.

***

#### Asynchronous Programming

##### Promises, `async`/`await` and Unhandled Rejections

```js
async function main() {
  // BAD: sequential awaits for independent work
  // const a = await fetchA(); const b = await fetchB();

  // GOOD: run independent work concurrently
  const [a, b] = await Promise.all([fetchA(), fetchB()]);

  // When partial failure is acceptable, inspect each outcome
  const results = await Promise.allSettled([fetchA(), Promise.reject(new Error('down'))]);
  const failures = results.filter((r) => r.status === 'rejected').map((r) => r.reason.message);
  return { a, b, failures };
}

const fetchA = async () => 'A';
const fetchB = async () => 'B';
main().then((r) => console.log(r));
```

- An unhandled rejection terminates the process with exit code 1. Every promise must be awaited, returned or given a `.catch()`. Flag "fire-and-forget" calls such as `saveAudit(x);` inside request handlers.
- `async` callbacks passed to `array.forEach`, `EventEmitter.on` or `setTimeout` are not awaited by the caller; their rejections become unhandled. Use `for…of` with `await`, or `Promise.all(items.map(async …))`.
- `return await` inside `try` is required to catch the rejection locally; outside `try` it is redundant.
- `process.on('unhandledRejection')` handlers should log and exit, not keep running in an unknown state.

**Bounded concurrency without `Promise.withResolvers` (Node 22):**

```js
async function mapLimit(items, limit, fn) {
  const results = new Array(items.length);
  let next = 0;
  async function worker() {
    while (next < items.length) {
      const i = next++;
      results[i] = await fn(items[i], i);
    }
  }
  await Promise.all(Array.from({ length: Math.min(limit, items.length) }, worker));
  return results;
}

// A deferred, the Node 18 way
function deferred() {
  let resolve, reject;
  const promise = new Promise((res, rej) => { resolve = res; reject = rej; });
  return { promise, resolve, reject };
}

mapLimit([1, 2, 3, 4, 5], 2, async (n) => n * 10).then((r) => console.log(r));
const d = deferred(); d.promise.then((v) => console.log('deferred', v)); d.resolve(1);
```

##### Cancellation and Timeouts with `AbortSignal`

`AbortController`, `AbortSignal.timeout()` (17.3 / 16.14) and `signal.throwIfAborted()` (17.3) are available; **`AbortSignal.any()` requires 18.17.0**. `events.addAbortListener()` (18.18.0) avoids listener leaks and the "listener added after abort" race.

```js
const { setTimeout: sleep } = require('node:timers/promises');

async function withDeadline(fn, ms, outerSignal) {
  // AbortSignal.any: Node 18.17+; on earlier 18.x combine manually (see below)
  const signal = AbortSignal.any([AbortSignal.timeout(ms), outerSignal].filter(Boolean));
  return fn(signal);
}

async function slowJob(signal) {
  await sleep(1_000, undefined, { signal });   // rejects with AbortError when the signal aborts
  return 'done';
}

const user = new AbortController();
withDeadline(slowJob, 50, user.signal).catch((err) => {
  // timers/promises rejects with AbortError; the reason (TimeoutError) is err.cause
  console.log(err.name, err.code, err.cause?.name);  // AbortError ABORT_ERR TimeoutError
});
```

Pre-18.17 fallback for combining signals:

```js
function anySignal(signals) {
  const controller = new AbortController();
  for (const s of signals) {
    if (s.aborted) { controller.abort(s.reason); break; }
    s.addEventListener('abort', () => controller.abort(s.reason), { once: true, signal: controller.signal });
  }
  return controller.signal;
}

const s = anySignal([new AbortController().signal, AbortSignal.abort(new Error('x'))]);
console.log(s.aborted, s.reason.message);
```

`AbortSignal.timeout()` timers are unref'd: they do not keep the process alive (see exit code 13 above). `fetch` rejects with a `TimeoutError` `DOMException` (`code` 23) for `AbortSignal.timeout()`, with an `AbortError` `DOMException` (`code` 20) for `controller.abort()`, and with the **reason itself** for `controller.abort(reason)` — so `err.name === 'AbortError'` checks miss custom reasons.

##### Events

```js
const { EventEmitter, once, on } = require('node:events');

const bus = new EventEmitter({ captureRejections: true });  // route async listener rejections to 'error'
bus.on('error', (err) => console.log('handled:', err.message));
bus.on('job', async () => { throw new Error('listener failed'); });
bus.emit('job');

async function waitForReady(emitter, signal) {
  const [value] = await once(emitter, 'ready', { signal });  // rejects on 'error' too
  return value;
}

const e = new EventEmitter();
waitForReady(e, AbortSignal.timeout(1_000)).then((v) => console.log('ready', v));
e.emit('ready', 42);
```

- An `'error'` event with no listener throws and usually crashes the process — every long-lived emitter, stream and socket needs an `'error'` handler.
- `MaxListenersExceededWarning` indicates a leak (listeners added per request). Fix the leak; do not raise the limit blindly. `events.setMaxListeners(n, signal)` applies to `EventTarget`s and `AbortSignal`s.
- `for await (const [x] of on(emitter, 'data', { signal }))` never ends on its own; always pass a signal or break.

##### `AsyncLocalStorage` for Request Context

```js
const { AsyncLocalStorage } = require('node:async_hooks');
const context = new AsyncLocalStorage();

async function handle(reqId) {
  return context.run({ reqId }, async () => {
    await new Promise((r) => setImmediate(r));
    return log('processing');
  });
}
function log(msg) { return `${context.getStore()?.reqId ?? '-'} ${msg}`; }

Promise.all([handle('r1'), handle('r2')]).then((lines) => console.log(lines));
// AsyncLocalStorage.bind(fn) and AsyncLocalStorage.snapshot() exist from 18.16.0
```

Use `AsyncLocalStorage` instead of `async_hooks.createHook` or the `domain` module (deprecated) for request-scoped data. Do not use `enterWith()` in library code.

##### Event Loop Hygiene

- No synchronous I/O (`fs.readFileSync`, `child_process.execSync`, `crypto.pbkdf2Sync`, `zlib.gzipSync`) on request paths; it is fine at startup.
- Split CPU-heavy loops or move them to `worker_threads`. `os.availableParallelism()` (18.14.0) is the correct pool size hint; `os.cpus().length` can return 0 on some platforms and ignores CPU affinity.
- `process.nextTick` callbacks run before promise microtasks and can starve I/O if scheduled recursively; prefer `queueMicrotask` or `setImmediate`.

***

#### `fetch` and Web APIs on Node 18

##### New in 18: `fetch` Is Experimental — Know Its Behavior

`fetch` (undici **5.x**; 5.29.0 in 18.20.8) is enabled by default since 18.0.0; `--no-experimental-fetch` disables it; the `ExperimentalWarning` was removed in 18.13.0; it became stable only in Node 21.[^4][^8]

```js
// file: fetch-demo.mjs
import http from 'node:http';

const server = http.createServer((req, res) => {
  if (req.url === '/slow') return void setTimeout(() => res.end('late'), 2_000);
  if (req.url === '/missing') { res.statusCode = 404; return void res.end('nope'); }
  res.setHeader('content-type', 'application/json');
  res.end(JSON.stringify({ ok: true }));
}).listen(0);
const base = `http://127.0.0.1:${server.address().port}`;

async function getJson(url, { timeoutMs = 5_000, signal } = {}) {
  const res = await fetch(url, { signal: signal ?? AbortSignal.timeout(timeoutMs) });
  if (!res.ok) {
    await res.body?.cancel();                 // release the connection
    throw new Error(`GET ${url} failed: HTTP ${res.status}`);
  }
  return res.json();
}

console.log(await getJson(`${base}/`));
await getJson(`${base}/missing`).catch((e) => console.log(e.message));          // HTTP errors do not reject by themselves
await getJson(`${base}/slow`, { timeoutMs: 100 }).catch((e) => console.log(e.name)); // TimeoutError
await fetch('http://127.0.0.1:59999/').catch((e) => console.log(e.message, e.cause?.code)); // fetch failed ECONNREFUSED
await fetch('/relative').catch((e) => console.log(e.name));                     // TypeError: no base URL in Node
server.close();
```

Review rules for `fetch` on Node 18:

- **HTTP 4xx/5xx do not reject** — check `res.ok`/`res.status`.
- **Network errors are `TypeError: fetch failed`** with the real reason in `err.cause` (`ECONNREFUSED`, `UND_ERR_CONNECT_TIMEOUT`, `UND_ERR_HEADERS_TIMEOUT`, …). Log `err.cause`, not just `err.message`.
- **Always consume or cancel the body** (`await res.text()`, `res.arrayBuffer()`, `res.body.cancel()`); unconsumed bodies keep pooled connections busy.
- **Set a timeout** (`signal: AbortSignal.timeout(ms)`). undici's own defaults (300 s headers and body timeouts) are far too long for service-to-service calls.
- **Relative URLs throw** — there is no document base URL.
- **Proxy environment variables are ignored.** `HTTP_PROXY`/`HTTPS_PROXY` have no effect on Node 18 `fetch` (verified); proxying needs an explicit undici dispatcher (the `undici` package's `ProxyAgent`, undici 6 supports Node ≥ 18.17) or a different client. (`NODE_USE_ENV_PROXY` is Node 24+.)
- **Streaming request bodies require `duplex: 'half'`**, otherwise `TypeError: RequestInit: duplex option is required when sending a body`.
- `res.body` is a **web** `ReadableStream`; convert with `Readable.fromWeb(res.body)` for Node stream APIs.
- `Headers#getSetCookie()` is available in 18.20.8 (bundled undici 5.29.0); `headers.get('set-cookie')` joins values with `, `, which is ambiguous for cookies.
- `FormData` file parts: append a `Blob` with a filename (`form.append('file', blob, 'report.txt')`). On 18.20.8 this internally creates a `buffer.File` and prints *"ExperimentalWarning: buffer.File is an experimental feature"* once (verified) — harmless, but it shows up in logs. The global `File` is Node 20+; `require('node:buffer').File` (18.13+) prints the same warning.
- Security fixes to the bundled undici landed in 18.14.1, 18.18.2, 18.19.1 (`fetch()` brotli DoS), 18.20.1 and 18.20.6 (insufficiently random multipart boundary, CVE-2025-22150); anything before 18.20.6 has known `fetch` vulnerabilities, and later undici CVEs are not fixed in any 18.x.[^1][^32]

```js
// file: fetch-stream.mjs
import http from 'node:http';
import { Readable } from 'node:stream';

const server = http.createServer(async (req, res) => {
  let size = 0;
  for await (const chunk of req) size += chunk.length;
  res.end(String(size));
}).listen(0);
const url = `http://127.0.0.1:${server.address().port}/`;

// Streaming upload: duplex 'half' is mandatory
const body = Readable.toWeb(Readable.from(['a', 'b', 'c']));
const up = await fetch(url, { method: 'POST', body, duplex: 'half' });
console.log('uploaded bytes:', await up.text());

// Streaming download into Node stream APIs
const down = await fetch(url, { method: 'POST', body: 'xyz' });
let text = '';
for await (const chunk of Readable.fromWeb(down.body)) text += chunk;
console.log('downloaded:', text);

// Multipart with a named Blob
const form = new FormData();
form.append('file', new Blob(['hello'], { type: 'text/plain' }), 'report.txt');
const r = await fetch(url, { method: 'POST', body: form });
console.log('multipart bytes:', Number(await r.text()) > 0);
server.close();
```

##### Web Crypto Is Not a Global on Node 18

```js
// file: webcrypto.cjs
// BAD on Node 18 (works on 19+): ReferenceError: crypto is not defined
try { crypto.randomUUID(); } catch (e) { console.log(e.name); }

// GOOD on Node 18
const nodeCrypto = require('node:crypto');
console.log(nodeCrypto.randomUUID().length);                  // 36
const { webcrypto } = nodeCrypto;
const bytes = webcrypto.getRandomValues(new Uint8Array(16));  // call as a method
console.log(bytes.length);

// BAD: detached Web Crypto method loses its receiver
const { getRandomValues } = webcrypto;
try { getRandomValues(new Uint8Array(4)); } catch (e) { console.log(e.code); } // ERR_INVALID_THIS
```

Pitfall for reviewers: in `node -e`/`node -p` and the REPL, built-in modules are exposed as globals, so `crypto` *is* defined there (it is `node:crypto`, not Web Crypto). A snippet that "works in the REPL" fails in a file. `--experimental-global-webcrypto` installs the real global on 18 but is not appropriate for production code.

##### Other Web Platform APIs

- Globals on 18: `AbortController`, `AbortSignal`, `Blob`, `BroadcastChannel`, `DOMException`, `Event`, `EventTarget`, `MessageChannel`, `MessagePort`, `TextEncoder`/`TextDecoder`, `URL`, `URLSearchParams`, `atob`/`btoa` (legacy; prefer `Buffer.from(s, 'base64')`), `performance`, `queueMicrotask`, `structuredClone`, Web Streams classes, `fetch` family.
- `URL.canParse()` and `URLSearchParams#size` require 18.17.0 / 18.16.0. `URLSearchParams#has(name, value)`/`delete(name, value)` require 18.18.0.
- The WHATWG URL parser was replaced by **Ada** in 18.16.0 (faster, stricter WHATWG compliance); `url.parse()` remains the legacy, lenient parser — do not use it for security decisions.

***

#### HTTP Servers and Clients

##### New in 18: Server Timeouts and Defaults

| Setting | Node 16 | Node 18 | Node 19/20 |
|---|---|---|---|
| `server.headersTimeout` | 60000 | 60000 | 60000 |
| `server.requestTimeout` | 0 (none) | **300000** | 300000 |
| `server.keepAliveTimeout` | 5000 | 5000 | 5000 |
| `server.connectionsCheckingInterval` | — | 30000 | 30000 |
| `noDelay` (Nagle disabled) | false | **true** | true |
| `http.globalAgent.keepAlive` | false | **false** | **true** |
| `server.close()` closes idle keep-alive sockets | no | **no** | yes (19.0) |

All values were read from the running binaries. Behind a load balancer, set `server.keepAliveTimeout` **higher** than the balancer's idle timeout (and `headersTimeout` higher than `keepAliveTimeout`) to avoid sporadic 502s. Long uploads or streaming responses beyond 5 minutes need an explicit `requestTimeout`.[^17]

##### Graceful Shutdown on Node 18

`server.close()` stops accepting new connections but on Node 18 **waits for idle keep-alive connections to time out** (verified: the callback fires only after `keepAliveTimeout`; on 19+ it fires immediately). Call `closeIdleConnections()` (18.2.0).

```js
const http = require('node:http');

const server = http.createServer((req, res) => res.end('ok'));
server.keepAliveTimeout = 65_000;
server.headersTimeout = 66_000;

function shutdown() {
  server.close((err) => {
    if (err) console.error(err);
    console.log('closed');
  });
  server.closeIdleConnections();                       // Node 18: required for a prompt close
  setTimeout(() => server.closeAllConnections(), 10_000).unref(); // hard stop for stuck requests
}

server.listen(0, () => {
  const agent = new http.Agent({ keepAlive: true });
  http.get({ port: server.address().port, agent }, (res) => {
    res.resume();
    res.on('end', () => setImmediate(shutdown));
  });
});
process.once('SIGTERM', shutdown);
```

##### Outgoing HTTP: Use an Agent with `keepAlive`

`http.globalAgent` does **not** keep connections alive on 18 (it does from 19). For `http.request`/`https.request`-based clients (axios, got, node-fetch v2), create a shared `new http.Agent({ keepAlive: true, maxSockets })`; never create an agent per request. `fetch` (undici) pools connections itself.

##### New Since 16: DNS Ordering and `localhost` (17.0)

Since 17.0.0, `dns.lookup()` returns addresses in the order the resolver gives them (`verbatim: true`); before, IPv4 was sorted first. On hosts where `localhost` maps to `::1` first, a client connecting to `localhost` gets `ECONNREFUSED ::1:PORT` when the server listens on `127.0.0.1` only. Happy-eyeballs (`autoSelectFamily`) exists on 18.13+ but is **off by default** until Node 20.[^5][^30][^31]

- Bind servers to `::` (dual stack) or connect to `127.0.0.1` explicitly.
- If necessary: `dns.setDefaultResultOrder('ipv4first')` or `--dns-result-order=ipv4first`, or `net.connect({ autoSelectFamily: true })` / `--enable-network-family-autoselection` (18.18.0).

```js
const dns = require('node:dns');
console.log(dns.getDefaultResultOrder());   // 'verbatim' (getter added in 18.17.0)
```

##### HTTP Security

- The HTTP parser was hardened repeatedly during the 18.x line (request-smuggling fixes in 18.5.0, 18.9.1, 18.16.1 and 18.20.1; unbounded chunk-extension DoS fix in 18.19.1). Anything older than 18.20.8 lacks some of the published security fixes. Never enable `--insecure-http-parser` or `insecureHTTPParser: true`.
- Validate and cap request bodies yourself (`node:http` has no body size limit); `maxHeaderSize` defaults to 16 KiB.
- Header values containing CR/LF throw `ERR_INVALID_CHAR`; never build headers from unvalidated input anyway.
- `response.writeEarlyHints()` (18.11.0) is available for `103 Early Hints`.

##### HTTP/2

`node:http2` is stable, but several HTTP/2 denial-of-service fixes reached 18.x late (CVE-2023-44487 "Rapid Reset" via nghttp2 in 18.18.2, CVE-2024-27983 in 18.20.1, CVE-2025-23085 memory leak in 18.20.6) and the January 2026 HTTP/2 server crash on malformed HEADERS frames (CVE-2025-59465) has no 18.x fix.[^1][^7] Do not expose a Node 18 HTTP/2 server directly to the internet; terminate HTTP/2 at a maintained proxy.

##### Frameworks: Async Errors in Express 4 vs 5

Express 4 does not handle rejected promises from `async` route handlers: the error never reaches the error middleware, the request hangs until the client times out, and the rejection is unhandled (verified with express 4.22.3 on 18.20.8). Express 5 (supports Node ≥ 18) forwards rejections to error middleware.

```js
// Express 4 — wrap async handlers (or upgrade to Express 5)
const asyncHandler = (fn) => (req, res, next) => Promise.resolve(fn(req, res, next)).catch(next);

// app.get('/users/:id', asyncHandler(async (req, res) => {
//   res.json(await users.find(req.params.id));
// }));
console.log(typeof asyncHandler(async () => {}));
```

***

#### Streams

##### Always Use `pipeline` — Never Bare `.pipe()`

```js
const { pipeline } = require('node:stream/promises');
const { Readable, Transform, Writable } = require('node:stream');
const zlib = require('node:zlib');

async function run() {
  const out = [];
  await pipeline(
    Readable.from(['a', 'b', 'c']),
    async function* upper(source) {                // async generator transform
      for await (const chunk of source) yield String(chunk).toUpperCase();
    },
    zlib.createGzip(),
    zlib.createGunzip(),
    new Writable({ write(chunk, _enc, cb) { out.push(chunk.toString()); cb(); } }),
  );
  return out.join('');
}

run().then((s) => console.log(s));   // ABC
```

`a.pipe(b)` does not forward errors or destroy the other streams on failure — leaked file descriptors and hung requests. `pipeline` (from `node:stream/promises`) propagates errors, destroys all streams, and supports `{ signal }`.

##### Stream Helpers and Consumers

```js
const { Readable } = require('node:stream');
const { json, text } = require('node:stream/consumers');

(async () => {
  const doubled = await Readable.from([1, 2, 3]).map((n) => n * 2).toArray();   // experimental helpers (16.14+)
  const parsed = await json(Readable.from(['{"a":', '1}']));
  const joined = await text(Readable.from(['x', 'y']));
  console.log(doubled, parsed, joined);
})();
```

- `readable.map/filter/forEach/reduce/toArray/take/drop` are **experimental** on 18; `map(fn, { concurrency })` runs `fn` concurrently but preserves order.
- `for await (const chunk of stream)` handles backpressure; `break` destroys the stream.
- Respect `write()`'s return value or use `pipeline`; ignoring it buffers unboundedly.
- `stream.getDefaultHighWaterMark()`/`setDefaultHighWaterMark()` require 18.17.0.
- Since 18.0, returning a promise (thenable) from `_construct`, `_write`, `_final`, etc. is no longer awaited — call the callback (DEP0157).
- Web streams interop: `Readable.toWeb()`, `Readable.fromWeb()`, `Writable.toWeb()`, `Duplex.from(webStreamPair)` (18.17.0).[^28]

##### New in 18: Web Streams as Globals

```js
// file: webstreams.mjs — no imports needed on Node 18
const source = new ReadableStream({
  start(controller) {
    controller.enqueue('hello ');
    controller.enqueue('web streams');
    controller.close();
  },
});
const upper = new TransformStream({
  transform(chunk, controller) { controller.enqueue(chunk.toUpperCase()); },
});
const compressed = source
  .pipeThrough(upper)
  .pipeThrough(new TextEncoderStream())
  .pipeThrough(new CompressionStream('gzip'));

const text = await new Response(compressed.pipeThrough(new DecompressionStream('gzip'))).text();
console.log(text);                                  // HELLO WEB STREAMS

// A reader locks the stream until released
const rs = new ReadableStream({ start(c) { c.enqueue(1); c.close(); } });
const reader = rs.getReader();
console.log(await reader.read(), rs.locked);        // { value: 1, done: false } true
reader.releaseLock();
console.log(rs.locked);                             // false
```

Web streams were experimental in 16 (module `stream/web` only) and are globals without a warning in 18, but they are still marked experimental in the 18 docs. Within Node-centric code, prefer Node streams plus `pipeline`; use web streams at `fetch`/`Response` boundaries and convert with `Readable.fromWeb()`/`Readable.toWeb()`.

***

#### File System and Paths

##### Prefer `fs/promises` and Close Every `FileHandle`

```js
// file: fs-demo.mjs
import { mkdtemp, writeFile, readFile, open, readdir, rm, mkdir } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import path from 'node:path';

const dir = await mkdtemp(path.join(tmpdir(), 'app-'));
try {
  await mkdir(path.join(dir, 'a', 'b'), { recursive: true });
  await writeFile(path.join(dir, 'a', 'b', 'x.txt'), 'line1\nline2\n', 'utf8');

  // FileHandle: close in finally (no `await using` on Node 18)
  const fh = await open(path.join(dir, 'a', 'b', 'x.txt'));
  try {
    for await (const line of fh.readLines()) console.log('line:', line);   // 18.11.0+
  } finally {
    await fh.close().catch(() => {});      // readLines() closes the handle when done; close() again is harmless
  }

  // recursive readdir: 18.17.0+
  const entries = await readdir(dir, { recursive: true, withFileTypes: true });
  for (const d of entries) {
    // Dirent.parentPath: 18.20.0+; Dirent.path (doc-deprecated) exists since 18.17.0
    console.log(d.isFile() ? 'file' : 'dir ', path.relative(dir, path.join(d.parentPath ?? d.path, d.name)));
  }
  console.log((await readFile(path.join(dir, 'a', 'b', 'x.txt'), 'utf8')).length);
} finally {
  await rm(dir, { recursive: true, force: true });
}
```

- `fs.cp()`/`fsPromises.cp()` exist (16.7) but are **experimental** on 18.
- `fs.statfs()` requires 18.15.0.[^27]
- `fs.watch(dir, { recursive: true })` throws `ERR_FEATURE_UNAVAILABLE_ON_PLATFORM` on **Linux** in Node 18 (works on macOS/Windows; Linux support is 19.1). Use chokidar or watch individual directories.
- Pass strings/Buffers/TypedArrays to `fs.write*`; passing an object with its own `toString` prints DEP0162 on 18 and throws on 19+.
- Avoid `fs.exists()`; check-then-act is racy. Open with the right flag (`'wx'` for create-exclusive) and handle `EEXIST`/`ENOENT`.

##### Path Traversal

```js
const path = require('node:path');

function safeJoin(root, userPath) {
  const base = path.resolve(root);
  const target = path.resolve(base, userPath);
  // path.relative must not start with '..' and must not be absolute (other drive on Windows)
  const rel = path.relative(base, target);
  if (rel === '' || (!rel.startsWith('..') && !path.isAbsolute(rel))) return target;
  throw Object.assign(new Error('path escapes root'), { code: 'EPATH' });
}

console.log(safeJoin('/srv/files', 'a/b.txt'));
try { safeJoin('/srv/files', '../../etc/passwd'); } catch (e) { console.log(e.code); }
try { safeJoin('/srv/files', '/etc/passwd'); } catch (e) { console.log(e.code); }
```

- `target.startsWith(base)` alone is wrong (`/srv/files-evil` passes).
- Resolve symlinks (`fs.realpath`) before the check if users can create links.
- On Windows, reserved device names and drive-relative paths need extra care; 18.20.6 fixed a `path.normalize()` traversal (CVE-2025-23084), and later Windows path fixes (2025–2026) are not in any 18.x.[^1]

***

#### Child Processes and Worker Threads

```js
const { execFile } = require('node:child_process');
const { promisify } = require('node:util');
const execFileAsync = promisify(execFile);

async function gitLog(repoDir, ref) {
  // GOOD: no shell, argument array, timeout, bounded output
  if (!/^[\w./-]+$/.test(ref)) throw new Error('invalid ref');
  const { stdout } = await execFileAsync('git', ['-C', repoDir, 'log', '--oneline', '-n', '5', ref, '--'], {
    timeout: 10_000,
    maxBuffer: 1024 * 1024,
  });
  return stdout;
}

// BAD: shell interpolation of user input
// exec(`git log ${ref}`)  -> ref = "main; rm -rf ~"

gitLog('.', 'HEAD').then((s) => console.log(typeof s), (e) => console.log('git unavailable:', e.code));
```

- `exec()` and `spawn(..., { shell: true })` run a shell; never interpolate untrusted data. Pass `--` before positional user arguments to stop option injection.
- On **Windows**, since 18.20.2 `spawn`/`execFile` of `.bat`/`.cmd` files without `shell: true` throws `EINVAL` (CVE-2024-27980 "BatBadBut" fix; 18.20.4 fixed an incomplete fix, CVE-2024-36138). Code that relied on running batch files directly must now use `shell: true` with carefully escaped arguments — or better, avoid batch files.[^20]
- Always pass `timeout`/`signal` and consume or ignore stdio (`stdio: 'ignore'`); a child blocked on a full pipe never exits.[^29]
- `subprocess[Symbol.dispose]()` exists since 18.18.0, but without `using` syntax you call `subprocess.kill()` yourself.

**Worker threads** for CPU-bound work:

```js
const { Worker, isMainThread, parentPort, workerData } = require('node:worker_threads');

if (isMainThread) {
  const worker = new Worker(__filename, { workerData: { n: 30 }, name: 'fib-worker' }); // name visible in inspector (18.16+)
  worker.once('message', (r) => console.log('fib =', r));
  worker.once('error', (e) => console.error(e));
} else {
  const fib = (n) => (n < 2 ? n : fib(n - 1) + fib(n - 2));
  parentPort.postMessage(fib(workerData.n));
}
```

Use a pool (piscina or a hand-written queue) sized by `os.availableParallelism()`; spawning a worker per request is expensive. Values cross threads via structured clone — functions and class prototypes do not survive.

***

#### Security

##### Node 18 Is End-of-Life — Treat It as a Finding

- No security fixes since 18.20.8 (March 27, 2025); CVEs published since then are assumed to affect 18.x.[^6] Examples of fixes that exist only in supported lines: the January 13, 2026 release (CVE-2025-55131 non-zero-filled `Buffer.alloc`/`Uint8Array` when allocations are interrupted by a `vm` `timeout`, CVE-2025-59465 HTTP/2 server crash on malformed HEADERS, CVE-2025-59466 uncatchable stack overflow via `async_hooks`, CVE-2026-21637 TLS callback exceptions bypassing error handlers).[^7]
- OpenSSL 3.0 (bundled 3.0.16) is out of upstream support; certificate store updates stopped at NSS 3.108.[^1][^24]
- Running 18 on the public internet requires compensating controls (patched reverse proxy terminating TLS and HTTP/2, WAF) or a commercial extended-support build; the recommendation is to upgrade.

##### New Since 16: OpenSSL 3 Implications (Node 17+)

Verified on 18.20.8 (OpenSSL 3.0.16) vs 16.20.2 (OpenSSL 1.1.1):

| Operation | Node 16 | Node 18 |
|---|---|---|
| `createHash('md4')`, `createHash('whirlpool')` | ok | `ERR_OSSL_EVP_UNSUPPORTED` |
| `createCipheriv('bf-cbc' / 'rc4' / 'des-cbc', …)` | ok | `ERR_OSSL_EVP_UNSUPPORTED` |
| `createHash('md5' / 'sha1' / 'sha3-256' / 'blake2b512')` | ok | ok |
| `des-ede3-cbc` | ok | ok |
| `privateDecrypt` with `RSA_PKCS1_PADDING` | ok | `ERR_INVALID_ARG_VALUE` since 18.19.1 (Marvin attack, CVE-2023-46809) |

- `ERR_OSSL_EVP_UNSUPPORTED` from old toolchains (webpack 4 uses MD4 for hashing) is fixed by upgrading the tool, **not** by `--openssl-legacy-provider` or `NODE_OPTIONS=--openssl-legacy-provider` in production; that flag re-enables broken algorithms process-wide.
- `--security-revert=CVE-2023-46809` re-enables PKCS#1 v1.5 decryption — flag it; migrate to `RSA_PKCS1_OAEP_PADDING` with SHA-256.[^19]
- Since 18.5.0 Node reads only the `nodejs_conf` section of `openssl.cnf` by default (CVE-2022-32223); `--openssl-shared-config` restores the shared section and its risks.[^33]
- `crypto.createCipher()`/`createDecipher()` (no IV, MD5 key derivation) are runtime-deprecated (DEP0106) and removed in Node 22; use `createCipheriv` with a random IV, preferably AES-256-GCM.

##### Cryptography Idioms

```js
const crypto = require('node:crypto');

// Tokens and IDs
const token = crypto.randomBytes(32).toString('base64url');
const id = crypto.randomUUID();

// Constant-time comparison (lengths must match first)
function safeEqual(a, b) {
  const ba = Buffer.from(a);
  const bb = Buffer.from(b);
  return ba.length === bb.length && crypto.timingSafeEqual(ba, bb);
}

// Password hashing with scrypt (salted, memory-hard)
function hashPassword(password) {
  const salt = crypto.randomBytes(16);
  const hash = crypto.scryptSync(password, salt, 64, { N: 16384, r: 8, p: 1 });
  return `${salt.toString('base64')}:${hash.toString('base64')}`;
}
function verifyPassword(password, stored) {
  const [s, h] = stored.split(':').map((x) => Buffer.from(x, 'base64'));
  const candidate = crypto.scryptSync(password, s, h.length, { N: 16384, r: 8, p: 1 });
  return crypto.timingSafeEqual(candidate, h);
}

// Authenticated encryption
function encrypt(key, plaintext) {
  const iv = crypto.randomBytes(12);
  const cipher = crypto.createCipheriv('aes-256-gcm', key, iv);
  const ct = Buffer.concat([cipher.update(plaintext, 'utf8'), cipher.final()]);
  return { iv, ct, tag: cipher.getAuthTag() };
}
function decrypt(key, { iv, ct, tag }) {
  const decipher = crypto.createDecipheriv('aes-256-gcm', key, iv);
  decipher.setAuthTag(tag);
  return Buffer.concat([decipher.update(ct), decipher.final()]).toString('utf8');
}

const key = crypto.randomBytes(32);
console.log(token.length, id.length, safeEqual('a', 'a'), verifyPassword('pw', hashPassword('pw')), decrypt(key, encrypt(key, 'secret')));
```

- `Math.random()` is never acceptable for tokens, passwords, nonces or IDs that must be unguessable.
- Never reuse a GCM IV with the same key; never truncate auth tags (pass `authTagLength` when decrypting with nonstandard tags).
- `===` on secrets leaks timing; use `timingSafeEqual`.
- Use the async `crypto.scrypt`/`pbkdf2` on request paths (the `*Sync` variants block the loop).
- `crypto.hash()` (one-shot) is Node 21.7+; on 18 use `createHash(alg).update(data).digest()`.[^18]

##### Injection, Deserialization and Sandboxing

- **SQL**: always parameterized queries; never template literals into SQL.
- **`eval`, `new Function`, `vm.runInNewContext`** on untrusted input are remote code execution. `node:vm` is **not** a security boundary.
- **`--experimental-policy`** (integrity manifests) had multiple bypass CVEs in 18.x (18.14.1, 18.16.1, 18.17.1, 18.18.2) and was removed in Node 22; do not treat it as a sandbox. There is no permission model on 18 (`--experimental-permission` is Node 20).
- **Regular expressions** on user input: avoid nested quantifiers (`(a+)+`), cap input length; V8 has no linear-time engine enabled by default.
- **Prototype pollution**: see Objects section; validate JSON with a schema (ajv, zod) and reject unknown keys.
- **YAML/XML**: use safe loaders (`yaml` default schema, no custom tags; disable XML external entities).
- **Open redirects / SSRF**: parse with `new URL()`, allow-list hosts and protocols, and resolve DNS before connecting if you block internal ranges (DNS rebinding).

##### Supply Chain

- Commit `package-lock.json`; install with `npm ci` in CI and images. npm 10.8.2 is bundled.
- Consider `npm ci --ignore-scripts` (then run needed build scripts explicitly) for untrusted dependency trees.
- `npm audit --omit=dev` in CI; note that many fixed versions of dependencies now require Node ≥ 20, so auditing an 18 project increasingly surfaces unfixable advisories — another reason to upgrade.
- Corepack (0.32.0, experimental) pins pnpm/yarn via `packageManager`; current npm 11 does not support Node 18.

***

#### Performance

- **Measure first**: `node --cpu-prof`, `--heap-prof`, `--prof`, `--inspect`, `perf_hooks.monitorEventLoopDelay()`. `v8.setHeapSnapshotNearHeapLimit()`/`--heapsnapshot-near-heap-limit` capture OOMs.
- **Containers**: V8's default heap limit depends on available memory; set `--max-old-space-size` explicitly to ~75% of the container limit. `process.constrainedMemory()` (18.15.0, experimental) reports the cgroup limit; `process.availableMemory()` is Node 22.
- **Startup**: `--build-snapshot` (18.8.0, experimental) can snapshot initialized state; the V8 compile cache for `vm.Script` was restored in 18.20.0 (helps Jest).
- **Buffers**: `Buffer.allocUnsafe()` is faster but contains old memory — only when every byte is overwritten. Use `buf.subarray()` to avoid copies and `Buffer.concat(list, totalLength)`.
- **JSON**: `JSON.parse` of very large payloads blocks the loop; stream-parse or move to a worker.
- **Logging**: `process.stdout`/`stderr` writes are synchronous for files and TTYs on POSIX (asynchronous for pipes); heavy `console.log` use in hot paths blocks the loop — use an asynchronous structured logger (pino).
- **Class fields and private methods** initialize as fast as ordinary property stores since V8 10.1 (Node 18); no need to avoid them for performance.[^4]
- **URL parsing**: Ada (18.16/18.17) made `new URL()` much faster; do not hand-roll URL parsing for speed.
- **Keep-alive**: the default `http.globalAgent` closes connections (see HTTP) — set `keepAlive: true` for outbound HTTP-heavy services.

##### Memory Leaks and Caches

```js
// Bounded cache instead of an unbounded Map
class LruCache {
  #max;
  #map = new Map();
  constructor(max) { this.#max = max; }
  get(key) {
    if (!this.#map.has(key)) return undefined;
    const value = this.#map.get(key);
    this.#map.delete(key);
    this.#map.set(key, value);                 // refresh recency (Map keeps insertion order)
    return value;
  }
  set(key, value) {
    this.#map.delete(key);
    this.#map.set(key, value);
    if (this.#map.size > this.#max) this.#map.delete(this.#map.keys().next().value);
    return this;
  }
  get size() { return this.#map.size; }
}

const cache = new LruCache(2);
cache.set('a', 1).set('b', 2);
cache.get('a');
cache.set('c', 3);
console.log(cache.size, cache.get('b'), cache.get('a'));   // 2 undefined 1
```

Typical leak sources a reviewer should flag: module-level `Map`/arrays keyed by request data without eviction, listeners added per request (`MaxListenersExceededWarning`), timers/intervals never cleared, closures capturing large buffers in long-lived callbacks, and unconsumed `fetch` bodies. `WeakRef`/`FinalizationRegistry` are available but collection timing is unspecified — never use them for correctness (closing files, releasing locks).

***

#### New in 18: Testing with `node:test`

##### Feature Availability by 18.x Minor

| Capability | Since |
|---|---|
| `test()`, subtests, `skip`/`todo`/`only` options, TAP output | 18.0.0 |
| `node --test` (directory/file discovery) | 18.1.0 |
| `describe`/`it` (18.6.0), `before`/`after`/`beforeEach`/`afterEach` (18.8.0) | 18.6–18.8 |
| `--test-name-pattern` | 18.11.0 |
| `mock.fn()`, `mock.method()`, `t.mock` | 18.13.0 |
| Reporters (`--test-reporter=spec|tap|dot`, `--test-reporter-destination`), `--experimental-test-coverage`, `describe.only` | 18.15.0 |
| `test.skip/.todo/.only` shorthands, `node:test/reporters` | 18.17.0 |
| `MockTimers` (`t.mock.timers`, experimental), `--test-concurrency`, `--test-shard`, junit reporter | 18.19.0 |

Not on 18: snapshots, `mock.module()`, `--test-timeout`, `--test-force-exit`, `--test-skip-pattern`, glob arguments to `node --test` (Node 21; on 18 a quoted glob fails with *"Could not find …"*, unquoted globs are expanded by the shell), `Date` mocking in `MockTimers`.

```js
// file: test/math.test.js — run: node --test   (or node --test --test-reporter=spec)
const { describe, it, test, mock } = require('node:test');
const assert = require('node:assert/strict');

const add = (a, b) => a + b;
const later = (cb) => setTimeout(() => cb('done'), 5_000);

describe('add', () => {
  it('adds', () => assert.equal(add(1, 2), 3));
  it.todo('handles bigint');                              // shorthand: 18.17+
});

test('mock.fn and t.mock.method', (t) => {
  const fn = mock.fn((x) => x * 2);
  assert.equal(fn(2), 4);
  assert.equal(fn.mock.calls.length, 1);
  const repo = { find: () => 'real' };
  t.mock.method(repo, 'find', () => 'fake');              // restored after the test
  assert.equal(repo.find(), 'fake');
});

test('mock timers (18.19+, experimental)', (t) => {
  t.mock.timers.enable(['setTimeout']);                   // array form only on 18
  const cb = t.mock.fn();
  later(cb);
  t.mock.timers.tick(5_000);
  assert.deepEqual(cb.mock.calls[0].arguments, ['done']);
});

test('async errors', async () => {
  await assert.rejects(Promise.reject(new TypeError('x')), TypeError);
});
```

Node 18 test-runner pitfalls (verified on 18.20.8):

- **`MockTimers.tick(ms)` fires a `setInterval` callback at most once per `tick()` call** on 18.20.8: `tick(350)` with a 100 ms interval runs it once (Node 20+ runs it three times). Tick in interval-sized steps on 18.
- **Subtests must be awaited**: `t.test(...)` without `await` is cancelled when the parent finishes (*"test did not finish before its parent and was cancelled"*).
- The default reporter is `spec` on a TTY and **TAP** otherwise (CI logs); pass `--test-reporter=spec` (or junit plus `--test-reporter-destination`) explicitly in CI.
- `--experimental-test-coverage` is experimental and prints a text table; use c8 for thresholds/lcov.
- Default discovery: files named `*.test.{js,cjs,mjs}`, `*-test.*`, `*_test.*`, `test-*.*`, `test.*`, and every `.js/.cjs/.mjs` under `test/` directories (`node_modules` excluded).[^16]
- Use `node:assert/strict` (or `assert.strictEqual`/`deepStrictEqual`); loose `assert.equal`/`deepEqual` coerce types. Since 18.0, deep-equality also compares `RegExp` `lastIndex`.

**Third-party runners**: Jest 29/30 (30 requires ≥ 18.14), Vitest ≤ 3, Mocha ≤ 11 support Node 18; Vitest 4/5 and Mocha 12 do not. Jest 30's `--experimental-vm-modules` ESM support relies on `vm` APIs that are experimental on 18.

***

#### Tooling

##### `package.json`, Engines and Version Files

```json
{
  "engines": { "node": ">=18.17.0 <19" },
  "packageManager": "npm@10.8.2",
  "scripts": {
    "test": "node --test --test-reporter=spec",
    "dev": "node --watch src/server.js"
  }
}
```

- Put the exact minor floor your code needs in `engines` (18.17 for `AbortSignal.any`/`URL.canParse`/recursive `readdir`; 18.19 for `module.register`/mock timers; 18.20 for import attributes/`parentPath`). npm only warns on mismatches unless `engine-strict=true` is set in `.npmrc`.
- `node --watch` (18.11.0, experimental, prints a warning) restarts on changes of imported files; `--watch-path` fails on Linux in 18 (`ERR_FEATURE_UNAVAILABLE_ON_PLATFORM`). Use it for development only.[^15]

##### TypeScript Configuration for a Node 18 Runtime

`@tsconfig/node18` sets `target: "es2022"`, `lib: ["es2023"]`, `module`/`moduleResolution: "node16"`.[^25] `lib: es2023` declares the change-array-by-copy methods that Node 18 lacks — verified: code using `xs.toSorted()` type-checks under `lib: ["es2023"]` and fails at runtime with `TypeError: xs.toSorted is not a function`. With `lib: ["es2022"]`, `findLast` (which Node 18 has) is not typed.

Options:

- Use `"lib": ["es2022"]` and add a small declaration for `findLast`/`findLastIndex`, or
- keep `es2023` and forbid the Node 20 methods with a lint rule (for example ESLint `no-restricted-properties` for `toSorted`, `toReversed`, `toSpliced`, `with`).
- Pin `@types/node` to `^18` (latest 18.19.x) so Node 20+ APIs are not typed as available.
- `module: "node16"`/`"nodenext"` matches Node's ESM resolution (extensions required in relative imports).

##### ESLint and Formatting

- ESLint 9 (flat config) supports Node ^18.18; ESLint 10 does not. Set `languageOptions.ecmaVersion: 2022` (or 2023 for parsing only) and `globals` from the `globals` package's `node` set — but note that set includes globals Node 18 lacks (`crypto`, `File`, `navigator`, `WebSocket` depending on version).
- `eslint-plugin-n` 17.x (supports Node ^18.18; 18.x requires Node 20.19+) reads the supported range from `package.json` `engines` or the shared setting `settings.node.version`; its `n/no-unsupported-features/es-builtins`, `es-syntax` and `node-builtins` rules flag post-18 APIs.
- Prettier 3 supports Node ≥ 14.

##### Snapshots and Single Executable Applications (New in 18.8 / 18.16, Experimental)

- `node --snapshot-blob snap.blob --build-snapshot entry.js` (18.8.0) serializes the heap after running `entry.js`; `node --snapshot-blob snap.blob` restores it, optionally running a function registered with `v8.startupSnapshot.setDeserializeMainFunction()` (verified on 18.20.8). Snapshot entry scripts must avoid open handles and user-land modules that are not snapshot-safe.
- Single executable applications (18.16.0) work only by injecting a script into a copy of the `node` binary with `postject` (`NODE_JS_CODE` resource and the documented sentinel fuse). The `node:sea` module and `--experimental-sea-config` are Node 20+. Both features are experimental on 18; prefer a container image for distribution.

##### Docker and CI

- Use an exact tag (`node:18.20.8-bookworm-slim`), not `node:18` or `node:lts` (which moved to newer majors long ago). The official `node:18*` images were last rebuilt in May 2025, so their OS packages carry every Debian/Alpine fix since then as an open vulnerability.
- Run as the non-root `node` user, set `NODE_ENV=production`, use `npm ci --omit=dev`, and start `node` directly (or with `--init`/tini) so `SIGTERM` reaches the process.
- `actions/setup-node` with `node-version: 18.20.8` (or `18.x`) still resolves; add the target upgrade version (24 or 26) to the CI matrix to track breakage.

***

#### Observability and Diagnostics

- **Diagnostics channels**: `diagnostics_channel.subscribe(name, fn)` (doc-deprecated `channel.subscribe(fn)` form aside); built-in `http.client.request.start`, `http.server.request.start`, `net.client.socket`, `undici:request:create` (from undici) channels exist; `tracingChannel()` arrived in 18.19.0 (experimental).
- **Performance**: `performance.now()`, `performance.mark/measure`, `PerformanceObserver` from `node:perf_hooks` (not a global on 18), `monitorEventLoopDelay()`, `performance.eventLoopUtilization()`.
- **Reports**: `process.report.writeReport()`/`--report-on-fatalerror` produce JSON diagnostics (heap info extended in 18.4).
- **Source maps**: `--enable-source-maps` for transpiled code (cost on stack-trace creation; fine for services).
- **Stack traces**: `Error.stackTraceLimit` defaults to 10; async stack traces are on by default for `await` chains.
- **Logging**: structured JSON (pino), one logger per module with child loggers carrying request IDs from `AsyncLocalStorage`; never log secrets, tokens or full request bodies; log `err` objects so `cause` and `stack` are serialized.
- **Signals**: handle `SIGTERM`/`SIGINT` for graceful shutdown; `process.exitCode = 1` plus natural exit is preferable to `process.exit()` (which can truncate pending stdout writes to pipes).

```js
const { monitorEventLoopDelay, performance, PerformanceObserver } = require('node:perf_hooks');

const h = monitorEventLoopDelay({ resolution: 20 });
h.enable();
const obs = new PerformanceObserver((list) => {
  for (const e of list.getEntries()) console.log(e.name, e.duration >= 0);
  obs.disconnect();
});
obs.observe({ entryTypes: ['measure'] });
performance.mark('a');
for (let i = 0; i < 1e6; i++);
performance.mark('b');
performance.measure('loop', 'a', 'b');
setTimeout(() => { h.disable(); console.log('p99 ms', (h.percentile(99) / 1e6) >= 0); }, 50);
```

***

#### CLI Programs, Processes and Configuration

##### Argument Parsing with `util.parseArgs` (18.3.0; `default` option 18.11.0)

```js
// file: cli.mjs
import { parseArgs } from 'node:util';
import { setInterval as every, setTimeout as sleep } from 'node:timers/promises';

const { values, positionals } = parseArgs({
  args: ['--port', '8080', '-v', '--tag', 'a', '--tag', 'b', 'input.txt'], // omit `args` to use process.argv
  options: {
    port: { type: 'string', default: '3000' },
    verbose: { type: 'boolean', short: 'v' },
    tag: { type: 'string', multiple: true },
  },
  allowPositionals: true,
  strict: true,                                       // unknown options throw
});
const port = Number.parseInt(values.port, 10);       // values are strings or booleans only
if (!Number.isInteger(port) || port < 1 || port > 65535) throw new Error('invalid --port');
console.log(port, values.verbose, values.tag, positionals);

try {
  parseArgs({ args: ['--unknown'], options: {} });
} catch (err) {
  console.log(err.code);                              // ERR_PARSE_ARGS_UNKNOWN_OPTION
}

// Async-iterable interval from timers/promises; `break` clears it
let ticks = 0;
for await (const _ of every(10)) {
  if (++ticks === 3) break;
}
await sleep(5);
console.log('ticks', ticks);
```

`parseArgs` has no type coercion, no `number` type, no help generation and no subcommands; validate every value. Its result object has a null prototype.

##### Interactive Input with `readline/promises` (experimental on 18)

```js
// file: ask.mjs
import * as readline from 'node:readline/promises';
import { Readable } from 'node:stream';

// In a real CLI: readline.createInterface({ input: process.stdin, output: process.stdout })
const rl = readline.createInterface({ input: Readable.from(['alice\n']), terminal: false });
try {
  const name = await rl.question('Name? ');
  console.log(`hello ${name}`);
} finally {
  rl.close();
}
```

##### Configuration and Environment Variables

- There is **no `--env-file` and no `process.loadEnvFile()`** on 18 (Node 20.6 / 21.7). Use `dotenv` (`import 'dotenv/config'`) or inject variables from the process manager.
- `process.env` values are always strings: `process.env.N = 5` stores `'5'`, and assigning `undefined` stores the string `'undefined'`. Use `delete process.env.X` to unset.
- Since 18.0, `Object.defineProperty(process.env, …)` with accessors throws `ERR_INVALID_OBJECT_DEFINE_PROPERTY`.
- Parse and validate configuration once at startup (schema, defaults, fail fast) and pass a frozen config object around; do not read `process.env` throughout the code base.
- `NODE_OPTIONS` can inject flags such as `--require` or `--openssl-legacy-provider`; review Dockerfiles and CI for it.

##### Exit Codes and Signals

```js
const { setTimeout: sleep } = require('node:timers/promises');

async function main() {
  await sleep(10);
  throw new Error('config invalid');
}

main().catch((err) => {
  console.error(err);
  process.exitCode = 1;          // let pending I/O flush; avoid process.exit() in libraries
});

process.once('SIGINT', () => { console.error('interrupted'); process.exitCode = 130; });
```

- Exit code 1: uncaught exception or unhandled rejection; 13: unsettled top-level `await`; 130/143: conventional for SIGINT/SIGTERM when the process re-raises or sets them.
- Fatal error output includes the Node version line (since 17), useful when triaging logs.
- `process.exit(code)` with a non-integer string is documentation-deprecated (DEP0164); pass integers.

##### Serialization Formats

- `JSON` is the interchange format; validate after parsing.
- `v8.serialize()` output from Node 18 **cannot be read by Node 16** (*"Unable to deserialize cloned data due to invalid or unsupported version"*, verified); Node 18 reads Node 16 data. Do not share `v8.serialize` blobs across runtime versions (caches, queues) during rolling upgrades.
- `structuredClone`/`postMessage` use the same algorithm as `v8.serialize` within one process.

***

#### Removed / Changed in Node 18 — Migration Notes

##### Upgrading from Node 16 to 18

1. **OpenSSL 3**: run the test suite; replace MD4/RC4/Blowfish/DES uses; upgrade webpack 4-era tooling instead of `--openssl-legacy-provider`; RSA PKCS#1 v1.5 private decryption is disabled from 18.19.1.
2. **DNS `verbatim` ordering**: `localhost` may resolve to `::1`; bind servers to `::` or connect to `127.0.0.1`.
3. **HTTP server**: new `requestTimeout` (5 min) may cut long uploads/streams; `noDelay` is on.
4. **Global `fetch`**: a global `fetch` now exists — code that feature-detects `typeof fetch` changes path; polyfills (`node-fetch`, `cross-fetch`) may be shadowed or shadow the built-in; `Headers`/`FormData` globals can conflict with polyfill-provided ones.
5. **Deprecations turned runtime**: `fs.write*` object coercion, `multipleResolves`.
6. **Native addons**: recompile (`NODE_MODULE_VERSION` 93 → 108).
7. **OS floor**: glibc 2.28 (CentOS 7 / Amazon Linux 2 binaries no longer run), macOS 10.15.
8. **Stream internals**: `finished()` errors on already-errored streams (17), no `'data'` after `'error'`, Duplex write/read errors if not writable/readable — custom stream implementations may need fixes.

##### Upgrading from Node 18 to 20/22/24 (What Will Break)

- `import … assert {…}` → SyntaxError on 22+; switch to `with` (18.20+) or explicit JSON reading.
- `http.globalAgent` keep-alive on (19+) and `server.close()` closes idle sockets (19+); tests that counted connections or relied on one-request agents change behavior.
- `fs.write*` object coercion throws (19+).
- `--experimental-specifier-resolution=node` gone (19+); `--experimental-policy` gone (22); `crypto.createCipher` gone (22); `--experimental-loader` still works but `module.register` itself is deprecated in 25.9+/26 in favor of `module.registerHooks`.
- Globals added in 19–22 (`crypto`, `File`, `CustomEvent`, `navigator`, `WebSocket`) may collide with user-defined globals or change polyfill detection.
- `require(esm)` becomes possible (22.12/20.19) — dual-package behavior can change.

***

#### Production / Review Checklist

##### Version and Support

- [ ] The pinned version is known (`engines`, `.nvmrc`, Docker tag, CI); `engines` carries the 18.x **minor** the code needs
- [ ] The review recommends upgrading to Node 24 or 26 (Node 18 EOL since 2025-04-30; OpenSSL 3.0 out of support)
- [ ] No Node 20+ APIs or syntax (`toSorted`, `Object.groupBy`, `Promise.withResolvers`, Set methods, `using`, global `crypto`/`File`/`navigator`/`WebSocket`, `require(esm)`, `import.meta.dirname`, `--env-file`, `fs.glob`, `util.styleText`, `URL.parse`, `node:sqlite`)
- [ ] APIs from late 18.x minors match `engines` (`AbortSignal.any`/`URL.canParse` 18.17, `--import`/`addAbortListener` 18.18, `module.register`/mock timers 18.19, `with` attributes/`parentPath` 18.20)
- [ ] TypeScript `lib` does not expose Node 20 array methods unchecked; `@types/node` pinned to 18
- [ ] Dependency majors still support Node 18 (ESLint ≤ 9, Mocha ≤ 11, Vitest ≤ 3, undici ≤ 6, npm 10)

##### Correctness

- [ ] Every promise awaited/returned/caught; no async callbacks in `forEach`/event handlers without error routing
- [ ] `??` instead of `||` where `0`/`''`/`false` are valid
- [ ] No in-place `sort()`/`reverse()` on arrays owned by callers
- [ ] `structuredClone` not used for class instances; `JSON` round-trips not used for `Date`/`Map`/`BigInt`
- [ ] Large integer IDs kept as strings/`BigInt`
- [ ] Errors wrapped with `{ cause }`; error codes compared, not messages
- [ ] Every emitter/stream/socket has an `'error'` listener; streams composed with `pipeline`
- [ ] ESM files use `fileURLToPath(import.meta.url)`, full relative specifiers with extensions; no `require()` of ESM-only packages
- [ ] Top-level `await` does not hang on unref'd handles (exit code 13)

##### HTTP and Networking

- [ ] `fetch` calls check `res.ok`, consume/cancel bodies, set `AbortSignal.timeout()`, log `err.cause`
- [ ] Proxy needs handled explicitly (Node 18 `fetch` ignores `HTTP(S)_PROXY`)
- [ ] Outbound `http.request` clients use a shared keep-alive `Agent`
- [ ] Server `keepAliveTimeout` > load balancer idle timeout; `headersTimeout` > `keepAliveTimeout`; `requestTimeout` fits long requests
- [ ] Graceful shutdown calls `server.closeIdleConnections()` after `server.close()` (Node 18 does not close idle sockets)
- [ ] `localhost`/IPv6 ordering considered (`verbatim` DNS)

##### Security

- [ ] No `--openssl-legacy-provider`, `--security-revert`, `--insecure-http-parser`, `--openssl-shared-config` in production
- [ ] No `crypto.createCipher`, MD5/SHA-1 for security, PKCS#1 v1.5 encryption, `Math.random()` secrets; `timingSafeEqual` for secret comparison; AES-GCM with unique IVs; scrypt/argon2/bcrypt for passwords
- [ ] No shell interpolation in `exec`/`spawn`; `--` before user arguments; timeouts on child processes; Windows batch-file spawning reviewed (18.20.2 change)
- [ ] Path traversal checks use `path.resolve` + `path.relative` (not `startsWith`)
- [ ] Prototype pollution guarded (`Object.create(null)`, `Map`, key filtering, schema validation)
- [ ] `vm`/`--experimental-policy` not used as a sandbox
- [ ] Lockfile committed, `npm ci`, audit in CI

##### Performance and Operations

- [ ] No sync I/O or CPU-heavy work on request paths; worker pool sized with `os.availableParallelism()`
- [ ] `--max-old-space-size` set for containers
- [ ] Exact Docker tag, non-root user, signals reach Node
- [ ] Structured logging with request context via `AsyncLocalStorage`; `err.cause` serialized
- [ ] Tests run with `node --test` and an explicit reporter, or a runner version that supports Node 18

***

[^1]: Node.js 18 changelog (CHANGELOG_V18.md, versions 18.0.0–18.20.8). https://github.com/nodejs/node/blob/main/doc/changelogs/CHANGELOG_V18.md
[^2]: Node.js Release schedule (schedule.json: start, LTS, maintenance and end dates per line). https://github.com/nodejs/Release/blob/main/schedule.json
[^3]: Node.js distribution index (release dates and bundled versions, e.g. v18.20.8 2025-03-27, v26.12.1 2026-10-21). https://nodejs.org/dist/index.json
[^4]: Node.js 18 is now available! (release announcement, April 19, 2022). https://nodejs.org/en/blog/announcements/v18-release-announce
[^5]: Node.js 17.0.0 release notes (OpenSSL 3, `dns.lookup` verbatim, `structuredClone`, readline promises). https://nodejs.org/en/blog/release/v17.0.0
[^6]: Updates on CVE assignment for End-of-Life versions. https://nodejs.org/en/blog/vulnerability/updates-cve-for-end-of-life
[^7]: Node.js security releases published January 13, 2026 (CVE-2025-55131, CVE-2025-59465, CVE-2025-59466, CVE-2026-21637, …). https://nodejs.org/en/blog/vulnerability/december-2025-security-releases
[^8]: Node.js v18 documentation — Global objects (`fetch`, `structuredClone`, `CustomEvent`, Web Crypto flags). https://nodejs.org/docs/latest-v18.x/api/globals.html
[^9]: Node.js v18 documentation — Deprecated APIs (DEP0005, DEP0147, DEP0156–DEP0178). https://nodejs.org/docs/latest-v18.x/api/deprecations.html
[^10]: V8 — Finding elements in arrays: `findLast` and `findLastIndex`. https://v8.dev/features/finding-in-arrays
[^11]: Node.js v18 documentation — ECMAScript modules (import attributes, `import.meta.resolve`, CommonJS interop). https://nodejs.org/docs/latest-v18.x/api/esm.html
[^12]: Node.js 18.20.0 release notes (import attributes `with`, `dirent.parentPath`, vm compile cache). https://nodejs.org/en/blog/release/v18.20.0
[^13]: Node.js 18.19.0 release notes (`module.register`, `import.meta.resolve` unflagged, `--experimental-default-type`, npm 10, test runner backports). https://nodejs.org/en/blog/release/v18.19.0
[^14]: Node.js v18 documentation — `node:module` (customization hooks, `register`). https://nodejs.org/docs/latest-v18.x/api/module.html
[^15]: Node.js v18 documentation — Command-line options (`--watch`, `--import`, `--test-*`, `--openssl-legacy-provider`). https://nodejs.org/docs/latest-v18.x/api/cli.html
[^16]: Node.js v18 documentation — Test runner. https://nodejs.org/docs/latest-v18.x/api/test.html
[^17]: Node.js v18 documentation — HTTP (`server.requestTimeout`, `closeIdleConnections`, `writeEarlyHints`). https://nodejs.org/docs/latest-v18.x/api/http.html
[^18]: Node.js v18 documentation — Crypto. https://nodejs.org/docs/latest-v18.x/api/crypto.html
[^19]: Node.js February 2024 security releases (CVE-2023-46809 Marvin attack, PKCS#1 v1.5 padding). https://nodejs.org/en/blog/vulnerability/february-2024-security-releases
[^20]: Node.js April 2024 security releases (CVE-2024-27980, Windows batch-file command injection). https://nodejs.org/en/blog/vulnerability/april-2024-security-releases-2
[^21]: Node.js current documentation (introduction versions of post-18 APIs: `require(esm)`, `fs.glob`, `util.styleText`, `URL.parse`, `node:sqlite`, …). https://nodejs.org/docs/latest/api/
[^22]: TC39 finished proposals (ES2023–ES2026 features: change-array-by-copy, array grouping, Set methods, iterator helpers, explicit resource management, …). https://github.com/tc39/proposals/blob/main/finished-proposals.md
[^23]: Node.js 19 changelog (CHANGELOG_V19.md). https://github.com/nodejs/node/blob/main/doc/changelogs/CHANGELOG_V19.md
[^24]: OpenSSL release strategy (3.0 listed as no longer supported). https://openssl-library.org/policies/releasestrat/
[^25]: @tsconfig/node18 (target es2022, lib es2023, module node16). https://github.com/tsconfig/bases/blob/main/bases/node18.json
[^26]: npm registry metadata (`engines` of eslint, mocha, vitest, jest, undici, npm). https://registry.npmjs.org/
[^27]: Node.js v18 documentation — File system (`readdir` recursive, `Dirent.parentPath`, `statfs`, `fs.watch`). https://nodejs.org/docs/latest-v18.x/api/fs.html
[^28]: Node.js v18 documentation — Stream. https://nodejs.org/docs/latest-v18.x/api/stream.html
[^29]: Node.js v18 documentation — Child process. https://nodejs.org/docs/latest-v18.x/api/child_process.html
[^30]: Node.js v18 documentation — DNS (`verbatim`, `setDefaultResultOrder`, `getDefaultResultOrder`). https://nodejs.org/docs/latest-v18.x/api/dns.html
[^31]: Node.js v18 documentation — Net (`autoSelectFamily`). https://nodejs.org/docs/latest-v18.x/api/net.html
[^32]: Node.js January 2025 security releases (CVE-2025-22150 undici `fetch`, CVE-2025-23084, CVE-2025-23085). https://nodejs.org/en/blog/vulnerability/january-2025-security-releases
[^33]: Node.js July 2022 security releases (CVE-2022-32223, `nodejs_conf` OpenSSL configuration). https://nodejs.org/en/blog/vulnerability/july-2022-security-releases
