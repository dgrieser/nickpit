### JavaScript on Node.js 22: The Complete Production Developer Guide

> **Version**: Node.js 22 "Jod" — 22.0.0 released 2024-04-24; latest patch **22.23.4 (2026-10-21, security release)**.
> **Engine**: V8 **12.4** (12.4.254.21-node.57 in 22.23.4), libuv 1.51.0, undici 6.29.1 (`fetch`/`WebSocket`), OpenSSL 3.5.9, SQLite 3.53.4, ICU 78.3 (Unicode 17), npm 10.9.9.
> **Support status (October 2026)**: **Maintenance LTS** since 2025-10-21; **end of life 2027-04-30**. Only security and critical fixes land now.
> **Scope**: JavaScript (`.js`, `.mjs`, `.cjs`) written for the Node.js 22.x runtime — the ECMAScript features V8 12.4 ships, the Node 22 runtime APIs, ESM/CJS interop, async patterns, errors, streams, HTTP, security, performance, testing and tooling. This guide is also the one to apply when a project pins Node 23.x (see "Version-aware review").
> **Note**: This document is self-contained. Sections marked **New in 22** cover behaviour that differs from Node 20/21; **22.x** markers name the exact minor that introduced a backported feature.

***

#### Overview

Node.js 22 is the "Jod" long-term-support line. It became Active LTS with 22.11.0 on 2024-10-29 and entered **Maintenance LTS on 2025-10-21**; it reaches **end of life on 2027-04-30**. The newest releases as of 2026-10-22 are **Node.js 26.12.1** (Current; it becomes LTS on 2026-10-28), **Node.js 24.22.1** ("Krypton", Maintenance LTS since 2026-10-20) and **22.23.4**. Node.js 20 reached end of life on 2026-04-30 (final release 20.20.2), Node 21, 23 and 25 are odd-numbered lines that are already EOL, and Node.js 27 exists only as nightly builds — its alpha channel is scheduled to open on 2026-10-28 and 27.0.0 is planned for April 2027. Always deploy the **latest 22.x patch**: 2026 alone brought security releases 22.22.0, 22.22.2, 22.23.0, 22.23.2 and 22.23.4 (TLS hostname checks, HTTP/2 memory exhaustion, permission-model bypasses, `http.Agent` response-queue poisoning, DNS NUL-byte injection).

Unlike most runtimes, a Node LTS line keeps receiving **semver-minor features** while it is Active LTS (and occasionally later). Many headline features of Node 22 therefore arrived in a 22.x minor, not in 22.0.0: `require(esm)` is unflagged from **22.12.0**, TypeScript type stripping is on by default from **22.18.0**, `node:sqlite` needs no flag from **22.13.0**, `module.registerHooks()` arrived in **22.15.0**, `import.meta.main` in **22.18.0**, built-in HTTP proxy support in **22.21.0**, and OpenSSL 3.5 replaced OpenSSL 3.0 in **22.20.0**. When a project pins a specific 22.x minor (for example `"engines": { "node": "22.11.x" }` or a `node:22.11` Docker tag), check the minor before accepting code that depends on these.

Global truths about JavaScript on Node.js 22:

- **One thread runs your JavaScript.** The event loop (libuv) multiplexes I/O; CPU-bound work blocks every request until it finishes. Use `worker_threads` or child processes for parallel CPU work.
- **Two module systems coexist.** ES modules (`import`/`export`, `.mjs` or `"type": "module"`) and CommonJS (`require`, `.cjs` or `"type": "commonjs"`). From 22.12, `require()` can load synchronous ES modules; `import` has always been able to load CommonJS.
- **Web platform APIs are globals.** `fetch`, `Request`/`Response`, `URL`, `AbortController`, `structuredClone`, `Blob`, `File`, `TextEncoder`, Web Streams, `CompressionStream`, `BroadcastChannel`, `crypto` (Web Crypto), `navigator` and — **new in 22** — `WebSocket` are available without imports.
- **Unhandled promise rejections crash the process** (`--unhandled-rejections=throw` is the default since Node 15).
- **V8 12.4 defines the language level.** Everything through ES2024 is available, plus Set methods, iterator helpers and `Array.fromAsync`; several ES2025 features (`Promise.try`, `RegExp.escape`, `Float16Array`, RegExp modifiers, duplicate named groups) and explicit resource management (`using`) are **not**.

***

#### Version-Aware Review

##### Where the Node version comes from

Check, in order: `package.json` `"engines"` and `"volta"`/`"devEngines"`, `.nvmrc` / `.node-version` / `.tool-versions`, the `FROM node:<tag>` line of every Dockerfile, CI matrices (`actions/setup-node` `node-version:`), and `@types/node` in `devDependencies` (its major should match the runtime major: `@types/node@22` for Node 22). A mismatch between these sources is itself a finding: code tested on Node 24 in CI but deployed on a `node:22` image fails at runtime on `using`, `RegExp.escape()` or `crypto.randomUUIDv7()`.

```json
{
  "engines": { "node": ">=22.12.0 <23" },
  "devDependencies": { "@types/node": "^22.19.0" }
}
```

Use a **floor at the minor you depend on** (`>=22.12.0` when you rely on unflagged `require(esm)`, `>=22.18.0` for default type stripping or `import.meta.main`, `>=22.21.0` for `NODE_USE_ENV_PROXY`). npm only warns on `engines` mismatches unless `engine-strict=true` is set in `.npmrc`.

##### Node 23 maps to this guide

Node 23 (EOL 2025-06-01) shipped V8 12.9, so a few language features *do* exist on 23 but not on 22 (`Promise.try`, RegExp modifiers `(?i:…)`, duplicate named capture groups, `Intl.DurationFormat`). Code that must run on both 22 and 23 must stay within the Node 22 feature set described here. Everything introduced in Node 24 or later is unavailable on both.

##### Verify before claiming something is unavailable

Many APIs that debuted in Node 23 or 24 were backported to a later 22.x minor (`require(esm)`, `node:sqlite` unflagging, `zlib.zstdCompress()`, `module.registerHooks()`, `assert.partialDeepStrictEqual()`, `fs.glob()` stability, `Worker[Symbol.asyncDispose]`, `http.Agent({ proxyEnv })`). Conversely, some 23.x/24.x features were never backported (`URLPattern`, `crypto.randomUUIDv7()`, `AsyncLocalStorage` scopes). Say "not available in Node 22" only when the pinned version proves it — `node -p "typeof require('node:zlib').zstdCompress"` on the exact patch is the authoritative check. If the project pins an *older* 22.x minor, report "requires Node ≥ 22.N" instead of "not available in Node 22".

```javascript
// fragment: runtime feature detection for libraries that support several 22.x minors
import { getBuiltinModule } from 'node:process';

const hasRequireEsm = process.features.require_module === true;   // 22.10+ getter, true from 22.12
const hasTypeStripping = typeof process.features.typescript === 'string'; // 22.10+ getter; 'strip' by default from 22.18
const zlib = getBuiltinModule('node:zlib');                         // process.getBuiltinModule: 22.3+
const hasZstd = typeof zlib.zstdCompress === 'function';            // 22.15+
console.log({ hasRequireEsm, hasTypeStripping, hasZstd });
```

`process.getBuiltinModule()` (22.3) loads a built-in synchronously from either module system without static imports, which is the idiomatic way to feature-detect.

***

#### What's New in Node.js 22 (vs. Node 20 / 21)

Node 21 (October 2023) was the odd-numbered line between the two LTS releases; everything it added is part of 22. Node 20 later received backports of several 22 features (20.19 unflagged `require(esm)` and module-syntax detection, for example), so "new in 22" below means "new compared with Node 20.0 / 21.0"; items also present in the final 20.20.2 are flagged *(also late 20.x)*.

##### Language (V8 11.3 → 12.4)

| Feature | Available since | Notes |
|---|---|---|
| `Object.groupBy()`, `Map.groupBy()` | Node 21 (V8 11.8) | Result object has a `null` prototype |
| `ArrayBuffer.prototype.transfer()`, `.transferToFixedLength()`, `.detached` | Node 21 | |
| `JSON.rawJSON()`, `JSON.isRawJSON()` (JSON.parse source text access) | Node 21 | Lossless big-number serialisation: `JSON.stringify({ id: JSON.rawJSON('12345678901234567890') })` |
| `Promise.withResolvers()` | **New in 22** (V8 12.4) | |
| `Array.fromAsync()` | **New in 22** | |
| Set methods: `union`, `intersection`, `difference`, `symmetricDifference`, `isSubsetOf`, `isSupersetOf`, `isDisjointFrom` | **New in 22** | Argument must be set-like (`size`, `has`, `keys`) |
| Iterator helpers: global `Iterator`, `Iterator.from()`, `.map/.filter/.take/.drop/.flatMap/.reduce/.toArray/.forEach/.some/.every/.find` | **New in 22** | Lazy; work on any iterator |
| RegExp `v` flag, `String.prototype.isWellFormed()/toWellFormed()`, change-array-by-copy (`toSorted`, `toReversed`, `toSpliced`, `with`), resizable `ArrayBuffer`, growable `SharedArrayBuffer`, `Atomics.waitAsync()` | Node 20 | Still available, listed for completeness |
| WebAssembly GC | **New in 22** | |

Feature-to-edition mapping follows the TC39 finished-proposals list.

##### Runtime, per 22.x minor

- **22.0.0** (2024-04-24): V8 12.4; `WebSocket` global enabled by default (stable from 22.4); `fs.glob()`/`fs.globSync()`/`fsPromises.glob()` (experimental, stable 22.17); `node --run <script>` (stable 22.10); `--watch` stable; stream default `highWaterMark` raised **16 KiB → 64 KiB**; `require(esm)` behind `--experimental-require-module`; **import assertions (`assert { type: 'json' }`) removed** — use import attributes (`with`); `crypto.createCipher()`/`createDecipher()` removed; `util.is*()` type checks, `util.log()`, `util._extend()`, `fs.Stats`, `Hash` and `Hmac` constructors runtime-deprecated; `process.availableMemory()` and `v8.queryObjects()` *(both also late 20.x)*; `util.styleText()` accepts arrays of formats; Maglev enabled (disabled again in 22.9).
- **22.1**: `NODE_COMPILE_CACHE` on-disk code cache; `dns.lookup({ order: 'ipv6first' })`; `URL.parse()` *(also late 20.x)*; `--test-skip-pattern`.
- **22.2**: `--experimental-policy` **removed**; `zlib.crc32()` *(also late 20.x)*; `fs.glob({ withFileTypes })`; test plans (`t.plan()`); `--inspect-wait`.
- **22.3**: `process.getBuiltinModule()` *(also late 20.x)*; test-runner snapshot testing and module mocking (`--experimental-test-module-mocks`); `node --run` walks up to the nearest `package.json`; `Blob.prototype.bytes()` *(also late 20.x)*; `EventSource` client behind `--experimental-eventsource`.
- **22.4**: Web Storage (`localStorage`/`sessionStorage`) behind `--experimental-webstorage`; `util.parseArgs()` `allowNegative`.
- **22.5**: `node:sqlite` behind `--experimental-sqlite`; `path.matchesGlob()` and `worker_threads.postMessageToThread()` *(both also late 20.x)*; `require(esm)` namespaces get `__esModule`.
- **22.6**: `--experimental-strip-types`; `stream.duplexPair()` *(also late 20.x)*; `--experimental-network-imports` **removed**.
- **22.7**: `--experimental-transform-types` (enums, namespaces, parameter properties); **module syntax detection enabled by default** *(also late 20.x)*; `AsyncLocalStorage` reimplemented without `async_hooks`.
- **22.8**: `module.enableCompileCache()`; test coverage thresholds (`--test-coverage-lines/branches/functions`); `vm.constants.DONT_CONTEXTIFY`.
- **22.9**: **Maglev disabled by default** (V8 codegen bugs); `util.getCallSite()` (renamed `getCallSites()` in 22.12); `--env-file-if-exists` *(also late 20.x)*; `tls.createSecureContext({ allowPartialTrustChain })`.
- **22.10**: `"module-sync"` exports condition; `node --run` stable; `process.features.require_module` and `process.features.typescript`; `KeyObject.prototype.toCryptoKey()`; `X509Certificate` `validFromDate`/`validToDate`.
- **22.11** (2024-10-29): becomes Active LTS — no functional changes.
- **22.12**: **`require(esm)` enabled by default** *(also 20.19+)*; resizable-`ArrayBuffer` support in `Buffer`; import attributes and JSON modules marked stable; SQLite session extension.
- **22.13**: **Permission model stable** (`--permission`); **`node:sqlite` unflagged** (still emits an ExperimentalWarning); Web Crypto `Ed25519`/`X25519` stable; `assert.partialDeepStrictEqual()`; `module.stripTypeScriptTypes()`; `StatementSync.prototype.iterate()`; `net.SocketAddress.parse()`; `--trace-env`; `require(esm)` no longer warns by default; `process.features.{ipv6,uv,tls_*}` doc-deprecated.
- **22.14**: `process.ref()`/`process.unref()`; `module.findPackageJSON()`; `fs.glob` `exclude` accepts glob patterns; `t.waitFor()`, `t.assert.fileSnapshot()`, `assert.register()` in the test runner; `--disable-sigusr1`; TypeScript on `--eval`/stdin.
- **22.15**: `zlib` **Zstandard** (`zstdCompress`/`zstdDecompress`); **`module.registerHooks()`** (synchronous, in-thread hooks); `process.execve()`; `tls.getCACertificates()`; `--use-system-ca` (system trust store); `util.diff()`; `dns.resolveTlsa()`; `v8.getCppHeapStatistics()`; partial error comparison in `assert`.
- **22.16**: `--experimental-config-file` / `node.config.json`; `StatementSync.prototype.columns()`; `import.meta.dirname`/`filename` marked stable; `worker.getHeapStatistics()`; `util.types.isFloat16Array()` (the `Float16Array` global itself is **not** in V8 12.4).
- **22.17**: `fs.glob()` and many stream/fs APIs stable; `assert.partialDeepStrictEqual()` stable; `fs.Dir` gains `Symbol.asyncDispose`; `--permission` implicitly allows reading the entry point; `util.styleText('none', …)`; HTTP/2 priority signaling, `options.shell = ''` and calling `node:http` constructors without `new` doc-deprecated.
- **22.18**: **type stripping enabled by default** (no warning); `import.meta.main`; `Worker[Symbol.asyncDispose]()`; `url.fileURLToPathBuffer()`; `--watch-kill-signal`; permission flags propagate to child processes; `DatabaseSync` `readBigInts` option.
- **22.19**: `NODE_USE_SYSTEM_CA=1`; `tls.setDefaultCACertificates()`; `--experimental-wasm-modules` unflagged; `process.threadCpuUsage()`; `server.keepAliveTimeoutBuffer` (default 1000 ms); `dns` `maxTimeout`.
- **22.20**: **OpenSSL 3.5** (was 3.0; hybrid post-quantum `X25519MLKEM768` key exchange is now negotiated by default); `CompressionStream('brotli')`; `mock.property()`; `execArgv` in SEA config; worker CPU profiling.
- **22.21**: `NODE_USE_ENV_PROXY` / `--use-env-proxy` and `http.Agent({ proxyEnv })`; `http.createServer({ shouldUpgradeCallback })`; `--max-old-space-size-percentage`; `.env` file support marked stable.
- **22.22–22.23.4**: security releases only, but **22.23.0 removed HTTP/2 priority signaling** (DEP0194 is End-of-Life) and a `TLSSocket` default `'error'` handler was added in 22.22.0.

