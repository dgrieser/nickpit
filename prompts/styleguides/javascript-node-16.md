### JavaScript on Legacy Node.js (16 and Older: 14, 15, 17) — Production Review Guide

> **Version**: Node.js 16 "Gallium" — 16.0.0 released 2021-04-20; final release **16.20.2 (2023-08-08)**; **end-of-life since 2023-09-11**.[^1][^2][^3]
> **Also covers**: Node.js 14 "Fermium" — 14.0.0 released 2020-04-21; final release **14.21.3 (2023-02-16)**; **end-of-life since 2023-04-30**. Node.js 15 (final 15.14.0, EOL 2021-06-01) and Node.js 17 (final 17.9.1, EOL 2022-06-01) were odd-numbered, never-LTS lines.[^1][^3]
> **Status as of October 2026**: every line covered here is **unsupported and receives no security fixes**. Supported lines are Node.js 22 (Maintenance LTS, EOL 2027-04-30), Node.js 24 (LTS, Maintenance since 2026-10-20, EOL 2028-04-30) and Node.js 26 (Current, latest 26.12.1 of 2026-10-21; scheduled to become LTS on 2026-10-28). Node.js 20 reached EOL on 2026-04-30 and 18 on 2025-04-30. Node.js 27 exists only as a planned alpha channel (scheduled to start 2026-10-28; first stable 27.0.0 planned for 2027-04-22).[^1][^3]
> **Scope**: JavaScript (CommonJS and ESM) running on Node.js 16, and — because the same rules apply to any project pinned **below Node 18** — on 14, 15 and 17. Every feature that needs 16 is marked **(Node 16+; not in 14)**, and where a 16.x *minor* matters it is named (for example **(16.9+)**). Features introduced in 17 or 18+ are listed as **not available**.
> **Note**: This document is self-contained. Claims about availability were executed on Node.js 14.21.3, 15.14.0, 16.20.2, 17.9.1 and 18.20.8 (the final release of each line); minor-version boundaries inside 16.x and 14.x come from the official changelogs and the per-release V8 versions in the Node.js release index.[^3][^4][^5]

***

#### Table of Contents