***

#### Do Not Use or Recommend (Not Available in Node.js 22)

These features exist in newer Node.js or V8 releases. On 22.23.4 they are a `SyntaxError`, `undefined`, or an unknown flag. Never suggest them for code that targets Node 22 (or 23).

##### Language features missing from V8 12.4

- **Explicit resource management**: `using` / `await using` declarations, `DisposableStack`, `AsyncDisposableStack`, `SuppressedError` (Node 24, V8 13.6). Node 22 defines `Symbol.dispose`/`Symbol.asyncDispose` and several built-ins implement them, but `using x = …` is a `SyntaxError`; call `[Symbol.dispose]()` in `finally` instead.
- **`Promise.try()`**, **RegExp modifiers** (`/(?i:a)b/`), **duplicate named capture groups** (`/(?<y>a)|(?<y>b)/`), **`Intl.DurationFormat`** — Node 23 (V8 12.9). They exist on 23 but not on 22.
- **`Float16Array`**, **`Math.f16round()`**, **`RegExp.escape()`**, **`Error.isError()`**, **`Atomics.pause()`** — Node 24 (V8 13.6).
- **`Uint8Array.fromBase64()` / `.toBase64()` / `fromHex()` / `toHex()`** — Node 25 (V8 14.1). Use `Buffer.from(str, 'base64')` / `buf.toString('base64url')`.
- **`Temporal`**, **`Map.prototype.getOrInsert()`/`getOrInsertComputed()`** (upsert), **`Iterator.concat()`** — Node 26 (V8 14.6).
- **Decorators** (`@dec class …`) — not shipped by any Node version; requires a transpiler.
- **`Math.sumPrecise()`**, `ShadowRealm` — not shipped by any Node version through 26.

##### Node.js APIs missing from 22.x

- **Node 23**: `URLPattern` as an export of `node:url` (23.8; global from 24); the `'module.exports'` export name on the namespace of an `import`ed CommonJS module; global `CloseEvent`; `TimeoutNegativeWarning` for negative timer delays; `spec` as the default test reporter in non-TTY output.
- **Node 24** (several items arrived only in late 24.x minors as backports from 26: `crypto.randomUUIDv7()` and `req.signal` 24.16, `Buffer.poolSize` 64 KiB 24.18, `Blob.prototype.textStream()` and `--experimental-import-text` 24.19, `als.withScope()` 24.20, `MIMEType.parse()` 24.21, `crypto.parsePKCS12()`, `util.debounce()`/`throttle()`/`markPromiseAsHandled()` 24.22): global **`URLPattern`**; `crypto.randomUUIDv7()`; `crypto.argon2()`; `crypto.parsePKCS12()`; Web Crypto `SubtleCrypto.supports()`, ML-KEM/ML-DSA, SHA-3, AES-OCB, ChaCha20-Poly1305 algorithms; `AsyncLocalStorage.prototype.withScope()` and the `name`/`defaultValue` constructor options; `AsyncLocalStorage` backed by `AsyncContextFrame` by default (opt-in on 22 via `--experimental-async-context-frame`); `http.setGlobalProxyFromEnv()`; `IncomingMessage.prototype.signal` (`req.signal`); `fs.Utf8Stream`; `fs.mkdtempDisposableSync()`; `stream/consumers` `bytes()`; `Blob.prototype.textStream()`; `util.debounce()`/`util.throttle()`, `util.markPromiseAsHandled()`, `util.convertProcessSignalToExitCode()`, `MIMEType.parse()`; `navigator.locks`; `node:sqlite` tagged templates (`createTagStore`), `StatementSync.prototype.close()`, `limits`, authorizer; test runner auto-awaits subtests (on 22 an un-awaited `t.test()` is **cancelled**); `--test-randomize`; global test setup/teardown; `Buffer.poolSize` default 64 KiB (22: 8 KiB); `import text from './x.txt' with { type: 'text' }` (`--experimental-import-text`).
- **Node 25**: `--allow-net` (the Node 22 permission model does **not** restrict network access); `localStorage` enabled by default; global `ErrorEvent`; `--build-sea`; ESM entry points in SEA.
- **Node 26**: `node:ffi`, and `node:vfs`/`node:bench` behind `--experimental-vfs`/`--experimental-bench`; `fs.openAsBlobSync()`; `Buffer.stringLength()`, `buffer.isLatin1()`; `http.isValidHeaderName()`; `sqlite` binding `undefined` as `NULL` (on 22 it throws `ERR_INVALID_ARG_TYPE`).

##### Flags that do not exist on Node 22

`--test-isolation` (22 spells it `--experimental-test-isolation=none`), `--allow-net`, `--allow-inspector`, `--permission-audit`, `--build-sea`, `--experimental-import-text`, `--test-randomize` — all rejected with `bad option` on 22.23.4. `--localstorage-file` is accepted on 22 but does nothing without `--experimental-webstorage` (`localStorage` stays `undefined`). `--experimental-permission` still works on 22 as an alias of `--permission`.

##### Older APIs: removed or deprecated in 22

| API | Status in 22 | Replacement |
|---|---|---|
| `import x from './a.json' assert { type: 'json' }` | **Removed (22.0)** — `SyntaxError` | `with { type: 'json' }` |
| `crypto.createCipher()` / `createDecipher()` | **Removed (22.0)** — `TypeError: not a function` | `createCipheriv()` with a random IV and a KDF-derived key |
| `--experimental-policy`, `--experimental-network-imports` | **Removed** (22.2 / 22.6) | Permission model; bundle remote code |
| HTTP/2 priority signaling (`stream.priority()`, `priority` options) | **End-of-Life in 22.23.0** (DEP0194) | Remove; it was ignored by most peers |
| `util.isArray()`, `util.isBoolean()` … `util.isUndefined()`, `util.log()`, `util._extend()` | Runtime-deprecated (22.0); **removed in 23** except `util.isArray`/`_extend` | `Array.isArray()`, `typeof`, `Object.assign()` |
| `new crypto.Hash()`, `crypto.Hmac()` constructors, `new fs.Stats()` | Runtime-deprecated (22.0) | `crypto.createHash()`, `createHmac()` |
| `punycode` module | Runtime-deprecated | `url.domainToASCII()` or userland `punycode/` |
| `url.parse()` | Doc-deprecated (DEP0169); runtime-deprecated in 24 | `new URL()` / `URL.parse()` |
| `Buffer()` without `new` / `new Buffer()` | Runtime-deprecated (DEP0005) | `Buffer.alloc()`, `Buffer.from()` |
| `fs.rmdir(path, { recursive: true })` | Runtime-deprecated (DEP0147) | `fs.rm(path, { recursive: true, force: true })` |
| `assert.CallTracker` | Runtime-deprecated | `mock.fn()` from `node:test` |
| `dirent.path` | Doc-deprecated (22); removed in 24 | `dirent.parentPath` |
| `fs.F_OK`/`R_OK`/`W_OK`/`X_OK` | Doc-deprecated; removed in 25 | `fs.constants.F_OK` … |
| `require('node:_http_*')`, `_stream_*`, `_tls_*` | Doc-deprecated | Public modules |
| `options.shell = ''` in `child_process`, `args` with `shell: true` | Doc-deprecated (DEP0196, DEP0190) | Pass a shell path explicitly, or avoid `shell` |
| `util.getCallSite()` | Renamed (ExperimentalWarning); removed in 24 | `util.getCallSites()` |
| `crypto.createHash('shake128' / 'shake256')` without `outputLength` | Doc-deprecated (DEP0198, 22.18; warns under `--pending-deprecation`) | Pass `{ outputLength }` explicitly |

All of these were verified on 22.23.4: removed APIs throw, runtime-deprecated ones print a `DeprecationWarning` with the DEP code on first use.

***

#### Language Fundamentals (V8 12.4)

##### Declarations, equality and coercion

```javascript
// Prefer const; use let only when the binding is reassigned; never var.
const config = Object.freeze({ retries: 3 });
let attempt = 0;

// Strict equality everywhere; the single idiomatic exception is `== null`
// (true for null and undefined only).
function isMissing(value) {
  return value == null;
}

// Nullish coalescing keeps legitimate falsy values; || does not.
const port = Number(process.env.PORT ?? 3000);   // '0' stays 0 instead of becoming 3000
const timeoutMs = 0;
console.log(timeoutMs || 5000, timeoutMs ?? 5000); // 5000 0  — `||` is a bug for numeric options

// Optional chaining short-circuits on null/undefined only.
const user = { profile: null };
console.log(user.profile?.name ?? 'anonymous', isMissing(undefined), config.retries, attempt);

// Logical assignment
const opts = {};
opts.retries ??= 3;     // assign only if null/undefined
opts.verbose ||= false; // assign if falsy
console.log(opts);
```

Review rules:

- `Number(x)` parses the whole string (`Number('12px')` is `NaN`); `parseInt(x, 10)` stops at the first non-digit (`12`). Always pass the radix to `parseInt`. Validate with `Number.isInteger()` / `Number.isFinite()`, never the coercing global `isNaN()`/`isFinite()`.
- `typeof null === 'object'`, `typeof [] === 'object'`: use `Array.isArray()` and explicit `=== null` checks.
- `0.1 + 0.2 !== 0.3`: money and exact decimal arithmetic must use integer minor units or `BigInt`, not floats.
- `Number.MAX_SAFE_INTEGER` is 2^53−1. IDs from databases (`BIGINT`) or JSON payloads above that lose precision silently; keep them as strings or `BigInt`.

##### Numbers and BigInt

```javascript
const big = 2n ** 64n;
console.log(big, typeof big);                          // 18446744073709551616n 'bigint'
console.log(Number.isSafeInteger(2 ** 53), 1_000_000); // false 1000000

// BigInt and Number never mix implicitly.
try { console.log(1n + 1); } catch (e) { console.log(e.constructor.name); } // TypeError

// JSON.stringify throws on BigInt; convert explicitly.
const payload = JSON.stringify({ id: 1n }, (_k, v) => (typeof v === 'bigint' ? v.toString() : v));
console.log(payload); // {"id":"1"}
```

##### Strings and Unicode

```javascript
const name = 'Zoë';
console.log(`Hello, ${name}!`);                 // template literals over concatenation
console.log('a-b-c'.replaceAll('-', '+'));      // a+b+c (no regex needed)
console.log('abc'.at(-1));                      // 'c'
console.log([...'👍🏽'].length, '👍🏽'.length);   // 2 4 — code points vs UTF-16 units

// Well-formedness (Node 20+): lone surrogates break TextEncoder/URI APIs
const broken = 'abc\uD800';
console.log(broken.isWellFormed());               // false
console.log(broken.toWellFormed().at(-1) === '\uFFFD'); // true

// Grapheme-aware segmentation for user-visible lengths
const seg = new Intl.Segmenter('en', { granularity: 'grapheme' });
console.log([...seg.segment('👍🏽e\u0301')].length);   // 2
```

Use `localeCompare()` or `Intl.Collator` for user-facing sorting, and `normalize('NFC')` before comparing user input that may contain combining characters. ICU is bundled with full data (ICU 78.3 in 22.23.4), so `Intl` works for every locale without extra packages.

##### Regular expressions — the `v` flag, and what is missing

```javascript
// `v` (unicodeSets, Node 20+): set operations and properties of strings
const nonAsciiLetter = /^[\p{L}--[a-zA-Z]]$/v;
console.log(nonAsciiLetter.test('ä'), nonAsciiLetter.test('a')); // true false
console.log(/^\p{RGI_Emoji}$/v.test('👍🏽'));                     // true

// `d` flag: match indices
const m = /(?<year>\d{4})-(?<month>\d{2})/d.exec('on 2024-05');
console.log(m.groups.year, m.indices.groups.year);               // 2024 [ 3, 7 ]
```

**Not in Node 22**: inline modifiers `(?i:…)` and duplicate named groups in alternatives are `SyntaxError`s (Node 23+); `RegExp.escape()` is `undefined` (Node 24+). Escape user input yourself:

```javascript
// BAD: user input interpolated into a RegExp — ReDoS and wrong matches
const needle = 'a+b(c';
try { new RegExp(needle); } catch (e) { console.log('BAD:', e.constructor.name); } // SyntaxError

// GOOD on Node 22: escape metacharacters (RegExp.escape() does not exist yet)
const escapeRegExp = (s) => s.replace(/[\\^$.*+?()[\]{}|/-]/g, '\\$&');
console.log(new RegExp(escapeRegExp(needle)).test('xa+b(cx')); // true
```

Review every regex built from or applied to untrusted input for catastrophic backtracking (nested quantifiers such as `(a+)+`, `(.*a){n}`). V8 has no backtracking limit; one request can pin the event loop. Bound input length before matching, or use `RE2` bindings for user-supplied patterns.

##### Collections — New in 22: Set methods

```javascript
const admins = new Set(['ann', 'bob']);
const active = new Set(['bob', 'cid']);

console.log([...admins.union(active)]);               // [ 'ann', 'bob', 'cid' ]
console.log([...admins.intersection(active)]);        // [ 'bob' ]
console.log([...admins.difference(active)]);          // [ 'ann' ]
console.log([...admins.symmetricDifference(active)]); // [ 'ann', 'cid' ]
console.log(new Set(['bob']).isSubsetOf(admins), admins.isDisjointFrom(new Set(['zed']))); // true true

// The argument must be set-like (size, has, keys): a Map works, an array does not.
console.log([...admins.union(new Map([['dan', 1]]))]);  // [ 'ann', 'bob', 'dan' ]
try { admins.union(['dan']); } catch (e) { console.log(e.constructor.name, e.message); } // TypeError The .size property is NaN
```

Flag code that passes arrays to Set methods (wrap them: `new Set(array)`) and hand-rolled `[...a].filter((x) => b.has(x))` intersections that can now be replaced.

`Map`/`Set` keep insertion order and compare keys with SameValueZero: objects are compared by identity, so `map.get({ id: 1 })` never finds a previously stored `{ id: 1 }`. Use `WeakMap`/`WeakRef`/`FinalizationRegistry` only for caches keyed by object identity — never for program logic that depends on GC timing.

##### Grouping (Node 21+)

```javascript
const orders = [
  { id: 1, status: 'open', total: 10 },
  { id: 2, status: 'paid', total: 25 },
  { id: 3, status: 'open', total: 7 },
];

const byStatus = Object.groupBy(orders, (o) => o.status);
console.log(Object.keys(byStatus));                       // [ 'open', 'paid' ]
console.log(Object.getPrototypeOf(byStatus) === null);    // true: no hasOwnProperty, toString, ...
console.log(Object.hasOwn(byStatus, 'open'));             // true — use Object.hasOwn, not byStatus.hasOwnProperty

const byBucket = Map.groupBy(orders, (o) => (o.total >= 10 ? 'large' : 'small'));
console.log(byBucket.get('large').map((o) => o.id));      // [ 1, 2 ]
```

`Object.groupBy` returns a **null-prototype** object: `byStatus.hasOwnProperty('open')` throws `TypeError`. Use `Map.groupBy` when keys are not strings (objects, numbers you do not want stringified).

##### Arrays — copying methods and `findLast`

```javascript
const scores = [30, 10, 20];
const sorted = scores.toSorted((a, b) => a - b); // new array, scores unchanged
console.log(sorted, scores);                    // [ 10, 20, 30 ] [ 30, 10, 20 ]
console.log(scores.toReversed(), scores.with(0, 99), scores.toSpliced(1, 1));
console.log(scores.findLast((s) => s > 15), scores.findLastIndex((s) => s > 15)); // 20 2

// Default sort is lexicographic — always pass a comparator for numbers.
console.log([10, 9, 1].toSorted());             // [ 1, 10, 9 ]  (BAD)
console.log([10, 9, 1].toSorted((a, b) => a - b)); // [ 1, 9, 10 ] (GOOD)
```

Prefer `toSorted()`/`toReversed()` over `sort()`/`reverse()` when the input array belongs to the caller (props, function arguments, cached data): mutating it is a classic shared-state bug.

##### Iterator helpers — New in 22

```javascript
function* naturals() {
  let n = 0;
  while (true) yield n++;
}

// Lazy pipelines over any iterator — nothing is materialised until toArray()/for-of.
const firstOddSquares = naturals()
  .filter((n) => n % 2 === 1)
  .map((n) => n * n)
  .take(3)
  .toArray();
console.log(firstOddSquares);                        // [ 1, 9, 25 ]

// Works on Map/Set iterators without an intermediate array.
const totals = new Map([['a', 3], ['b', 9]]);
console.log(totals.values().reduce((sum, v) => sum + v, 0)); // 12

// Iterator.from() wraps iterator-like objects so they gain the helpers.
const it = Iterator.from({ i: 0, next() { return this.i < 2 ? { value: this.i++, done: false } : { done: true, value: undefined }; } });
console.log(it.toArray());                           // [ 0, 1 ]
```

Iterator helpers are **synchronous** only: there are no async iterator helpers in Node 22 (`asyncIterable.map` is `undefined`). Use `for await`, `Array.fromAsync()` or stream utilities (`Readable.prototype.map()` with `concurrency`) for async sequences. `Iterator.concat()` does not exist on 22.

##### ArrayBuffer: transfer and resizable buffers

```javascript
const buf = new ArrayBuffer(8, { maxByteLength: 16 });
const view = new Uint8Array(buf);    // length-tracking view
buf.resize(12);
console.log(view.length);            // 12

const moved = buf.transfer();        // Node 21+: zero-copy move, source is detached
console.log(buf.detached, buf.byteLength, moved.byteLength); // true 0 12

// 22.12+: a Buffer over a resizable ArrayBuffer tracks resizes
const ab = new ArrayBuffer(10, { maxByteLength: 20 });
const b = Buffer.from(ab);
ab.resize(15);
console.log(b.byteLength);           // 15
```

Using a detached buffer throws a `TypeError` on most operations; when code transfers a buffer to a worker (`postMessage(x, [x])`) it must not touch the original afterwards.

##### Classes, private state and static blocks

```javascript
class Counter {
  static #instances = 0;
  #count = 0;

  static {
    // static initialisation block: runs once when the class is evaluated
    Counter.created = new Date(0).toISOString();
  }

  constructor() {
    Counter.#instances += 1;
  }

  increment() {
    this.#count += 1;
    return this.#count;
  }

  static isCounter(value) {
    return #count in value; // ergonomic brand check, no try/catch
  }

  static get instances() {
    return Counter.#instances;
  }
}

const c = new Counter();
c.increment();
console.log(c.increment(), Counter.instances, Counter.isCounter(c), Counter.isCounter({})); // 2 1 true false
console.log(Object.keys(c), JSON.stringify(c)); // [] {} — private fields are invisible to reflection
```

- `#private` fields are real privacy; `_underscore` is convention only. Accessing `#x` on an object without that field throws `TypeError` — a `Proxy` around a class instance breaks private-field access.
- Class fields are defined with `[[Define]]` semantics: a subclass field shadows a base-class accessor of the same name, and fields are initialised **after** `super()` returns, so a base constructor that calls an overridden method sees uninitialised subclass fields.
- **No decorators**: `@decorator` is a `SyntaxError` on every Node version; TypeScript/Babel must transpile them (type stripping does not).

##### Functions, `this` and closures

```javascript
class Button {
  label = 'ok';
  // Arrow function class field: `this` is bound per instance — safe to pass as a callback.
  onClick = () => this.label;
  // Prototype method: `this` depends on the call site.
  describe() { return this?.label; }
}

const b = new Button();
const { onClick, describe } = b;
console.log(onClick(), describe.call(undefined)); // 'ok' undefined (BAD if passed as a callback)

// Default parameters and rest
function createUser({ name, email, role = 'user' } = {}) {
  return { name, email, role };
}
console.log(createUser({ name: 'Ann' }));
```

Do not use `arguments`; use rest parameters. Avoid `Function.prototype.bind` in hot paths when an arrow function suffices. `new Function(…)`/`eval()` on any input is a code-injection finding (and is blocked by `--disallow-code-generation-from-strings`).

##### Objects and immutability

```javascript
const defaults = { retries: 3, headers: { accept: 'application/json' } };
const merged = { ...defaults, retries: 5 };          // shallow copy
merged.headers.accept = 'text/plain';
console.log(defaults.headers.accept);                // 'text/plain' — shared nested object (BAD)

const safe = structuredClone(defaults);              // deep copy (no functions, no class prototypes)
safe.headers.accept = 'text/html';
console.log(defaults.headers.accept, Object.hasOwn(safe, 'retries')); // 'text/plain' true

const frozen = Object.freeze({ a: { b: 1 } });
frozen.a.b = 2;                                       // freeze is shallow
console.log(frozen.a.b);                              // 2
```

`structuredClone()` throws `DataCloneError` for functions, symbols-as-values and class instances lose their prototype. For untrusted JSON, merge into `Object.create(null)` or validate keys before spreading (see Security: prototype pollution).

***

#### Errors

##### Throw `Error` instances, chain with `cause`

```javascript
class AppError extends Error {
  constructor(message, { code, statusCode = 500, cause } = {}) {
    super(message, { cause });
    this.name = new.target.name;   // 'AppError' or the subclass name
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
    JSON.parse('{bad json');
  } catch (err) {
    throw new NotFoundError('user', id, { cause: err }); // keep the original error
  }
}

try {
  loadUser(42);
} catch (err) {
  console.log(err instanceof NotFoundError, err instanceof AppError, err.name, err.code);
  console.log(err.cause instanceof SyntaxError);   // true
}
```

- Throw only `Error` objects: `throw 'x'` or `throw { message }` loses the stack trace and breaks `instanceof`/`err.code` checks. `Error.isError()` is **not** available on 22 (Node 24+); use `err instanceof Error` (or `util.types.isNativeError(err)` across realms).
- `Error.captureStackTrace(this, Ctor)` is unnecessary in subclasses — V8 records the stack in the `Error` constructor; keep it only when you want to hide frames.
- Node's own errors carry a stable `err.code` (`ENOENT`, `ERR_INVALID_ARG_TYPE`, `ECONNRESET`, `ABORT_ERR`). Branch on `code`, never on `message` text.
- `AggregateError` (from `Promise.any`) holds `.errors`; `SuppressedError` does not exist on 22.

##### Error handling review rules

```javascript
// BAD: swallowing errors
async function bad() {
  try { await Promise.reject(new Error('db down')); } catch { /* ignored */ }
}

// BAD: losing context by re-wrapping without cause, or logging *and* rethrowing at every layer
// GOOD: handle where you can act; otherwise add context and rethrow once
async function good() {
  try {
    await Promise.reject(Object.assign(new Error('db down'), { code: 'ECONNREFUSED' }));
  } catch (err) {
    if (err.code === 'ECONNREFUSED') {
      throw new Error('user lookup failed: database unreachable', { cause: err });
    }
    throw err;
  }
}

await bad();
await good().catch((e) => console.log(e.message, '<-', e.cause.code));
```

##### Process-level handlers

```javascript
// Fail fast: log and exit. Do not try to keep serving after an uncaught exception —
// the process may be in an inconsistent state.
process.on('uncaughtException', (err, origin) => {
  console.error(`fatal ${origin}:`, err);
  process.exitCode = 1;
  // flush logs, then exit; a supervisor (systemd, Kubernetes) restarts the process
});

// Unhandled rejections already throw by default (--unhandled-rejections=throw).
// A handler here *replaces* that crash — make sure it still exits.
process.on('unhandledRejection', (reason) => {
  console.error('unhandled rejection:', reason);
  process.exit(1);
});

console.log('handlers installed');
```

Installing an `unhandledRejection` handler that only logs turns crashes into silent corruption. `process.exitCode = n` is preferred over `process.exit(n)` when pending I/O (log flushing, stdout pipes) must complete. Since Node 20, `process.exit(code)`/`process.exitCode` throw `ERR_OUT_OF_RANGE` for non-integer values such as `1.5` (DEP0164 End-of-Life); integer strings like `'1'` are still accepted.

***

#### Modules: ESM, CommonJS and Interop

##### How Node 22 decides the module system

| File | Loaded as |
|---|---|
| `.mjs` | ES module |
| `.cjs` | CommonJS |
| `.js` with nearest `package.json` `"type": "module"` | ES module |
| `.js` with `"type": "commonjs"` | CommonJS |
| `.js` with **no** `"type"` field | CommonJS, unless it fails to parse as CommonJS because of ESM syntax — then it is re-parsed as ESM (**module syntax detection, default since 22.7**) |
| `.ts` / `.mts` / `.cts` | Same rules after type stripping (22.18+ by default) |

Sources: Node 22 packages and ESM documentation.

Syntax detection costs a second parse, and when the nearest `package.json` lacks `"type"`, Node prints a `MODULE_TYPELESS_PACKAGE_JSON` warning for every such file that turns out to be ESM (a script with no `package.json` above it is detected silently). **Always set `"type"` explicitly** in every `package.json` you own; a missing `"type"` is a review finding for new code. `--experimental-default-type` still exists on 22 but is removed in 23 — do not use it.

```json
{
  "name": "my-service",
  "type": "module",
  "engines": { "node": ">=22.12.0" },
  "exports": {
    ".": "./src/index.js",
    "./package.json": "./package.json"
  },
  "imports": {
    "#config": "./src/config.js",
    "#db/*": "./src/db/*.js"
  }
}
```

- `"exports"` encapsulates the package: deep imports not listed there fail with `ERR_PACKAGE_PATH_NOT_EXPORTED`. Adding `"exports"` to an existing library is a breaking change.
- `"imports"` (`#name` specifiers) replaces fragile `../../..` relative paths and works in both module systems. Patterns must start with `#`; the `#/` prefix form (`"#/*"`) is Node 24.14+/25.4+ only — on 22 it fails with `ERR_INVALID_MODULE_SPECIFIER`.
- ESM import specifiers need **full file extensions** (`./util.js`, not `./util`) and cannot import directories; this differs from CommonJS resolution and from bundlers.

##### ESM essentials

```javascript
import { readFile } from 'node:fs/promises';
import path from 'node:path';

// CommonJS globals do not exist in ESM:
console.log(typeof __dirname, typeof require);  // undefined undefined

// Node 20.11+/21.2+ (stable in 22.16): import.meta.dirname / filename
const here = import.meta.dirname;
const pkg = JSON.parse(await readFile(path.join(here, 'package.json'), 'utf8').catch(() => '{}'));
console.log(typeof pkg, import.meta.filename.endsWith('.mjs'));

// import.meta.resolve is synchronous and returns a URL string
console.log(import.meta.resolve('node:fs'));     // 'node:fs'

// import.meta.main (22.18+): true when this module is the process entry point
if (import.meta.main) {
  console.log('running as a script');
}
```

- Always use the `node:` prefix for built-ins (`node:fs`, `node:test`). Some modules are **only** reachable with the prefix: `node:sqlite`, `node:test`, `node:sea` (`require('sqlite')` looks for an npm package).
- Top-level `await` is allowed in ESM. It blocks evaluation of every module that imports this one, and a module graph containing top-level `await` cannot be loaded with `require()`.
- Need `require` inside ESM (for example for a native addon or a JSON file path resolved at runtime)? Use `createRequire(import.meta.url)` from `node:module`.

##### Import attributes and JSON modules

```javascript
// GOOD (stable since 22.12): import attributes use `with`
// import config from './config.json' with { type: 'json' };
const { default: data } = await import('data:application/json,{"ok":true}', { with: { type: 'json' } });
console.log(data.ok);  // true
```

```javascript
// fragment: BAD on Node 22 — import assertions were removed in 22.0 (SyntaxError: Unexpected identifier 'assert')
// import config from './config.json' assert { type: 'json' };
// BAD: omitting the attribute → TypeError [ERR_IMPORT_ATTRIBUTE_MISSING]
// import config from './config.json';
```

`assert { type: 'json' }` still works on Node 20, so code copied from older projects breaks on 22. JSON modules only expose a `default` export (no named exports), and the object is shared and mutable across all importers — treat it as read-only or `structuredClone()` it.

##### Importing CommonJS from ESM

```javascript
// fragment: named imports from CommonJS rely on static analysis (cjs-module-lexer)
// named.cjs:  exports.a = 1; module.exports.c = 3;    → import { a, c } from './named.cjs' works
// obj.cjs:    module.exports = { a: 1 };               → only `default` is detected; import { a } fails
// obj.cjs:    module.exports = someFactory();          → only `default` is detected
// Safe pattern that always works:
// import pkg from './obj.cjs'; const { a } = pkg;
```

The namespace of an imported CommonJS module on Node 22 has `default` (= `module.exports`) plus statically detected names. The extra `'module.exports'` key is Node 23+.

##### New in 22: `require(esm)` (unflagged in 22.12.0)

```javascript
// CommonJS (consumer.cjs) — on 22.12+ this works without flags:
// const lib = require('./lib.mjs');            // returns the module namespace object
// lib.default, lib.namedExport                 // default export is under .default
// lib.__esModule === true                      // added for transpiler interop (22.5)
//
// A module that exports the string name 'module.exports' controls what require() returns:
// export { value as 'module.exports' };       // require() returns `value` itself (22.12)
//
// Top-level await anywhere in the required graph:
// require('./tla.mjs')  → throws ERR_REQUIRE_ASYNC_MODULE
console.log(process.features.require_module); // true on 22.12+
```