1. [Support status and upgrade advice](#support-status-and-upgrade-advice)
2. [Version-aware review](#version-aware-review)
3. [What's new in Node.js 16 (vs. 14 and 15)](#whats-new-in-nodejs-16-vs-14-and-15)
4. [Feature availability matrix](#feature-availability-matrix)
5. [Do not use or recommend](#do-not-use-or-recommend)
6. [Language: the ES2021/ES2022 subset you actually have](#language-the-es2021es2022-subset-you-actually-have)
7. [Code style house rules](#code-style-house-rules)
8. [Modules: CommonJS, ESM and interop](#modules-commonjs-esm-and-interop)
9. [Async programming](#async-programming)
10. [Errors](#errors)
11. [Streams, files and buffers](#streams-files-and-buffers)
12. [HTTP and networking without fetch](#http-and-networking-without-fetch)
13. [Child processes and worker threads](#child-processes-and-worker-threads)
14. [Security](#security)
15. [Performance](#performance)
16. [Testing](#testing)
17. [Tooling, packaging and TypeScript](#tooling-packaging-and-typescript)
18. [Observability and diagnostics](#observability-and-diagnostics)
19. [Removed / changed — migration notes](#removed--changed--migration-notes)
20. [Production / review checklist](#production--review-checklist)

***

#### Support status and upgrade advice

Node.js 16 was originally scheduled to be supported until April 2024. Because it ships **OpenSSL 1.1.1**, whose upstream support ended on 2023-09-11, the Node.js project moved Node 16's end-of-life forward by seven months to **2023-09-11**.[^2] The last release, 16.20.2 (2023-08-08), was a security release; no 16.x release has been made since.[^3][^4] Node.js 14 ended on 2023-04-30 with 14.21.3 as its last release.[^1][^3]

Consequences a reviewer must keep in mind:

- **Known vulnerabilities stay unfixed.** For example, the October 2023 security release fixed HTTP/2 "Rapid Reset" (CVE-2023-44487, High) and an undici cookie leak only on the then-active lines 18.x and 20.x — Node 16 never received those fixes.[^6] Every later Node.js security release likewise excludes 14/15/16/17. The Node.js project states it does not assess security reports for EOL release lines.[^7]
- **OpenSSL 1.1.1 is itself EOL** (Node 14 and 16 both bundle it: 14.21.3 ships 1.1.1t, 16.20.2 ships 1.1.1v+quic).
- **The ecosystem has moved on.** Current majors of most tools require Node 18+ (see [Tooling](#tooling-packaging-and-typescript)), so dependency updates — including security updates of dependencies — become progressively impossible.

**Review stance.** Recommend an upgrade plan (to Node 22 or 24 LTS) once per review, in the summary, not on every line. Then review the code *as code for its pinned version*: do not flag the absence of APIs the runtime does not have, and do flag any use of APIs it lacks.

***

#### Version-aware review

##### Determine which Node.js version applies

Check, in this order: `engines.node` in `package.json`; `.nvmrc` / `.node-version` / `.tool-versions`; `FROM node:16…` / `node:14…` in Dockerfiles; `node-version:` in CI workflows; `@types/node` major; `@tsconfig/node14`/`node16` bases; `volta.node` in `package.json`. If they disagree, the **lowest** version that the project claims to run on is binding for library code; for an application, the deployed image wins.

##### Minors matter inside 16.x

Node 16 shipped seven V8 updates and many backports. A project that declares `"engines": { "node": ">=16" }` promises to work on **16.0.0**, which lacks `Array.prototype.at` (16.6), `Object.hasOwn` and `error.cause` (16.9), class static blocks (16.11), `AbortSignal.timeout()` (16.14), JSON module imports without a flag (16.15), `node:test` and `util.parseArgs()` (16.17). Raise this as a finding when such APIs are used and the declared range starts lower; the fix is `"engines": { "node": ">=16.17" }` (or whatever the code needs) or avoiding the API.

| 16.x release | Date | V8 | Notable for reviewers |
|---|---|---|---|
| 16.0.0 | 2021-04-20 | 9.0 | RegExp `/d` flag; `atob`/`btoa`/`performance` globals; `timers/promises` stable; npm 7; `require('node:…')`; `fs.rmdir(recursive)` runtime-deprecated |
| 16.4.0 | 2021-06-23 | 9.1 | `#priv in obj` brand checks; `--dns-result-order` |
| 16.5.0 | 2021-07-14 | 9.1 | `stream/web` (experimental Web Streams, not global) |
| 16.6.0 | 2021-07-29 | 9.2 | `Array.prototype.at`, `String.prototype.at`, `TypedArray.prototype.at` |
| 16.7.0 | 2021-08-18 | 9.2 | `fs.cp()` / `fsPromises.cp()` (experimental); `stream/consumers` |
| 16.9.0 | 2021-09-07 | 9.3 | `Object.hasOwn`, `Error` `cause` option; Corepack (experimental) |
| 16.11.0 | 2021-10-08 | 9.4 | class `static { }` blocks; npm 8 |
| 16.13.0 | 2021-10-26 | 9.4 | becomes LTS "Gallium" |
| 16.14.0 | 2022-02-08 | 9.4 | `AbortSignal.timeout()`, `signal.reason`, `Readable#map/filter`, import assertions (`assert { type: 'json' }`) |
| 16.15.0 | 2022-04-26 | 9.4 | `--experimental-fetch` flag; JSON modules unflagged (still warns); `Readable#toArray/forEach/reduce` |
| 16.17.0 | 2022-08-16 | 9.4 | `node:test` (experimental), `util.parseArgs()` (experimental), `signal.throwIfAborted()`, `module.isBuiltin()` |
| 16.18.0 | 2022-10-12 | 9.4 | `node:test` `before/after/beforeEach/afterEach` hooks |
| 16.19.0 | 2022-12-13 | 9.4 | `--watch` (experimental) |
| 16.20.2 | 2023-08-08 | 9.4 | final release (security); npm 8.19.4 |

Dates and V8 versions are from the Node.js release index; features from the 16.x changelog.[^3][^4]

##### Rules for claiming something is unavailable

- Say "not available in Node 16" only when the API is absent in **16.20.2** (the final 16.x). Say "needs 16.x or later" when the declared `engines` range starts below the minor that introduced it.
- **17.x is not a superset of late 16.x.** Several features were backported to 16.17+ but are *absent from 17.9.1*: `node:test`, `util.parseArgs()`, `module.isBuiltin()`, `os.machine()`, `diagnostics_channel.subscribe()`, `v8.startupSnapshot`, `http.setMaxIdleHTTPParsers()` (all verified absent on 17.9.1, present on 16.20.2). Likewise **14.21.3 has** `require('node:fs')`, `os.devNull`, `util.getSystemErrorMap()`, `util.toUSVString()`, `--dns-result-order` and `--no-addons` (backports in 14.18/14.19) that **15.14.0 lacks** (verified).
- Check whether a feature is only present behind a flag (`--experimental-fetch`, `--experimental-global-webcrypto`, `--experimental-abortcontroller` on 14) before assuming application code can use it. Flags in production start scripts are a review finding of their own.
- Third-party code may polyfill (e.g. `abort-controller`, `node-fetch`, `@ungap/structured-clone`). A call is fine when the polyfill is imported explicitly; do not assume a global exists because a type definition says so.

***

#### What's new in Node.js 16 (vs. 14 and 15)

Node.js 15 (October 2020) and 16 (April 2021) together brought the following over Node 14. Items marked **(Node 15+)** are therefore also missing in Node 14; a Node 14 project must not use any of them.

##### JavaScript (V8 8.4 → 9.4)

| Feature | Available from | Not in |
|---|---|---|
| Logical assignment `a ||= b`, `a &&= b`, `a ??= b` | Node 15.0 (V8 8.5) | 14 — **SyntaxError** |
| `String.prototype.replaceAll` | Node 15.0 | 14 |
| `Promise.any`, `AggregateError` | Node 15.0 | 14 |
| RegExp match indices (`/d`, `.indices`) | Node 16.0 (V8 9.0) | 14, 15 — **SyntaxError** |
| `Intl.Segmenter` | Node 16.0 | 14, 15 |
| `Atomics.waitAsync` | Node 16.0 | 14, 15 |
| `#field in obj` (private brand checks) | Node 16.4 (V8 9.1) | 14, 15 — **SyntaxError** |
| `.at()` on Array / String / TypedArray | Node 16.6 (V8 9.2) | 14, 15 |
| `Object.hasOwn` | Node 16.9 (V8 9.3) | 14, 15 |
| `new Error(msg, { cause })` | Node 16.9 (V8 9.3) | 14, 15 — option silently ignored |
| Class `static { … }` initialization blocks | Node 16.11 (V8 9.4) | 14, 15 — **SyntaxError** |

Already in **14** (safe for both): optional chaining `?.`, nullish coalescing `??`, `Promise.allSettled`, `String.prototype.matchAll`, `BigInt`, `globalThis`, `Object.fromEntries`, `Array.prototype.flat/flatMap`, numeric separators `1_000`, public and private class fields and private methods (`#x`, `#m()`), `static` class fields, `WeakRef` / `FinalizationRegistry`, `Intl.DisplayNames`/`Intl.ListFormat`, hashbang lines, stable `Array.prototype.sort`. All were verified on 14.21.3.[^8]

##### Node.js APIs

| Feature | Available from | Notes |
|---|---|---|
| Unhandled promise rejections crash the process (`--unhandled-rejections=throw` default) | **Node 15.0** | Node 14 only warns and keeps running |
| npm 7+ (installs peer dependencies, lockfile v2) | Node 15.0 (npm 7), 16.11 (npm 8) | Node 14 ships npm 6 (lockfile v1) |
| `AbortController`, `AbortSignal`, `EventTarget`, `Event`, `MessageChannel` globals | Node 15.0 | 14.17+: `AbortController` only behind `--experimental-abortcontroller` |
| `timers/promises` (`setTimeout`, `setImmediate`; `setInterval` 15.9) | Node 15.0 (stable in 16.0) | not in 14 |
| `stream/promises` (`pipeline`, `finished`) | Node 15.0 | 14: use `util.promisify(stream.pipeline)` |
| `dns/promises`, `assert/strict`, `path/posix`, `path/win32`, `util/types` module specifiers | Node 15.0 | 14: `require('dns').promises`, `require('assert').strict`, … |
| `crypto.webcrypto` (experimental), `crypto.hkdf`, `crypto.generatePrime`, `X509Certificate` | Node 15.0–15.8 | not in 14 |
| `crypto.randomUUID()` | Node 15.6, **backported to 14.17** | |
| `require('node:fs')` (the `node:` scheme in `require`) | Node 16.0, **backported to 14.18** | **not in 15.14** |
| `atob()`, `btoa()` globals (legacy; prefer `Buffer`) | Node 16.0 | not global on 14/15 (`require('buffer').atob` exists on 15.13+ and on 14.21.3, also legacy) |
| `performance` global | Node 16.0 | 14: `require('perf_hooks').performance` |
| `fs.cp()` / `fsPromises.cp()` (experimental) | Node 16.7 | |
| `stream/consumers`, `stream/web` (experimental) | Node 16.7, 16.5 | |
| `AbortSignal.timeout()`, `signal.reason` | Node 16.14 | |
| `Readable#map/filter/forEach/toArray/some/every/find/reduce/flatMap/drop/take` (experimental) | Node 16.14–16.17 | |
| `signal.throwIfAborted()` | Node 16.17 | |
| `node:test`, `util.parseArgs()` (experimental) | Node 16.17 | **not in 17.x**, not in 14/15 |
| `--watch` (experimental) | Node 16.19 | not in 17.x |

Sources: Node.js 15/16 changelogs and the `added:` metadata of the 16.20.2 API docs.[^4][^9][^10]

##### Platform

- **V8 9.4** in 16.11+ (9.0 at 16.0); Node 14 uses V8 8.4; Node 17 uses V8 9.5/9.6; Node 18 uses V8 10.x.[^3]
- **OpenSSL 1.1.1** in both 14 and 16 (17 switched to OpenSSL 3.0).[^11]
- **ICU** full data is built in (`Intl` works for all locales); 16.20.2 ships ICU 71.1 / Unicode 14.
- Node 16 is the first line with prebuilt Apple Silicon (arm64 macOS) binaries.[^4]

***

#### Feature availability matrix

Executed with `typeof X` / feature probes on the final release of each line. "flag" means available only behind a CLI flag.

| API / syntax | 14.21.3 | 15.14.0 | 16.20.2 | 17.9.1 | 18.20.8 |
|---|---|---|---|---|---|
| `a ??= b` | ✗ SyntaxError | ✓ | ✓ | ✓ | ✓ |
| `[].at()`, `Object.hasOwn`, `error.cause` | ✗ | ✗ | ✓ (16.6/16.9) | ✓ | ✓ |
| `static { }` blocks, `#x in o`, `/d` flag | ✗ | ✗ | ✓ | ✓ | ✓ |
| `[].findLast()`, `findLastIndex()` | ✗ | ✗ | ✗ | ✗ | ✓ |
| `structuredClone()` | ✗ | ✗ | ✗ | ✓ | ✓ |
| `fetch`, `Headers`, `Request`, `Response`, `FormData` | ✗ | ✗ | flag | flag | ✓ (experimental) |
| `Blob` global | ✗ | ✗ | ✗ | ✗ | ✓ |
| `ReadableStream` etc. as globals | ✗ | ✗ | ✗ | ✗ | ✓ |
| `AbortController` global | flag | ✓ | ✓ | ✓ | ✓ |
| `AbortSignal.timeout()` | ✗ | ✗ | ✓ | ✓ | ✓ |
| `AbortSignal.any()` | ✗ | ✗ | ✗ | ✗ | ✓ (18.17) |
| `crypto` global (Web Crypto) | ✗ | ✗ | flag | flag | ✗ (flag; global from 19) |
| `crypto.getRandomValues()` on the `crypto` module | ✗ | ✗ | ✗ (`crypto.webcrypto.getRandomValues`) | ✓ | ✓ |
| `DOMException` global | ✗ | ✗ | ✗ | ✓ | ✓ |
| `BroadcastChannel` global | ✗ | ✗ | ✗ (`worker_threads`) | ✗ | ✓ |
| `atob`/`btoa`/`performance` globals | ✗ | ✗ | ✓ | ✓ | ✓ |
| `require('timers/promises')`, `require('stream/promises')` | ✗ | ✓ | ✓ | ✓ | ✓ |
| `require('readline/promises')` | ✗ | ✗ | ✗ | ✓ | ✓ |
| `require('node:test')` | ✗ | ✗ | ✓ (16.17) | ✗ | ✓ |
| `util.parseArgs` | ✗ | ✗ | ✓ (16.17) | ✗ | ✓ (18.3) |
| `require('node:fs')` | ✓ (14.18) | ✗ | ✓ | ✓ | ✓ |
| `crypto.randomUUID()` | ✓ (14.17) | ✓ | ✓ | ✓ | ✓ |
| `os.availableParallelism()` | ✗ | ✗ | ✗ | ✗ | ✓ (18.14) |
| `fs.statfs()` | ✗ | ✗ | ✗ | ✗ | ✓ |
| `server.closeAllConnections()` | ✗ | ✗ | ✗ | ✗ | ✓ (18.2) |
| `import … with { type: 'json' }` | ✗ | ✗ | ✗ SyntaxError | ✗ | ✓ (18.20) |
| `import … assert { type: 'json' }` | ✗ | ✗ | ✓ (16.14) | ✓ | ✓ (deprecated warning) |
| `require()` of an ES module | ✗ `ERR_REQUIRE_ESM` | ✗ | ✗ | ✗ | ✗ |
| md4 hash (`createHash('md4')`) | ✓ | ✓ | ✓ | ✗ OpenSSL 3 | ✗ |

***

#### Do not use or recommend

The reviewer must never suggest these as fixes for Node 16 code (and a fortiori not for 14). If the code uses one without a polyfill, it is a **bug**: it throws `TypeError: … is not a function` / `ReferenceError` / `SyntaxError` at runtime.

##### Language features from newer V8 (not in 16.20.2)

- `Array.prototype.findLast()` / `findLastIndex()` (Node 18, ES2023) — use a reverse loop.
- `Array.prototype.toSorted()` / `toReversed()` / `toSpliced()` / `with()` (Node 20) — use `[...arr].sort()`.
- `Object.groupBy()` / `Map.groupBy()` (Node 21) — use `reduce`.
- `Promise.withResolvers()` (Node 22) — create the executor manually.
- `Array.fromAsync()` (Node 22); Iterator helpers (`Iterator.from`, `iter.map`) (Node 22); `Set.prototype.union/intersection/…` (Node 22).
- RegExp `v` flag (Node 20), `String.prototype.isWellFormed()/toWellFormed()` (Node 20), resizable `ArrayBuffer` and `ArrayBuffer.prototype.transfer()` (Node 20/21).
- `using` / `await using` declarations (Node 24). `Symbol.dispose`/`Symbol.asyncDispose` exist as Node-provided symbols on late 18.x and 20+, but not on 16 (verified `undefined` on 16.20.2).
- `Promise.try()` (Node 23), `Error.isError()`, `RegExp.escape()`, `Float16Array` (Node 24).
- `import x from './a.json' with { type: 'json' }` — import attributes (Node 18.20 / 20.10+). Node 16.14+ only understands the older `assert { type: 'json' }` form, which newer Node removes (22+).

##### Globals and APIs from Node 17+

- `structuredClone()` (17.0) — use `v8.deserialize(v8.serialize(x))` (see [Language](#language-the-es2021es2022-subset-you-actually-have)).
- Global `fetch`, `Headers`, `Request`, `Response`, `FormData` (18.0, stable 21) — use `undici@5`, `node-fetch@2` or `http`/`https`. Do not recommend `--experimental-fetch` for production on 16.
- Global `Blob` (18), `File` (20), `ReadableStream`/`WritableStream`/`TextDecoderStream`/`CompressionStream` globals (18), `BroadcastChannel` global (18), `DOMException` global (17), `navigator` (21), `WebSocket` (22), `CustomEvent` global (19; flag in 16.17+).
- Global `crypto` (Web Crypto) (19; flag in 16.15+/17.6+), `crypto.getRandomValues()` on the `crypto` module (17.4), `crypto.hash()` (21.7/20.12).
- `AbortSignal.any()` (20.3 / 18.17), `events.addAbortListener()` (20.5 / 18.18), `util.aborted()` (19.7 / 18.16).
- `readline/promises` (17.0), `util.styleText()` (21.7 / 20.12), `util.parseEnv()` (21.7), `util.MIMEType` (19.1 / 18.13).
- `process.loadEnvFile()` and `--env-file` (20.6 / 21.7) — use the `dotenv` package.
- `os.availableParallelism()` (18.14) — use `os.cpus().length`.
- `fs.glob()` (22), `fs.openAsBlob()` (19.8), `fs.statfs()` (18.15/19.6), `fsPromises.readdir(…, { recursive: true })` (20.1), `filehandle.readLines()` (18.11).
- `http.Server#closeAllConnections()` / `closeIdleConnections()` (18.2) — track sockets yourself (see [HTTP](#http-and-networking-without-fetch)).
- `stream.duplexPair()` (22.6), `Buffer.copyBytesFrom()` (19.8), `buffer.isUtf8()` (19.4), `buffer.File` (19.2).
- `AsyncLocalStorage.snapshot()`/`bind()` (19.8 / 18.16), `diagnostics_channel.tracingChannel()` (19.9), `module.register()` (20.6), `--import` (19.0 / 18.18).
- `node:sqlite` (22.5), `node:sea` (single executable apps, 19.7+/20), WASI without a flag (20), the permission model `--experimental-permission`/`--permission` (20 / 22), `--experimental-strip-types` (22.6), `node --run` (22), `--experimental-require-module` / `require(esm)` (20.17 flag, 22.12/20.19 default).
- `node:test` features after the 16.17 backport: `mock` / `t.mock` (18.13 / 19.1), `--test-reporter` (18.15 / 19.6), `--test-name-pattern` (18.11), `--experimental-test-coverage` (18.15 / 19.7), `mock.timers` (20.4). Verified: `test.mock` and `t.mock` are `undefined` on 16.20.2.

##### Old APIs that are deprecated in 16

See [Removed / changed](#removed--changed--migration-notes) for the full list: `new Buffer()` (runtime DEP0005), `fs.rmdir(path, { recursive: true })` (runtime DEP0147), `crypto.createCipher`/`createDecipher` (runtime DEP0106), `require('sys')` (runtime DEP0025), `OutgoingMessage#_headers` (runtime DEP0066), `exports` folder mappings `"./": "./"` (runtime DEP0148), `process.binding()` (doc-only DEP0111, several bindings runtime-deprecated in 16.0), `punycode` (doc-only DEP0040), `util.isArray()` and other `util.is*()` (doc-only), `buffer.slice()` (doc-only DEP0158 since 16.15; prefer `subarray()`).[^9]

***

#### Language: the ES2021/ES2022 subset you actually have

Node 16.20.2 implements ES2021 completely and most of ES2022 (class fields, private methods, `static` blocks, `#x in obj`, `.at()`, `Object.hasOwn`, `error.cause`, RegExp `/d`, top-level `await` in ES modules). It does **not** implement ES2023+ (no `findLast`, no change-array-by-copy methods). Node 14 is roughly ES2020 plus class fields and private methods.

##### Variables, equality and nullish handling

```javascript
// Prefer const, then let; never var.
const config = { retries: 0, timeoutMs: undefined, name: '' };

// ?? only replaces null/undefined; || also replaces 0, '' and false.
const retriesBad = config.retries || 3;   // ❌ 3 — a configured 0 is lost
const retriesGood = config.retries ?? 3;  // ✅ 0
console.log(retriesBad, retriesGood);     // 3 0

// Optional chaining short-circuits the whole chain.
const user = { profile: null };
console.log(user.profile?.address?.city);  // undefined
console.log(user.getName?.());             // undefined (method missing)

// == null is the one sanctioned loose comparison (null or undefined).
console.log(config.timeoutMs == null);     // true
```

##### Logical assignment (Node 15+; not in 14)

```javascript
const opts = { headers: null, cache: undefined, retries: 0 };
opts.headers ??= {};         // assigns because null
opts.retries ||= 3;          // ❌ overwrites a deliberate 0
opts.cache &&= opts.cache.trim();
console.log(opts);           // { headers: {}, cache: undefined, retries: 3 }
```

In code that must also run on Node 14, `a ??= b` is a **SyntaxError that prevents the whole file from loading**, not a runtime error in one branch. Write `if (a == null) a = b;` instead. TypeScript/Babel only down-level it when the compile target is ES2020 or lower (verified: `tsc --target es2021` emits `??=` verbatim, which Node 14 rejects).

##### `.at()`, `Object.hasOwn`, `error.cause` (Node 16.6/16.9+; not in 14)

```javascript
const items = [10, 20, 30];
console.log(items.at(-1));                 // 30
console.log(items[items.length - 1]);      // 30 — the portable form for 14 / <16.6

const dict = Object.create(null);          // no prototype: no hasOwnProperty method
dict.key = 1;
console.log(Object.hasOwn(dict, 'key'));                          // true (16.9+)
console.log(Object.prototype.hasOwnProperty.call(dict, 'key'));   // true (portable)
// dict.hasOwnProperty('key') would throw TypeError: not a function
```

##### Classes: fields, private members, static blocks

```javascript
class Connection {
  static #instances = 0;           // private static field (14+)
  static defaultPort = 5432;       // public static field (14+)
  #socket = null;                  // private instance field (14+)
  state = 'idle';                  // public instance field (14+)

  static {                         // static initialization block (16.11+; SyntaxError on 14/15)
    Connection.registry = new Map();
  }

  constructor(name) {
    this.name = name;
    Connection.#instances += 1;
    Connection.registry.set(name, this);
  }

  #assertOpen() {                  // private method (14+)
    if (this.state !== 'open') throw new Error(`${this.name} is not open`);
  }

  static isConnection(obj) {
    return #socket in obj;         // private brand check (16.4+; SyntaxError on 14/15)
  }

  static get count() {
    return Connection.#instances;
  }
}

const c = new Connection('primary');
console.log(Connection.count, Connection.isConnection(c), Connection.isConnection({})); // 1 true false
```

Class fields use **define semantics**: a field declared in a subclass shadows an accessor of the base class, and a field initializer runs *after* `super()` returns. Flag code that reads a subclass field from the base constructor:

```javascript
class Base {
  constructor() { this.describe(); }
  describe() { console.log('kind =', this.kind); }
}
class Child extends Base {
  kind = 'child';                  // assigned after Base's constructor ran
}
new Child();                       // kind = undefined
```

##### Cloning without `structuredClone` (Node 17+)

`structuredClone()` does not exist in 16. Options, in order of preference:

```javascript
const v8 = require('v8');

const original = { when: new Date(0), tags: new Set(['a']), byId: new Map([[1, { n: 1 }]]), big: 10n };

// 1. v8.serialize/deserialize: structured clone algorithm (Date, Map, Set, BigInt, typed arrays, RegExp).
const copy = v8.deserialize(v8.serialize(original));
console.log(copy.when instanceof Date, copy.byId.get(1).n, copy.byId !== original.byId); // true 1 true

// 2. JSON round-trip: loses Date (becomes string), Map/Set (become {}), undefined, and throws on BigInt.
try {
  JSON.parse(JSON.stringify(original));
} catch (e) {
  console.log(e.message);          // Do not know how to serialize a BigInt
}

// Functions and class instances' prototypes are not cloneable by either method.
try {
  v8.serialize({ fn() {} });
} catch (e) {
  console.log(e.message);          // fn() {} could not be cloned.
}
```

`v8.serialize()` drops prototypes (class instances come back as plain objects), just like `structuredClone`.

##### Strings, numbers and dates

```javascript
// replaceAll (15+): a RegExp argument must have the g flag.
console.log('a.b.c'.replaceAll('.', '/'));          // a/b/c
try {
  'aa'.replaceAll(/a/, 'b');
} catch (e) {
  console.log(e.constructor.name);                  // TypeError
}
// Portable for 14: split/join or a global regex with replace().
console.log('a.b.c'.split('.').join('/'));          // a/b/c

// Length counts UTF-16 code units, not characters.
console.log('👍'.length, [...'👍'].length);         // 2 1
// Grapheme clusters need Intl.Segmenter (16+; not in 14).
const seg = new Intl.Segmenter('en', { granularity: 'grapheme' });
console.log([...seg.segment('👨‍👩‍👧x')].length);  // 2

// Money: never binary floats.
console.log(0.1 + 0.2 === 0.3);                    // false
const cents = 10n + 20n;                            // BigInt or integer cents
console.log(cents);                                 // 30n

// Date parsing: date-only ISO strings are UTC, date-time strings without offset are local.
console.log(new Date('2024-03-10').toISOString());  // 2024-03-10T00:00:00.000Z
```

##### Iteration and collections

```javascript
const users = [
  { id: 1, name: 'Ann', active: true },
  { id: 2, name: 'Bob', active: false },
];
// Map/Set for lookups instead of repeated Array#find in loops.
const byId = new Map(users.map((u) => [u.id, u]));
console.log(byId.get(2).name);                       // Bob

// No Object.groupBy (21+): use reduce.
const groups = users.reduce((acc, u) => {
  const key = u.active ? 'active' : 'inactive';
  (acc[key] ||= []).push(u.name);                    // ||= needs 15+; write acc[key] = acc[key] || [] on 14
  return acc;
}, {});
console.log(groups);                                 // { active: [ 'Ann' ], inactive: [ 'Bob' ] }

// No findLast (18+): iterate backwards.
function findLast(arr, pred) {
  for (let i = arr.length - 1; i >= 0; i -= 1) if (pred(arr[i])) return arr[i];
  return undefined;
}
console.log(findLast([1, 2, 3, 4], (n) => n % 2 === 1)); // 3

// No toSorted (20+): copy first — sort() mutates in place.
const nums = [3, 1, 2];
const sorted = [...nums].sort((a, b) => a - b);
console.log(nums, sorted);                           // [ 3, 1, 2 ] [ 1, 2, 3 ]
// Default sort is lexicographic, a classic bug:
console.log([10, 9, 1].sort());                      // [ 1, 10, 9 ]
```

Mutating an array while iterating it, `for…in` over arrays (iterates inherited enumerable keys and yields strings), and `forEach` with an `async` callback (the promises are never awaited) are review findings in any version.

***

#### Code style house rules

These rules from the general JavaScript guide remain correct for 14/16:

- `const` by default, `let` when reassigned, never `var`.
- Strict equality (`===`, `!==`) everywhere; the only exception is `x == null`.
- Template literals over concatenation; destructuring and default parameters; object shorthand.
- Arrow functions for callbacks; regular functions or methods when `this` binding matters.
- Do not mutate function arguments; return new objects (`{ ...options, processed: true }`).
- Map/Set for frequent lookups; avoid creating closures in hot loops only when profiling shows it matters.
- `async`/`await` for sequential logic; `Promise.all` for independent work; `.then()` for single-step transforms and combinators.
- Throw `Error` subclasses, never strings or plain objects.
- Use `'use strict'` in CommonJS files that are not compiled from TypeScript/ESM (ES modules and class bodies are always strict).

```javascript
'use strict';

function createUser({ name, email, role = 'user' } = {}) {
  if (!name || !email) throw new TypeError('name and email are required');
  return { name, email, role, createdAt: new Date() };
}

function withDefaults(options) {
  return { retries: 3, timeoutMs: 5000, ...options };   // later spread wins
}

console.log(createUser({ name: 'Ann', email: 'a@example.com' }).role, withDefaults({ retries: 0 }).retries); // user 0
```

***

#### Modules: CommonJS, ESM and interop

##### Module type decision

A file is an ES module if it ends in `.mjs`, or ends in `.js` and the nearest `package.json` has `"type": "module"`. A file is CommonJS if it ends in `.cjs`, or `.js` without `"type": "module"` (or with `"type": "commonjs"`). The ESM implementation was marked stable in 15.3 (and 12.22); the 16 docs mark ECMAScript modules "Stability: 2 – Stable". CommonJS named-export detection exists since 14.13, unflagged top-level `await` since 14.8.[^12]

##### No `require()` of ES modules in 14/15/16/17

`require()` of an ES module throws **`ERR_REQUIRE_ESM`** on every version covered here (verified on 14.21.3 through 18.20.8). Synchronous `require(esm)` only arrived in Node 22.12 / 20.19 — do not recommend it. Use dynamic `import()` from CommonJS (works in 14 and 16), which is asynchronous:

```javascript
// fragment: CommonJS consuming an ESM-only package (e.g. node-fetch@3, chalk@5, execa@6, nanoid@4)
// const chalk = require('chalk');            // ❌ ERR_REQUIRE_ESM
async function main() {
  const { default: chalk } = await import('chalk'); // ✅ dynamic import works in CJS
  console.log(chalk.green('ok'));
}
```

Many popular packages went ESM-only in majors that *still support Node 14/16* (`node-fetch@3`, `chalk@5`, `execa@6`, `got@12`, `nanoid@4`). Upgrading one of them in a CommonJS project is a breaking change; staying on the last CJS major (`node-fetch@2`, `chalk@4`, `execa@5`, `got@11`, `nanoid@3`) is the usual fix.[^13]

##### TypeScript and Babel can silently turn `import()` into `require()`

With `"module": "commonjs"`, TypeScript rewrites `await import('pkg')` into `Promise.resolve().then(() => require('pkg'))`, so the ESM-only package still fails with `ERR_REQUIRE_ESM`. With `"module": "node16"` (TS 4.7+) the `import()` is preserved in `.cts`/CommonJS output. Verified with TypeScript 5.9: the `commonjs` output fails on 16.20.2, the `node16` output loads the ESM package.

```json
{
  "compilerOptions": {
    "module": "node16",
    "moduleResolution": "node16",
    "target": "es2021"
  }
}
```

##### ESM rules that bite CommonJS developers

```javascript
// esm
// 1. Relative specifiers need the full file name: no extension search, no directory index.
//    import x from './util';       // ❌ ERR_MODULE_NOT_FOUND
//    import x from './util/index.js'; // ✅
// 2. No require, module, exports, __filename, __dirname.
import { fileURLToPath } from 'url';
import path from 'path';
import { createRequire } from 'module';

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);
const require = createRequire(import.meta.url);   // for JSON, optional deps, CJS-only tools

console.log(typeof __dirname, typeof require);    // string function
// 3. Top-level await works in ES modules (14.8+), not in CommonJS.
const value = await Promise.resolve(42);
console.log(value);                               // 42
```

`--experimental-specifier-resolution=node` restores extension search but is experimental and stops working in Node 19 (the flag is still accepted there but no longer resolves extensionless specifiers — verified on 19.9.0 and 20.20.2). Do not use it in production start scripts.

##### Importing JSON

- **CommonJS**: `require('./config.json')` (cached; mutations are shared across importers).
- **ESM on 16.14+ and 17.x**: `import data from './data.json' assert { type: 'json' };` — needs `--experimental-json-modules` on 16.14, works without a flag from 16.15, and always prints `ExperimentalWarning: Importing JSON modules is an experimental feature` on 16.20.2. The `assert` keyword is a SyntaxError on 14/15, is replaced by `with` in 18.20+/20.10+, and is a SyntaxError again on 22+ (verified on 22.23.4). Neither form is portable across this range.
- **ESM, portable**: `createRequire(import.meta.url)('./data.json')` or `JSON.parse(await readFile(new URL('./data.json', import.meta.url), 'utf8'))`.

##### Importing CommonJS from ESM: named exports are a static guess

`import cjs from './x.cjs'` always gives `module.exports` as the default export. Named imports work only for names that `cjs-module-lexer` detects statically.[^14] Verified on 14.21.3 and 16.20.2:

| CommonJS source | Detected named exports |
|---|---|
| `exports.x = 1; exports.y = function () {};` | `x`, `y` |
| `const a = 1, b = 2; module.exports = { a, b };` | `a`, `b` |
| `module.exports = { a, b: 2 };` | `a` only — detection stops at the first non-shorthand property |
| `module.exports = { b: 2, a };` | none |
| `Object.assign(module.exports, { c: 3 });` | none — `import { c }` is a **SyntaxError at link time** |
| `module.exports = class S {}` | none |
| `exports[computed] = …` | none |
| `Object.defineProperty(exports, '__esModule', …); exports.default = …` | `default` is still the *whole* `module.exports` object; the transpiled default is at `.default` |

```javascript
// esm
// fragment: safe pattern for CJS dependencies
import pkg from 'some-cjs-package';
const { helper } = pkg;   // destructure from the default export instead of a named import
```

##### `exports` and `imports` in `package.json`

```json
{
  "name": "@acme/lib",
  "version": "2.0.0",
  "type": "commonjs",
  "main": "./dist/index.js",
  "exports": {
    ".": {
      "import": "./dist/index.mjs",
      "require": "./dist/index.js"
    },
    "./feature/*": "./dist/feature/*.js",
    "./package.json": "./package.json"
  },
  "imports": {
    "#internal/*": "./src/internal/*.js"
  },
  "engines": { "node": ">=16.17" }
}
```

- `exports` **encapsulates**: any subpath not listed throws `ERR_PACKAGE_PATH_NOT_EXPORTED`, including `require('pkg/package.json')` unless exported. Adding `exports` to an existing package is a breaking change.
- Subpath patterns `"./feature/*"` need 14.13+/12.20+; pattern trailers (`"./features/*.js"`) need 16.9/14.19. Folder mappings `"./lib/": "./lib/"` are runtime-deprecated in 16 (DEP0148 warning, verified) and **fail in 17+** — flag them.[^9]
- `"imports"` (`#internal/*`) needs 14.6+.
- Conditional exports are matched in object order; put `"types"` first for TypeScript and `"default"` last.
- `main` is still used by Node versions without `exports` support and by bundlers; keep it.
- **Dual-package hazard**: shipping both ESM and CJS builds can load two copies of the package (two module instances, two singletons, `instanceof` failures across them). Prefer one implementation with a thin ESM wrapper re-exporting the CJS build.[^15]

##### `node:` scheme

`import fs from 'node:fs'` works from 14.13.1/12.20 in ESM. `require('node:fs')` works from **16.0 and 14.18** — it throws `Cannot find module 'node:fs'` on 15.x and on 14.0–14.17. `node:test` exists **only** with the prefix (`require('test')` looks for an npm package). In a library whose `engines` admits 14.0–14.17 or 15, `require('node:…')` is a bug.

##### Module loading pitfalls

- Circular `require()` returns a partially initialized `module.exports`; reassigning `module.exports` later breaks cycles. Flag cycles that read exports at load time.
- `require` cache is keyed by resolved path; symlinked duplicates (`npm link`) are distinct modules.
- `process.mainModule` (doc-deprecated since 14.0) and `module.parent` (doc-deprecated) — use `require.main === module` in CJS.
- ESM loader hooks (`--experimental-loader`) changed shape in 16.12 (`resolve`/`load`) and again in 16.17 (chaining); loaders written for 14 or early 16 (`getFormat`, `getSource`, `transformSource`) do not work on 16.12+.[^4]

***

#### Async programming

##### Unhandled promise rejections — the biggest 14 → 16 behaviour change

On **Node 15+** an unhandled rejection is raised as an uncaught exception and the process **exits with code 1**. On **Node 14** it only prints `UnhandledPromiseRejectionWarning` plus `DEP0018` and the process **keeps running** with exit code 0.[^16] Verified:

```javascript
// fragment: behaviour comparison — run with node 14 and node 16
Promise.reject(new Error('boom'));
setTimeout(() => console.log('still alive'), 50);
// node 14.21.3: warning, prints "still alive", exit code 0
// node 16.20.2: crash with the stack trace, exit code 1, no "still alive"
```

Review consequences:

- Code tested on 14 may hide fire-and-forget promise bugs that crash 16 in production. Every promise must be awaited, returned, or given a `.catch()`.
- For Node 14 services, run with `--unhandled-rejections=strict` (or `throw`) to get 16 semantics.
- A `process.on('unhandledRejection', …)` handler suppresses the crash; it must log **and** exit (or rethrow), never swallow.

```javascript
// BAD: fire-and-forget — a rejection crashes 16, is silently ignored on 14
async function audit(event) {
  if (!event.user) throw new Error('missing user');
}
function handle(event) {
  audit(event);                       // ❌ promise dropped
  return 'ok';
}

// GOOD: await it, or attach an explicit handler when fire-and-forget is intended
async function handleGood(event) {
  await audit(event);
  return 'ok';
}
function handleBackground(event) {
  audit(event).catch((err) => console.error('audit failed:', err.message)); // ✅ deliberate, handled
  return 'ok';
}

console.log(handleBackground({}));                         // ok (printed first)
handleGood({ user: 'ann' }).then(console.log);            // ok (printed after "audit failed: missing user")
void handle;
```

```javascript
// Last-resort handlers: log, then exit. Do not keep serving after unknown state corruption.
process.on('unhandledRejection', (reason) => {
  console.error('unhandledRejection', reason);
  process.exitCode = 1;
  throw reason;                          // escalate to uncaughtException (16 default behaviour)
});
process.on('uncaughtException', (err, origin) => {
  console.error('fatal', origin, err.message);
  process.exit(1);                       // flush logs first in real code
});
Promise.reject(new Error('demo'));        // prints both lines, exits 1
```

##### Sequential vs. parallel, and the combinators

```javascript
const sleep = (ms, v) => new Promise((resolve) => setTimeout(resolve, ms, v));

async function main() {
  // Independent work: start everything, then await together.
  const [a, b] = await Promise.all([sleep(20, 'a'), sleep(20, 'b')]);  // ~20 ms, not 40
  console.log(a, b);                                                    // a b

  // Need every outcome: allSettled (12.9+).
  const results = await Promise.allSettled([sleep(5, 1), Promise.reject(new Error('x'))]);
  console.log(results.map((r) => r.status));                            // [ 'fulfilled', 'rejected' ]

  // First success: Promise.any (15+; not in 14). Rejects with AggregateError if all fail.
  try {
    await Promise.any([Promise.reject(new Error('a')), Promise.reject(new Error('b'))]);
  } catch (e) {
    console.log(e.constructor.name, e.errors.map((x) => x.message));    // AggregateError [ 'a', 'b' ]
  }
}
main();
```

`Promise.all` rejects on the first failure but **does not cancel** the other operations; they keep running and their later rejections are handled by `Promise.all` (no unhandled-rejection crash). Pass an `AbortSignal` if the losers must stop.

##### Bounded concurrency

`Promise.all(items.map(fetchOne))` on 10 000 items opens 10 000 sockets/file handles. Use a small pool (or `p-limit@3`, the last CommonJS major):

```javascript
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

mapLimit([1, 2, 3, 4, 5], 2, async (n) => n * 2).then((r) => console.log(r)); // [ 2, 4, 6, 8, 10 ]
```

##### Cancellation and timeouts with `AbortController` (Node 15+; 14 only behind a flag)

`AbortController`/`AbortSignal` are globals in 15+ (14.17+ needs `--experimental-abortcontroller`). On 16, signals are accepted by `timers/promises`, `events.once/on`, `stream.pipeline`/`addAbortSignal`, `fs.readFile/writeFile`, `child_process.spawn/exec/execFile`, `http.request`/`https.request`, and `readline`.

```javascript
const { setTimeout: sleep } = require('timers/promises');

async function main() {
  // AbortSignal.timeout(ms) — 16.14+ (not in 14/15, not in 16.0–16.13).
  try {
    await sleep(1000, null, { signal: AbortSignal.timeout(20) });
  } catch (e) {
    console.log(e.name, e.code, e.cause && e.cause.name);   // AbortError ABORT_ERR TimeoutError
  }

  // Abort with a reason — signal.reason is 16.14+; the reason becomes error.cause.
  const ac = new AbortController();
  ac.abort(new Error('shutdown'));
  console.log(ac.signal.aborted, ac.signal.reason.message); // true shutdown
  try {
    await sleep(10, null, { signal: ac.signal });
  } catch (e) {
    console.log(e.name, e.cause.message);                    // AbortError shutdown
  }

  // throwIfAborted — 16.17+.
  try {
    ac.signal.throwIfAborted();
  } catch (e) {
    console.log(e.message);                                  // shutdown (the reason itself is thrown)
  }
}
main();
```

Portable timeout for code that must run on 16.0–16.13 (no `AbortSignal.timeout`) — and always clear the timer so it does not keep the process alive:

```javascript
function withTimeout(promiseFactory, ms) {
  const ac = new AbortController();
  const timer = setTimeout(() => ac.abort(), ms);
  return promiseFactory(ac.signal).finally(() => clearTimeout(timer));
}

const { setTimeout: sleep } = require('timers/promises');
withTimeout((signal) => sleep(1000, 'late', { signal }), 20)
  .then(console.log, (e) => console.log('timed out:', e.name));   // timed out: AbortError
```

```javascript
// BAD: Promise.race timeout leaks the timer and does not stop the work
function raceTimeoutBad(p, ms) {
  return Promise.race([p, new Promise((_, rej) => setTimeout(() => rej(new Error('timeout')), ms))]);
}
// The losing setTimeout keeps the event loop alive for `ms`, and `p` keeps running.
raceTimeoutBad(Promise.resolve('fast'), 10).then(console.log);   // fast (process waits for the timer)
```

There is **no `AbortSignal.any()`** in 16 (20.3 / 18.17). Combine signals manually:

```javascript
function anySignal(signals) {
  const ac = new AbortController();
  for (const s of signals) {
    if (s.aborted) { ac.abort(s.reason); break; }
    s.addEventListener('abort', () => ac.abort(s.reason), { once: true });
  }
  return ac.signal;
}
const user = new AbortController();
const combined = anySignal([user.signal, AbortSignal.timeout(1000)]);
user.abort(new Error('user cancelled'));
console.log(combined.aborted, combined.reason.message);           // true user cancelled
```

Listeners added this way stay attached to long-lived signals; remove them (or let the short-lived controller be garbage-collected) to avoid `MaxListenersExceededWarning` on signals reused across many requests.

##### `timers/promises` (Node 15+; not in 14)

```javascript
const { setTimeout: sleep, setInterval: every } = require('timers/promises');

async function poll() {
  let n = 0;
  for await (const startedAt of every(10, Date.now())) {   // setInterval: 15.9+
    n += 1;
    if (n === 3) break;                                     // break stops the interval
    void startedAt;
  }
  return n;
}
sleep(5, 'done').then(console.log);                          // done
poll().then((n) => console.log('ticks', n));                 // ticks 3
```

Node 14 equivalent: `const sleep = require('util').promisify(setTimeout);` (works on 14 and 16). `timers/promises` `scheduler.wait()` (16.14) is experimental.

##### Events

```javascript
const { EventEmitter, once, on } = require('events');

async function main() {
  const ee = new EventEmitter();

  // once() resolves with the args array and rejects if 'error' is emitted first.
  setTimeout(() => ee.emit('ready', 1, 2), 5);
  console.log(await once(ee, 'ready'));                 // [ 1, 2 ]

  setTimeout(() => ee.emit('error', new Error('E')), 5);
  try { await once(ee, 'data'); } catch (e) { console.log('rejected:', e.message); } // rejected: E

  // on() returns an async iterator; it never ends by itself — break or abort it.
  const ac = new AbortController();
  setTimeout(() => { ee.emit('tick', 'a'); ee.emit('tick', 'b'); ac.abort(); }, 5);
  const seen = [];
  try {
    for await (const [v] of on(ee, 'tick', { signal: ac.signal })) seen.push(v);
  } catch (e) {
    console.log(seen, e.name);                          // [ 'a', 'b' ] AbortError
  }
}
main();
```

- An `EventEmitter` that emits `'error'` with no listener throws and usually crashes the process: every stream, socket, server and child process needs an `'error'` handler (or `pipeline`/`once`).
- `captureRejections: true` (marked supported in 16.14) routes rejected promises returned by `async` listeners to `'error'`; without it, an `async` listener that throws produces an unhandled rejection.
- `MaxListenersExceededWarning` usually means a leak (adding a listener per request). Fix the leak; do not just raise `setMaxListeners`.

##### Async context: `AsyncLocalStorage`

`AsyncLocalStorage` (13.10/12.17) is the supported way to carry request IDs; `run()`/`getStore()` were marked stable in 16.4.[^4] Prefer it over the `domain` module (deprecated) and over `cls-hooked`.

```javascript
const { AsyncLocalStorage } = require('async_hooks');
const requestContext = new AsyncLocalStorage();

function log(msg) {
  const ctx = requestContext.getStore();
  console.log(`[${ctx ? ctx.requestId : '-'}] ${msg}`);
}

async function handler() {
  await new Promise((r) => setTimeout(r, 5));
  log('handled');                                   // [req-1] handled
}
requestContext.run({ requestId: 'req-1' }, handler);
log('outside');                                     // [-] outside
```

##### Blocking the event loop

Synchronous APIs (`fs.readFileSync`, `crypto.pbkdf2Sync`, `zlib.gzipSync`, `child_process.execSync`, huge `JSON.parse`/`JSON.stringify`, catastrophic RegExps) inside request handlers stall every connection. Allowed at startup; flag them in hot paths. CPU-bound work belongs in `worker_threads`.

***

#### Errors

##### Error classes and `cause` (cause: Node 16.9+; silently ignored on 14/15)

```javascript
class AppError extends Error {
  constructor(message, { code, statusCode = 500, cause } = {}) {
    super(message, cause === undefined ? undefined : { cause });   // ES2022 cause (16.9+)
    this.name = this.constructor.name;
    this.code = code;
    this.statusCode = statusCode;
  }
}
class NotFoundError extends AppError {
  constructor(resource, id) {
    super(`${resource} ${id} not found`, { code: 'NOT_FOUND', statusCode: 404 });
    this.resource = resource;
  }
}

async function loadUser(id) {
  try {
    throw new Error('connection refused');               // pretend the DB failed
  } catch (err) {
    throw new AppError(`loading user ${id} failed`, { code: 'DB', cause: err });
  }
}

loadUser(7).catch((err) => {
  console.log(err.name, err.code, err.cause.message);    // AppError DB connection refused
  console.log(err instanceof AppError, new NotFoundError('user', 7).statusCode); // true 404
});
```

- On **Node 14/15** `new Error('m', { cause })` does not throw — the option is **silently ignored** and `err.cause` is `undefined` (verified). Code shared with 14 must assign `this.cause = cause` explicitly.
- `cause` is an own, non-enumerable property: `JSON.stringify(err)` gives `{}`. `util.inspect`/`console.log` shows `[cause]` from 16.14 (it is printed on 16.20.2).
- Logging: serialize `name`, `message`, `code`, `stack` and the `cause` chain explicitly (or use a logger with an error serializer).

##### Rules a reviewer enforces

```javascript
// ❌ throwing strings or plain objects — no stack trace, instanceof checks fail
function bad(n) { if (n < 0) throw 'negative'; }      // eslint: no-throw-literal
// ✅
function good(n) { if (n < 0) throw new RangeError(`n must be >= 0, got ${n}`); return n; }

// ❌ swallowing
function parseBad(s) { try { return JSON.parse(s); } catch (e) { return null; } }
// ✅ handle the expected error, rethrow the rest
function parseConfig(s) {
  try {
    return JSON.parse(s);
  } catch (e) {
    if (e instanceof SyntaxError) throw new Error(`invalid config JSON: ${e.message}`, { cause: e });
    throw e;
  }
}

try { bad(-1); } catch (e) { console.log(typeof e, e.stack); }            // string undefined
try { good(-1); } catch (e) { console.log(e.name); }                       // RangeError
try { parseConfig('{'); } catch (e) { console.log(e.cause.name); }         // SyntaxError
console.log(parseBad('{'));                                                // null — the caller can't tell why
```

- Return/throw consistently: an `async` function signals failure by rejecting; never mix `callback(err)` and `throw`.
- Callback APIs: always check `err` first (`if (err) return cb(err);`) — note the `return`. Call the callback exactly once.
- Prefer `util.promisify()` or the `fs/promises`-style APIs to hand-written promise wrappers.
- Node core errors carry `err.code` (`'ENOENT'`, `'ERR_INVALID_ARG_TYPE'`, `'ABORT_ERR'`); match on `code`, not on `message` text (messages change between versions).
- `instanceof Error` fails for errors from another realm (`vm` contexts); `util.types.isNativeError()` works.
- `AggregateError` (15+) carries `errors`; log them all.

***

#### Streams, files and buffers

##### Use `pipeline`, never bare `.pipe()` chains

`.pipe()` does not forward errors or destroy the other streams on failure — a classic file-descriptor and memory leak.

```javascript
const { pipeline } = require('stream/promises');        // 15+; on 14: util.promisify(require('stream').pipeline)
const { Readable, Transform, Writable } = require('stream');

async function main() {
  const upper = new Transform({
    transform(chunk, _enc, cb) { cb(null, chunk.toString().toUpperCase()); },
  });
  const out = [];
  await pipeline(
    Readable.from(['a', 'b', 'c']),
    upper,
    new Writable({ write(chunk, _enc, cb) { out.push(chunk.toString()); cb(); } }),
  );
  console.log(out.join(''));                               // ABC

  // Async generators as pipeline stages (13.10+, so also 14); signal support for generators 16.10+.
  const lines = [];
  await pipeline(
    async function* () { yield 'x'; yield 'y'; },
    async function* (source) { for await (const c of source) yield `${c}!`; },
    new Writable({ write(c, _e, cb) { lines.push(String(c)); cb(); } }),
  );
  console.log(lines);                                      // [ 'x!', 'y!' ]

  // Cancellation.
  const ac = new AbortController();
  setTimeout(() => ac.abort(), 10);
  const endless = new Readable({ read() { setTimeout(() => this.push('x'), 5); } });
  try {
    await pipeline(endless, new Writable({ write(_c, _e, cb) { cb(); } }), { signal: ac.signal });
  } catch (e) {
    console.log(e.name);                                   // AbortError
  }
}
main();
```

```javascript
// fragment: BAD — errors on `src` are not seen by `dest`; on error the file descriptors leak
// fs.createReadStream(input).pipe(zlib.createGzip()).pipe(fs.createWriteStream(output));
```

##### Backpressure

When writing manually, respect the return value of `write()`:

```javascript
const { Writable } = require('stream');
const { once } = require('events');

async function writeMany(stream, count) {
  for (let i = 0; i < count; i += 1) {
    if (!stream.write(`line ${i}\n`)) await once(stream, 'drain');   // ✅ wait when the buffer is full
  }
  stream.end();
  await once(stream, 'finish');
}

let bytes = 0;
const slow = new Writable({ highWaterMark: 16, write(c, _e, cb) { bytes += c.length; setImmediate(cb); } });
writeMany(slow, 100).then(() => console.log('wrote', bytes));        // wrote 790
```

##### Readable helpers are experimental in 16

`readable.map/filter/forEach/toArray/some/every/find/reduce/flatMap/drop/take` (16.14–16.17) are **Stability 1 – Experimental** in the 16 docs and absent in 14/15. `stream.compose()` (16.9) and `readable.iterator()` (16.3) are experimental too. Using them in a library that claims `>=16` is a version bug; in an application pinned to 16.17+ it is acceptable but should be noted.[^17]

```javascript
const { Readable } = require('stream');
Readable.from([1, 2, 3]).map((x) => x * 2).toArray().then(console.log);  // [ 2, 4, 6 ] (16.15+)
```

##### Line-by-line reading

```javascript
const readline = require('readline');
const { Readable } = require('stream');

async function countLines(input) {
  const rl = readline.createInterface({ input, crlfDelay: Infinity });
  let n = 0;
  for await (const line of rl) if (line.trim() !== '') n += 1;   // works on 14 and 16
  return n;
}
countLines(Readable.from(['a\nb\n', 'c'])).then((n) => console.log(n));  // 3
```

`readline/promises` is Node 17+. On 16 (and 14), `rl.question()` has a custom promisified form: `const question = util.promisify(rl.question).bind(rl);` returns a promise for the answer (verified on 16.20.2), even though the callback is not error-first.

##### Files

```javascript
const fs = require('fs');
const fsp = require('fs/promises');      // 'fs/promises' specifier works on 14 and 16
const os = require('os');
const path = require('path');

async function main() {
  const dir = await fsp.mkdtemp(path.join(os.tmpdir(), 'review-'));
  const file = path.join(dir, 'data.json');

  // Atomic-ish replace: write a temp file, then rename over the target.
  const tmp = `${file}.${process.pid}.tmp`;
  await fsp.writeFile(tmp, JSON.stringify({ ok: true }), { encoding: 'utf8', mode: 0o600 });
  await fsp.rename(tmp, file);

  // FileHandle: always close in finally (GC closing is deprecated, DEP0137).
  const fh = await fsp.open(file, 'r');
  try {
    console.log(JSON.parse(await fh.readFile('utf8')).ok);   // true
  } finally {
    await fh.close();
  }

  // ENOENT is a normal condition: check err.code, don't pre-check with existsSync (TOCTOU).
  try {
    await fsp.readFile(path.join(dir, 'missing'));
  } catch (e) {
    if (e.code !== 'ENOENT') throw e;
    console.log('missing ok');                                 // missing ok
  }

  // Recursive delete: fs.rm (14.14+). fs.rmdir(..., { recursive: true }) is deprecated (DEP0147).
  await fsp.rm(dir, { recursive: true, force: true });
  console.log(fs.existsSync(dir));                             // false
}
main();
```

- `fs.rmdir(path, { recursive: true })` emits runtime **DEP0147** on 16 and, unlike 14, **throws `ENOENT`** for a missing path (16.0 removed the "permissive" behaviour; verified). It is removed in later Node versions. Use `fs.rm(path, { recursive: true, force: true })`.
- `fs.cp()` (16.7) is experimental in 16; it is absent in 14.
- `fs.exists()` (callback) is deprecated; `fs.existsSync()` is fine for startup checks but a TOCTOU race before an open.
- Pass `encoding` explicitly (`'utf8'`), otherwise you get a `Buffer`.
- Use `path.join`/`path.resolve`, never string concatenation with `/`.

##### Buffers

```javascript
// new Buffer() is deprecated (DEP0005, runtime warning outside node_modules) and unsafe.
const a = Buffer.alloc(8);                     // zero-filled
const b = Buffer.from('héllo', 'utf8');        // from data
const u = Buffer.allocUnsafe(8).fill(0);       // uninitialized memory: fill before exposing
console.log(a.length, b.length, u.every((x) => x === 0));   // 8 6 true

// slice()/subarray() share memory with the original — mutations are visible.
const view = b.subarray(0, 1);
view[0] = 0x48;                                 // 'H'
console.log(b.toString());                      // Héllo

// base64url encoding: 15.7 / 14.18+.
console.log(Buffer.from('ÿÿ>?', 'latin1').toString('base64url'));  // __8-Pw
// atob/btoa (16+) are legacy and Latin-1 only; use Buffer for base64.
```

`buffer.slice()` is documentation-deprecated in favour of `subarray()` (same semantics); new code should use `subarray()`.[^9]

***

#### HTTP and networking without fetch

##### No global `fetch` in 16

`fetch` is **absent** on 14/15/16/17 without flags (verified `typeof fetch === 'undefined'`). `--experimental-fetch` exists from 16.15 (and 17.5) and prints an `ExperimentalWarning`; it is an early undici build and must not be used in production on an EOL runtime. Options:

- **`undici@5`** (last major supporting `>=14.0`; `undici@6` needs 18.17+, `undici@7` needs 20.18.1+).[^13]
- **`node-fetch@2`** (CommonJS); `node-fetch@3` is ESM-only.
- **`axios`**, `got@11` (CJS; `got@12` is ESM-only and needs 14.16+).
- Core `http`/`https` for small needs.

When code calls a bare `fetch(...)` in a Node 16 project, check that a polyfill is installed and imported; otherwise it is a `ReferenceError` in production. TypeScript does not catch this when `@types/node` is newer than the runtime (see [Tooling](#tooling-packaging-and-typescript)).

##### Core `http` client with timeouts and cancellation

```javascript
const http = require('http');

function getJson(url, { timeoutMs = 5000, signal } = {}) {
  return new Promise((resolve, reject) => {
    const req = http.get(url, { signal, timeout: timeoutMs }, (res) => {
      if (res.statusCode < 200 || res.statusCode >= 300) {
        res.resume();                                     // drain, or the socket stays busy
        reject(new Error(`HTTP ${res.statusCode}`));
        return;
      }
      res.setEncoding('utf8');
      let body = '';
      res.on('data', (c) => { body += c; });
      res.on('end', () => {
        try { resolve(JSON.parse(body)); } catch (e) { reject(e); }
      });
      res.on('error', reject);
    });
    // 'timeout' only signals inactivity; the request must be destroyed explicitly.
    req.on('timeout', () => req.destroy(new Error(`timeout after ${timeoutMs} ms`)));
    req.on('error', reject);
  });
}

const server = http.createServer((req, res) => {
  if (req.url === '/slow') return;                        // never responds
  res.setHeader('content-type', 'application/json');
  res.end(JSON.stringify({ ok: true }));
});
server.listen(0, async () => {
  const base = `http://127.0.0.1:${server.address().port}`;
  console.log(await getJson(`${base}/`));                 // { ok: true }
  await getJson(`${base}/slow`, { timeoutMs: 50 }).catch((e) => console.log(e.message)); // timeout after 50 ms
  const ac = new AbortController();
  const p = getJson(`${base}/slow`, { signal: ac.signal });
  ac.abort();
  await p.catch((e) => console.log(e.name));              // AbortError
  server.close();
});
```

Review points:

- `http.request` has **no default timeout**. Every outbound call needs `timeout` + `destroy`, or a signal.
- Always consume or `resume()` the response body, even on error status codes.
- `http.globalAgent` has `keepAlive: false` on 14–18 (verified); create a dedicated `new http.Agent({ keepAlive: true, maxSockets })` for high-volume clients.
- `res.on('aborted')` / `req.aborted` are documentation-deprecated since 16.12 (DEP0156); listen for `'close'` and check `res.complete` / `writableFinished`.[^9]

##### Server timeouts (Node 16 defaults are unsafe)

Verified defaults on 16.20.2: `server.requestTimeout = 0` (**no limit** on the time to receive a whole request), `headersTimeout = 60000`, `keepAliveTimeout = 5000`, `timeout = 0`, `maxHeaderSize = 16384`. Node 18.0 changed `requestTimeout` to 300 000 ms.[^18] A Node 16 server exposed directly to clients is open to slow-body (Slowloris-style) resource exhaustion unless `requestTimeout` is set:

```javascript
const http = require('http');
const server = http.createServer((req, res) => res.end('ok'));

server.requestTimeout = 30_000;      // whole request must arrive within 30 s (14.11+)
server.headersTimeout = 20_000;      // headers within 20 s
server.keepAliveTimeout = 65_000;    // > load balancer idle timeout (e.g. AWS ALB default 60 s)
server.maxRequestsPerSocket = 1000;  // 16.10+
console.log(server.requestTimeout, server.headersTimeout, server.keepAliveTimeout);  // 30000 20000 65000
```

`keepAliveTimeout` must exceed the upstream load balancer's idle timeout, otherwise the LB reuses a socket Node just closed and clients see sporadic 502s.

##### Graceful shutdown without `closeAllConnections()`

`server.close()` stops accepting new connections but waits for **existing keep-alive connections** to end; `closeAllConnections()`/`closeIdleConnections()` only exist from 18.2. Verified on 16.20.2: with one idle keep-alive client, the `close` callback had not fired after 1.5 s. Track sockets yourself:

```javascript
const http = require('http');

function createGracefulServer(handler) {
  const sockets = new Set();
  const server = http.createServer(handler);
  server.on('connection', (socket) => {
    sockets.add(socket);
    socket.on('close', () => sockets.delete(socket));
  });
  server.shutdown = (graceMs = 10_000) => new Promise((resolve) => {
    server.close(resolve);
    for (const s of sockets) s.end();                    // ask idle keep-alive sockets to close
    setTimeout(() => { for (const s of sockets) s.destroy(); }, graceMs).unref(); // force after grace
  });
  return server;
}

const server = createGracefulServer((req, res) => res.end('ok'));
server.listen(0, () => {
  const agent = new http.Agent({ keepAlive: true });
  http.get({ port: server.address().port, agent }, (res) => {
    res.resume();
    res.on('end', async () => {
      const t0 = Date.now();
      await server.shutdown(2000);
      console.log('closed quickly:', Date.now() - t0 < 1000);   // closed quickly: true
      agent.destroy();
    });
  });
});
```

Ending sockets mid-response truncates in-flight requests; production code sets a draining flag, answers new requests on old keep-alive sockets with `Connection: close`, and only destroys sockets after the grace period. Libraries such as `stoppable`/`http-terminator` implement this.

##### DNS: IPv4-first ordering in 16, verbatim in 17+

On 14/16 `dns.lookup()` sorts IPv4 addresses before IPv6 by default (`verbatim: false`). **Node 17 changed the default to `verbatim: true`** (resolver order), so `localhost` may resolve to `::1` after an upgrade and connections to services bound only to `127.0.0.1` fail with `ECONNREFUSED ::1`.[^19] `--dns-result-order=ipv4first` / `dns.setDefaultResultOrder()` (16.4 / 14.18) make the order explicit. Prefer explicit `127.0.0.1` in configs.

##### Request validation and headers

- Validate `Content-Length`/body size; set a body limit (the `body-parser` default is 100 kB).
- Never echo user input into headers without validation: Node rejects CR/LF in header values with `ERR_INVALID_CHAR` (`http.validateHeaderValue()` 14.3+), but redirects to user-supplied URLs (`Location`) are an open-redirect risk.
- `--insecure-http-parser` disables request-smuggling protections; flag it in start scripts.

***

#### Child processes and worker threads

```javascript
const { execFile, spawn } = require('child_process');
const { promisify } = require('util');
const execFileAsync = promisify(execFile);

async function main() {
  // ✅ argument array, no shell — user input cannot inject commands
  const userInput = 'hello; rm -rf /';
  const { stdout } = await execFileAsync('echo', [userInput], { timeout: 5000, maxBuffer: 1024 * 1024 });
  console.log(stdout.trim());                                   // hello; rm -rf /

  // spawn timeout (14.18+) and signal (15.5 / 14.17+)
  const child = spawn('sleep', ['5'], { timeout: 50 });
  child.on('exit', (code, sig) => console.log('killed by', sig)); // killed by SIGTERM

  const ac = new AbortController();
  const c2 = spawn('sleep', ['5'], { signal: ac.signal });
  c2.on('error', (e) => console.log('spawn', e.name));           // spawn AbortError
  ac.abort();
}
main();
```

- `exec()`/`execSync()` and `spawn(…, { shell: true })` run a shell: string interpolation of untrusted input is command injection. Use `execFile`/`spawn` with an argument array.
- Always handle the child's `'error'` event (ENOENT when the binary is missing) and set `timeout`/`maxBuffer`.
- `execSync` blocks the event loop — startup scripts only.

**Worker threads** (stable) for CPU-bound work. Use `os.cpus().length` to size pools (`os.availableParallelism()` is 18.14+):

```javascript
const { Worker, isMainThread, parentPort, workerData } = require('worker_threads');
const os = require('os');

if (isMainThread) {
  const worker = new Worker(__filename, { workerData: { n: 30 }, resourceLimits: { maxOldGenerationSizeMb: 64 } });
  worker.once('message', (r) => console.log('fib =', r, 'cpus>0:', os.cpus().length > 0)); // fib = 832040 cpus>0: true
  worker.once('error', (e) => console.error('worker failed', e));
  worker.once('exit', (code) => { if (code !== 0) console.error('worker exit', code); });
} else {
  const fib = (n) => (n < 2 ? n : fib(n - 1) + fib(n - 2));
  parentPort.postMessage(fib(workerData.n));
}
```

Creating a worker per request is expensive; use a pool (for example `piscina` — it declares no `engines` field, so test the major you pin on the target runtime). `cluster` is fine for multi-process HTTP servers; in containers, prefer one process per container and scale horizontally.

***

#### Security

##### Runtime status is the first finding

An EOL runtime with an EOL OpenSSL is a security finding in itself (see [Support status](#support-status-and-upgrade-advice)). Within the code, review the following.

##### Prototype pollution

```javascript
// BAD: recursive merge of untrusted JSON
function naiveMerge(target, source) {
  for (const key in source) {
    if (source[key] && typeof source[key] === 'object') {
      target[key] = naiveMerge(target[key] || {}, source[key]);
    } else {
      target[key] = source[key];
    }
  }
  return target;
}
naiveMerge({}, JSON.parse('{"__proto__": {"isAdmin": true}}'));
console.log(({}).isAdmin);                      // true — every object is now "admin"
delete Object.prototype.isAdmin;

// GOOD: own keys only, block dangerous keys, use Object.create(null) or Map for dictionaries
function safeMerge(target, source) {
  for (const key of Object.keys(source)) {
    if (key === '__proto__' || key === 'constructor' || key === 'prototype') continue;
    const value = source[key];
    target[key] = value && typeof value === 'object' && !Array.isArray(value)
      ? safeMerge(Object.prototype.hasOwnProperty.call(target, key) ? target[key] : {}, value)
      : value;
  }
  return target;
}
safeMerge({}, JSON.parse('{"__proto__": {"isAdmin": true}}'));
console.log(({}).isAdmin);                      // undefined
```

`node --disable-proto=throw` (13.12 / 12.17+) makes `obj.__proto__` throw `ERR_PROTO_ACCESS`; it is a useful defence for applications (not libraries). Old releases of `lodash`, `minimist`, `qs`, `set-value` and similar merge/parse helpers had prototype-pollution CVEs — check `npm audit` and pin patched releases.

##### Path traversal

```javascript
const path = require('path');

function resolveInside(root, userPath) {
  const resolvedRoot = path.resolve(root);
  const full = path.resolve(resolvedRoot, userPath);
  if (full !== resolvedRoot && !full.startsWith(resolvedRoot + path.sep)) {
    throw new Error('path escapes root');
  }
  return full;
}

for (const p of ['a/b.txt', '../etc/passwd', '/etc/passwd', 'a/../../files2/x']) {
  try { console.log('ok', resolveInside('/srv/files', p)); } catch { console.log('blocked', p); }
}
// ok /srv/files/a/b.txt, blocked ../etc/passwd, blocked /etc/passwd, blocked a/../../files2/x

// ❌ missing path.sep: '/srv/files2/x'.startsWith('/srv/files') is true
console.log(path.resolve('/srv/files', '../files2/x').startsWith('/srv/files'));   // true
```

Decode URL-encoded input *before* the check, reject NUL bytes, and resolve symlinks (`fs.realpath`) when the root may contain links. `express.static`/`serve-static` handle this; hand-written file servers usually do not.

##### Injection

- **Command injection**: see [Child processes](#child-processes-and-worker-threads).
- **SQL/NoSQL**: parameterized queries only; for MongoDB reject objects where strings are expected (`{ "$gt": "" }`).
- **`eval`, `new Function`, `vm`**: `vm` is **not a security sandbox** (the Node docs say so); `vm2` is deprecated with unfixed sandbox escapes. Never run untrusted code in-process.
- **ReDoS**: user-controlled input against regexes with nested quantifiers (`/(a+)+$/`) blocks the event loop; Node 16 has no regex timeout. Bound input length and simplify patterns.
- **Template/HTML**: escape output; set `Content-Type` explicitly.

##### Crypto

```javascript
const crypto = require('crypto');

// Random identifiers and tokens
console.log(crypto.randomUUID().length);                    // 36 (15.6 / 14.17+)
console.log(crypto.randomBytes(32).toString('base64url').length); // 43
console.log(Number.isInteger(crypto.randomInt(1, 7)));       // true — never Math.random() for secrets

// Password hashing: scrypt (built-in), with a per-user salt
const salt = crypto.randomBytes(16);
const hash = crypto.scryptSync('correct horse', salt, 64);
const verify = crypto.scryptSync('correct horse', salt, 64);
console.log(crypto.timingSafeEqual(hash, verify));           // true

// timingSafeEqual throws on different lengths: compare fixed-size digests
function safeEqual(a, b) {
  const ha = crypto.createHash('sha256').update(String(a)).digest();
  const hb = crypto.createHash('sha256').update(String(b)).digest();
  return crypto.timingSafeEqual(ha, hb);
}
console.log(safeEqual('token', 'token'), safeEqual('token', 'token2'));  // true false

// Authenticated encryption: AES-256-GCM with a random 12-byte IV per message
const key = crypto.randomBytes(32);
const iv = crypto.randomBytes(12);
const cipher = crypto.createCipheriv('aes-256-gcm', key, iv);
const ct = Buffer.concat([cipher.update('secret', 'utf8'), cipher.final()]);
const tag = cipher.getAuthTag();
const decipher = crypto.createDecipheriv('aes-256-gcm', key, iv);
decipher.setAuthTag(tag);
console.log(Buffer.concat([decipher.update(ct), decipher.final()]).toString());  // secret
```

- `crypto.createCipher()`/`createDecipher()` (no IV, weak key derivation) are **runtime-deprecated (DEP0106)** and removed in Node 22. Flag any use.
- `pbkdf2(…)` requires an explicit `digest` (`'sha256'`/`'sha512'`); never `'sha1'` for new code.
- Web Crypto is at `require('crypto').webcrypto` (15+, experimental in 16), **not** a global `crypto` (the global needs `--experimental-global-webcrypto` on 16.15+, and is default only in 19+). `crypto.getRandomValues()` on the module is 17.4+; in 16 use `crypto.webcrypto.getRandomValues()` or `randomBytes()`.
- Legacy hashes such as `md4` work on 16 (OpenSSL 1.1.1) but throw `ERR_OSSL_EVP_UNSUPPORTED` on 17+ (OpenSSL 3, verified). webpack 4 hashes with md4: it builds on 16 and breaks on upgrade. Do not "fix" with `--openssl-legacy-provider` (that flag does not exist on 16, verified "bad option").
- TLS: do not set `rejectUnauthorized: false` or `NODE_TLS_REJECT_UNAUTHORIZED=0`. Use `--tls-min-v1.2` defaults (TLS 1.2 is the default minimum on 14/16).

##### Secrets, dependencies and supply chain

- Configuration via environment variables; `dotenv` (16.x and 17.x still declare `>=12`) since `--env-file` is 20.6+. `process.env` values are always strings — parse numbers/booleans explicitly (`'false'` is truthy).
- `npm ci` with a committed lockfile in CI and Docker builds; `npm audit` reviewed.
- Dependabot/Renovate will increasingly propose majors that require Node 18+; check `engines` before merging (`npm install` with npm 8 only warns on engine mismatch unless `engine-strict=true` is set in `.npmrc`).
- `npm install` runs lifecycle scripts of dependencies; `--ignore-scripts` in CI where possible.
- Never commit `.npmrc` with `_authToken`.

***

#### Performance

- **V8 9.4** (16.11+): no Maglev, no newer regexp optimizations; still, idiomatic code (monomorphic objects, avoiding `delete` on hot objects, avoiding megamorphic call sites) is fast.
- **Measure first**: `node --cpu-prof` (writes `.cpuprofile`), `--heap-prof`, `--prof`, `perf_hooks.monitorEventLoopDelay()`, `performance.now()` / `process.hrtime.bigint()`. All available on 14 and 16.
- **Heap limits**: V8 picks the default heap limit itself (16.20.2 reported a ~4 GiB `heap_size_limit` on a 16 GiB host); do not assume it matches a container memory limit. Set `--max-old-space-size` (MiB) explicitly, leaving headroom for off-heap memory (Buffers, native code).
- **Avoid sync I/O and `JSON` on huge payloads** in request paths; stream large responses with `pipeline`.
- **String building**: `array.push()` + `join()` or template literals; avoid `+=` in very hot loops only when profiling shows it.
- **Repeated `Buffer.concat(acc, chunk)` / `str += chunk` on a growing accumulator** is O(n²) copying; collect chunks in an array and concat once.
- **Logging**: synchronous `console.log` to a pipe/file can block; use an asynchronous structured logger and check that the pinned major still supports the runtime.
- **`Array.prototype.includes` in loops** → `Set`. **`Object.keys(obj).length` for counting** in hot paths → keep a counter or use `Map#size`.
- **Timers**: `setTimeout(fn, 0)` clamps to 1 ms; `setImmediate` runs after I/O; `process.nextTick` recursion starves I/O.
- **Unbounded caches** (`Map` without eviction) are the most common memory leak; use an LRU (`lru-cache@7` declares `>=12`; `lru-cache@11` requires Node 20+).

```javascript
const { performance, monitorEventLoopDelay } = require('perf_hooks');
const h = monitorEventLoopDelay({ resolution: 10 });
h.enable();
const t0 = performance.now();
const end = Date.now() + 30;
while (Date.now() < end) { /* simulate blocking work */ }
setTimeout(() => {
  h.disable();
  console.log('elapsed>=30:', performance.now() - t0 >= 30, 'p99 delay recorded:', h.percentile(99) > 0); // elapsed>=30: true p99 delay recorded: true
}, 20);
```

***

#### Testing

##### `node:test` — available only on 16.17+ (and 18+), experimental, minimal

The built-in test runner was added in 18.0 and backported to **16.17.0** ("with all the changes up to 18.7.0"); hooks (`before`, `after`, `beforeEach`, `afterEach`) arrived in **16.18.0**.[^4] It is **Stability 1 – Experimental** in 16 and prints an `ExperimentalWarning`. It is **absent on 14, 15 and 17.9.1** (verified). On 16.20.2 the module exports `after, afterEach, before, beforeEach, describe, it, run, test` — **no `mock`** (verified `t.mock` and `test.mock` are `undefined`), no reporters, no coverage, no `--test-name-pattern`. Only `--test` and `--test-only` CLI flags exist. Output is TAP.

```javascript
// fragment: test/math.test.js — run with: node --test test/   (16.17+)
const test = require('node:test');           // the node: prefix is mandatory
const assert = require('assert/strict');      // 'assert/strict' specifier: 15+; on 14 use require('assert').strict
const { describe, it, before } = test;

describe('sum', () => {
  let sum;
  before(() => { sum = (a, b) => a + b; });   // hooks: 16.18+
  it('adds', () => { assert.equal(sum(1, 2), 3); });
  it('handles async', async () => { assert.deepEqual(await Promise.resolve([1]), [1]); });
});

test('top-level test', (t) => {
  assert.ok(true);
});
```

For Node 16 projects, a mature runner is still the better choice:

| Tool | Last major supporting 16 (and 14) | Engines (from npm) |
|---|---|---|
| Jest | **29.x** (Jest 30 needs `^18.14 \|\| ^20 \|\| ^22 \|\| >=24`) | `^14.15.0 \|\| ^16.10.0 \|\| >=18.0.0` |
| ts-jest | 29.x | `^14.15.0 \|\| ^16.10.0 \|\| ^18.0.0 \|\| >=20.0.0` |
| Vitest | **0.34.x** (Vitest 1 needs 18+) | `>=v14.18.0` |
| Mocha | **10.x** (Mocha 11 needs 18.18+) | `>= 14.0.0` |
| c8 | 9.x (`>=14.14.0`; c8 10 needs 18) | |
| nyc | 15.x (nyc 17 needs 18) | |
| Supertest | 7.x | `>=14.18.0` |
| msw | 1.x | `>=14` |

Engine ranges come from the npm registry metadata of the latest release of each major.[^13]

##### What reviewers check in tests

- Async tests `await` (or return) every promise — a dropped promise passes on 14 and may crash the runner on 16.
- Fake timers are restored (`jest.useRealTimers()` / `clock.restore()`), servers closed, and sockets/agents destroyed in `afterAll`/`after`, otherwise the runner hangs (`--detectOpenHandles` in Jest).
- Assertions use `assert/strict` (or `assert.strictEqual`/`deepStrictEqual`), not the legacy loose `assert.equal`.
- Error assertions match `code` / class, not exact message strings (messages differ between Node versions).
- The CI matrix runs the **lowest** version in `engines` (e.g. `16.0.0` or `14.17.0` if declared) — not just the latest 16.x — when the project is a library.
- Tests do not depend on `fetch`, `structuredClone` or other 17+/18+ globals that exist on the developer's newer local Node.

```javascript
const assert = require('assert');
// Loose vs. strict assertions: the legacy API coerces.
assert.equal(1, '1');                                      // passes (==)
try { assert.strictEqual(1, '1'); } catch (e) { console.log(e.code); }    // ERR_ASSERTION
assert.deepStrictEqual({ a: [1] }, { a: [1] });
assert.rejects(Promise.reject(new TypeError('x')), TypeError).then(() => console.log('rejects ok')); // rejects ok
assert.throws(() => { throw new RangeError('bad'); }, { name: 'RangeError' });
console.log(typeof assert.match);                          // function (assert.match stable from 16.0)
```

***

#### Tooling, packaging and TypeScript

##### Package managers

- **npm**: Node 16.20.2 bundles **npm 8.19.4** (lockfile v2, readable by npm 6 and 7+); Node 14.21.3 bundles **npm 6.14.18** (lockfile v1; verified). npm 7+ installs **peer dependencies automatically** and fails on conflicting peers (`ERESOLVE`) — `--legacy-peer-deps` hides real incompatibilities and should be justified. npm 9 still supports `^14.17.0 || ^16.13.0 || >=18`; npm 10+ requires 18.17+.[^13]
- **Corepack** is bundled (experimental) from 16.9 / 14.19 (16.20.2 ships corepack 0.17.0). Current Corepack releases require Node 22+, so `corepack prepare pnpm@latest` style workflows may fetch package-manager versions that do not support 16.
- **pnpm 8** needs 16.14+; **pnpm 9** needs 18.12+; **pnpm 7** supports 14.6+. **Yarn 1** runs everywhere.[^13]
- `engines` is advisory for applications unless `engine-strict=true` is in `.npmrc`. Libraries should declare the real minimum including the minor.

```json
{
  "engines": { "node": ">=16.17.0 <17" },
  "volta": { "node": "16.20.2" }
}
```

##### ESLint

- **ESLint 8** is the last major supporting 14/16 (`^12.22.0 || ^14.17.0 || >=16.0.0`); ESLint 9 requires 18.18+.[^13] Use `.eslintrc.*` with `parserOptions.ecmaVersion: 2022` (or `2021` for 14) and `env: { node: true, es2021: true }`.
- `eslint-plugin-n` (16.x supports Node 16; 17.x needs 18.18+) checks `n/no-unsupported-features/es-syntax`, `n/no-unsupported-features/es-builtins` and `n/no-unsupported-features/node-builtins` against `engines.node`. This is the single most effective guard against using 17+/18+ APIs.
- Recommended rules for these versions: `no-floating-promises` (via `@typescript-eslint`), `no-throw-literal`, `prefer-promise-reject-errors`, `n/no-deprecated-api`, `no-restricted-globals` for `fetch`/`structuredClone` when no polyfill exists.

```json
{
  "root": true,
  "env": { "node": true, "es2021": true },
  "parserOptions": { "ecmaVersion": 2022, "sourceType": "script" },
  "plugins": ["n"],
  "extends": ["eslint:recommended", "plugin:n/recommended"],
  "rules": {
    "no-throw-literal": "error",
    "prefer-promise-reject-errors": "error",
    "no-restricted-globals": ["error", "fetch", "structuredClone"]
  }
}
```

##### TypeScript

- Pin **`@types/node@16`** (latest 16.18.x) or **`@types/node@14`** (14.18.x). With newer `@types/node`, `fetch`, `structuredClone`, `AbortSignal.any` and `fs.glob` type-check and then fail at runtime. (The shared toolchain used to verify this guide had `@types/node` 26 installed — exactly the misconfiguration that hides such bugs.)
- `@tsconfig/node16` sets `target`/`lib` to **`es2021`**, `module`/`moduleResolution` to `node16`; `@tsconfig/node14` uses `es2020`.[^20] With `lib: es2021`, `.at()` and `Object.hasOwn` are compile errors (`TS2550 … try changing the 'lib' compiler option to 'es2022'`, verified) even though Node 16.9+ supports them. Projects pinned to ≥16.11 may use `target`/`lib` `es2022`; projects that must run on 14 must use `es2020`, because `es2021` emits `??=`/`||=` verbatim, which is a SyntaxError on 14 (verified).
- TypeScript itself still runs on 14.17+ (TS 5.9 and 6.0 declare `>=14.17`).[^13]
- `esModuleInterop: true` for default-importing CommonJS; `module: node16` to keep `import()` as a real dynamic import in CommonJS output.
- `tsx@4` and `esbuild@0.25` require Node 18+; `ts-node@10` works on 16 (no `engines` field) — check the version you pin.

##### Formatting

Prettier 3 declares `>=14`; run `prettier --check` in CI. Do not hand-align code.

##### Docker

- `FROM node:16-alpine` / `node:16-bullseye-slim` images receive no Node.js updates after 16.20.2; their OS layers are frozen at their last build. Pin a digest, scan images, and plan the upgrade.
- Run as the `node` user, use `npm ci --omit=dev` (npm 8) / `npm ci --production` (npm 6), handle `SIGTERM` (PID 1 does not get default signal handlers; use `--init` or `tini`).

***

#### Observability and diagnostics

- **Structured logging** with a logger that serializes errors (stack, `code`, `cause`). Never log secrets, tokens or full request bodies.
- **`--enable-source-maps`** (stable from 14.18/15.11+) for TypeScript/bundled stack traces; it has a measurable cost on stack-heavy workloads.
- **Diagnostic report**: `--report-on-fatalerror`, `--report-uncaught-exception`, `process.report.writeReport()` (stable since 13.12).
- **Heap snapshots near OOM**: `--heapsnapshot-near-heap-limit=<n>` (15.1 / 14.18+), `v8.writeHeapSnapshot()`.
- **`diagnostics_channel`** is experimental in 16 (`channel()`; module-level `subscribe()` is 16.17+, absent in 17.9.1). APM agents use it; application code rarely should.
- **`process.getActiveResourcesInfo()`** (16.14+, experimental) helps find what keeps the process alive.
- **Event-loop lag**: `perf_hooks.monitorEventLoopDelay()` (see Performance) — export p99 as a metric.
- **Warnings**: `process.on('warning')` and `--trace-warnings`/`--trace-deprecation` reveal deprecated-API use before an upgrade; run CI once with `--throw-deprecation` to inventory them.

```javascript
process.on('warning', (w) => console.log('warning:', w.name, w.code));
const legacy = new Buffer(4);   // eslint-disable-line n/no-deprecated-api
void legacy;
// warning: DeprecationWarning DEP0005
```

***

#### Removed / changed — migration notes

##### 14 → 16 (things that break when a Node 14 project moves to 16)

| Change | Since | Effect |
|---|---|---|
| Unhandled rejections throw | 15.0 | Process exits with code 1 instead of logging a warning |
| npm 7+ | 15.0 | Peer dependencies installed and validated (`ERESOLVE`), lockfile v2, workspaces |
| `fs.rmdir(…, { recursive: true })` | 16.0 | Runtime-deprecated (DEP0147); throws `ENOENT` on a missing path (14 ignored it) |
| `process.binding('…')` for `crypto`, `url`, `http_parser`, `v8`, `signal_wrap`, `async_wrap` | 16.0 | Runtime-deprecated; `process.binding('util')` reduced to type checkers |
| `module.createRequireFromPath()` | 16.0 | Removed (use `module.createRequire()`) |
| `exports` folder mappings (`"./x/": "./x/"`) | 16.0 | Runtime DEP0148 warning (14.21.3 shows none); removed in 17 |
| ESM main/index and extension lookup (DEP0151) | 16.0 | Runtime warning when an ES module package (`"type": "module"`) declares `"main": "./dist"` without file name and extension (verified; CommonJS packages imported from ESM do not warn) |
| Invalid `package.json` `main` falling back to `index.js` (DEP0128) | 16.0 | Runtime-deprecated (verified: warns when the package is imported from ESM) |
| Changing `process.config` | 16.0 | Runtime-deprecated |
| `http.IncomingMessage` `autoDestroy: true` | 16.0 | Incoming request streams destroy themselves after `'end'` |
| ICU minimum 68 | 16.0 | `Intl` behaviour/locale data updates |
| JSON module import needs `assert { type: 'json' }` | 16.14 | ESM JSON imports without assertion fail |
| ESM loader hooks reshaped | 16.12, 16.17 | `getFormat`/`getSource`/`transformSource` → `resolve`/`load`; chaining with `shortCircuit` |

##### 16 → 18+ (what to watch for in an upgrade PR)

| Change | Since | Effect |
|---|---|---|
| OpenSSL 3.0 | 17.0 | `md4` and other legacy algorithms throw `ERR_OSSL_EVP_UNSUPPORTED` (webpack 4, old `crypto` code) |
| `dns.lookup` `verbatim: true` default | 17.0 | `localhost` may resolve to `::1` first → `ECONNREFUSED ::1` |
| `exports` folder mappings removed | 17.0 | `ERR_PACKAGE_PATH_NOT_EXPORTED` for old packages |
| Global `fetch`, `FormData`, `Headers`, Web Streams, `Blob` | 18.0 | Polyfills that check `typeof fetch` change behaviour; `undici` versions differ |
| `server.requestTimeout` default 300 s | 18.0 | Long uploads (> 5 min) fail unless configured |
| `node:test`, `--watch` without backport caveats | 18.x | |
| `server.close()` also closes idle keep-alive connections | 19.0 | Verified: `close` callback fires promptly on 19.9.0/20.20.2 but not on 16.20.2/18.20.8; socket-tracking shutdown code can be simplified |
| Global `crypto` (Web Crypto) | 19.0 | Feature checks such as `typeof crypto !== 'undefined'` start succeeding; isomorphic libraries switch code paths |
| `import … assert` → `with` | 18.20 / 20.10; `assert` removed in 22 | Rewrite JSON imports |
| `require(esm)` | 20.19 / 22.12 | ESM-only packages become `require`-able |

##### Deprecated APIs in 16 — replacements

| Deprecated | Code | Status in 16 | Replacement |
|---|---|---|---|
| `new Buffer()` / `Buffer()` | DEP0005 | Runtime | `Buffer.alloc`, `Buffer.from` |
| `fs.rmdir(p, { recursive })` | DEP0147 | Runtime | `fs.rm(p, { recursive: true, force: true })` |
| `crypto.createCipher` / `createDecipher` | DEP0106 | Runtime | `createCipheriv` with random IV and a KDF |
| `require('sys')` | DEP0025 | Runtime | `require('util')` |
| `OutgoingMessage#_headers`, `_headerNames` | DEP0066 | Runtime | `getHeaders()`, `getHeaderNames()` |
| `zlib` `bytesRead` | DEP0108 | Runtime | `bytesWritten` |
| `assert.fail()` with more than one argument | DEP0094 | Runtime | `assert.fail(message)` |
| Closing `FileHandle` on GC | DEP0137 | Runtime | `await fh.close()` in `finally` |
| `process.binding()` | DEP0111 | Documentation-only (some bindings runtime) | public APIs (`util.types`, …) |
| `punycode` | DEP0040 | Documentation-only | `url.domainToASCII()` or the userland `punycode` package |
| `util.isArray()`, `util.isString()`, … | DEP0044 etc. | Documentation-only | `Array.isArray`, `typeof` |
| `buffer.slice()` | DEP0158 | Documentation-only | `buffer.subarray()` |
| `res.on('aborted')`, `req.aborted` | DEP0156 | Documentation-only (16.12+) | `'close'` + `res.complete` |
| `process.on('multipleResolves')` | DEP0160 | Documentation-only (16.15+) | — |
| `url.parse()` | DEP0116 | **Revoked** in 16 ("legacy", not deprecated) | prefer WHATWG `new URL()`; `url.parse` has known hostname-spoofing pitfalls |
| Unhandled rejections | DEP0018 | End-of-Life (15+) | handle every rejection |

Statuses come from the Node 16.20.2 deprecations document; the runtime ones were reproduced on 16.20.2.[^9]

```javascript
// WHATWG URL instead of url.parse: validates and normalizes.
const u = new URL('/path?q=1', 'https://example.com');
console.log(u.hostname, u.searchParams.get('q'));    // example.com 1
try { new URL('not a url'); } catch (e) { console.log(e.code); }   // ERR_INVALID_URL
```

***

#### Production / review checklist

##### Runtime and version hygiene

- [ ] Summary notes that Node 14/16 are EOL (16: 2023-09-11, final 16.20.2; 14: 2023-04-30, final 14.21.3) and recommends an upgrade path to Node 22/24 LTS
- [ ] Deployed runtime is the final patch (16.20.2 / 14.21.3), not an older 16.x/14.x
- [ ] `engines.node` states the real minimum *including minor* for APIs used (`.at()` 16.6, `Object.hasOwn`/`cause` 16.9, `static {}` 16.11, `AbortSignal.timeout` 16.14, `node:test` 16.17)
- [ ] No Node 17/18+ APIs without polyfill: `fetch`, `structuredClone`, `findLast`, `toSorted`, `Object.groupBy`, `AbortSignal.any`, `readline/promises`, `os.availableParallelism`, `closeAllConnections`, `--env-file`, `require(esm)`, `import … with`
- [ ] Code shared with Node 14: no `??=`/`||=`/`&&=`, `replaceAll`, `Promise.any`, `static {}`, `#x in o`, `/d`, `.at()`, `Object.hasOwn`, `timers/promises`, `stream/promises`, `require('node:…')` (unless ≥14.18), `AbortController` (unless flagged); `cause` assigned manually
- [ ] No production `--experimental-*` flags (`--experimental-fetch`, `--experimental-specifier-resolution`, `--experimental-loader`) unless justified
- [ ] `@types/node` major matches the runtime; TS `target`/`lib` match (`es2021`/`es2022` for 16.11+, `es2020` for 14)

##### Correctness

- [ ] Every promise awaited, returned or `.catch()`-ed; no `async` callbacks in `forEach`
- [ ] `process.on('unhandledRejection'/'uncaughtException')` handlers log and exit
- [ ] Errors are `Error` subclasses with `code`; `cause` preserved; no swallowed `catch`
- [ ] `??` used where `0`, `''`, `false` are valid values
- [ ] `sort()` has a comparator for numbers; copies before sorting shared arrays
- [ ] ESM: explicit file extensions, `import.meta.url`-based paths, no named imports from CJS that `cjs-module-lexer` cannot detect
- [ ] No `require()` of ESM-only packages; TS `module` setting does not rewrite `import()` to `require()`
- [ ] `exports` map complete (including `./package.json` if consumers need it); no folder mappings

##### Streams, I/O, HTTP

- [ ] `pipeline` (with `await`) instead of `.pipe()`; every stream/socket/child has an `'error'` handler
- [ ] Backpressure respected (`write()` return value, `'drain'`)
- [ ] `fs.rm` instead of `fs.rmdir(recursive)`; `FileHandle`s closed in `finally`
- [ ] Outbound HTTP has timeouts/abort signals and consumes response bodies; dedicated keep-alive agent
- [ ] Server sets `requestTimeout` (default 0 on 16), `headersTimeout`, `keepAliveTimeout` > LB idle timeout
- [ ] Graceful shutdown tracks sockets (no `closeAllConnections` on 16)
- [ ] DNS/IPv6 assumptions explicit (`127.0.0.1`, `--dns-result-order`) — upgrade-safe

##### Security

- [ ] No `exec`/`shell: true` with untrusted input; `execFile`/`spawn` with arrays, timeouts
- [ ] Path traversal check uses `path.resolve` + `root + path.sep`
- [ ] No recursive merge of untrusted objects without key filtering; dictionaries via `Map`/`Object.create(null)`
- [ ] `crypto.randomBytes`/`randomUUID`/`randomInt` for secrets; `timingSafeEqual` on equal-length digests; no `createCipher`; AES-GCM with random IV
- [ ] No `rejectUnauthorized: false`, no `--insecure-http-parser`, no `vm` as a sandbox
- [ ] `npm ci` with lockfile; `npm audit` reviewed; dependency majors checked against `engines`

##### Performance and operations

- [ ] No sync I/O / `*Sync` crypto / `execSync` in request paths
- [ ] Bounded concurrency for fan-out; bounded caches
- [ ] `--max-old-space-size` set for containers; `SIGTERM` handled; process runs as non-root
- [ ] CPU-bound work in `worker_threads` pool sized by `os.cpus().length`
- [ ] Event-loop delay, heap usage and restarts monitored; `--enable-source-maps` for compiled code

##### Testing and tooling

- [ ] Test runner version supports the runtime (Jest 29, Vitest 0.34, Mocha 10, ESLint 8)
- [ ] `node:test` only if the minimum is 16.17 (hooks 16.18), with `node:` prefix, and without `mock`
- [ ] CI runs the lowest supported Node version from `engines`
- [ ] `eslint-plugin-n` `no-unsupported-features/*` enabled against `engines.node`
- [ ] Strict assertions; open handles closed; fake timers restored

***

[^1]: Node.js Release Working Group — release schedule (`schedule.json`: start, LTS, maintenance and end dates for v14–v27). https://github.com/nodejs/Release/blob/main/schedule.json
[^2]: Bringing forward the End-of-Life Date for Node.js 16 (Node.js blog). https://nodejs.org/en/blog/announcements/nodejs16-eol
[^3]: Node.js distribution index (release dates, V8, npm, OpenSSL versions per release). https://nodejs.org/dist/index.json
[^4]: Node.js 16 changelog (16.0.0–16.20.2). https://github.com/nodejs/node/blob/main/doc/changelogs/CHANGELOG_V16.md
[^5]: Node.js 14 changelog (14.0.0–14.21.3). https://github.com/nodejs/node/blob/main/doc/changelogs/CHANGELOG_V14.md
[^6]: Node.js October 2023 Security Releases (fixed only in 18.18.2 and 20.8.1). https://nodejs.org/en/blog/vulnerability/october-2023-security-releases
[^7]: Node.js — Updates on CVE handling for End-of-Life versions. https://nodejs.org/en/blog/vulnerability/updates-cve-for-end-of-life
[^8]: V8 feature explainers: logical assignment, `Promise.any`, RegExp match indices, private brand checks, `.at()`, `Object.hasOwn`, error cause, class static initializer blocks. https://v8.dev/features
[^9]: Node.js v16 deprecations (DEP0005, DEP0018, DEP0040, DEP0106, DEP0116, DEP0137, DEP0147, DEP0148, DEP0151, DEP0156, DEP0158). https://nodejs.org/docs/latest-v16.x/api/deprecations.html
[^10]: Node.js v16 API documentation — globals, util, stream, timers, events, crypto (`added:` metadata and stability indexes). https://nodejs.org/docs/latest-v16.x/api/
[^11]: Node.js 17 changelog (OpenSSL 3.0, `dns.lookup` verbatim default, V8 9.5). https://github.com/nodejs/node/blob/main/doc/changelogs/CHANGELOG_V17.md
[^12]: Node.js v16 ECMAScript modules and Packages documentation (`exports`, `imports`, conditional exports, dual package hazard). https://nodejs.org/docs/latest-v16.x/api/esm.html , https://nodejs.org/docs/latest-v16.x/api/packages.html
[^13]: npm registry metadata (`engines.node`) for undici, node-fetch, got, chalk, execa, nanoid, jest, ts-jest, vitest, mocha, c8, nyc, supertest, msw, eslint, eslint-plugin-n, typescript, prettier, pnpm, npm, dotenv, lru-cache. https://www.npmjs.com/
[^14]: cjs-module-lexer — CommonJS named-export detection used by Node.js. https://github.com/nodejs/cjs-module-lexer
[^15]: Node.js v16 Packages — Dual package hazard. https://nodejs.org/docs/latest-v16.x/api/packages.html#dual-package-hazard
[^16]: Node.js 15 changelog — "Throw On Unhandled Rejections" (#33021), npm 7, V8 8.6. https://github.com/nodejs/node/blob/main/doc/changelogs/CHANGELOG_V15.md
[^17]: Node.js v16 Stream documentation (experimental Readable helpers, `stream/promises`). https://nodejs.org/docs/latest-v16.x/api/stream.html
[^18]: Node.js 18 changelog (`server.requestTimeout` default 300000 ms). https://github.com/nodejs/node/blob/main/doc/changelogs/CHANGELOG_V18.md
[^19]: nodejs/node#39987 — dns: default to verbatim=true in dns.lookup(). https://github.com/nodejs/node/pull/39987
[^20]: tsconfig/bases — `@tsconfig/node14` and `@tsconfig/node16`. https://github.com/tsconfig/bases