Rules a reviewer must check:

- `require()` of an ES module returns the **namespace object**, not the default export: `require('./x.mjs')` is `{ default: fn, … }`, so `require('./x.mjs')()` is a `TypeError`. Use `.default`, or have the ESM module export `'module.exports'`.
- The required graph must be **fully synchronous**. Top-level `await` anywhere in it throws `ERR_REQUIRE_ASYNC_MODULE`; use `await import()` instead. Cycles that cross CJS↔ESM through `require(esm)` throw `ERR_REQUIRE_CYCLE_MODULE`.
- No ExperimentalWarning is printed since 22.13; `--trace-require-module=all` (or `=no-node-modules`) prints one per use for auditing. `--no-experimental-require-module` turns it off, and on Node ≤ 22.11 it needs `--experimental-require-module`.
- Libraries that still ship dual CJS+ESM builds suffer the **dual-package hazard** (two copies of module state, `instanceof` failing across copies). With `require(esm)` available on every maintained line (22.12+, 24, 26 — and on 20.19+ before Node 20's EOL), new libraries can ship ESM only. During a transition, the `"module-sync"` exports condition (22.10+) lets Node load the ESM build for both `require` and `import`:

```json
{
  "type": "module",
  "exports": {
    "module-sync": "./index.js",
    "default": "./dist/index.cjs"
  }
}
```

##### Dynamic `import()` and lazy loading

```javascript
// Load an optional dependency only when needed; handle absence explicitly.
async function loadOptional(name) {
  try {
    return await import(name);
  } catch (err) {
    if (err.code === 'ERR_MODULE_NOT_FOUND') return null;
    throw err;
  }
}
console.log(await loadOptional('definitely-not-installed-pkg'));       // null
console.log(typeof (await loadOptional('node:os')).availableParallelism); // 'function'
```

Never pass user input to `import()`/`require()` — it is code execution.

##### Module customization hooks

```javascript
// New in 22.15: module.registerHooks() — synchronous, in-thread hooks for both require and import.
import { registerHooks } from 'node:module';

const hooks = registerHooks({
  resolve(specifier, context, nextResolve) {
    if (specifier === 'virtual:config') return { url: 'virtual:config', shortCircuit: true };
    return nextResolve(specifier, context);
  },
  load(url, context, nextLoad) {
    if (url === 'virtual:config') {
      return { format: 'module', source: 'export default { env: "test" };', shortCircuit: true };
    }
    return nextLoad(url, context);
  },
});

const { default: cfg } = await import('virtual:config');
console.log(cfg);       // { env: 'test' }
hooks.deregister();
```

- `module.registerHooks()` (22.15, Stability 1.1) runs hooks synchronously on the main thread and also affects `require()`. `module.register()` (20.6+) runs **asynchronous** hooks on a separate loader thread and requires `--allow-worker` under the permission model. Register either from a file preloaded with `--import ./register.js`.
- `--experimental-loader`/`--loader` is discouraged; use `--import` with `register()`/`registerHooks()`.
- Hooks are process-global: a library must not install them as an import side effect.

##### Compile cache (22.1 / 22.8)

```javascript
// Speeds up repeated startups (CLIs, serverless cold starts) by caching V8 code on disk.
import module from 'node:module';
import os from 'node:os';
import path from 'node:path';

const result = module.enableCompileCache(path.join(os.tmpdir(), 'my-cli-compile-cache'));
console.log(result.status === module.constants.compileCacheStatus.ENABLED ||
            result.status === module.constants.compileCacheStatus.ALREADY_ENABLED);
```

Equivalent for end users: `NODE_COMPILE_CACHE=/path node app.js`. The cache is keyed by Node version and file contents; disable it when collecting precise V8 coverage.

***

#### TypeScript in Node 22: Type Stripping

##### Timeline (verify against the pinned minor)

- **22.6.0**: `--experimental-strip-types` — erases type annotations from `.ts`/`.mts`/`.cts`.
- **22.7.0**: `--experimental-transform-types` — additionally transforms enums, namespaces and parameter properties (emits a warning; removed again in Node 26).
- **22.13/22.14**: `module.stripTypeScriptTypes()`; TypeScript via `--eval`/stdin; `ERR_UNSUPPORTED_TYPESCRIPT_SYNTAX`.
- **22.18.0**: **type stripping on by default, without an ExperimentalWarning**; disable with `--no-experimental-strip-types`. Stability: 1.2 (release candidate) in 22.20+.

On 22.23.4 `process.features.typescript` is `'strip'`; with `--experimental-transform-types` it is `'transform'`; with `--no-experimental-strip-types` it is `false`.

##### What runs and what fails

```typescript
// a.ts — runs with `node a.ts` on 22.18+
import type { Foo } from './types.ts';    // `import type` is required for type-only imports
import { helper } from './helper.ts';     // explicit .ts extension is required
const f: Foo = { n: 41 };
console.log(helper(f.n));
```

Verified failures on 22.23.4:

| Code | Error |
|---|---|
| `enum Color { Red }` | `ERR_UNSUPPORTED_TYPESCRIPT_SYNTAX: TypeScript enum is not supported in strip-only mode` |
| `constructor(private x: number) {}` | `ERR_UNSUPPORTED_TYPESCRIPT_SYNTAX` (parameter property) |
| `namespace N { export const x = 1 }` (with runtime code) | `ERR_UNSUPPORTED_TYPESCRIPT_SYNTAX` |
| `import { Foo } from './types.ts'` where `Foo` is an interface | `SyntaxError: The requested module './types.ts' does not provide an export named 'Foo'` |
| `import { helper } from './helper'` (no extension) | `ERR_MODULE_NOT_FOUND` |
| a `.ts` file inside `node_modules` | `ERR_UNSUPPORTED_NODE_MODULES_TYPE_STRIPPING` |
| decorators, `tsconfig.json` `paths` aliases, JSX/`.tsx` | not supported (no tsconfig is read at all) |

Recommended `tsconfig.json` for code executed directly by Node 22 (TypeScript ≥ 5.8 for `erasableSyntaxOnly`):

```json
{
  "compilerOptions": {
    "target": "es2023",
    "lib": ["es2024", "esnext.array", "esnext.collection", "esnext.iterator"],
    "module": "nodenext",
    "moduleResolution": "nodenext",
    "noEmit": true,
    "allowImportingTsExtensions": true,
    "rewriteRelativeImportExtensions": true,
    "erasableSyntaxOnly": true,
    "verbatimModuleSyntax": true,
    "strict": true
  }
}
```

`erasableSyntaxOnly` (TypeScript 5.8) makes `tsc` reject enums and parameter properties (TS1294) and `verbatimModuleSyntax` rejects type imports without `type` (TS1484) — the exact constructs Node cannot strip. Type stripping never type-checks: CI must still run `tsc --noEmit`. Publish compiled `.js` + `.d.ts` to npm; Node refuses to strip types under `node_modules`. The `lib` list matches V8 12.4: `es2024` adds `Object.groupBy`/`Promise.withResolvers`, the three `esnext.*` entries add `Array.fromAsync`, Set methods and iterator helpers, while `Promise.try` and `RegExp.escape` stay type errors (TS2339). `lib: ["esnext"]` would let them through. Note that `tsc` happily *downlevels* `using` declarations, but type stripping passes them to V8 unchanged — `using x = …` in a `.ts` file run by Node 22 is a `SyntaxError`.

***

#### Asynchronous Programming

##### Event loop ordering

```javascript
const order = [];
setTimeout(() => order.push('timeout'), 0);
setImmediate(() => order.push('immediate'));
Promise.resolve().then(() => order.push('promise microtask'));
queueMicrotask(() => order.push('queueMicrotask'));
process.nextTick(() => order.push('nextTick'));
order.push('sync');

await new Promise((resolve) => setTimeout(() => setImmediate(resolve), 10));
console.log(order.join(' → '));
// In an ES module: sync → promise microtask → queueMicrotask → nextTick → timeout/immediate (order of these two varies)
```

In ESM the module body itself runs inside a microtask, so `process.nextTick` callbacks run **after** already-queued promise callbacks; in CommonJS `nextTick` runs first. Do not write code whose correctness depends on that order. Prefer `queueMicrotask()` over `process.nextTick()` in new code. A recursive `nextTick`/microtask loop starves I/O completely.

##### Sequential vs. concurrent awaits

```javascript
const delay = (ms, v) => new Promise((r) => setTimeout(r, ms, v));

// BAD: independent operations awaited one after another (≈300 ms)
let t = performance.now();
const a = await delay(100, 'a');
const b = await delay(100, 'b');
const c = await delay(100, 'c');
console.log('sequential', Math.round(performance.now() - t) >= 290, a + b + c);

// GOOD: start them together (≈100 ms)
t = performance.now();
const [x, y, z] = await Promise.all([delay(100, 'x'), delay(100, 'y'), delay(100, 'z')]);
console.log('parallel', Math.round(performance.now() - t) < 200, x + y + z);

// When partial failure is acceptable, use allSettled and inspect each result.
const results = await Promise.allSettled([delay(1, 1), Promise.reject(new Error('boom'))]);
console.log(results.map((r) => r.status)); // [ 'fulfilled', 'rejected' ]
```

- `Promise.all` rejects on the first failure but does **not** cancel the others; they keep running and their later rejections are silently handled by `Promise.all`. Pass an `AbortSignal` if the remaining work must stop.
- `Promise.race` with a timeout promise leaves the timer pending; use `AbortSignal.timeout()` instead.
- `async` callbacks in `array.forEach()` are fire-and-forget: errors become unhandled rejections and the caller does not wait. Use `for…of` with `await` (sequential) or `Promise.all(array.map(async …))` (concurrent).

```javascript
const ids = [1, 2, 3];
const fakeFetch = async (id) => ({ id });

// BAD: forEach does not await; `out` is still empty when logged
const out = [];
ids.forEach(async (id) => out.push(await fakeFetch(id)));
console.log(out.length);   // 0

// GOOD
const users = await Promise.all(ids.map((id) => fakeFetch(id)));
console.log(users.length); // 3
```

##### Bounded concurrency

`Promise.all` over thousands of items opens thousands of sockets/file handles at once (`EMFILE`, rate limits, memory spikes). Bound it:

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

let running = 0, peak = 0;
const out = await mapLimit([...Array(20).keys()], 4, async (n) => {
  running++; peak = Math.max(peak, running);
  await new Promise((r) => setTimeout(r, 5));
  running--;
  return n * 2;
});
console.log(out.at(-1), 'peak concurrency', peak); // 38 'peak concurrency' 4
```

For streams of work, `Readable.from(iterable).map(fn, { concurrency: n })` gives the same bound with backpressure (see Streams).

##### New in 22: `Promise.withResolvers()` and `Array.fromAsync()`

```javascript
import { EventEmitter } from 'node:events';

// Promise.withResolvers replaces the "deferred" pattern with outer `let resolve`.
function waitForEvent(emitter, name) {
  const { promise, resolve, reject } = Promise.withResolvers();
  emitter.once(name, resolve);
  emitter.once('error', reject);
  return promise;
}
const ee = new EventEmitter();
setTimeout(() => ee.emit('ready', 42), 1);
console.log(await waitForEvent(ee, 'ready'));     // 42

// Array.fromAsync collects an async iterable (sequentially — it awaits one item at a time).
async function* pages() {
  for (let p = 1; p <= 3; p++) yield { page: p };
}
console.log((await Array.fromAsync(pages())).length); // 3
console.log(await Array.fromAsync([Promise.resolve(1), 2], (v) => v * 10)); // [ 10, 20 ]
```

`Promise.try()` is **not** available on 22 (Node 23+). Write `new Promise((resolve) => resolve(fn()))` or `(async () => fn())()` to turn synchronous throws into rejections.

The `waitForEvent` helper above leaks the listener that did not fire; for production code prefer `events.once(emitter, name, { signal })`, which removes both listeners and rejects on `'error'`.

##### Cancellation with `AbortSignal`

```javascript
import { once } from 'node:events';
import { EventEmitter } from 'node:events';
import { setTimeout as sleep } from 'node:timers/promises';

// AbortSignal.timeout(ms): a signal that aborts with a TimeoutError DOMException
try {
  await sleep(1_000, null, { signal: AbortSignal.timeout(10) });
} catch (err) {
  console.log(err.name, err.code); // AbortError ABORT_ERR (the timer reports AbortError; reason is TimeoutError)
}

// AbortSignal.any(): abort when *either* the caller cancels or the deadline passes
const userCancel = new AbortController();
const signal = AbortSignal.any([userCancel.signal, AbortSignal.timeout(5_000)]);
setTimeout(() => userCancel.abort(new Error('client went away')), 5);
try {
  await once(new EventEmitter(), 'never', { signal });
} catch (err) {
  console.log(err.name, signal.reason.message); // AbortError client went away
}
```

```javascript
// fetch with a deadline: undici aborts the request and rejects with the signal's reason
try {
  await fetch('http://10.255.255.1/', { signal: AbortSignal.timeout(50) });
} catch (err) {
  console.log(err.name); // TimeoutError
}
```

Review checklist for cancellation: every outbound call (`fetch`, database client, child process) needs a deadline; long-lived listeners added to a shared signal must be removed (`{ once: true }` or `events.addAbortListener()`, which returns a disposable), otherwise each request leaks a listener on a long-lived controller and Node prints `MaxListenersExceededWarning`.

##### Timers

```javascript
import { setTimeout as sleep, setInterval as every } from 'node:timers/promises';

await sleep(5);                                   // promise-based delay

let ticks = 0;
for await (const _ of every(2)) {                 // async-iterable interval
  if (++ticks === 3) break;                       // break clears the interval
}
console.log('ticks', ticks);

const t = setInterval(() => {}, 60_000);
t.unref();                                        // does not keep the process alive
clearInterval(t);
```

- Timer delays are clamped to `[1, 2^31-1]` ms; a delay above ~24.8 days fires after 1 ms (Node 22 prints `TimeoutOverflowWarning`; a negative delay is silently treated as 1 ms — the `TimeoutNegativeWarning` is Node 23+).
- `setInterval` with an `async` callback does not wait for the previous run; overlapping executions are a common bug. Use the `for await (… of setInterval())` form or reschedule with `setTimeout` at the end of each run.

##### Async iteration and events

```javascript
import { on, EventEmitter } from 'node:events';

const ee = new EventEmitter();
const ac = new AbortController();
setTimeout(() => { ee.emit('data', 1); ee.emit('data', 2); ac.abort(); }, 1);

const seen = [];
try {
  for await (const [value] of on(ee, 'data', { signal: ac.signal })) seen.push(value);
} catch (err) {
  if (err.name !== 'AbortError') throw err;
}
console.log(seen); // [ 1, 2 ]
```

- An `EventEmitter` that emits `'error'` with no listener **throws** (and usually crashes the process). Every stream, socket, server, child process and custom emitter needs an `'error'` handler or must be consumed via `pipeline()`/`once()`/`on()`, which convert errors into rejections.
- Async listeners: a rejected promise returned from a listener becomes an unhandled rejection unless the emitter is created with `{ captureRejections: true }`.

##### `AsyncLocalStorage` for request context

```javascript
import { AsyncLocalStorage } from 'node:async_hooks';

const requestContext = new AsyncLocalStorage();

function log(msg) {
  const ctx = requestContext.getStore();
  console.log(`[${ctx?.requestId ?? '-'}] ${msg}`);
}

async function handle(requestId) {
  return requestContext.run({ requestId }, async () => {
    await new Promise((r) => setTimeout(r, Math.random() * 5));
    log('handled');              // correct id even with interleaved requests
  });
}

await Promise.all([handle('a'), handle('b')]);
log('outside');                  // [-] outside
```

- The store propagates through `await`, timers and promise callbacks. A `.then()` callback sees the context of the code that **registered** it, not of the code that resolved the promise.
- `EventEmitter` listeners run in the context of `emit()`, not of the code that added them; use `AsyncResource.bind()` / `AsyncLocalStorage.bind()` when that matters.
- Since 22.7, `AsyncLocalStorage` no longer uses `async_hooks` internally; the faster `AsyncContextFrame` implementation is opt-in on 22 (`--experimental-async-context-frame`) and default only from Node 24. `als.withScope()` and the `name`/`defaultValue` options are Node 24+.
- Do not use `async_hooks.createHook()` in application code (Stability 1, high overhead); `AsyncLocalStorage` and `diagnostics_channel` cover the use cases.

##### CPU-bound work blocks everything

```javascript
import { Worker } from 'node:worker_threads';

// BAD in a server: synchronous CPU work on the main thread (blocks all requests)
// const hash = crypto.pbkdf2Sync(password, salt, 600_000, 64, 'sha512');

// GOOD: async crypto APIs use the libuv thread pool; bespoke CPU work goes to a Worker
const worker = new Worker(
  `const { parentPort, workerData } = require('node:worker_threads');
   let sum = 0; for (let i = 0; i < workerData; i++) sum += i;
   parentPort.postMessage(sum);`,
  { eval: true, workerData: 1e6 },
);
const [result] = await (await import('node:events')).once(worker, 'message');
console.log(result);              // 499999500000
await worker.terminate();
```

Other main-thread blockers to flag in request handlers: `fs.*Sync`, `child_process.execSync`/`spawnSync`, `zlib.*Sync` on large inputs, `JSON.parse`/`JSON.stringify` of multi-megabyte payloads, catastrophic regexes, and `crypto.*Sync` key derivation. The libuv thread pool has 4 threads by default (`UV_THREADPOOL_SIZE`); `fs`, `dns.lookup`, async `crypto` and `zlib` share it, so a burst of `pbkdf2` calls also delays file I/O.

***

#### Buffers, Text and Binary Data

```javascript
import { StringDecoder } from 'node:string_decoder';

// Encoding conversions (Uint8Array.fromBase64/toBase64 are Node 25+; use Buffer on 22)
const b = Buffer.from('héllo', 'utf8');
console.log(b.length, b.toString('base64url'), Buffer.from('aMOpbGxv', 'base64').toString()); // 6 aMOpbGxv héllo

// BAD: decoding chunk by chunk splits multi-byte characters
const chunks = [b.subarray(0, 2), b.subarray(2)];
console.log(chunks.map((c) => c.toString('utf8')).join(''));        // 'h��llo' — two U+FFFD replacement characters
// GOOD: a streaming decoder keeps partial sequences between chunks
const dec = new StringDecoder('utf8');
console.log(chunks.map((c) => dec.write(c)).join('') + dec.end());   // héllo
const td = new TextDecoder();
console.log(chunks.map((c) => td.decode(c, { stream: true })).join('') + td.decode()); // héllo

// Buffer.alloc zero-fills; allocUnsafe may expose old memory
console.log(Buffer.alloc(4).every((x) => x === 0));                   // true

// subarray/slice share memory with the original
const view = b.subarray(0, 1);
view[0] = 0x48;
console.log(b.toString());                                           // Héllo
```

- `buf.slice()` on a `Buffer` is a view (unlike `Array#slice` and `Uint8Array#slice`, which copy); it is doc-deprecated in favour of `subarray()` precisely because of that confusion.
- Compare binary secrets with `crypto.timingSafeEqual`, ordinary buffers with `buf.equals()`/`Buffer.compare()` — never `==`.
- `JSON.parse` of untrusted input: wrap in `try/catch`, bound the input size first, and validate the result's shape; `JSON.parse` never executes code but large or deeply nested payloads block the event loop.

***

#### Streams

##### New in 22: default `highWaterMark` is 64 KiB

Node 22.0 raised the default buffer threshold of byte streams from 16 KiB to **64 KiB** (object-mode streams stay at 16 objects). Throughput improves, but every buffered stream may hold four times more memory before backpressure kicks in. Services that keep thousands of concurrent streams (proxies, WebSocket fan-out) should measure and, if needed, set `highWaterMark` per stream or globally with `stream.setDefaultHighWaterMark(false, 16 * 1024)`.

```javascript
import { getDefaultHighWaterMark, Readable } from 'node:stream';

console.log(getDefaultHighWaterMark(false), getDefaultHighWaterMark(true)); // 65536 16 (Node 20: 16384 16)
console.log(new Readable({ read() {} }).readableHighWaterMark);              // 65536
```

##### Always use `pipeline()`

```javascript
import { pipeline } from 'node:stream/promises';
import { createReadStream, createWriteStream } from 'node:fs';
import { writeFile, readFile, rm, mkdtemp } from 'node:fs/promises';
import { createGzip, createGunzip } from 'node:zlib';
import { tmpdir } from 'node:os';
import path from 'node:path';

const dir = await mkdtemp(path.join(tmpdir(), 'pipe-'));
const src = path.join(dir, 'in.txt');
await writeFile(src, 'line\n'.repeat(10_000));

// BAD: .pipe() does not forward errors or destroy the other streams on failure
// createReadStream(src).pipe(createGzip()).pipe(createWriteStream(dst));

// GOOD: pipeline propagates errors, destroys all streams and supports AbortSignal
await pipeline(
  createReadStream(src),
  createGzip(),
  createWriteStream(path.join(dir, 'in.txt.gz')),
  { signal: AbortSignal.timeout(5_000) },
);

// Async generator stages are first-class transform steps
let lines = 0;
await pipeline(
  createReadStream(path.join(dir, 'in.txt.gz')),
  createGunzip(),
  async function* (source) {
    for await (const chunk of source) lines += chunk.toString().split('\n').length - 1;
  },
);
console.log(lines);                       // 10000
await rm(dir, { recursive: true });
```

- `stream.pipeline` (callback) and `stream/promises` `pipeline` wait for the destination's `'close'` before completing (22.5); use `finished()` from `node:stream/promises` to await the end (or failure) of a single stream.
- Writing in a loop must respect backpressure: if `writable.write()` returns `false`, wait for `'drain'` (`await once(stream, 'drain')`). Ignoring it buffers unboundedly.
- Readable streams are async iterables; `for await (const chunk of readable)` handles backpressure and destroys the stream on `break`/`throw`.
- Line-by-line reading: `readline.createInterface({ input, crlfDelay: Infinity })` is async iterable.

##### Stream operators with concurrency

```javascript
import { Readable } from 'node:stream';

const ids = Readable.from([1, 2, 3, 4, 5, 6]);
const results = await ids
  .map(async (id) => {
    await new Promise((r) => setTimeout(r, 5));
    return id * 10;
  }, { concurrency: 3 })            // at most 3 in flight, order preserved
  .filter((n) => n > 20)
  .toArray();
console.log(results);               // [ 30, 40, 50, 60 ]
```

`Readable.prototype.map/filter/flatMap/forEach/reduce/take/drop/toArray/some/every/find` exist on 22 (still marked experimental). They are the async counterpart of iterator helpers.

##### Web Streams interop

```javascript
import { Readable } from 'node:stream';
import { text } from 'node:stream/consumers';

const web = new ReadableStream({
  start(controller) { controller.enqueue('hello '); controller.enqueue('web'); controller.close(); },
});
const node = Readable.fromWeb(web, { encoding: 'utf8' });
console.log(await text(node));                                    // hello web

const backToWeb = Readable.toWeb(Readable.from(['a', 'b']));
console.log(backToWeb instanceof ReadableStream);                 // true

// CompressionStream: gzip/deflate everywhere; 'brotli' added in 22.20 (Node-specific)
const compressed = new Blob(['x'.repeat(1000)]).stream().pipeThrough(new CompressionStream('gzip'));
console.log((await new Response(compressed).arrayBuffer()).byteLength < 100); // true
```

`stream/consumers` offers `text()`, `json()`, `buffer()`, `arrayBuffer()`, `blob()` (no `bytes()` until Node 24). `Readable.fromWeb`/`toWeb`, `Duplex.fromWeb`/`toWeb` and `Writable.fromWeb`/`toWeb` are stable since 22.17.

***

#### File System

##### Promises API, encodings and error codes

```javascript
import { readFile, writeFile, mkdir, rm, stat, mkdtemp, rename } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import path from 'node:path';

const dir = await mkdtemp(path.join(tmpdir(), 'fs-demo-'));
await mkdir(path.join(dir, 'a/b'), { recursive: true });           // no error if it exists

// Atomic replace: write to a temp file in the same directory, then rename
const target = path.join(dir, 'config.json');
const tmp = `${target}.${process.pid}.tmp`;
await writeFile(tmp, JSON.stringify({ v: 1 }), { encoding: 'utf8', mode: 0o600 });
await rename(tmp, target);

console.log(JSON.parse(await readFile(target, 'utf8')).v);         // 1 — always pass an encoding for text

try {
  await stat(path.join(dir, 'missing'));
} catch (err) {
  if (err.code !== 'ENOENT') throw err;                            // branch on err.code
  console.log('missing as expected');
}
await rm(dir, { recursive: true, force: true });
```

- Check-then-act (`existsSync()` followed by `readFile`/`open`) is a race (TOCTOU). Just perform the operation and handle `ENOENT`/`EEXIST`; for exclusive creation use the `'wx'` flag.
- `fs.exists()` (callback) is deprecated; `existsSync()` remains (passing non-string/Buffer/URL types is doc-deprecated on 22 and warns on 24).
- `fs.rmdir(path, { recursive: true })` warns (DEP0147); use `fs.rm(path, { recursive: true, force: true })`.
- Use `fs.cp(src, dest, { recursive: true })` (stable since 22.3) instead of shelling out to `cp -r`.
- `FileHandle`s must be closed (`try/finally { await fh.close() }`); relying on GC to close them prints a warning (DEP0137). `using`/`await using` is not available on 22.

##### New in 22: `fs.glob()`

```javascript
import { glob, readdir } from 'node:fs/promises';
import { globSync } from 'node:fs';
import path from 'node:path';

// Async iterator of relative paths (stable since 22.17; exclude patterns since 22.14)
const files = [];
for await (const f of glob('**/*.{js,mjs}', { cwd: import.meta.dirname, exclude: ['node_modules/**'] })) {
  files.push(f);
}
console.log(files.length >= 1);

// withFileTypes yields Dirent objects (22.2)
const [first] = await Array.fromAsync(glob('*.mjs', { cwd: import.meta.dirname, withFileTypes: true }));
console.log(first.isFile(), typeof first.parentPath);   // true 'string'

console.log(globSync('*.mjs', { cwd: import.meta.dirname }).length >= 1);
console.log(path.matchesGlob('src/lib/a.js', 'src/**/*.js'));  // true (22.5, stable 22.20)

// Recursive directory listing without globbing (Node 20.1+)
const all = await readdir(import.meta.dirname, { recursive: true });
console.log(Array.isArray(all));
```

`fs.glob` replaces the `glob`/`fast-glob` packages for simple cases. It does not read `.gitignore`. Returned paths are relative to `cwd`; join them with `path.join(cwd, f)`. Use `dirent.parentPath` (not the deprecated `dirent.path`).

##### Path handling

```javascript
import path from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';

console.log(path.join('/srv/uploads', '../etc/passwd'));      // /srv/etc/passwd — join does NOT sandbox
console.log(path.resolve('/srv/uploads', '/etc/passwd'));     // /etc/passwd — absolute segment wins

// Convert between file URLs and paths with the url helpers, never string slicing
const p = fileURLToPath(import.meta.url);
console.log(pathToFileURL(p).href === import.meta.url);       // true
```

See Security → Path traversal for the containment check.

***

#### HTTP, `fetch` and WebSocket

##### `fetch` (undici 6.x)

Node 22.23.4 bundles undici 6.29.1; `fetch`, `Request`, `Response`, `Headers`, `FormData` and `WebSocket` are globals.

```javascript
import { createServer } from 'node:http';

const server = createServer((req, res) => {
  res.writeHead(req.url === '/missing' ? 404 : 200, { 'content-type': 'application/json' });
  res.end(JSON.stringify({ path: req.url }));
}).listen(0, '127.0.0.1');
await new Promise((r) => server.once('listening', r));
const base = `http://127.0.0.1:${server.address().port}`;

async function getJson(url, { signal } = {}) {
  const res = await fetch(url, { signal: signal ?? AbortSignal.timeout(5_000), headers: { accept: 'application/json' } });
  if (!res.ok) {
    await res.body?.cancel();                         // release the connection
    throw new Error(`GET ${url} failed: ${res.status}`, { cause: { status: res.status } });
  }
  return res.json();
}

console.log(await getJson(`${base}/ok`));             // { path: '/ok' }
await getJson(`${base}/missing`).catch((e) => console.log(e.message, e.cause.status));
server.close();
```

- `fetch` resolves on **any** HTTP status; a 404/500 is not a rejection. Check `res.ok`/`res.status`.
- A network failure rejects with `TypeError: fetch failed`; the real reason (`ECONNREFUSED`, `ENOTFOUND`, `UND_ERR_CONNECT_TIMEOUT`) is on `err.cause`. Log `err.cause`, not just `err.message`.
- There is **no overall request timeout**; undici's defaults are 10 s to connect and 300 s each for headers and body. Always pass `signal`.
- Always consume or cancel the body (`await res.text()`, `res.body?.cancel()`): undici does not release the connection of an unread body promptly, which reduces connection reuse and can stall the pool.
- `fetch` ignores `HTTP_PROXY`/`HTTPS_PROXY` **unless** `NODE_USE_ENV_PROXY=1` or `--use-env-proxy` is set (22.21+; undici prints an `EnvHttpProxyAgent is experimental` warning on 22). Verified: without the variable the request goes direct even when `HTTP_PROXY` is set; with it, undici 6 tunnels even plain `http://` requests through `CONNECT host:80` (whereas `http.request` with `proxyEnv` sends an absolute-form `GET`), and a proxy that refuses `CONNECT` makes `fetch` retry until its signal aborts — another reason every `fetch` needs a deadline.
- Undici is a separate HTTP stack from `node:http`: `http.globalAgent`/`http.Agent` options and the `diagnostics_channel` `http.client.*` events do not apply to `fetch` (it publishes its own `undici:*` channels). Configure pooling with an undici `Dispatcher` instead.

##### `node:http` server defaults and hardening

```javascript
import http from 'node:http';

const server = http.createServer({
  requestTimeout: 30_000,        // default 300_000 ms: whole request must arrive within this
  headersTimeout: 10_000,        // default 60_000 ms (slowloris protection)
  keepAliveTimeout: 5_000,       // default 5_000 ms; 22.19+ adds keepAliveTimeoutBuffer (default 1_000 ms)
  maxHeaderSize: 16 * 1024,      // default 16 KiB
}, (req, res) => {
  res.setHeader('content-type', 'text/plain; charset=utf-8');
  res.end('ok');
});
server.listen(0, () => {
  console.log(server.requestTimeout, server.headersTimeout, server.keepAliveTimeout, server.keepAliveTimeoutBuffer);
  server.close();
});
```

- Behind a load balancer, `keepAliveTimeout` must be **longer** than the balancer's idle timeout or clients see intermittent `ECONNRESET`/502.
- Read request bodies with a size limit; `for await (const chunk of req)` without a cap is a memory-exhaustion vector.
- `res.writeHead()` / `res.setHeader()` with user-controlled values: Node rejects CR/LF (`ERR_INVALID_CHAR`), but redirects (`Location`) built from user input are still open-redirect findings.
- `req.headersDistinct`/`trailersDistinct` objects have a null prototype since 22.22.2 (CVE-2026-21710); do not call `.hasOwnProperty` on them.
- Graceful shutdown: `server.close()` stops accepting connections, closes idle keep-alive connections (since Node 19) and waits for in-flight requests; call `server.closeAllConnections()` after a deadline so slow clients cannot block exit forever.
- `http.createServer({ shouldUpgradeCallback })` (22.21) decides per request whether an `Upgrade` is accepted; without an `'upgrade'` listener, upgrade requests are treated as normal requests.
- Calling `node:http` constructors (`IncomingMessage`, `ServerResponse`, `Agent`, …) without `new` is doc-deprecated since 22.17.
- `http.Agent({ proxyEnv: process.env })` (22.21) routes `http.request` through `HTTP_PROXY`/`HTTPS_PROXY`/`NO_PROXY`; with `NODE_USE_ENV_PROXY=1` the global agent does so automatically. `http.setGlobalProxyFromEnv()` is Node 24+.

##### New in 22: global `WebSocket` client

```javascript
import http from 'node:http';
import { createHash } from 'node:crypto';

// Minimal server that accepts the upgrade and sends one text frame (for the demo only).
const server = http.createServer();
server.on('upgrade', (req, socket) => {
  const accept = createHash('sha1')
    .update(req.headers['sec-websocket-key'] + '258EAFA5-E914-47DA-95CA-C5AB0DC85B11')
    .digest('base64');
  socket.write(`HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Accept: ${accept}\r\n\r\n`);
  const msg = Buffer.from('hello');
  socket.end(Buffer.concat([Buffer.from([0x81, msg.length]), msg]));
});
await new Promise((r) => server.listen(0, '127.0.0.1', r));

const ws = new WebSocket(`ws://127.0.0.1:${server.address().port}`);
const message = await new Promise((resolve, reject) => {
  ws.addEventListener('message', (ev) => resolve(ev.data), { once: true });
  ws.addEventListener('error', () => reject(new Error('websocket error')), { once: true });
});
console.log(message);   // hello
ws.close();
server.close();
```

- The `WebSocket` global (browser API, undici implementation) is enabled by default since 22.0 and stable since 22.4. It is a **client** only — servers still need a library (`ws`) or a framework.
- Errors arrive as `error` events (`ev.message`), not exceptions; reconnect logic, heartbeats (pings are not exposed) and backpressure (`ws.bufferedAmount`) are the application's job.
- `EventSource` (Server-Sent Events client) exists but requires `--experimental-eventsource` on 22.

***

#### Workers and Child Processes

##### `worker_threads`

```javascript
import { Worker, isMainThread } from 'node:worker_threads';
import { once } from 'node:events';

const source = `
  const { parentPort } = require('node:worker_threads');
  parentPort.on('message', ({ n }) => {
    if (n < 0) throw new RangeError('negative');
    parentPort.postMessage(n * n);
  });
`;

const worker = new Worker(source, { eval: true, resourceLimits: { maxOldGenerationSizeMb: 64 } });
worker.postMessage({ n: 7 });
const [squared] = await once(worker, 'message');
console.log(isMainThread, squared);           // true 49

worker.postMessage({ n: -1 });
const [err] = await once(worker, 'error');    // uncaught errors in the worker surface here
console.log(err.name, err.message);           // RangeError negative
await worker.terminate();                     // 22.18+: Worker also has [Symbol.asyncDispose]()
```

- Each worker is a full V8 isolate with its own heap and event loop; use a fixed-size pool (e.g. `piscina`) for request-driven work instead of a worker per request.
- Messages are copied with the structured-clone algorithm; transfer large `ArrayBuffer`s (`postMessage(buf, [buf])`) or share `SharedArrayBuffer` + `Atomics` to avoid copies. Functions and class prototypes do not survive cloning; `worker_threads.markAsUncloneable(obj)` (22.10) makes accidental cloning throw.
- Always listen for `'error'` and `'exit'`; a crashed worker otherwise disappears silently and callers wait forever.

##### `child_process` safety

```javascript
import { execFile, spawn } from 'node:child_process';
import { promisify } from 'node:util';
const execFileAsync = promisify(execFile);

const userInput = 'file.txt; rm -rf ~';

// BAD: a shell interprets metacharacters in user input (command injection)
// exec(`wc -c ${userInput}`)
// spawn('wc', ['-c', userInput], { shell: true })   // doc-deprecated pattern (DEP0190)

// GOOD: no shell, arguments passed as an array, bounded time and output
const { stdout } = await execFileAsync('echo', ['--', userInput], { timeout: 5_000, maxBuffer: 1024 * 1024 });
console.log(stdout.trim());                      // -- file.txt; rm -rf ~ (printed literally)

const child = spawn('node', ['-e', 'process.exit(3)'], { stdio: 'inherit' });
const [code] = await (await import('node:events')).once(child, 'exit');
console.log('exit code', code);                  // 3
```

- `exec()`/`execSync()` always use a shell. Prefer `execFile`/`spawn` without `shell`. `options.shell = ''` is doc-deprecated (22.17).
- Spawning `.bat`/`.cmd` files on Windows without `shell: true` fails with `EINVAL` since the April 2024 fix for CVE-2024-27980 (part of 22.0.0; the bypass CVE-2024-36138 was fixed in 22.4.1). With `shell: true` the arguments must be escaped manually (not executable on the Linux test machine; documented behaviour).
- Pass a minimal `env` to children; `process.env` may contain secrets. With the permission model, `--allow-child-process` is required and child processes inherit the permission flags (22.18).
- `process.execve()` (22.15; not available on Windows or IBM i) replaces the current process image; it does not return.

***

#### `node:sqlite` — Built-in SQLite (New in 22)

`node:sqlite` arrived in 22.5 behind `--experimental-sqlite`, is **unflagged since 22.13**, and is still **Stability 1.1 (active development)**: every process that loads it prints `ExperimentalWarning: SQLite is an experimental feature`. It is synchronous (`DatabaseSync`) — fine for CLIs, tests and small services, but each query blocks the event loop.

```javascript
import { DatabaseSync } from 'node:sqlite';

const db = new DatabaseSync(':memory:', { timeout: 1_000 });   // busy timeout in ms (option added in 22.16)
try {
  db.exec(`CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT NOT NULL, quota INTEGER) STRICT`);

  // GOOD: prepared statements with bound parameters
  const insert = db.prepare('INSERT INTO users (name, quota) VALUES (?, ?)');
  console.log(insert.run('alice', 2n ** 60n));       // { lastInsertRowid: 1, changes: 1 }

  const byName = db.prepare('SELECT id, name, quota FROM users WHERE name = :name');
  const hostile = "x' OR '1'='1";
  console.log(byName.all({ name: hostile }).length); // 0 — no injection

  // Integers above 2^53 throw unless BigInt reading is enabled
  try { byName.get({ name: 'alice' }); } catch (e) { console.log(e.code); }   // ERR_OUT_OF_RANGE
  byName.setReadBigInts(true);
  const row = byName.get({ name: 'alice' });
  console.log(row.quota, Object.getPrototypeOf(row) === null);               // 1152921504606846976n true

  // Transactions are explicit SQL
  db.exec('BEGIN');
  try {
    insert.run('bob', 1);
    db.exec('COMMIT');
  } catch (err) {
    db.exec('ROLLBACK');
    throw err;
  }
  console.log([...db.prepare('SELECT name FROM users ORDER BY id').iterate()].map((r) => r.name)); // [ 'alice', 'bob' ]

  // undefined cannot be bound on 22 (Node 26 binds it as NULL) — pass null explicitly
  try { insert.run('carol', undefined); } catch (e) { console.log(e.code); }  // ERR_INVALID_ARG_TYPE
} finally {
  db.close();                                         // no `using` on 22
}
```

- **BAD**: building SQL with template literals (`db.exec(\`… '${name}'\`)`). `node:sqlite` has no tagged-template API on 22 (`createTagStore` is Node 24+); always `prepare()` with `?` or `:named` parameters.
- Rows are **null-prototype objects**; `row.hasOwnProperty()` throws.
- `readBigInts` can also be set per database (`new DatabaseSync(path, { readBigInts: true })`, 22.18).
- `allowExtension`/`enableLoadExtension()` lets SQL load native code; keep it off (extensions cannot be loaded at all under `--permission`).
- **The permission model does not cover `node:sqlite`**: verified on 22.23.4, `node --permission` with no `--allow-fs-*` flags still opens, creates and writes `new DatabaseSync('/any/path.db')`. The docs list this as a known gap; validate database paths yourself.
- Not on 22: `StatementSync.prototype.close()`, `db.limits`, `db.setAuthorizer()`, tagged-template statements (all present on 24.22.1, absent on 22.23.4).

***

#### Security

##### Keep the runtime patched

Node 22 is in maintenance: fixes arrive only as patch releases, frequently as coordinated security releases. 2026 security releases for 22.x fixed, among others: TLS server-identity checks and session reuse bound to the wrong host (22.23.0), HTTP/2 memory exhaustion via header accounting and `RST_STREAM` handling (22.23.2), permission-model bypasses (`realpathSync`, `fs/promises`, trace events, report paths, `FileHandle.utimes`), `http.Agent` response-queue poisoning, NUL bytes in DNS/hostnames, `headersDistinct` prototype pollution (22.22.2), hash-collision DoS (22.22.2), a race that could make `Buffer.alloc()`/`Uint8Array` return non-zero-filled memory (CVE-2025-55131, 22.22.0) and a brotli decoder bug (22.23.4). A Dockerfile pinned to an old patch (`node:22.11-alpine`) is a finding; prefer `node:22-alpine` plus digest pinning updated by a bot, or an explicit current patch.

##### New in 22: the permission model is stable (22.13)

```javascript
// Run with: node --permission --allow-fs-read=/srv/app --allow-fs-write=/srv/app/tmp app.js
// (`--experimental-permission` is an accepted alias on 22; Node 20 only knows the experimental flag)
if (process.permission) {
  console.log(process.permission.has('fs.read', '/etc/shadow'));   // false
  console.log(process.permission.has('child'));                    // false unless --allow-child-process
} else {
  console.log('permission model not enabled');
}
```

Verified on 22.23.4 under `--permission` with no allow-flags: `fs.readFileSync('/etc/hostname')` throws `ERR_ACCESS_DENIED` (`err.permission === 'FileSystemRead'`), `execSync` throws (`ChildProcess`), `new Worker()` throws. Since 22.17 the entry-point file is readable implicitly.

What the 22.x permission model does **not** do — never present it as a sandbox:

- **No network restriction** (`--allow-net` is Node 25+); `fetch`, `net.connect`, `dgram` and DNS are unrestricted.
- **No coverage of `node:sqlite`** file access, existing file descriptors, or files read before startup (`--env-file`, `--openssl-config`).
- Symlinks are followed even outside the allowed paths; `--allow-child-process` and `--allow-addons` effectively disable it (Node prints a `SecurityWarning`).
- It protects against accidental access by trusted code, not against malicious code; the Node threat model treats the code you run as trusted.

##### Path traversal

```javascript
import path from 'node:path';

const ROOT = path.resolve('/srv/uploads');

function safeResolve(userPath) {
  const resolved = path.resolve(ROOT, userPath);
  // path.relative is robust against prefix tricks like /srv/uploads-evil
  const rel = path.relative(ROOT, resolved);
  if (rel === '' || rel.startsWith('..') || path.isAbsolute(rel)) {
    throw Object.assign(new Error('path escapes upload root'), { code: 'EACCES' });
  }
  return resolved;
}

console.log(safeResolve('avatars/1.png'));                 // /srv/uploads/avatars/1.png
for (const bad of ['../etc/passwd', '/etc/passwd', '../uploads-evil/x']) {
  try { safeResolve(bad); } catch (e) { console.log('blocked', bad); }
}
// BAD: resolved.startsWith(ROOT) accepts '/srv/uploads-evil/x'
console.log(path.resolve(ROOT, '../uploads-evil/x').startsWith(ROOT)); // true
```

Decode URL-encoded input (`%2e%2e%2f`) **before** the check, reject NUL bytes, and resolve symlinks with `fs.realpath()` when the directory contains user-created links. On Windows, `path.normalize()` treated device names (`CON`, `PRN`, `AUX`) unsafely before 22.17.1 (CVE-2025-27210).

##### Prototype pollution and untrusted objects

```javascript
const payload = JSON.parse('{"__proto__": {"isAdmin": true}, "name": "x"}');

// BAD: recursive merge of untrusted JSON into a normal object pollutes Object.prototype
function naiveMerge(target, src) {
  for (const [k, v] of Object.entries(src)) {
    if (v && typeof v === 'object') naiveMerge((target[k] ??= {}), v);
    else target[k] = v;
  }
  return target;
}
naiveMerge({}, payload);
console.log({}.isAdmin);                      // true — every object is now "admin"
delete Object.prototype.isAdmin;

// GOOD: skip dangerous keys and use null-prototype targets or Map
const FORBIDDEN = new Set(['__proto__', 'constructor', 'prototype']);
function safeMerge(target, src) {
  for (const [k, v] of Object.entries(src)) {
    if (FORBIDDEN.has(k)) continue;
    if (v && typeof v === 'object') safeMerge((target[k] ??= Object.create(null)), v);
    else target[k] = v;
  }
  return target;
}
safeMerge(Object.create(null), payload);
console.log({}.isAdmin);                      // undefined
```

- Lookups such as `if (options[key])` or `handlers[req.query.type]()` on plain objects reach `Object.prototype` members (`constructor`, `toString`); use `Object.hasOwn()` or a `Map`.
- Validate untrusted input with a schema (zod, ajv, valibot) at the boundary; do not trust `typeof` checks scattered through business logic.
- `--disable-proto=delete` (or `=throw`) removes the `__proto__` accessor process-wide as defence in depth.

##### SSRF and outbound requests

```javascript
import net from 'node:net';

// Parse without throwing (URL.parse: 22.1, also late 20.x) and allow-list schemes/hosts
function parseOutboundUrl(input) {
  const url = URL.parse(input);
  if (!url || !['https:', 'http:'].includes(url.protocol)) return null;
  return url;
}
console.log(parseOutboundUrl('file:///etc/passwd'), parseOutboundUrl('https://api.example.com/x')?.host);

// Block private ranges at connect time (net.connect({ blockList }) is 22.13+)
const blockList = new net.BlockList();
blockList.addSubnet('10.0.0.0', 8);
blockList.addSubnet('127.0.0.0', 8);
blockList.addSubnet('169.254.0.0', 16);     // cloud metadata endpoints
blockList.addSubnet('192.168.0.0', 16);
console.log(blockList.check('169.254.169.254'), net.BlockList.isBlockList(blockList)); // true true
```

Checking the hostname string is not enough (DNS rebinding, `0x7f.1`, IPv6-mapped addresses); enforce the policy on the resolved address (`blockList` on `net.connect`/`net.Server` since 22.13, or a custom `lookup`). `fetch` has no `blockList` option — use an undici `Agent` with a custom `connect.lookup`, or an egress proxy.

##### Cryptography

```javascript
import { randomBytes, randomUUID, randomInt, timingSafeEqual, scrypt, createHash, hash,
         createCipheriv, createDecipheriv } from 'node:crypto';
import { promisify } from 'node:util';
const scryptAsync = promisify(scrypt);

// Tokens and IDs: CSPRNG only (never Math.random)
const token = randomBytes(32).toString('base64url');
console.log(token.length, randomUUID().length, randomInt(1, 7) <= 6);  // 43 36 true

// Password hashing: slow KDF with per-user salt (argon2 is Node 24+ in core; use scrypt or a library)
const salt = randomBytes(16);
const derived = await scryptAsync('correct horse', salt, 64, { N: 2 ** 15, r: 8, p: 1, maxmem: 64 * 1024 * 1024 });
const again = await scryptAsync('correct horse', salt, 64, { N: 2 ** 15, r: 8, p: 1, maxmem: 64 * 1024 * 1024 });
console.log(timingSafeEqual(derived, again));   // constant-time comparison (lengths must match)

// Fast non-secret hashing: crypto.hash() one-shot (Node 21.7/20.12+, Stability 1.2)
console.log(hash('sha256', 'abc') === createHash('sha256').update('abc').digest('hex')); // true

// Authenticated encryption: AES-256-GCM with a random 12-byte IV and explicit tag length
const key = randomBytes(32);
const iv = randomBytes(12);
const cipher = createCipheriv('aes-256-gcm', key, iv, { authTagLength: 16 });
const ct = Buffer.concat([cipher.update('secret', 'utf8'), cipher.final()]);
const tag = cipher.getAuthTag();
const decipher = createDecipheriv('aes-256-gcm', key, iv, { authTagLength: 16 });
decipher.setAuthTag(tag);
console.log(Buffer.concat([decipher.update(ct), decipher.final()]).toString()); // secret
```

- `crypto.createCipher()`/`createDecipher()` (no IV, MD5-based key derivation) were **removed in 22.0** — legacy code fails with `TypeError`.
- Never reuse an IV with the same GCM key; never accept truncated GCM tags (set `authTagLength`, DEP0182).
- Compare secrets (HMACs, API keys, session tokens) with `timingSafeEqual` on equal-length `Buffer`s, not `===`.
- `md5`/`sha1` only for non-security checksums. `crypto.randomUUIDv7()` and `crypto.argon2()` do not exist on 22.
- Web Crypto (`globalThis.crypto.subtle`) supports `Ed25519`/`X25519` as stable since 22.13; ML-KEM/ML-DSA in Web Crypto are Node 24+.

##### TLS and certificates

- `rejectUnauthorized: false` and `NODE_TLS_REJECT_UNAUTHORIZED=0` disable certificate validation for **every** connection — never in production. Use `ca:` with the private CA, `NODE_EXTRA_CA_CERTS`, or the system store (`--use-system-ca` 22.15, `NODE_USE_SYSTEM_CA=1` 22.19).
- `tls.getCACertificates('bundled' | 'system' | 'extra' | 'default')` (22.15) and `tls.setDefaultCACertificates()` (22.19) replace manual `ca` juggling.
- **New in 22.20: OpenSSL 3.5.** Verified with a Node 24 client against a 22.23.4 server: TLS 1.3 now negotiates the hybrid post-quantum group `X25519MLKEM768` (a 20.x server negotiates `X25519`). The larger ClientHello can trip old middleboxes and TLS-inspecting proxies. `tls.DEFAULT_MIN_VERSION` stays `TLSv1.2`. On a 22.x client, `tlsSocket.getEphemeralKeyInfo()` returns `{}` for this hybrid group (Node 24 reports `{ type: 'TLSGroup', name: 'X25519MLKEM768' }`), so monitoring code must not assume `name` is present.
- Since 22.22.0, `TLSSocket` has a default `'error'` handler, so a malformed HTTP/2 `HEADERS` frame that resets the socket no longer crashes an HTTP/2 server that attached no `'error'` handler (CVE-2025-59465, rated High). Older 22.x patches are remotely crashable; upgrade, and still attach error handlers.

##### Secrets and configuration

```javascript
// Node 20.12+/21.7+: parse .env without dotenv
import { parseEnv } from 'node:util';
console.log(parseEnv('A=1\nB="two words" # comment\nexport C=3'));  // { A: '1', B: 'two words', C: '3' }
// process.loadEnvFile('.env') loads into process.env; `node --env-file=.env app.js` does it at startup.
// Variables already present in the environment win over the file.
```

- `--env-file` is stable since 22.21; `--env-file-if-exists` (22.9) does not fail when the file is absent. Neither is subject to the permission model.
- Never log `process.env`, request headers or full error objects that may contain credentials; `--trace-env` (22.13) prints the *names* (not values) of every environment variable access to stderr — useful for auditing which secrets a dependency reads.
- Commit a lockfile, install with `npm ci`, review new dependencies' install scripts (`npm ci --ignore-scripts` where possible), and run `npm audit --omit=dev` in CI.

***

#### Performance

##### Engine facts for 22.x

- **Maglev is off by default since 22.9.0.** It was enabled in 22.0 but disabled after codegen bugs that could not be backported to the frozen V8 12.4 (`node --v8-options` shows `default: --no-maglev` on 22.23.4; Node 23+ enables it again). Do not turn it on with `--maglev` in production; JIT tiers on 22 are Ignition → Sparkplug → TurboFan.
- V8 stays at 12.4 for the whole 22.x line. Performance changes inside 22.x come from Node itself (for example faster `Buffer.copy`/`write` in 22.7 and cheaper `AbortSignal` creation in 22.0) — benchmark on the exact patch you deploy.
- **Startup**: enable the compile cache (`NODE_COMPILE_CACHE` / `module.enableCompileCache()`) for CLIs and short-lived processes; ship a single ESM entry rather than thousands of tiny files; avoid top-level work in modules that are imported eagerly.

##### Memory and containers

```javascript
import v8 from 'node:v8';
import os from 'node:os';

const heapLimitMb = Math.round(v8.getHeapStatistics().heap_size_limit / 1024 / 1024);
console.log(heapLimitMb > 0, typeof process.constrainedMemory(), typeof process.availableMemory());
console.log(os.availableParallelism() >= 1);   // use for pool sizes instead of os.cpus().length
```

- The default V8 heap limit is a heuristic based on system memory and may not match the container's limit. Set it explicitly in containers: `--max-old-space-size=<MiB>` or, **new in 22.21**, `--max-old-space-size-percentage=75` (a percentage of available memory).
- `process.constrainedMemory()` reports the OS/cgroup memory limit (`0` when unknown; since 22.0 it always returns a number) and `process.availableMemory()` (22.0) the remaining budget; both are stable since 22.16.
- `Buffer.allocUnsafe()` and `Buffer.from(string)` for small sizes slice an 8 KiB shared pool; holding a tiny slice keeps the whole 8 KiB alive. `Buffer.allocUnsafeSlow()` or `Buffer.from(buf)` copies when retaining small slices long-term. Never return `allocUnsafe` memory to callers without overwriting it fully.
- Streams buffer up to 64 KiB by default on 22 (see Streams).

##### Hot-path review rules

- Avoid `JSON.parse(JSON.stringify(x))` for cloning (slow, loses `Date`/`Map`/`BigInt`); use `structuredClone` or explicit copies.
- Avoid `delete obj.prop` on hot objects (shape transitions); set to `undefined` or use a `Map`.
- Do not build large strings with `+=` in loops over huge inputs when an array `join()` or a stream fits better; do not `await` inside tight loops when the work is independent.
- Prefer `for…of`/indexed loops over `.forEach` with closures only where profiling shows it matters; readability wins elsewhere.
- Cache compiled `RegExp`s and `Intl.*Format` instances outside functions — constructing `Intl.DateTimeFormat` per call is expensive.
- `async_hooks.createHook()`, `--trace-*` flags and `util.inspect()` on large objects in log statements all cost measurable CPU; guard debug logging behind level checks.
- Measure with `--cpu-prof` (stable since 22.4), `--heap-prof`, `--heapsnapshot-near-heap-limit=N` (stable in 22.22.1), `perf_hooks.monitorEventLoopDelay()` and `performance.eventLoopUtilization()` — not with ad-hoc `Date.now()` loops.

***

#### Testing with `node:test`

##### Structure and assertions

```javascript
import { describe, it, before, after, mock } from 'node:test';
import assert from 'node:assert/strict';

describe('pricing', () => {
  let rates;
  before(() => { rates = new Map([['EUR', 1], ['USD', 1.1]]); });
  after(() => mock.restoreAll());

  it('converts with a plan', (t) => {
    t.plan(2);                                         // 22.2: fails if the count differs
    t.assert.strictEqual(Math.round(1000 * rates.get('USD')), 1100); // compare money in minor units
    t.assert.ok(rates.has('EUR'));
  });

  it('matches a subset (stable in 22.17)', () => {
    assert.partialDeepStrictEqual({ id: 1, meta: { tags: ['a'], v: 2 } }, { meta: { v: 2 } });
  });

  it('checks rejections precisely', async () => {
    await assert.rejects(Promise.reject(new TypeError('bad input')), { name: 'TypeError', message: /bad/ });
  });

  it.todo('handles currencies without a rate');
});
```

- Import `node:assert/strict` (or `assert.strict`): the legacy loose `assert.equal`/`deepEqual` use `==` semantics.
- Use the test context (`t.assert`, `t.mock`, `t.plan`, `t.signal`) inside tests so mocks are restored and plans are scoped per test.
- **Await subtests.** On Node 22, `t.test()` returns a promise; a subtest that is still running when its parent finishes is **cancelled** and reported as a failure (`'test did not finish before its parent and was cancelled'`). Node 24 waits automatically — code written for 24 that omits `await t.test(…)` breaks on 22.

```javascript
import test from 'node:test';
import assert from 'node:assert/strict';

test('parent awaits its subtests', async (t) => {
  await t.test('child', async () => {
    await new Promise((r) => setTimeout(r, 5));
    assert.equal(1 + 1, 2);
  });
});
```

##### Mocks, timers and snapshots

```javascript
import { test, mock } from 'node:test';
import assert from 'node:assert/strict';

test('mock.fn and mock.method', (t) => {
  const send = t.mock.fn(async (msg) => ({ ok: true, msg }));
  const mailer = { send };
  mailer.send('hi');
  assert.equal(send.mock.callCount(), 1);
  assert.deepEqual(send.mock.calls[0].arguments, ['hi']);

  const clock = { now: () => 1 };
  t.mock.method(clock, 'now', () => 42);
  assert.equal(clock.now(), 42);
});

test('mock.property (22.20)', (t) => {
  const config = { level: 'info' };
  t.mock.property(config, 'level', 'debug');
  assert.equal(config.level, 'debug');
});

test('fake timers', (t) => {
  t.mock.timers.enable({ apis: ['setTimeout', 'Date'], now: 0 });  // still experimental: prints a warning
  let fired = false;
  setTimeout(() => { fired = true; }, 60_000);
  t.mock.timers.tick(60_000);
  assert.equal(fired, true);
  assert.equal(Date.now(), 60_000);
});

test('waitFor (22.14)', async (t) => {
  let ready = false;
  setTimeout(() => { ready = true; }, 10);
  await t.waitFor(() => assert.ok(ready), { interval: 5, timeout: 1_000 });
});
```

- Snapshot testing: `t.assert.snapshot(value)` (stable since 22.14) writes `<test file>.snapshot` on `node --test --test-update-snapshots`; commit the snapshot files. `t.assert.fileSnapshot(value, path)` (22.14) targets a custom file.
- Module mocking (`mock.module()`) requires `--experimental-test-module-mocks` on 22; without it `mock.module` is `undefined` and the test fails with `TypeError`.
- `mock.restoreAll()`/`t.mock` cleanup: mocks created with the global `mock` survive across tests unless restored.

##### Running tests

```bash
node --test   # default globs: **/*.test.{cjs,mjs,js}, **/*-test.{cjs,mjs,js}, **/*_test.{cjs,mjs,js}, **/test-*.{cjs,mjs,js}, **/test.{cjs,mjs,js}, **/test/**/*.{cjs,mjs,js}
node --test --test-reporter=spec --test-reporter-destination=stdout \
            --test-reporter=junit --test-reporter-destination=junit.xml
node --test --experimental-test-coverage --test-coverage-lines=80 --test-coverage-branches=70
node --test --test-concurrency=4 --test-shard=1/3
node --test --experimental-test-isolation=none # run all files in one process (22.8)
node --test --test-name-pattern="pricing" --test-skip-pattern="slow"
```

- With type stripping on (22.18+), the default globs also match `.ts`/`.mts`/`.cts` test files.
- On 22 the default reporter is `spec` on a TTY and **TAP when stdout is not a TTY** (CI logs); Node 23 made `spec` the default everywhere. Pass `--test-reporter` explicitly in CI.
- Coverage still needs `--experimental-test-coverage`; thresholds make the run exit with code 1.
- `--test-isolation` (without `experimental-`) and `--test-randomize` do not exist on 22.
- `--test-force-exit` hides leaked handles (open servers, intervals); fix the leak instead.

***

#### Tooling and Project Hygiene

##### Bundled tools and scripts

- npm **10.9.9** ships with 22.23.4 (npm 11 is bundled from Node 24). Corepack (0.36.0 in 22.23.4) is still bundled with Node 22 but is **not enabled by default**, and Node 25+ no longer ships it; pin the package manager with `"packageManager": "pnpm@x.y.z"` and enable it explicitly (`corepack enable`) or install the manager directly.
- **`node --run <script>`** (stable 22.10) runs `package.json` scripts much faster than `npm run`, walks up to the nearest `package.json` (22.3) and adds every ancestor `node_modules/.bin` to `PATH`. It intentionally does **not** run `pre`/`post` scripts and does not define `npm_*` variables (`npm_package_version`, `npm_config_*`); it sets `NODE_RUN_SCRIPT_NAME` and `NODE_RUN_PACKAGE_JSON_PATH` instead. Scripts that rely on `pretest` or `npm_package_*` must keep using `npm run`.
- **`node --watch`** (stable 22.0) restarts on changes to imported files; `--watch-path` works only on macOS and Windows; `--watch-preserve-output`; `--watch-kill-signal` (22.18). Development only.
- `--env-file`, `--env-file-if-exists`, `process.loadEnvFile()` replace `dotenv` for simple cases.
- `--experimental-config-file=node.config.json` / `--experimental-default-config-file` (22.16) are Stability 1.0 (early development) and print an ExperimentalWarning — do not depend on them in production.
- Single executable applications: `--experimental-sea-config` + `postject`; ESM entry points and `--build-sea` are Node 25+.

##### Linting and formatting

- ESLint 9 flat config (`eslint.config.js`) with `languageOptions.ecmaVersion: 2024` and `sourceType: 'module'` matches V8 12.4 closely (it does not flag `Promise.try` or `RegExp.escape`; `eslint-plugin-n` with `n/no-unsupported-features/es-builtins` and `n/no-unsupported-features/node-builtins` reads `engines.node` and does).
- Useful rules for Node 22 code: `no-floating-promises` (typescript-eslint), `require-await`, `no-return-await` is obsolete (returning `await` inside `try` is correct), `prefer-object-has-own`, `n/prefer-node-protocol`, `n/no-deprecated-api`.
- Format with Prettier or Biome; do not hand-align.

##### Docker and deployment

```dockerfile
# Pin the major and keep the patch current (rebuild on security releases)
FROM node:22-bookworm-slim
ENV NODE_ENV=production
WORKDIR /app
COPY package.json package-lock.json ./
RUN npm ci --omit=dev --ignore-scripts
COPY . .
USER node
# exec form: node is PID 1 and receives SIGTERM directly (no npm wrapper)
CMD ["node", "--enable-source-maps", "src/server.js"]
```

- Run `node` directly (not `npm start`) so `SIGTERM` reaches the process; handle `SIGTERM` by closing servers and draining work.
- `NODE_ENV=production` changes behaviour of frameworks, not of Node itself.
- `--enable-source-maps` makes stack traces of transpiled code readable at some performance cost on stack capture.

***

#### Observability and Diagnostics

```javascript
import diagnostics_channel from 'node:diagnostics_channel';
import http from 'node:http';
import { getCallSites, styleText } from 'node:util';

// Subscribe to built-in HTTP events without monkey-patching
diagnostics_channel.subscribe('http.server.request.start', ({ request }) => {
  console.log('request', request.method, request.url);
});

const server = http.createServer((req, res) => res.end('ok')).listen(0);
await new Promise((r) => server.once('listening', r));
await fetch(`http://127.0.0.1:${server.address().port}/health`).then((r) => r.text());
server.close();

// util.getCallSites() (22.9 as getCallSite, renamed in 22.12)
const [site] = getCallSites();
console.log(typeof site.functionName, typeof site.lineNumber); // 'string' 'number'

// util.styleText respects NO_COLOR/FORCE_COLOR and non-TTY streams by default (22.8; stable in 22.13)
console.log(JSON.stringify(styleText('red', 'error')));        // plain "error" when piped
```

- Structured logs: use a JSON logger (pino) writing to stdout; avoid `console.log` of objects in hot paths (`util.inspect` is synchronous and can be slow). `process.stdout`/`stderr` writes to **pipes are asynchronous on POSIX** (synchronous for files and TTYs) — a `process.exit()` right after logging into a pipe (Docker, systemd, `| tee`) can drop lines; set `process.exitCode` and let the process end.
- `diagnostics_channel` channels on 22 include `http.client.request.created/start/error`, `http.server.request.start`, `http.server.response.finish`, `http2.*` stream events, `net.client.socket`/`net.server.socket`, `module.require.*`/`module.import.*` (22.4), `child_process`, `worker_threads`, and undici's `undici:request:*` for `fetch`. `TracingChannel` gives start/end/asyncStart/asyncEnd/error events.
- `process.report.writeReport()` / `--report-on-fatalerror` produce a JSON diagnostic report; it contains environment variables unless `--report-exclude-env` (22.13) is used.
- `--heapsnapshot-signal=SIGUSR2`, `--heapsnapshot-near-heap-limit=2`, `--cpu-prof`, `--heap-prof` and the inspector (`--inspect`, `--inspect-wait` 22.2) cover most production debugging. `--disable-sigusr1` (22.14) stops `SIGUSR1` from opening the inspector.
- `--experimental-network-inspection` (22.6, experimental) shows outgoing `http`/`https` traffic in Chrome DevTools.

***

#### Process Lifecycle and CLI Programs

##### Graceful shutdown

```javascript
import http from 'node:http';
import { once } from 'node:events';

const server = http.createServer((req, res) => res.end('ok')).listen(0);
await once(server, 'listening');

let shuttingDown = false;
async function shutdown(signal) {
  if (shuttingDown) return;
  shuttingDown = true;
  console.log(`received ${signal}, draining`);
  const deadline = setTimeout(() => {
    server.closeAllConnections();                 // force after the grace period
  }, 10_000).unref();
  server.close();                                 // stop accepting; also closes idle keep-alive sockets (Node 19+)
  await once(server, 'close');
  clearTimeout(deadline);
  // close DB pools, flush logs/metrics here
  process.exitCode = 0;
}

process.once('SIGTERM', () => void shutdown('SIGTERM'));
process.once('SIGINT', () => void shutdown('SIGINT'));

setTimeout(() => process.kill(process.pid, 'SIGTERM'), 20);   // demo only
```

- Without a `SIGTERM` handler Node exits immediately, dropping in-flight requests. With one, the process stays alive until the event loop is empty — make sure every server, interval and pool is closed or `unref()`ed.
- `process.on('exit')` handlers must be synchronous; async work there never completes.
- `process.ref(obj)`/`process.unref(obj)` (22.14, experimental) generalise `.ref()`/`.unref()` to any object implementing them.

##### Argument parsing without dependencies

```javascript
import { parseArgs } from 'node:util';

const { values, positionals } = parseArgs({
  args: ['--port', '8080', '--no-color', '-v', 'serve'],
  options: {
    port: { type: 'string', default: '3000' },
    color: { type: 'boolean', default: true },
    verbose: { type: 'boolean', short: 'v' },
  },
  allowPositionals: true,
  allowNegative: true,          // 22.4: enables --no-<flag> for boolean options
  strict: true,                 // unknown options throw ERR_PARSE_ARGS_UNKNOWN_OPTION
});
console.log(Number(values.port), values.color, values.verbose, positionals); // 8080 false true [ 'serve' ]
```

`parseArgs` returns strings; validate and convert (`Number.parseInt(values.port, 10)`, range checks) yourself. In real programs omit `args` to parse `process.argv.slice(2)`.

##### URLs

```javascript
// WHATWG URL everywhere; url.parse() is legacy (DEP0169, runtime warning from Node 24)
const u = new URL('/search?q=a b&lang=de', 'https://example.com');
u.searchParams.append('page', '2');
console.log(u.href);                         // https://example.com/search?q=a+b&lang=de&page=2
console.log(URL.canParse('not a url'), URL.parse('nope'));  // false null — no try/catch needed
```

- Build query strings with `URLSearchParams`, never by string concatenation of user input.
- `URLPattern` is **not** available on 22 (global from Node 24); use `path-to-regexp` or explicit parsing.

***

#### Removed / Changed in Node.js 22 — Migration Notes

Upgrading from Node 20 (or 18) to 22 — check these first:

1. **Import assertions removed.** `import … assert { type: 'json' }` → `with { type: 'json' }`. TypeScript accepts `with` since 5.3; TS 5.8 reports `assert` as error TS2880 under `--module nodenext` (TS 5.3 still accepted it silently).
2. **`crypto.createCipher`/`createDecipher` removed.** Re-encrypting existing data requires reproducing OpenSSL's `EVP_BytesToKey` (MD5) derivation once, then migrating to `createCipheriv` with a proper KDF and random IV.
3. **Stream buffering 4× larger** (64 KiB `highWaterMark`): re-check memory budgets of stream-heavy services.
4. **Runtime deprecation warnings** for `util.is*()`, `util.log()`, `util._extend()`, `new fs.Stats()`, `new crypto.Hash()`/`Hmac()`. They are hard removals in Node 23 (`util.isBoolean` etc.), so fix them now. `--throw-deprecation` in CI turns warnings into failures.
5. **Module syntax detection (22.7)**: typeless `.js` files containing `import`/`export` now run as ESM instead of failing; files that relied on the failure, or on CommonJS globals in ambiguous files, behave differently. Add `"type"` everywhere.
6. **`require(esm)` (22.12)**: packages that use `try { require(x) } catch (e) { if (e.code === 'ERR_REQUIRE_ESM') return import(x) }` now take the `require` branch and get a namespace object instead of `module.exports`; code that checked `ERR_REQUIRE_ESM` should check `ERR_REQUIRE_ASYNC_MODULE` too.
7. **Type stripping on by default (22.18)**: `node file.ts` now executes instead of failing with `ERR_UNKNOWN_FILE_EXTENSION`. Scripts that assumed `.ts` needs a loader, or that use enums, parameter properties or `paths` aliases, either keep their loader (`tsx`, `ts-node`) or run with `--no-experimental-strip-types`, which restores the old behaviour.
8. **Maglev**: enabled in 22.0–22.8, disabled from 22.9; performance baselines taken on early 22.x are not comparable.
9. **OpenSSL 3.5 (22.20)**: post-quantum hybrid key exchange by default; the bundled root store (NSS) is updated in most patches — connections to servers chaining to a removed public root fail after an upgrade.
10. **HTTP/2 priority signaling removed (22.23.0)**: `http2stream.priority()` and `priority` options are End-of-Life.
11. **Test runner**: subtests must be awaited; `mock.timers` and module mocks remain experimental; coverage still needs the flag.
12. **Removed flags**: `--experimental-policy` (22.2), `--experimental-network-imports` (22.6). `--experimental-default-type` (still present on 22) is removed in 23.
13. **`process.binding()`** still works but is deprecated (DEP0111, documentation-only with `--pending-deprecation` support); Node 24 removed several bindings.
14. **Native addons**: `NODE_MODULE_VERSION` is 127 for 22.x — prebuilt binaries for 20.x (115) do not load; prefer Node-API addons, which are ABI-stable across majors.

Upgrading from 22 to 24 (what will break later): `url.parse()`, `SlowBuffer`, `fs.F_OK` and friends warn at runtime; `dirent.path`, `tls.createSecurePair`, `process.assert`, `util.isBoolean` & co are gone; un-awaited subtests stop being cancelled; `AsyncLocalStorage` switches to `AsyncContextFrame`; `Buffer.poolSize` becomes 64 KiB (from 24.18); `URLPattern` becomes global.

***

#### Production / Review Checklist

##### Version and compatibility

- [ ] `engines.node` floor matches the newest 22.x feature used (`>=22.12` for `require(esm)`, `>=22.18` for default type stripping / `import.meta.main`, `>=22.21` for `NODE_USE_ENV_PROXY`)
- [ ] CI runs the same Node major (and ideally patch) as production; `@types/node` major is 22
- [ ] No Node 23+ language features: `using`, `Promise.try`, RegExp modifiers or duplicate named groups, `RegExp.escape`, `Float16Array`, `Error.isError`, `Uint8Array.fromBase64`, `Temporal`, decorators
- [ ] No Node 23.8+/24+ APIs: `URLPattern`, `crypto.randomUUIDv7`/`argon2`, `als.withScope`, `http.setGlobalProxyFromEnv`, `req.signal`, `fs.Utf8Stream`, `util.debounce`
- [ ] Deployed on the latest 22.x patch (22.23.4 as of 2026-10-21); plan the move to Node 24 LTS before 2027-04-30

##### Modules

- [ ] Every `package.json` declares `"type"`; built-ins use the `node:` prefix
- [ ] Import attributes use `with`, never `assert`
- [ ] `require(esm)` call sites use `.default` / `'module.exports'` correctly and no required graph uses top-level `await`
- [ ] No user input reaches `import()`/`require()`/`eval`/`new Function`
- [ ] TypeScript run directly by Node uses only erasable syntax, explicit `.ts` extensions and `import type`; `tsc --noEmit` runs in CI

##### Async and errors

- [ ] No floating promises; no `async` callbacks in `forEach`; independent awaits run concurrently with bounded concurrency
- [ ] Every outbound call has an `AbortSignal` deadline; listeners on shared signals are removed
- [ ] Every `EventEmitter`/stream/socket/child/worker has an `'error'` path; streams combined with `pipeline()`
- [ ] Errors are `Error` instances with `cause`; code branches on `err.code`; `unhandledRejection` handlers still exit
- [ ] No sync I/O or CPU-heavy work on the request path; heavy work in a worker pool
- [ ] Subtests are awaited in `node:test`

##### Security

- [ ] `child_process` without a shell for untrusted arguments; timeouts and `maxBuffer` set
- [ ] Path containment via `path.relative()`; no `startsWith(root)` checks
- [ ] Untrusted objects merged safely (no `__proto__`/`constructor` keys), validated by schema
- [ ] Outbound URLs allow-listed; private address ranges blocked at connect time (SSRF)
- [ ] `crypto.randomBytes`/`randomUUID` for tokens; `timingSafeEqual` for secret comparison; scrypt (or a vetted argon2/bcrypt library) for passwords; AES-GCM with random IV and full tag
- [ ] TLS verification never disabled; private CAs via `ca`, `NODE_EXTRA_CA_CERTS` or `--use-system-ca`
- [ ] Permission model, if used, is not treated as a sandbox (no network, no `node:sqlite` coverage)
- [ ] `node:sqlite` queries use bound parameters only
- [ ] Lockfile committed; `npm ci`; no secrets in logs, reports or images

##### Performance and operations

- [ ] Heap size set explicitly in containers (`--max-old-space-size` or `--max-old-space-size-percentage`)
- [ ] HTTP server timeouts reviewed; `keepAliveTimeout` above the load balancer idle timeout
- [ ] Graceful `SIGTERM` handling; `node` is PID 1 in the container (exec form `CMD`)
- [ ] Stream-heavy services re-measured after the 64 KiB `highWaterMark` change
- [ ] Maglev not force-enabled; compile cache considered for CLIs
- [ ] Structured logging; diagnostics via `diagnostics_channel`, `--cpu-prof`, heap snapshots; reports exclude env (`--report-exclude-env`)
