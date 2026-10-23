### JavaScript on Node.js 26: The Complete Production Developer Guide

> **Version**: Node.js 26.0.0 was released on **2026-05-05**; the latest patch is **Node.js 26.12.1 (2026-10-21, security release)**. It ships V8 **14.6** (`14.6.202.34-node.40`), Undici **8.12.2**, OpenSSL **3.5.9**, SQLite **3.54.0**, ICU 78.3 / Unicode 17 and npm 11.20.0.[^1][^2]
> **Support status (as of October 2026)**: Node.js 26 is the **Current** release line. It is scheduled to be promoted to **Active LTS on 2026-10-28**, enters **Maintenance LTS on 2027-10-20** and reaches **End-of-Life on 2029-04-30**. Node.js 24 "Krypton" moved to Maintenance LTS on 2026-10-20 (EOL 2028-04-30), Node.js 22 is in Maintenance LTS until 2027-04-30, and Node.js 25 reached EOL on 2026-06-01.[^3]
> **Scope**: Server-side and tooling JavaScript running on Node.js 26.x: the ECMAScript features shipped by V8 14.6 (ES2025 plus most of ES2026 and Temporal) and the Node.js 26 runtime APIs — modules, async patterns, errors, streams, HTTP, file system, child processes, security, performance, testing, tooling and observability.
> **Note**: This document is self-contained. Everything from earlier Node.js versions that is still valid is carried forward. Sections marked **New in 26** cover behaviour introduced in the 26.x line; **New in 25** marks features that Node.js 26 inherits from the (now EOL) 25.x line and that Node.js 24 does not have.

***

#### Overview

Node.js 26 is the **last release line under the old two-majors-per-year model**. Starting with Node.js 27, the project ships **one major release per year**: a six-month **Alpha** channel (October–March, prerelease versions such as `27.0.0-alpha.1`, semver-major changes still allowed), a **Current** phase from April to October, and then **30 months of LTS** — every major becomes LTS, the odd/even distinction disappears and version numbers follow the calendar year of the first Current release (27.0.0 in 2027, 28.0.0 in 2028). The Node.js 27 alpha channel starts in late October 2026 (scheduled 2026-10-28), and **27.0.0 is planned for April 2027** (2027-04-22 in the release schedule). As of 2026-10-22 no 27.x release — not even `27.0.0-alpha.1` — has been published; only untested automated nightly builds (`v27.0.0-nightly…`) exist, and Node.js 26.12.1 is the newest release of any line.[^3][^4]

Headline changes in Node.js 26 relative to 25 and 24:[^1][^5]

- **Temporal is enabled by default** (`globalThis.Temporal`, `Date.prototype.toTemporalInstant()`), backed by V8's Rust-based implementation.
- **V8 14.6** (Chromium 146) brings **upsert** (`Map`/`WeakMap.prototype.getOrInsert()` and `getOrInsertComputed()`) and **iterator sequencing** (`Iterator.concat()`), and removes the old `Intl.Locale` getters (`weekInfo`, `hourCycles`, `timeZones`, ...).
- **Undici 8**: `fetch()` now negotiates **HTTP/2 over TLS by default**.
- **Behaviour changes**: `readable.read()` without a size returns **one buffered chunk** instead of the whole buffer; HTTP upgrade requests with bodies are parsed as bodies; `localStorage` is `undefined` unless `--localstorage-file` is given; `util.inspect()` marks proxies as `Proxy(...)`; assertion messages accept printf-style format arguments.
- **Removals**: `response.writeHeader()`, the `_stream_*` internal modules, AES-GCM short tags without `authTagLength` (DEP0182 end-of-life), `--experimental-transform-types`.
- **New runtime deprecations**: `module.register()` (use `module.registerHooks()`), passing a `CryptoKey` to `node:crypto` APIs (DEP0203/DEP0204), `Duplex.toWeb({ type })` (use `readableType`).
- **Built-in TypeScript type stripping is stable** (since 25.2, backported to 24.x) and `require(esm)` is stable (since 25.4, also backported).
- 26.x minor releases added `node:ffi` (experimental, on by default since 26.9), `util.debounce()`/`util.throttle()`, `crypto.randomUUIDv7()`, `node:sqlite` renames (`Database`/`Statement`), ZIP support in `node:zlib`, `fs.Stats` `Temporal.Instant` fields and many more (see the minor-release table).

***

#### What's New in Node.js 26

##### Version facts a reviewer needs

| Item | Node.js 24.22.1 (Maintenance LTS) | Node.js 25.9.0 (EOL) | Node.js 26.12.1 (Current → LTS) |
|---|---|---|---|
| V8 | 13.6 | 14.1 | 14.6 |
| Undici (`fetch`) | 7.30.1 | 7.24.4 | 8.12.2 |
| `NODE_MODULE_VERSION` (native addon ABI) | 137 | 141 | 147 |
| `Temporal` global | no | no | **yes** |
| `Uint8Array` base64/hex | no | yes | yes |
| `Map.prototype.getOrInsert*`, `Iterator.concat` | no | no | **yes** |
| Type stripping (`node file.ts`) | yes, stable | yes, stable | yes, stable |
| `require(esm)` | yes, stable | yes, stable | yes, stable |
| `fetch()` uses HTTP/2 over TLS | no | no | **yes** |

Native addons must be rebuilt for Node.js 26 (ABI 147) unless they use Node-API. Check `process.versions` at runtime, not the major number, when a dependency version matters.

##### Language changes (V8 14.6) **New in 26**

| Feature | Status in 26 | In 25? | In 24? |
|---|---|---|---|
| `Temporal` (Stage 4, expected in ES2027) | on by default | no (not compiled in) | no |
| `Map/WeakMap.prototype.getOrInsert()`, `getOrInsertComputed()` (ES2026) | shipped | no | no |
| `Iterator.concat()` (ES2026) | shipped | no | no |
| `Date.prototype.toTemporalInstant()` | shipped | no | no |
| `Intl.Locale.prototype.{calendars, collations, hourCycles, numberingSystems, textInfo, timeZones, weekInfo}` getters | **removed** — use `getCalendars()`, `getWeekInfo()`, ... | present | present |
| `WebAssembly.Memory.prototype.toFixedLengthBuffer()`/`toResizableBuffer()` | shipped | no | no |
| `Math.sumPrecise()` (ES2026) | **not shipped** (only behind `--js-sum-precise`) | no | no |

Everything from ES2025 and earlier is available: iterator helpers, new `Set` methods, `RegExp.escape()`, `Promise.try()`, `Float16Array`/`Math.f16round()`, RegExp modifiers `(?i:...)`, duplicate named capture groups, import attributes and JSON modules. From ES2026, `Error.isError()`, `Array.fromAsync()`, `JSON.rawJSON()`/source-text access in `JSON.parse` revivers, `Uint8Array` base64/hex, upsert and `Iterator.concat()` ship; `Math.sumPrecise()` does not. Explicit resource management (`using`, `await using`, `DisposableStack`, `AsyncDisposableStack`, `SuppressedError`) and `Atomics.pause()` — both Stage 4 and expected in ES2027 — ship in 26 (and already in 24).[^6][^7]

##### Runtime changes in 26.0.0 **New in 26**

- **Temporal on by default.** `--no-harmony-temporal` disables it; APIs that need it then throw `ERR_NO_TEMPORAL` (added 26.2.0; verified with `fs.statSync('.').mtimeInstant`).[^1][^8]
- **Undici 8.** `fetch()` negotiates HTTP/2 via ALPN for `https:` URLs (verified: a local `http2.createSecureServer({ allowHTTP1: true })` sees `req.httpVersion === '2.0'` on 26.12.1 and `'1.1'` on 24/25). Legacy undici handler wrappers and "fake" `Blob` objects are gone.[^9]
- **`readable.read()` returns one chunk at a time** (PR #60441). Code that calls `read()` with no argument and expects all buffered data concatenated now gets only the first chunk.
- **HTTP upgrades with bodies** (PR #60016): `Content-Length`/`Transfer-Encoding` on an `Upgrade` request are now honoured. The request body is no longer exposed raw on the socket argument: when a body is received, the second argument of the `'upgrade'` event is a `Duplex` that emits socket data only after the body, and the parsed body is emitted from `req`. Use `req.socket` for socket-specific fields.
- **`localStorage` returns `undefined`** when `--localstorage-file` is not set (25.x exposed an object); `sessionStorage` is still an in-memory `Storage` object.
- **`util.inspect()` marks proxies**: `Proxy({ a: 1 })` instead of `{ a: 1 }` — snapshot tests that print proxied objects change.
- **`assert` printf-style messages**: `assert.strictEqual(1, 2, 'val %d', 7)` reports `val 7` (24/25 print `val %d`).
- **`QuotaExceededError`** is a global `DOMException` subclass with `quota`/`requested`.
- **ML-KEM / ML-DSA PKCS#8 export defaults to the seed-only format.**
- **`diagnostics_channel.tracePromise()`** behaves consistently when the traced function returns a non-promise.
- **Removed:** `--experimental-transform-types` (the flag is now a `bad option`), `response.writeHeader()` (DEP0063 end-of-life), `require('_stream_readable')` and the other `_stream_*` modules (DEP0193, DEP0125 end-of-life), AES-GCM decryption with a short auth tag and no `authTagLength` (DEP0182 end-of-life, now `ERR_CRYPTO_INVALID_AUTH_TAG`).
- **Runtime-deprecated:** `module.register()` (DEP0205), passing a `CryptoKey` to `node:crypto` functions (DEP0203), `KeyObject.from()` with a non-extractable `CryptoKey` (DEP0204), `Duplex.toWeb(stream, { type })` (DEP0201).
- Smaller 26.0 changes: raw key formats in the `KeyObject` APIs, an Ed25519 context parameter, async crypto job errors decorated with OpenSSL details, the SQLite Percentile extension enabled, and `stream.finished()` preserving the `AsyncLocalStorage` context only when needed.
- Build/platform: GCC 13.2 to build from source, Python 3.9 dropped from the build, AIX/IBM i targets Power 9, Windows SDK 11; Alpine Linux promoted to tier 2 in 26.11.0.

##### Additions in 26.x minor releases **New in 26**

Many of these were later backported to Node.js 24.x[^27] (24.22.1 has `util.debounce`/`throttle`, `crypto.randomUUIDv7()`, `process.permission.drop()` and others), but **not to 25.x**. Within the 26 line the minor version matters: `engines: { "node": ">=26" }` does not guarantee any API below.

| Release (date) | Notable additions |
|---|---|
| 26.1.0 (2026-05-07) | experimental `node:ffi` (flag `--experimental-ffi`, `--allow-ffi` under the permission model); `crypto.randomUUIDv7()`; `req.signal` on `http.IncomingMessage`; `signal` option for `fs.stat()`; `statfs.frsize`; `process.execve()` throws instead of aborting on failure; test runner `--test-randomize`/`--test-random-seed`, mock timers for `AbortSignal.timeout`; `util.styleText()` hex colours; `v8.startHeapProfile()`; `diagnostics_channel.boundedChannel()`; `sqlite` `database.serialize()`/`deserialize()` |
| 26.2.0 (2026-05-20) | `stream.compose` stable; `fs.Stats`/`BigIntStats` `atimeInstant`/`mtimeInstant`/`ctimeInstant`/`birthtimeInstant` (`Temporal.Instant`); `response.writeInformation()` for arbitrary 1xx; test tags (`--experimental-test-tag-filter`) |
| 26.3.0 (2026-06-01) | `Buffer.poolSize` default 8 KiB → **64 KiB** (also in 24.18.0+); `httpValidation` server option; `process.permission.drop()` |
| 26.3.1 (2026-06-18) | security release (TLS hostname/SNI fixes, `http.Agent` response-queue poisoning, permission-model fixes) |
| 26.4.0 (2026-06-24) | `fs.readFile()` with caller-supplied buffers; package maps (`--experimental-package-map`); `net` `setKeepAlive()` interval/count; TLS `certificateCompression`; experimental `node:vfs` (`--experimental-vfs`); `net.BoundSocket`; `dgram` `bindSync()`/`connectSync()` |
| 26.5.0 (2026-07-08) | `blob.textStream()`; `--experimental-import-text`; `ReadableStreamTee` in `node:stream/web`; negotiated TLS groups |
| 26.5.1 (2026-07-29) | security release (HTTP/2 RST, permission model, `https` session reuse, `sqlite` iterators, `zlib` bounds, max header count) |
| 26.6.0 (2026-08-03) | test runner `context.log()` and `test:log` event; `ffi.getCurrentEventLoop()` |
| 26.7.0 (2026-08-05) | crypto private keys from OpenSSL STORE loaders (`--allow-openssl-store`); Perfetto tracing; `Symbol.dispose` on `ModuleHooks`; `--test-coverage-include-all` |
| 26.8.0 (2026-08-26) | `zlib.ZipFile`/`ZipEntry`/`ZipBuffer` (early development); `MIMEType.parse()`; SIV/GCM-SIV ciphers; `net.BlockList` faster plus `addCIDR()`/`PRIVATE_RANGES`; `sqlite` `statement.close()`/`[Symbol.dispose]()`; `diagnostics_channel` `TracingChannel` stable; REPL syntax highlighting |
| 26.8.1 / 26.8.2 (2026-08-26 / 09-09) | `--version` fix; Undici 8.10.2; OpenSSL 3.5.8; DEP0208/DEP0209 documentation deprecations |
| 26.9.0 (2026-09-16) | `node:ffi` enabled by default (still experimental); `node:bench` (`--experimental-bench`); generic MAC API (`crypto.createMac()`, `crypto.getMacs()`); experimental DTLS; VFS integrated with module loaders; Web `Worker` global (`--experimental-web-worker`) |
| 26.10.0 (2026-09-22) | `util.debounce()`, `util.throttle()`, `util.markPromiseAsHandled()`; `crypto.parsePKCS12()`; `fs.openAsBlobSync()`; `sqlite` binds `undefined` as `NULL` |
| 26.11.0 / 26.11.1 (2026-10-07) | `Buffer.stringLength()`, `buffer.isLatin1()`; `http.isValidHeaderName()`/`isValidHeaderValue()`; `process.ref()`/`unref()` stable; **`sqlite.DatabaseSync`/`StatementSync` renamed to `Database`/`Statement`** (old names are deprecated aliases, DEP0210/DEP0211); `--process-timeout=<duration>`; `util.isPartialDeepStrictEqual()`; HTTP/2 `connectionWindowSize`; `perf_hooks` histogram `diff()`/`snapshot()`; Alpine tier 2. 26.11.1 only reverts documentation-tooling changes |
| 26.12.0 (2026-10-14) | `http.websocketMask()`/`websocketUnmask()`; startup throws when WebAssembly or RX memory is unavailable; `t.assert.called()`/`callCount()`/`lastCalledWith()`/`nthCalledWith()`; `process.finalization` marked release candidate; "module: synchronously load most ES modules" (#62530) landed here and was **reverted in 26.12.1** — do not rely on 26.12.0-only loader timing |
| 26.12.1 (2026-10-21) | security release: TLS non-canonical IP rejection and IP normalization, `dns` NUL-byte validation, Brotli decoder fix, `http` agent data guard, crypto random-cache reset across snapshots, permission-model fixes (wildcards, `realpathSync`), invalid ZIP local headers no longer cached, NSS 3.131.1 root certificates, Undici 8.12.2; reverts the 26.12.0 synchronous ESM loading change |

**Always pin the latest 26.x patch.** Three 26.x patch releases were security releases (26.3.1, 26.5.1, 26.12.1). A project on 26.x older than 26.12.1 runs with known High-severity TLS and DNS vulnerabilities.[^1]

##### New in 25 (inherited by 26, missing in 24)

Node.js 25 is EOL; projects on it should move to 26. Features that arrived in 25.x and are **not** in 24.22.1:[^10]

- `Uint8Array.fromBase64()`, `fromHex()`, `prototype.toBase64()`, `toHex()`, `setFromBase64()`, `setFromHex()` (V8 14.1).
- Web Storage globals (`localStorage`, `sessionStorage`, `Storage`) unflagged (25.0) — but see the 26 change: `localStorage` is `undefined` without `--localstorage-file`.
- Global `ErrorEvent`.
- `--allow-net` for the permission model (25.0; experimental warning, stability 1.1).
- `process.addUncaughtExceptionCaptureCallback()`.
- (`--permission-audit` and `asyncLocalStorage.withScope()` also arrived in 25.8/25.9 but were backported: 24.22.1 has them.)
- Removed in 25 (and therefore in 26): `SlowBuffer`, `fs.F_OK`/`R_OK`/`W_OK`/`X_OK` (use `fs.constants.*`), `fs.rmdir(path, { recursive: true })` (now throws `ERR_INVALID_ARG_VALUE` — use `fs.rm()`), `assert.CallTracker`, `assert.fail()` with more than one argument, `process.on('multipleResolves')`, the `asyncResource` property on bound functions, closing a `FileHandle` on GC, `dns.lookup()` with a falsy hostname, TLS `servername` set to an IP address, invalid ports in `url.parse()`.
- Runtime-deprecated in 25: SHAKE128/256 without `outputLength` (DEP0198), `ecdh.setPublicKey()` (DEP0031), `require('node:_tls_common')`/`_tls_wrap` (DEP0192).

***

#### Do Not Use or Recommend

The project's runtime is Node.js 26. The following do **not** exist in it (or exist only behind a flag that production code must not depend on). Never suggest them as available:

##### Node.js 27 (main branch, alpha channel from late October 2026 — unreleased)

As of 2026-10-22 these exist only on the Node.js `main` branch (built on V8 15.x), destined for the 27.0.0 alpha. Some semver-minor items may still be backported to a later 26.x release — check the installed version before claiming them:[^11]

- `http.Server` `keepAliveTimeout` default of **65 s**: in 26 it is still **5000 ms** (verified). Do not assume the longer default when reviewing keep-alive behind load balancers — set it explicitly.
- `node:net/promises` (`netPromises.connect()`, `netPromises.listen()`) and `server[Symbol.asyncIterator]()` for `net.Server`.
- `fs.mkstemp()`/`fsPromises.mkstemp()`/`fs.mkstempSync()`; `mkdtemp*` defaulting to `'buffer'` encoding for `Buffer` prefixes.
- `zlib.crc32c()`; `tlsSocket.socket`; the `"private"` field in `package.json` handled by Node; `--allow-env`; VFS unflagged; test-runner `ctx.mockReturnValue()`, `mockResolvedValue()`, `mockRejectedValue()`, `mockThrownValue()`.
- `process.finalization.register()`/`registerBeforeExit()` returning an unregister function.
- `--experimental-strip-private-modules` (type stripping for `.ts` files of `"private": true` packages inside `node_modules`); `dns.reverse()` no longer consulting hosts files; `WebSocket`/`CloseEvent` no longer disableable with `--no-experimental-websocket`; frozen `sourceMap.payload`.
- REPL top-level `await` without the ability to disable it (`--no-experimental-repl-await` still exists in 26).
- 27 removals/deprecations that have **not** happened in 26: `node:domain` runtime deprecation (DEP0032 is documentation-only in 26), passing `CryptoKey` to `node:crypto` becoming an error (DEP0203/0204 are warnings in 26), SHAKE without `outputLength` becoming an error (DEP0198 warns in 26), `zlib` classes without `new` throwing (DEP0184 warns in 26), runtime deprecation of OpenSSL engine options (DEP0183, e.g. `clientCertEngine`), `node:http` classes without `new` (DEP0195), `require('node:_http_*')` (DEP0199), `Hmac#digest()` twice (DEP0206, documentation-only in 26), `net.Server.prototype._listen2` (DEP0208), `AbortSignal` used to dispose resources (DEP0209); end-of-life of `require('node:_tls_common')`/`_tls_wrap` (DEP0192) and of the `domain` property on async resources/`MakeCallback` (DEP0097); new documentation deprecation of `http2` `.aborted`/`'aborted'` (DEP0207). In 26 these still work (with or without a warning).

##### ECMAScript features not shipped in V8 14.6

Verified on 26.12.1 (`typeof` is `undefined` or the syntax is a `SyntaxError`):

- `Math.sumPrecise()` (ES2026, behind `--js-sum-precise` only) — use a compensated summation helper.
- Joint iteration `Iterator.zip()`/`Iterator.zipKeyed()`, iterator chunking `Iterator.prototype.chunks()`/`windows()`, `Iterator.prototype.includes()`, `Iterator.prototype.join()` (all Stage 4 for ES2027, not in V8 14.6).
- Decorators (`@dec class` / `@dec method()`; `--js-decorators` is "in progress / experimental"), `Symbol.metadata`.
- `import defer` (deferred module evaluation), source phase imports for **JavaScript** modules (`import source` works only for `.wasm` modules — see below), `ShadowRealm`, Immutable `ArrayBuffer` (`transferToImmutable()`), `Intl.Locale.prototype.variants`, RegExp buffer boundaries `\A`/`\z`, `Promise.allKeyed()` (Await Dictionary), `String.dedent`, `Math.clamp`.
- `import text from './x.txt' with { type: 'text' }` works only with `--experimental-import-text` (26.5+).

##### Flagged or early-development Node APIs

Do not recommend these for production without the flag being deliberately set and the experimental status accepted: `node:vfs` (`--experimental-vfs`), `node:bench` (`--experimental-bench`), `node:stream/iter` (`--experimental-stream-iter`), Web `Worker` global (`--experimental-web-worker`), `EventSource` (`--experimental-eventsource`), package maps (`--experimental-package-map`), `mock.module()` (`--experimental-test-module-mocks`), DTLS (`--experimental-dtls`), QUIC (`node:quic`, early development), `zlib.ZipFile` and friends (early development), `node:ffi` (on by default since 26.9 but **Stability 1 – Experimental** and inherently memory-unsafe).

##### Removed or deprecated older APIs — flag them in review

| API | Status in 26 | Replacement |
|---|---|---|
| `response.writeHeader()` | **removed** (26.0) | `writeHead()` |
| `require('_stream_readable')` etc., `_stream_wrap` | **removed** (26.0) | `node:stream` |
| `--experimental-transform-types` | **removed** (26.0) | `erasableSyntaxOnly` TS code, or a build step |
| GCM tags < 16 bytes without `authTagLength` | **throws** (26.0) | pass `authTagLength` explicitly |
| `module.register()` | runtime-deprecated (26.0) | `module.registerHooks()` |
| `CryptoKey` passed to `node:crypto` | runtime-deprecated (26.0) | `KeyObject.from(cryptoKey)` or Web Crypto |
| `Duplex.toWeb(s, { type })` | runtime-deprecated (26.0) | `{ readableType }` |
| `sqlite.DatabaseSync`/`StatementSync` | doc-deprecated alias (26.11) | `Database`/`Statement` |
| `AbortSignal` used to dispose long-lived resources (`server.listen({ signal })`) | doc-deprecated (26.8.2, DEP0209) | `using`/`await using` |
| `SlowBuffer`, `fs.F_OK`…, `fs.rmdir({ recursive })`, `assert.CallTracker` | removed (25.0) | `Buffer.allocUnsafeSlow()`, `fs.constants`, `fs.rm()`, `mock.fn()` |
| `url.parse()` | runtime-deprecated (DEP0169) | `new URL()` / `URL.parse()` |
| `new Buffer()` / `Buffer()` | runtime-deprecated outside `node_modules` (DEP0005) | `Buffer.alloc()`/`from()` |
| `punycode` | runtime-deprecated (DEP0040) | `url.domainToASCII()` or a userland package |
| `util.isArray()` | runtime-deprecated (DEP0044) | `Array.isArray()` |
| `util.types.isNativeError()` | doc-deprecated (DEP0197) | `Error.isError()` |
| `child_process` `args` + `shell: true` | runtime-deprecated (DEP0190) | no shell, or escape yourself |
| `zlib` classes without `new` | runtime-deprecated (DEP0184) | `new zlib.Gzip()` or `zlib.createGzip()` |
| `node:domain` | doc-deprecated (DEP0032) | `AsyncLocalStorage`, explicit error handling |
| `process.binding()` | doc-deprecated (DEP0111) | public APIs |

***

#### Version-Aware Review

- **Determine the pinned version first.** Look at `engines.node`, `.nvmrc`/`.node-version`, `volta.node`, `devEngines`, CI `node-version`, and `FROM node:26…` images. A `26` pin without a minor means "any 26.x": prefer APIs that exist since 26.0.0 or require the minor explicitly (`"engines": { "node": ">=26.10.0" }` before using `util.throttle()`).
- **Verify before claiming something is unavailable.** Node 26 has many APIs that older training data does not know (`Temporal`, `getOrInsertComputed`, `Iterator.concat`, `RegExp.escape`, `Promise.try`, `Error.isError`, `Uint8Array.prototype.toBase64`, `util.throttle`, `crypto.randomUUIDv7`, `node:sqlite`, `fs.glob`, `import.meta.dirname`, `require(esm)`, type stripping). Say "not available in Node 26" only when the pinned version demonstrably lacks it.
- **Distinguish "exists" from "stable".** `node:sqlite` is a release candidate (1.2), `module.registerHooks()` is a release candidate, `node:ffi`/`node:vfs`/`process.execve()` are experimental, `mock.module()` and `zlib.ZipFile` are early development. Experimental APIs print an `ExperimentalWarning` and may change in a minor release.
- **Check backports when the project also supports 24.** A library with `"engines": { "node": ">=24" }` must not use Temporal, upsert, `Iterator.concat()`, `Uint8Array` base64 helpers, `Buffer.stringLength()`, `sqlite.Database` (the new name), `--allow-net` or the 26 behaviour of `read()`. 24.22.1 does have several 26.x backports, but earlier 24.x patches do not.
- **Never suggest Node 27 behaviour** (65 s keep-alive default, `node:net/promises`, `fs.mkstemp()`, V8 15 features) for a Node 26 project.
- **Use `process.versions` and feature detection** in libraries that span majors: `typeof Temporal !== 'undefined'`, `typeof Map.prototype.getOrInsertComputed === 'function'`, `process.features.typescript`, `process.features.require_module`.

***

#### Project Setup

##### package.json essentials

```json
{
  "name": "@acme/orders",
  "version": "1.4.0",
  "type": "module",
  "engines": { "node": ">=26.12.1 <27" },
  "exports": {
    ".": "./src/index.js",
    "./package.json": "./package.json"
  },
  "imports": {
    "#config": "./src/config.js",
    "#db/*": "./src/db/*.js"
  },
  "scripts": {
    "start": "node --env-file-if-exists=.env src/server.js",
    "test": "node --test",
    "lint": "eslint ."
  }
}
```

Review rules:

- **Set `"type"` explicitly.** Without it, a `.js` file that uses `import`/`export` is still loaded as ESM (syntax detection), but Node.js re-parses it and emits `MODULE_TYPELESS_PACKAGE_JSON` when a `package.json` without `"type"` is nearby. The warning is a performance smell, not a style issue.
- **`engines.node` should state the minimum minor** the code relies on (`>=26.10.0` for `util.throttle()`, `>=26.11.0` for `sqlite.Database`, `>=26.12.1` for the latest security fixes). npm only warns on an engine mismatch unless `engine-strict=true` is set in `.npmrc`.
- **`exports` is a hard boundary.** Anything not listed throws `ERR_PACKAGE_PATH_NOT_EXPORTED` (verified), including deep imports such as `pkg/lib/x.js`. Export `./package.json` if consumers need it.
- **Subpath imports (`"imports"`, `#name`)** replace fragile `../../..` paths and work in both ESM and CommonJS. Patterns that start with `#/` are allowed since 25.4.0 (also backported to 24.x).
- **Keep `.nvmrc`, CI `node-version`, the Docker base image and `engines` in sync** — a reviewer should flag `FROM node:24` in a repo whose `engines` says 26.
- Node.js 26 bundles npm 11; `npm ci` in CI, a committed lockfile, and `--ignore-scripts` (or an allowlist) for untrusted dependencies.

##### Running scripts and environment files

```bash
node --run test                    # runs package.json "scripts.test" without npm overhead
node --env-file=.env app.js        # missing file is an error
node --env-file-if-exists=.env app.js
node --watch --watch-path=src app.js
node --test --test-reporter=spec
```

`node --run` intentionally does not run `pre`/`post` scripts and does not define package-manager-specific environment variables (it does set `NODE_RUN_SCRIPT_NAME` and `NODE_RUN_PACKAGE_JSON_PATH`); switch to it only where that is acceptable. `process.loadEnvFile(path)` and `util.parseEnv(text)` load `.env` content programmatically. Never commit `.env` files with secrets.

***

#### Modules: ESM, CommonJS and Interop

##### Prefer ESM for new code

```mjs
// src/math.js (in a "type": "module" package)
export function add(a, b) {
  return a + b;
}
export const ZERO = 0;
```

```mjs
import { readFile } from 'node:fs/promises';
import path from 'node:path';

// import.meta.dirname / import.meta.filename replace the __dirname dance (20.11+)
const here = import.meta.dirname;
console.log(path.basename(import.meta.filename), typeof here);

// Built-in modules: always use the node: prefix
const pkg = JSON.parse(await readFile(path.join(here, 'package.json'), 'utf8').catch(() => '{}'));
console.log(typeof pkg);
```

- Always import built-ins with the `node:` prefix. It cannot be shadowed by a package in `node_modules`, and some modules (`node:test`, `node:sqlite`, `node:sea`, `node:ffi`) exist **only** with the prefix.
- Relative ESM specifiers need the file extension (`./util.js`). Missing extensions fail with `ERR_MODULE_NOT_FOUND`.
- `import.meta.main` (24.2+) is `true` when the module is the entry point — the ESM equivalent of `require.main === module`.
- Top-level `await` is fine in applications. In a library it blocks every importer and makes the module unusable from `require()` (see below).

##### JSON and other non-JS imports

```mjs
// BAD: ERR_IMPORT_ATTRIBUTE_MISSING
// import data from './data.json';

// GOOD: import attributes (ES2025)
import { createRequire } from 'node:module';
const require = createRequire(import.meta.url);
console.log(typeof require.resolve);
// import config from './config.json' with { type: 'json' };
```

JSON modules require `with { type: 'json' }` (the old `assert { ... }` syntax was removed in Node 22). JSON modules only expose a default export. Importing text files needs `--experimental-import-text` (26.5+). `import ... from './x.wasm'` works without a flag since 24.5.0/22.19.0 (Stability 1.2) but importing module *instances* prints an `ExperimentalWarning`; `import source mod from './x.wasm'` and `import.source()` (Wasm source phase imports, since 24.0) yield a `WebAssembly.Module` for custom instantiation.

##### `require(esm)` — stable

`require()` can load an ES module synchronously **if the ESM graph has no top-level `await`** (enabled by default since 23.0/22.12, marked stable in 25.4.0 and in 24.x):[^13]

```cjs
// main.cjs
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');

const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'reqesm-'));
fs.writeFileSync(path.join(dir, 'esm.mjs'), 'export default 1; export const named = 2;');
fs.writeFileSync(path.join(dir, 'tla.mjs'), 'await 0; export const x = 1;');

const ns = require(path.join(dir, 'esm.mjs'));
console.log(ns.default, ns.named, ns.__esModule); // 1 2 true

try {
  require(path.join(dir, 'tla.mjs'));
} catch (err) {
  console.log(err.code); // ERR_REQUIRE_ASYNC_MODULE
}
fs.rmSync(dir, { recursive: true });
```

- `require()` returns the **module namespace object**, not the default export. Code that did `const fn = require('./esm.mjs')` and calls `fn()` is wrong — use `require('./esm.mjs').default`, or let the ESM module export a name `"module.exports"` (`export { fn as 'module.exports' }`) to control what `require()` returns.
- `__esModule: true` is added to the namespace when there is a default export so transpiler interop (`_interopRequireDefault`) keeps working.
- Libraries can now ship ESM-only and still be consumed by CommonJS on 22.12+/24/26. The `"module-sync"` export condition selects a synchronous ESM entry for **both** `import` and `require()`.
- `--experimental-print-required-tla` shows where the offending top-level `await` is. `--no-require-module` disables the feature (only for debugging).

##### CommonJS still works — know the edges

```cjs
// In CommonJS, prefer named exports on module.exports and never reassign exports
const util = require('node:util');

function parse(text) {
  return JSON.parse(text);
}
module.exports = { parse };

// Lazy-load a heavy or ESM-only dependency
async function loadLater() {
  const { setTimeout: sleep } = await import('node:timers/promises');
  await sleep(1);
  return 'loaded';
}
loadLater().then((v) => console.log(v, typeof util.inspect));
```

- `exports = {...}` (without `module.`) silently exports nothing.
- `import cjs from './x.cjs'` gives `module.exports` as the default; named imports work only for names the CJS lexer can detect statically (`exports.foo = ...`, `module.exports = { foo }`). Since 25.6.1 the lexer is `merve` instead of `cjs-module-lexer`; detection results can differ slightly for unusual patterns.
- In ESM, `require`, `module`, `__filename` and `__dirname` do not exist. Use `import.meta.*` and `createRequire(import.meta.url)`.
- `process.getBuiltinModule('node:fs')` loads a built-in synchronously from anywhere (useful for isomorphic code that must not use static `import` of `node:` modules).

##### Module customization hooks

```mjs
import { registerHooks } from 'node:module';

// Synchronous, in-thread hooks for both import and require (release candidate)
const hooks = registerHooks({
  resolve(specifier, context, nextResolve) {
    if (specifier === 'virtual:answer') {
      return { url: 'data:text/javascript,export default 42', shortCircuit: true };
    }
    return nextResolve(specifier, context);
  },
});

const { default: answer } = await import('virtual:answer');
console.log(answer); // 42
hooks.deregister();
```

**New in 26:** `module.register()` (off-thread async hooks) emits a runtime deprecation warning (DEP0205).[^22] Migrate loaders to `module.registerHooks()`; `--import` a file that calls it instead of `--loader`. Since 26.7.0, the object returned by `registerHooks()` also implements `Symbol.dispose`, so `using hooks = registerHooks({...})` scopes a hook.

##### Compile cache

`module.enableCompileCache()` (or `NODE_COMPILE_CACHE=dir`) caches V8 code for faster startup of CLIs; `NODE_COMPILE_CACHE_READONLY=1` (26.8+) uses a prepared cache without writing to it — useful for read-only container images.

***

#### TypeScript: Built-in Type Stripping

Node.js 26 runs `.ts`, `.mts` and `.cts` files directly by **erasing type annotations** (stable; `process.features.typescript === 'strip'`). No warning, no flag:[^14]

```ts
// a.ts — run with: node a.ts
import type { IncomingMessage } from 'node:http';

interface Point {
  x: number;
  y: number;
}

function norm(p: Point): number {
  return Math.hypot(p.x, p.y);
}

const describe = (req: Pick<IncomingMessage, 'url'> | undefined): string => req?.url ?? '/';
console.log(norm({ x: 3, y: 4 }), describe(undefined)); // 5 '/'
```

Rules a reviewer must enforce (all verified on 26.12.1):

- **Only erasable syntax.** `enum`, `namespace` with values, parameter properties (`constructor(private x: number)`), and angle-bracket assertions (`<T>x`) fail with `ERR_UNSUPPORTED_TYPESCRIPT_SYNTAX`. `--experimental-transform-types` was **removed in 26.0.0** — use `as const` objects instead of enums, explicit fields instead of parameter properties, `x as T` instead of `<T>x`. Turn on `"erasableSyntaxOnly": true` (TypeScript 5.8+) so `tsc` rejects them too.[^25]
- **Type-only imports must be marked.** `import { Foo } from './types.ts'` where `Foo` is only a type fails at runtime with `SyntaxError: The requested module './types.ts' does not provide an export named 'Foo'`. Use `import type { Foo }` / `import { type Foo }` and enable `"verbatimModuleSyntax": true`.
- **Write the real extension.** `import { helper } from './helper.ts'`; `'./helper'` fails with `ERR_MODULE_NOT_FOUND`. In `tsconfig.json` use `"allowImportingTsExtensions": true` (with `noEmit`) or `"rewriteRelativeImportExtensions": true` if you also emit JS.
- **`tsconfig.json` is ignored** at runtime: `paths`, `baseUrl` and `target` downlevelling have no effect, and decorator syntax (`@d m() {}`) is a `SyntaxError` because V8 14.6 does not implement decorators. Use `package.json` `"imports"` instead of `paths`.
- **No type checking** happens at runtime — CI still needs `tsc --noEmit` (TypeScript 7.0, the native Go compiler, is the current `typescript` release).
- **`.ts` files inside `node_modules` are not stripped** (`ERR_UNSUPPORTED_NODE_MODULES_TYPE_STRIPPING`); publish compiled JavaScript plus `.d.ts`.
- Module format follows the same rules as JS: `.mts` → ESM, `.cts` → CJS, `.ts` → nearest `package.json` `"type"` (with syntax detection as fallback).

Recommended `tsconfig.json` for code that Node 26 runs directly:

```json
{
  "compilerOptions": {
    "target": "esnext",
    "module": "nodenext",
    "lib": ["esnext"],
    "types": ["node"],
    "noEmit": true,
    "strict": true,
    "erasableSyntaxOnly": true,
    "verbatimModuleSyntax": true,
    "allowImportingTsExtensions": true,
    "rewriteRelativeImportExtensions": true
  }
}
```

Use `@types/node@26` so the types match the runtime. TypeScript 6.0/7.0's `esnext` lib includes `Temporal` and `Map.prototype.getOrInsertComputed`, but **not** `Iterator.concat()` (verified: TS2339 with TypeScript 7.0.2) — a missing type is not proof that the runtime lacks the API.

***

#### Language Essentials and Style

##### Declarations, equality and naming

```mjs
// const by default, let when reassigned, never var
const MAX_RETRIES = 3;            // module-level constants: SCREAMING_SNAKE_CASE
let attempt = 0;                  // variables and functions: camelCase
class OrderService {}             // classes: PascalCase
const isReady = attempt < MAX_RETRIES; // booleans read as predicates

// Strict equality everywhere; the only accepted loose form is `x == null`
const value = 0;
console.log(value === 0, value == null, null == undefined, 0 == ''); // true false true true

// Object.is for NaN/-0 aware comparisons
console.log(NaN === NaN, Object.is(NaN, NaN), Object.is(0, -0)); // false true false
console.log(OrderService.name, isReady);
```

- ESM files are always strict mode; CommonJS files are sloppy unless they start with `'use strict'`. In CJS, missing `'use strict'` silently turns assignments to undeclared variables into globals.
- Use `??` (only `null`/`undefined`) instead of `||` when `0`, `''` or `false` are valid values: `const port = options.port ?? 3000`.
- `??=`, `||=`, `&&=` short-circuit; `obj.count ??= 0` only assigns when missing.
- Optional chaining stops at `null`/`undefined` but hides typos: `user?.adress?.city` is silently `undefined`. Use it for genuinely optional data, not to suppress errors.

##### Functions, destructuring and defaults

```mjs
// Options objects with destructured defaults; reject unknown shapes early
function createClient({ baseUrl, timeoutMs = 5_000, retries = 2 } = {}) {
  if (typeof baseUrl !== 'string') throw new TypeError('baseUrl must be a string');
  return { baseUrl, timeoutMs, retries };
}
console.log(createClient({ baseUrl: 'https://api.example' }));

// Defaults apply only to undefined, not null
function f(x = 1) { return x; }
console.log(f(undefined), f(null)); // 1 null

// Rest/spread copies are shallow
const defaults = { headers: { accept: 'application/json' } };
const opts = { ...defaults };
opts.headers.accept = 'text/plain';
console.log(defaults.headers.accept); // 'text/plain' — shared nested object
```

Use `structuredClone()` for deep copies of data (it throws `DataCloneError` for functions, class instances lose their prototype). Never mutate function arguments that the caller still owns.

##### Classes and private state

```mjs
class Account {
  static #count = 0;
  #balance = 0;                      // truly private (brand-checked)
  #log = [];

  constructor(owner) {
    this.owner = owner;
    Account.#count += 1;
  }

  deposit(amount) {
    if (!Number.isFinite(amount) || amount <= 0) throw new RangeError(`invalid amount: ${amount}`);
    this.#balance += amount;
    this.#log.push(amount);
    return this;
  }

  get balance() { return this.#balance; }
  static isAccount(x) { return #balance in x; }   // ergonomic brand check
  static get count() { return Account.#count; }

  toJSON() { return { owner: this.owner, balance: this.#balance }; }
}

const a = new Account('ann').deposit(10);
console.log(a.balance, Account.isAccount(a), Account.isAccount({}), JSON.stringify(a), Account.count);
```

- Prefer `#private` over `_underscore` conventions for invariants; `#x in obj` checks brand without try/catch.
- Static initialization blocks (`static { ... }`) run once at class definition.
- Arrow-function class fields (`handle = () => {}`) create one function per instance; fine for callbacks passed around, wasteful for hot classes with many instances.
- `extends` built-ins (`Error`, `Map`) works; always call `super()` first in derived constructors.
- **Decorators are not available** in Node 26 (`@dec` is a `SyntaxError`); TypeScript `experimentalDecorators` code needs a build step and cannot run under type stripping.

##### Mutability: frozen, sealed and readonly views

```mjs
const config = Object.freeze({ db: { host: 'localhost' } });
// Assigning config.db = 1 in strict mode throws TypeError; nested objects stay mutable:
config.db.host = 'evil';
console.log(config.db.host); // 'evil' — freeze is shallow
```

***

#### Collections and Iterators

##### Map, Set and upsert **New in 26**

`Map.prototype.getOrInsert(key, default)` and `getOrInsertComputed(key, callback)` (same on `WeakMap`) replace the `has`/`get`/`set` dance:

```mjs
// BAD (three lookups, easy to get wrong)
const legacy = new Map();
for (const w of ['a', 'b', 'a']) {
  if (!legacy.has(w)) legacy.set(w, 0);
  legacy.set(w, legacy.get(w) + 1);
}

// GOOD (Node 26)
const counts = new Map();
for (const w of ['a', 'b', 'a']) counts.set(w, counts.getOrInsert(w, 0) + 1);
console.log([...counts]); // [ [ 'a', 2 ], [ 'b', 1 ] ]

const byTeam = new Map();
for (const u of [{ team: 'x', id: 1 }, { team: 'x', id: 2 }]) {
  byTeam.getOrInsertComputed(u.team, () => []).push(u.id);   // callback only runs when the key is missing
}
console.log([...byTeam]); // [ [ 'x', [ 1, 2 ] ] ]

// WeakMap caches keyed by objects
const meta = new WeakMap();
const key = {};
meta.getOrInsertComputed(key, (k) => ({ created: k === key }));
console.log(meta.get(key)); // { created: true }
```

- `getOrInsert(key, value)` evaluates `value` eagerly — use `getOrInsertComputed()` when the default is expensive or allocates (`[]`, `new Set()`).
- `WeakMap#getOrInsert*` with a primitive key throws `TypeError`.
- These methods do not exist on 24/25 — a library supporting them needs a fallback.

**Set algebra** (ES2025): `union`, `intersection`, `difference`, `symmetricDifference`, `isSubsetOf`, `isSupersetOf`, `isDisjointFrom`:

```mjs
const admins = new Set(['ann', 'bob']);
const active = new Set(['bob', 'cy']);
console.log([...admins.intersection(active)], [...admins.difference(active)], admins.isDisjointFrom(new Set(['dan'])));
// [ 'bob' ] [ 'ann' ] true
```

- Use `Map` (not plain objects) for dynamic keys, especially keys from user input: object keys collide with `__proto__`, `constructor`, `toString`.
- `Object.groupBy()` returns a **null-prototype** object; `Map.groupBy()` keeps non-string keys.
- Array copy-methods `toSorted()`, `toReversed()`, `toSpliced()`, `with()` leave the original untouched — prefer them in reducers and shared state.
- `array.sort()` without a comparator sorts **as strings**: `[10, 1, 2].sort()` is `[1, 10, 2]`. Always pass `(a, b) => a - b` for numbers.
- `arr.at(-1)`, `findLast()`, `findLastIndex()`, `Object.hasOwn()` replace index arithmetic and `hasOwnProperty.call`.

##### Iterator helpers and `Iterator.concat()` **New in 26**

Iterator helpers (ES2025) are lazy and work on any iterator, including generators and `Map#entries()`:

```mjs
function* naturals() { let i = 0; while (true) yield i++; }

console.log(naturals().drop(2).take(3).toArray());                 // [ 2, 3, 4 ]
console.log(new Map([['a', 1], ['b', 2]]).keys().map((k) => k.toUpperCase()).toArray()); // [ 'A', 'B' ]

// Iterator.concat (Node 26): lazily chains iterables
function* head() { yield 1; yield 2; }
const all = Iterator.concat(head(), [3, 4], new Set([5]));
console.log(all.filter((n) => n % 2).toArray());                    // [ 1, 3, 5 ]

// Strings are iterable but rejected on purpose
try { Iterator.concat('ab'); } catch (e) { console.log(e.constructor.name); } // TypeError
```

- `take()`/`drop()` on infinite generators are safe; `toArray()` on an infinite iterator never returns.
- Helpers consume the iterator: an iterator can be traversed only once. Re-create it or materialize with `toArray()` when you need two passes.
- `Iterator.from(obj)` wraps an object with a `next()` method so helpers are available.
- `Array.fromAsync(asyncIterable)` collects async iterators (awaits sequentially — it is not `Promise.all`).
- `Iterator.zip()`, `chunks()`, `windows()`, `includes()` and `join()` are **not** available in Node 26.

***

#### Numbers, Strings and Binary Data

##### Numbers and BigInt

```mjs
console.log(0.1 + 0.2 === 0.3, Math.abs(0.1 + 0.2 - 0.3) < Number.EPSILON);  // false true
console.log(Number.MAX_SAFE_INTEGER + 2);                          // 9007199254740992 (precision lost)
console.log(2n ** 64n, typeof 10n);                                // 18446744073709551616n 'bigint'
console.log(Number.parseInt('08', 10), Number('12px'), Number.parseFloat('12px')); // 8 NaN 12
console.log(Number.isNaN('x'), isNaN('x'));                        // false true — global isNaN coerces
try { JSON.stringify({ n: 1n }); } catch (e) { console.log(e.name); } // TypeError: BigInt is not serializable
```

- Money: integer minor units (cents) or a decimal library; never binary floats. `Math.sumPrecise()` is **not** shipped in Node 26.
- IDs from databases that may exceed 2^53 (Snowflake IDs, `BIGINT`): keep them as strings or `BigInt`. In `JSON.parse`, a reviver's third argument exposes the original text (ES2026): `JSON.parse(t, (k, v, ctx) => k === 'id' ? BigInt(ctx.source) : v)`; `JSON.rawJSON('123456789012345678901')` serializes a big number verbatim.
- Always pass the radix to `parseInt`. Validate numeric input with `Number.isFinite()` / `Number.isSafeInteger()`.
- `Float16Array`, `Math.f16round()`, `DataView#getFloat16()` exist (ES2025) for ML/graphics interchange.

##### Strings and Unicode

```mjs
const s = 'naïve 👍';
console.log(s.length, [...s].length);                               // 8 7 — length counts UTF-16 units
console.log('\uD800'.isWellFormed(), '\uD800'.toWellFormed() === '\uFFFD'); // false true
const seg = new Intl.Segmenter('en', { granularity: 'grapheme' });
console.log([...seg.segment('👩‍👩‍👧')].length);                     // 1 grapheme cluster
console.log('Straße'.localeCompare('Strasse', 'de', { sensitivity: 'base' })); // 0
console.log('a-b-c'.replaceAll('-', '+'), 'x'.padStart(3, '0'));    // a+b+c 00x
```

- Use `Intl.Segmenter` for user-visible length/truncation, `toWellFormed()` before handing strings to APIs that reject lone surrogates (`encodeURIComponent` throws `URIError`).
- `String#replace` with a string pattern replaces only the first match, and `$&`, `$1` in the replacement string are interpreted even for string patterns — escape `$` (`$$`) or use a replacer function for untrusted replacement text.

##### `Intl.Locale` getters removed **New in 26**

V8 14 removed the accessor properties `calendars`, `collations`, `hourCycles`, `numberingSystems`, `textInfo`, `timeZones` and `weekInfo` from `Intl.Locale.prototype`; the Intl Locale Info proposal replaced them with `get*()` methods because each access returned a new object. On Node 26 they read as `undefined` — silently (verified against 24.22.1 and 25.9.0, where they still return values):[^24]

```mjs
const loc = new Intl.Locale('de-DE');
console.log(loc.weekInfo);                 // undefined on Node 26 (an object on 24/25)
console.log(loc.getWeekInfo().firstDay);   // 1 — use the method form
console.log(loc.getHourCycles(), loc.getTimeZones());
```

Flag any `locale.weekInfo`, `locale.hourCycles`, `locale.timeZones` etc. in code that targets Node 26.

##### Binary data: Buffer, Uint8Array base64/hex

```mjs
const bytes = new TextEncoder().encode('héllo?');

// ES2026 Uint8Array helpers (New in 25; not in 24)
console.log(bytes.toBase64());                                         // aMOpbGxvPw==
console.log(bytes.toBase64({ alphabet: 'base64url', omitPadding: true })); // aMOpbGxvPw
console.log(bytes.toHex());                                            // 68c3a96c6c6f3f
console.log(Uint8Array.fromHex('ff00'));                               // Uint8Array(2) [ 255, 0 ]
try { Uint8Array.fromBase64('not base64!'); } catch (e) { console.log(e.name); } // SyntaxError — strict

// Buffer is a Uint8Array subclass with Node-specific encodings
const buf = Buffer.from('héllo?', 'utf8');
console.log(buf.toString('base64url'), Buffer.byteLength('héllo?'));  // aMOpbGxvPw 7

// Buffer.stringLength (26.11+): UTF-16 length of the string that decoding WOULD produce,
// computed without decoding — the inverse of Buffer.byteLength()
console.log(Buffer.stringLength(buf), buf.toString().length);         // 6 6
```

- `Buffer.from(base64String)` is **lenient** — it silently ignores invalid characters. `Uint8Array.fromBase64()` throws `SyntaxError`. Prefer the strict one for untrusted input (tokens, signatures).
- `atob`/`btoa` are legacy (Latin-1 only); do not use them for binary data.
- `buf.slice()` is documentation-deprecated (DEP0158) in favour of `buf.subarray()` — both **share memory** with the original. Copy (`Buffer.from(sub)`) before storing a small slice of a large pooled buffer, otherwise the whole backing store stays alive.
- `Buffer.allocUnsafe(n)` returns uninitialized memory from the pool (default **64 KiB pool since 26.3.0** and 24.18.0, 8 KiB before). Never send it before filling every byte. `Buffer.alloc(n)` zero-fills.
- Compare secrets with `crypto.timingSafeEqual()` (equal-length buffers), never `===` or `buf.equals()`.
- `new Buffer()`/`Buffer()` are deprecated (DEP0005) and `SlowBuffer` is gone (25.0).

***

#### Dates and Times: Temporal **New in 26**

`Temporal` is a global in Node 26. Use it for new date/time code; keep `Date` only at boundaries that require it.[^19]

```mjs
const instant = Temporal.Instant.from('2026-10-22T10:00:00Z');
console.log(instant.epochMilliseconds);                         // 1792663200000

const berlin = instant.toZonedDateTimeISO('Europe/Berlin');
console.log(berlin.toString());                                 // 2026-10-22T12:00:00+02:00[Europe/Berlin]
console.log(berlin.add({ days: 10 }).toString());               // 2026-11-01T12:00:00+01:00[Europe/Berlin] (DST-aware)

// Calendar arithmetic clamps by default; ask for rejection explicitly
const jan31 = Temporal.PlainDate.from('2026-01-31');
console.log(jan31.add({ months: 1 }).toString());               // 2026-02-28
try { jan31.add({ months: 1 }, { overflow: 'reject' }); } catch (e) { console.log(e.name); } // RangeError

// Durations
const d = Temporal.Duration.from({ hours: 25 });
console.log(d.total('minutes'), d.round({ largestUnit: 'day' }).toString()); // 1500 P1DT1H
console.log(Temporal.PlainDate.from('2026-10-22').since('2026-01-01', { largestUnit: 'months' }).toString()); // P9M21D

// Comparison: never with < or ===
const x = Temporal.PlainDate.from('2026-01-01');
const y = Temporal.PlainDate.from('2026-01-01');
console.log(x === y, x.equals(y), Temporal.PlainDate.compare(x, y)); // false true 0
try { console.log(x < y); } catch (e) { console.log(e.name); }       // TypeError (valueOf throws)

// Interop with Date and JSON
console.log(new Date(0).toTemporalInstant().toString());       // 1970-01-01T00:00:00Z
console.log(new Date(instant.epochMilliseconds).toISOString());
console.log(JSON.stringify({ at: instant }));                  // {"at":"2026-10-22T10:00:00Z"}
```

Review rules:

- **Pick the right type**: `Instant` for timestamps (logs, DB `timestamptz`), `ZonedDateTime` for wall-clock events in a named zone (meetings, billing cut-offs), `PlainDate` for birthdays/due dates, `PlainTime`/`PlainDateTime` only when no zone applies. Storing a `PlainDateTime` for an event that happened at a real moment is a bug.
- **DST**: `Temporal.ZonedDateTime.from('2026-03-29T02:30[Europe/Berlin]')` (a non-existent local time) resolves to `03:30+02:00` by default (`disambiguation: 'compatible'`); pass `{ disambiguation: 'reject' }` where silent shifting is unacceptable.
- **Parsing is strict**: `Temporal.PlainDate.from('2026-02-30')` throws `RangeError` even with `overflow: 'constrain'` — `overflow` applies only to property bags (`{ year: 2026, month: 2, day: 30 }` → `2026-02-28`).
- `Temporal` objects are immutable; `add()`/`with()` return new objects. Code that ignores the return value (`date.add({ days: 1 });`) is a bug.
- `Temporal.Now.timeZoneId()` returns the host zone (`UTC` in most containers). Server code should not depend on the host zone: pass zone IDs explicitly.
- `JSON.stringify` produces ISO strings; deserialize explicitly with `Temporal.Instant.from()` in a reviver.
- `fs.Stats` exposes `atimeInstant`, `mtimeInstant`, `ctimeInstant`, `birthtimeInstant` (26.2+) with nanosecond precision — also on non-`bigint` stats, where `mtimeMs` is a lossy float (verified: `.123456789Z` vs `1767225600123.4568`).
- Libraries that also run on Node 24 must feature-detect `globalThis.Temporal` and fall back (for example to a polyfill); do not import a Temporal polyfill unconditionally in Node 26-only code — it shadows the native, faster implementation.

`Date` pitfalls that still matter at boundaries: months are 0-based (`new Date(2026, 0, 31)` is January), `new Date('2026-10-22')` is parsed as **UTC** while `new Date('2026-10-22T10:00')` is **local time**, and `Date` objects are mutable.

***

#### Errors

##### Throw `Error` objects, attach causes, keep codes stable

```mjs
class AppError extends Error {
  constructor(message, { code, status = 500, cause } = {}) {
    super(message, { cause });          // standard `cause` (ES2022)
    this.name = new.target.name;        // 'AppError' / subclass name
    this.code = code;
    this.status = status;
  }
}

class NotFoundError extends AppError {
  constructor(resource, id, options) {
    super(`${resource} ${id} not found`, { code: 'E_NOT_FOUND', status: 404, ...options });
  }
}

function loadUser(id) {
  try {
    throw new Error('ENOENT: users.json');
  } catch (err) {
    throw new NotFoundError('user', id, { cause: err });
  }
}

try {
  loadUser(42);
} catch (err) {
  console.log(err instanceof NotFoundError, err.name, err.code, err.status, err.cause.message);
  console.log(Error.isError(err), Error.isError({ message: 'fake' }));   // true false
}
```

- **`Error.isError(value)`** (ES2026) is the reliable brand check across realms: `instanceof Error` is `false` for an error created in another `vm` context or iframe-like realm. It replaces `util.types.isNativeError()` (doc-deprecated, DEP0197).
- Always rethrow with `{ cause }` instead of string-concatenating messages; `util.inspect()` and the default uncaught-error printer show the cause chain.
- Never `throw 'string'` or plain objects — they carry no stack. Lint with `no-throw-literal` / `prefer-promise-reject-errors`.
- Branch on `err.code` (`ENOENT`, `ERR_INVALID_ARG_TYPE`, `ECONNRESET`), never on `err.message` — Node.js messages may change in any release.
- Do not log-and-rethrow at every layer; handle once at the boundary.
- `Error.captureStackTrace(obj)` and `Error.stackTraceLimit` are V8-specific; fine in Node code. In current V8 (24, 25 and 26 alike) `err.stack` is an own accessor property installed at construction (the formatted string is computed lazily), so the stack reflects where the error was *created*, not where it was thrown.

##### Aggregates and suppressed errors

```mjs
const results = await Promise.allSettled([Promise.resolve(1), Promise.reject(new Error('boom'))]);
console.log(results.map((r) => r.status));                       // [ 'fulfilled', 'rejected' ]

try {
  await Promise.any([Promise.reject(new Error('a')), Promise.reject(new Error('b'))]);
} catch (e) {
  console.log(e.constructor.name, e.errors.length);              // AggregateError 2
}

// SuppressedError: a dispose() failure while another error is propagating
try {
  using r = { [Symbol.dispose]() { throw new Error('cleanup failed'); } };
  throw new Error('primary');
} catch (e) {
  console.log(e.constructor.name, e.error.message, e.suppressed.message); // SuppressedError cleanup failed primary
}
```

##### Process-level error handling

- An unhandled promise rejection **terminates the process** (`--unhandled-rejections=throw` is the default since Node 15). Do not "fix" it with `--unhandled-rejections=warn`; fix the missing `await`/`.catch()`.
- `process.on('uncaughtException')` handlers must log and exit — the process state is undefined after an uncaught exception. Never resume serving traffic.
- `util.markPromiseAsHandled(promise)` (26.10+) suppresses the unhandled-rejection tracking for a promise whose rejection is observed elsewhere — use it sparingly, never as a blanket fix.
- `node:domain` is deprecated (DEP0032); use `AsyncLocalStorage` for context and explicit error handling for control flow.

***

#### Asynchronous Programming

##### async/await done right

```mjs
import { setTimeout as sleep } from 'node:timers/promises';

const fetchUser = async (id) => { await sleep(10); return { id }; };
const fetchOrders = async (id) => { await sleep(10); return [{ userId: id }]; };

// BAD: sequential awaits for independent work (20 ms)
async function slow(id) {
  const user = await fetchUser(id);
  const orders = await fetchOrders(id);
  return { user, orders };
}

// GOOD: start both, await together (10 ms)
async function fast(id) {
  const [user, orders] = await Promise.all([fetchUser(id), fetchOrders(id)]);
  return { user, orders };
}

// BAD: forEach ignores returned promises — errors become unhandled rejections,
// and the function returns before the work is done
async function badLoop(ids) {
  ids.forEach(async (id) => { await fetchUser(id); });
}

// GOOD: for...of for sequential, Promise.all/allSettled for parallel
async function goodLoop(ids) {
  for (const id of ids) await fetchUser(id);
}

console.log(await slow(1), await fast(2));
await badLoop([1]);
await goodLoop([1, 2]);
```

- **Every promise must be awaited, returned or explicitly handled.** A floating promise that rejects kills the process. Enable `@typescript-eslint/no-floating-promises` (or `no-void` + review) in TS projects.
- `return await` inside `try` is required for the `catch`/`finally` to see the rejection; outside `try`, `return promise` is fine.
- `Promise.all` rejects on the first failure but does **not** cancel the others; they keep running. Pass an `AbortSignal` to make cancellation real.
- `Promise.allSettled` for best-effort batches; `Promise.any` for "first success"; `Promise.race` leaks the losers' work unless they are aborted.
- `Promise.try(fn)` (ES2025) turns a function that may throw synchronously or return a promise into a promise — removes the `new Promise((resolve) => resolve(fn()))` idiom.
- `Promise.withResolvers()` gives `{ promise, resolve, reject }` for bridging callback/event code — prefer `events.once()` when the source is an `EventEmitter`.
- `await` in a loop over thousands of items is fine for correctness, but `Promise.all(hugeArray.map(fn))` starts everything at once and can exhaust sockets, file descriptors and memory: **bound concurrency**.

##### Bounded concurrency

```mjs
// A small, dependency-free concurrency limiter
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

const out = await mapLimit([1, 2, 3, 4, 5], 2, async (n) => n * 10);
console.log(out); // [ 10, 20, 30, 40, 50 ]
```

##### `util.throttle()` and `util.debounce()` **New in 26.10**

```mjs
import { throttle, debounce } from 'node:util';

// At most 2 calls per 200 ms; extra calls are queued (default overflow: 'queue')
const t0 = Date.now();
const ping = throttle(async (i) => `${i}@${Math.round((Date.now() - t0) / 100)}`, 2, 200);
console.log(await Promise.all([1, 2, 3, 4, 5].map((i) => ping(i))));
// [ '1@0', '2@0', '3@2', '4@2', '5@4' ]

// overflow: 'drop' rejects excess calls with ERR_THROTTLED (rejection pre-marked as handled)
const once = throttle((x) => x, 1, 1000, { overflow: 'drop' });
console.log(await once(1), await once(2).catch((e) => e.code)); // 1 ERR_THROTTLED

// debounce: all callers in the window get the result of the last call
const save = debounce((x) => x * 2, 50);
console.log(await Promise.all([save(1), save(2), save(3)])); // [ 6, 6, 6 ]
```

Both return functions that return **promises**, and both accept a `signal`.[^20] `limit` must be a positive integer (`Infinity` throws `ERR_OUT_OF_RANGE`); use the `concurrency` option to cap in-flight calls. They exist from **26.10.0** (also in 24.22.x), not in 26.0–26.9 — check `engines`.

##### Cancellation with AbortSignal

```mjs
import { setTimeout as sleep } from 'node:timers/promises';
import { once } from 'node:events';
import { EventEmitter } from 'node:events';

async function work(signal) {
  await sleep(1_000, undefined, { signal });   // timers/promises honour signals
  return 'done';
}

// Timeout + caller cancellation combined
const caller = new AbortController();
const signal = AbortSignal.any([caller.signal, AbortSignal.timeout(50)]);
try {
  await work(signal);
} catch (err) {
  console.log(err.name, signal.reason?.name);  // AbortError TimeoutError
}

// Wait for one event with a timeout
const ee = new EventEmitter();
setImmediate(() => ee.emit('ready', 42));
const [value] = await once(ee, 'ready', { signal: AbortSignal.timeout(1_000) });
console.log(value); // 42
```

- Accept an optional `signal` in every long-running async API you write and pass it down (`fetch`, `timers/promises`, `fs.readFile`, `stream.pipeline`, `events.once`, `child_process.spawn`, `fs.stat` since 26.1).
- `signal.throwIfAborted()` at loop boundaries; check `signal.aborted` before starting expensive work.
- `AbortSignal.timeout(ms)` aborts with a `TimeoutError` `DOMException`; manual `abort()` uses `AbortError`. Distinguish them in error handling.
- `signal.addEventListener('abort', fn, { once: true })` — and remove listeners from long-lived signals, otherwise `MaxListenersExceededWarning` and leaks.
- **New in 26 (doc-deprecated, DEP0209):** do not use an `AbortSignal` to *dispose long-lived resources* (`server.listen({ signal })`); use `using`/`await using`. Signals remain the right tool for cancelling operations.

##### The event loop: do not block it

- CPU-heavy work (hashing large data, JSON over many MB, image processing, `zlib` sync APIs, regex on huge input, `crypto.pbkdf2Sync`/`scryptSync`) blocks **every** request. Use the async variants, `worker_threads`, or chunk the work.
- `fs.*Sync`, `child_process.execSync` and `zlib.*Sync` are acceptable at startup and in CLIs, not in request handlers.
- `process.nextTick` callbacks run before I/O and (outside of an already-running microtask, e.g. in CommonJS callbacks) before promise microtasks; recursive `nextTick` starves the loop. Use `setImmediate` to yield.
- `queueMicrotask(fn)` is the standard way to schedule a microtask.
- Measure with `perf_hooks.monitorEventLoopDelay()`; `--process-timeout=30s` (26.11+, active development) exits with code 124 after the duration (unit required: `ms`, `s`, `m`, `h`) and first prints what the main thread was doing and which handles kept the loop alive — helpful for "why does my CLI/test not exit".

##### AsyncLocalStorage for request context

```mjs
import { AsyncLocalStorage } from 'node:async_hooks';
import { setTimeout as sleep } from 'node:timers/promises';

const requestContext = new AsyncLocalStorage();

function log(msg) {
  const ctx = requestContext.getStore();
  console.log(`[${ctx?.requestId ?? '-'}] ${msg}`);
}

async function handle(requestId) {
  return requestContext.run({ requestId }, async () => {
    await sleep(5);
    log('handled');
  });
}

await Promise.all([handle('a'), handle('b')]); // [a] handled / [b] handled
log('outside');                                // [-] outside
```

Store an object per request and never mutate a shared store from another request. `AsyncLocalStorage` is the replacement for `domain` and for passing `req` through every function. `enterWith()` leaks context into the caller — prefer `run()`.

***

#### Explicit Resource Management (`using` / `await using`)

`using` and `await using` (with `DisposableStack`, `AsyncDisposableStack`, `SuppressedError`) are available in Node 26 (already in 24). Disposal runs in reverse order when the block exits, even on exceptions. Node 26 resources that implement the protocols (verified on 26.12.1):

| Resource | Protocol |
|---|---|
| `fs/promises` `FileHandle`, `fsPromises.mkdtempDisposable()` result | `Symbol.asyncDispose` |
| `fs.Dir` (`opendir`) | both |
| `http.Server`, `net.Server`, `worker_threads.Worker` | `Symbol.asyncDispose` |
| `sqlite.Database`, `sqlite.Statement` (26.8+), `ChildProcess`, `readline.Interface`, timers (`Timeout`) | `Symbol.dispose` |
| `module.registerHooks()` handle (26.7+), `net.BoundSocket`, `v8` profile handles | `Symbol.dispose` |

```mjs
import { open, mkdtempDisposable } from 'node:fs/promises';
import os from 'node:os';
import path from 'node:path';

async function writeReport(text) {
  await using tmp = await mkdtempDisposable(path.join(os.tmpdir(), 'report-'));  // removed on exit
  await using fh = await open(path.join(tmp.path, 'report.txt'), 'w');           // closed on exit
  await fh.writeFile(text);
  return (await fh.stat()).size;
}
console.log(await writeReport('hello')); // 5

// Compose ad-hoc cleanups
{
  using stack = new DisposableStack();
  const timer = stack.adopt(setInterval(() => {}, 1_000), clearInterval);
  stack.defer(() => console.log('deferred cleanup'));
  console.log(typeof timer);
} // prints "deferred cleanup", interval cleared
```

Review rules: `using` is a `SyntaxError` directly inside a `switch` `case` clause (wrap the case body in braces) and at the top level of a classic script (fine in ESM, CommonJS modules and blocks); the value must be an object with the method or `null`/`undefined`; do not `return` a resource declared with `using` — it is already disposed when the caller gets it.

***

#### Events

```mjs
import { EventEmitter, on } from 'node:events';

class Job extends EventEmitter {}
const job = new Job({ captureRejections: true });   // async listener rejections → 'error'
job.on('error', (err) => console.log('job error:', err.message));
job.on('run', async () => { throw new Error('listener failed'); });
job.emit('run');

// Async iteration over events with cancellation
const ac = new AbortController();
const ticker = new EventEmitter();
setImmediate(() => { ticker.emit('tick', 1); ticker.emit('tick', 2); ac.abort(); });
try {
  for await (const [n] of on(ticker, 'tick', { signal: ac.signal })) console.log('tick', n);
} catch (err) {
  console.log(err.name); // AbortError
}
```

- An `'error'` event without a listener **throws** and usually crashes the process. Every long-lived emitter (streams, sockets, servers, child processes) needs an `'error'` handler.
- Async listeners: without `captureRejections: true` a rejecting listener is an unhandled rejection.
- `MaxListenersExceededWarning` signals a leak (listeners added per request and never removed) — fix the leak instead of raising `setMaxListeners`.
- `EventTarget`/`Event` exist globally for web-compatible code; `events.listenerCount()` accepts `EventTarget`s since 25.4 (also 24.x).

***

#### Streams

##### Use `pipeline`, never bare `.pipe()`

```mjs
import { pipeline } from 'node:stream/promises';
import { Readable, Writable } from 'node:stream';
import { createGzip, createGunzip } from 'node:zlib';

const chunks = [];
await pipeline(
  Readable.from(['a,b\n', 'c,d\n']),
  async function* toUpper(source, { signal }) {        // async generator transform
    for await (const chunk of source) {
      signal.throwIfAborted();
      yield String(chunk).toUpperCase();
    }
  },
  createGzip(),
  createGunzip(),
  new Writable({
    write(chunk, _enc, cb) { chunks.push(chunk); cb(); },
  }),
);
console.log(Buffer.concat(chunks).toString()); // A,B\nC,D\n
```

`src.pipe(dst)` does not forward errors or destroy the other streams on failure — file descriptors and sockets leak and errors go unhandled. `pipeline()` from `node:stream/promises` handles errors, cleanup and backpressure, and accepts `{ signal }` as the last argument.

##### `readable.read()` returns one chunk **New in 26**

```mjs
import { Readable } from 'node:stream';

const r = new Readable({ read() {} });
r.push('ab'); r.push('cd'); r.push('ef');
r.once('readable', () => {
  const first = r.read();
  console.log(JSON.stringify(String(first)), r.readableLength);
  // Node 26: "ab" 4      Node 24/25: "abcdef" 0
  const rest = r.read(r.readableLength);          // explicit size = old behaviour
  console.log(String(rest));                      // cdef
});
```

Code that calls `read()` once per `'readable'` event and assumes it drained the buffer loses data on Node 26[^17] (or stalls, waiting for another `'readable'`). Loop `while ((chunk = r.read()) !== null)` or use `for await (const chunk of r)`.

##### Backpressure and object mode

```mjs
import { Writable } from 'node:stream';
import { once } from 'node:events';

const sink = new Writable({
  highWaterMark: 4,
  write(chunk, _enc, cb) { setImmediate(cb); },
});

for (let i = 0; i < 10; i++) {
  if (!sink.write(`line ${i}\n`)) {
    await once(sink, 'drain');           // BAD to ignore the false return: unbounded memory
  }
}
sink.end();
await once(sink, 'finish');
console.log('written');
```

- `write()` returning `false` means "stop until `'drain'`". Ignoring it buffers everything in memory.
- Default `highWaterMark` is 64 KiB for byte streams (since Node 22) and 16 objects in object mode.
- `stream.finished(stream)` / `finished` from `node:stream/promises` to wait for completion with error propagation.
- `stream.compose()` is stable (26.2); `Readable.prototype.map/filter/take` (stream helpers) are experimental — prefer `pipeline` with async generators in production.

##### Web Streams interop

```mjs
import { Readable } from 'node:stream';
import { text } from 'node:stream/consumers';

const web = new ReadableStream({
  start(controller) { controller.enqueue('hello '); controller.enqueue('web'); controller.close(); },
});
console.log(await text(Readable.fromWeb(web)));                       // hello web

const nodeStream = Readable.from([Buffer.from('x'), Buffer.from('y')]); // bytes, not strings
const back = Readable.toWeb(nodeStream);
console.log(await new Response(back).text());                         // xy
```

`Readable.from(['x', 'y'])` creates an **object-mode** stream of strings; converted with `toWeb()` and fed to `new Response()` it fails with `TypeError: Received non-Uint8Array chunk`. Emit `Buffer`/`Uint8Array` chunks when a byte stream is expected.

Use `node:stream/consumers` (`text`, `json`, `buffer`, `arrayBuffer`, `bytes` since 25.6/24.x) instead of hand-rolled concatenation. When a size limit matters (HTTP bodies), count bytes while reading and abort — the consumers read everything. `Duplex.toWeb(d, { type: 'bytes' })` is deprecated (DEP0201) — use `readableType`.

***

#### HTTP Client: `fetch()` and Undici 8

```mjs
import http from 'node:http';

const server = http.createServer((req, res) => {
  if (req.url === '/slow') return setTimeout(() => res.end('late'), 500);
  res.writeHead(req.url === '/missing' ? 404 : 200, { 'content-type': 'application/json' });
  res.end(JSON.stringify({ url: req.url }));
});
await new Promise((resolve) => server.listen(0, '127.0.0.1', resolve));
const base = `http://127.0.0.1:${server.address().port}`;

async function getJson(url, { signal } = {}) {
  const res = await fetch(url, {
    signal: signal ? AbortSignal.any([signal, AbortSignal.timeout(200)]) : AbortSignal.timeout(200),
    headers: { accept: 'application/json' },
  });
  if (!res.ok) {
    await res.body?.cancel();                    // release the connection
    throw new Error(`GET ${url} failed: ${res.status}`);
  }
  return res.json();
}

console.log(await getJson(`${base}/ok`));                              // { url: '/ok' }
await getJson(`${base}/missing`).catch((e) => console.log(e.message)); // ... failed: 404
await getJson(`${base}/slow`).catch((e) => console.log(e.name));       // TimeoutError
server.closeAllConnections();
server.close();
```

Review rules:

- **`fetch` does not reject on HTTP errors** — check `res.ok`/`res.status`.
- **Always consume or cancel the body.** An unread body keeps the connection (and memory) busy until GC; with HTTP/2 it holds a stream slot.
- **There is no default overall timeout**[^28] — undici only applies a 300 s headers timeout and a 300 s timeout *between* body chunks, so a slow server can hold a request for hours. Always pass `AbortSignal.timeout()`.
- **New in 26: HTTP/2 by default.** Undici 8 offers `h2` via ALPN for `https:` requests, so servers and proxies that negotiate HTTP/2 now receive HTTP/2 from `fetch`. Header names are lower-cased, connection-specific headers (`connection`, `keep-alive`, `transfer-encoding`) are invalid in HTTP/2, and server-side code that inspects `req.httpVersion` sees `'2.0'`.
- **Proxies**: `fetch` ignores `HTTP_PROXY`/`HTTPS_PROXY` unless the process runs with `NODE_USE_ENV_PROXY=1` or `--use-env-proxy` (active development), or calls `http.setGlobalProxyFromEnv()`; `NO_PROXY` is honoured in that mode.
- `fetch` follows redirects by default (`redirect: 'follow'`, up to 20). For SSRF-sensitive code use `redirect: 'manual'` and validate every hop.
- Response size limits: `await res.text()` reads everything; for untrusted servers stream `res.body` and stop after N bytes.
- Use the bundled `fetch`/`undici` globals (`process.versions.undici`), or pin the `undici` npm package (8.x requires Node ≥ 22.19) when you need `Agent`, `Pool` or `MockAgent` — do not mix dispatchers from a different major version with the global `fetch`.
- `WebSocket` is a stable global client; `http.websocketMask()`/`websocketUnmask()` (26.12) are low-level helpers for servers, not a WebSocket server.

***

#### HTTP Servers

```mjs
import http from 'node:http';

const server = http.createServer({
  requestTimeout: 30_000,      // whole request (default 300_000 ms)
  headersTimeout: 10_000,      // headers (default 60_000 ms)
  keepAliveTimeout: 65_000,    // default 5_000 ms in Node 26; must exceed the LB idle timeout
  maxHeaderSize: 16_384,
}, async (req, res) => {
  try {
    // req.signal (26.1+) aborts when the request is destroyed or the socket closes early;
    // before 26.7 it was also aborted after normal completion — do not treat that as an error
    const body = await readBody(req, 1_000_000, req.signal);
    res.writeHead(200, { 'content-type': 'application/json' });
    res.end(JSON.stringify({ bytes: body.length }));
  } catch (err) {
    const status = err.code === 'E_TOO_LARGE' ? 413 : 400;
    if (!res.headersSent) res.writeHead(status).end();
    else res.destroy(err);
  }
});

async function readBody(req, limit, signal) {
  const chunks = [];
  let size = 0;
  for await (const chunk of req) {
    signal?.throwIfAborted();
    size += chunk.length;
    if (size > limit) throw Object.assign(new Error('body too large'), { code: 'E_TOO_LARGE' });
    chunks.push(chunk);
  }
  return Buffer.concat(chunks);
}

await new Promise((resolve) => server.listen(0, '127.0.0.1', resolve));
const res = await fetch(`http://127.0.0.1:${server.address().port}/`, { method: 'POST', body: 'abc' });
console.log(res.status, await res.json());   // 200 { bytes: 3 }

// Graceful shutdown: stop accepting, close idle keep-alive sockets, wait
server.close();
server.closeIdleConnections();
```

Review rules:

- **Body size limits** on every endpoint that reads a body; a missing limit is a DoS.
- **`keepAliveTimeout`** defaults to 5 s in Node 26 (Node 27 changes it to 65 s). Behind a load balancer with a 60 s idle timeout, the server closes first and clients see sporadic `ECONNRESET`/502 — set it above the LB timeout.
- **`res.writeHeader()` is removed** in Node 26 — use `writeHead()`.
- **Never set headers from unvalidated input**: Node throws `ERR_INVALID_CHAR` on CR/LF, but values can still carry other injection (e.g., open redirects in `Location`). `http.isValidHeaderName()`/`isValidHeaderValue()` (26.11) validate before setting.
- **Error handling**: an exception thrown in an `async` request listener is an unhandled rejection that kills the process. Catch in the handler (or use a framework that does) and respond with 500.
- **HTTP upgrades (New in 26)**: request bodies on `Upgrade` requests are parsed;[^18] read the body from `req` and the upgraded protocol from the stream argument; use `req.socket` for socket-level fields.
- **1xx**: `res.writeInformation(status, headers)` (26.2) sends any 1xx (e.g., 103 Early Hints via `writeEarlyHints()`).
- **Shutdown**: on `SIGTERM` call `server.close()` (stops accepting), then `closeIdleConnections()`; give in-flight requests a deadline and finally `closeAllConnections()`. `http.Server` implements `Symbol.asyncDispose` (calls `server.close()`, which also closes idle connections since Node 19, and resolves when the server has closed).
- Security headers, CORS, rate limiting and request IDs belong in middleware; validate `Host` when generating absolute URLs.

***

#### File System and Paths

##### Promise APIs, explicit encodings, atomic writes

```mjs
import { readFile, writeFile, rename, mkdir, rm, glob } from 'node:fs/promises';
import os from 'node:os';
import path from 'node:path';

const dir = path.join(os.tmpdir(), `fsdemo-${process.pid}`);
await mkdir(dir, { recursive: true });                    // no error if it exists

// Atomic replace: write a temp file in the same directory, then rename
async function writeJsonAtomic(file, data) {
  const tmp = `${file}.${process.pid}.${Date.now()}.tmp`;
  await writeFile(tmp, JSON.stringify(data), { encoding: 'utf8', mode: 0o600 });
  await rename(tmp, file);
}
const file = path.join(dir, 'state.json');
await writeJsonAtomic(file, { ok: true });
console.log(JSON.parse(await readFile(file, 'utf8')));    // { ok: true }

// ENOENT is an expected condition, not an exception to swallow blindly
const missing = await readFile(path.join(dir, 'nope.json'), 'utf8').catch((err) => {
  if (err.code === 'ENOENT') return null;
  throw err;
});
console.log(missing);                                     // null

// fs.glob is stable (since 24.0); returns an async iterator. `followSymlinks` (26.1) and `maxDepth` (26.9) are new
for await (const f of glob('*.json', { cwd: dir })) console.log('found', f);

await rm(dir, { recursive: true, force: true });
```

- Pass an encoding (`'utf8'`) when you want a string; without it you get a `Buffer`.
- **TOCTOU**: `fs.existsSync(p)` / `access()` followed by `open()` is a race. Open directly and handle `ENOENT`/`EEXIST` (`flag: 'wx'` for "create, fail if exists").
- `fs.rm(path, { recursive: true, force: true })` replaces `rmdir({ recursive })`, which **throws** since 25.0.
- `fs.constants.F_OK` etc. — the `fs.F_OK` aliases were removed in 25.0.
- Use `fsPromises.mkdtemp()` / `mkdtempDisposable()` for temp directories; never predictable names in `/tmp`.
- `fs.watch` is platform-dependent; it has an `ignore` option since 25.5. For production file watching use a tested library or `node --watch` for dev.
- `fs.readFile()` loads the whole file in memory; use streams for large or untrusted-size files.
- `dirent.parentPath` (not `dirent.path`, end-of-life since 24.0 and `undefined` in Node 26) gives the directory of an entry from `readdir({ recursive: true, withFileTypes: true })`.
- `fs.openAsBlobSync()` (26.10) and `fs.openAsBlob()` return a `Blob` backed by the file — useful for `fetch` uploads.

##### Path traversal: resolve, then check containment

```mjs
import path from 'node:path';

const ROOT = path.resolve('/srv/uploads');

// BAD: path.join does not stop "..", and startsWith without a separator
// accepts sibling directories like /srv/uploads-evil
function badResolve(userPath) {
  const full = path.join(ROOT, userPath);
  return full.startsWith(ROOT) ? full : null;
}

// GOOD
function safeResolve(userPath) {
  if (userPath.includes('\0')) return null;              // NUL bytes are rejected by fs anyway
  const full = path.resolve(ROOT, userPath);             // absolute userPath replaces ROOT!
  const rel = path.relative(ROOT, full);
  if (rel === '' || rel.startsWith('..') || path.isAbsolute(rel)) return null;
  return full;
}

for (const p of ['a/b.txt', '../etc/passwd', '/etc/passwd', '../uploads-evil/x']) {
  console.log(p.padEnd(18), badResolve(p), safeResolve(p));
}
// a/b.txt            /srv/uploads/a/b.txt   /srv/uploads/a/b.txt
// ../etc/passwd      null                   null
// /etc/passwd        /srv/uploads/etc/passwd  null   (join treats it as relative, resolve as absolute)
// ../uploads-evil/x  /srv/uploads-evil/x    null   (startsWith bug)
```

- Decode URL paths **once** (`decodeURIComponent`) before the check, never after.
- Symlinks inside the root can still point outside: use `fs.realpath()` on the result (and on `ROOT`) before the containment check when users can create links, or run with the permission model's `--allow-fs-read` scoped to the directory.
- `path.normalize()` alone is not a security check.
- On Windows also reject drive letters and UNC paths (`path.win32.isAbsolute`).

***

#### Child Processes

```mjs
import { execFile, spawn } from 'node:child_process';
import { once } from 'node:events';
import { promisify } from 'node:util';

const execFileAsync = promisify(execFile);

// GOOD: no shell, arguments as an array, timeout and output cap
const userInput = 'hello; echo INJECTED';
const { stdout } = await execFileAsync('echo', [userInput], {
  timeout: 5_000,
  maxBuffer: 1024 * 1024,
  encoding: 'utf8',
});
console.log(JSON.stringify(stdout));                     // "hello; echo INJECTED\n" — literal

// Streaming output; 'error' (ENOENT when the binary is missing) must be handled — once() does it
const child = spawn('ls', ['-1', '/'], { stdio: ['ignore', 'pipe', 'inherit'] });
const closed = once(child, 'close');                      // subscribe BEFORE consuming output; rejects on 'error'
let lines = 0;
for await (const chunk of child.stdout) lines += String(chunk).split('\n').length - 1;
const [code] = await closed;
console.log(lines > 0, code);                             // true 0
```

- **`exec()`/`execSync()` and `{ shell: true }` with user input are command injection.** Since 23.11/22.15, passing an `args` array together with `shell: true` emits DEP0190 because the arguments are only space-joined, not escaped (verified: `spawnSync('echo', ['hello; echo INJECTED'], { shell: true })` runs the second command).
- On Windows, `.bat`/`.cmd` files run through `cmd.exe`; Node refuses to spawn them without `shell: true` since the 2024 security fix (CVE-2024-27980)[^23] — treat such code paths as shell execution.
- Always set `timeout` (or a `signal`) and `maxBuffer`; the default `maxBuffer` is 1 MiB and overflowing it kills the child.
- `stdio: 'inherit'` for CLI wrappers; pipe and consume `stdout`/`stderr` otherwise — an unread pipe fills and blocks the child.
- Pass a minimal `env` (`{ PATH: process.env.PATH, ... }`) to children that should not see secrets.
- `process.execve()` (experimental, Unix) replaces the current process; since 26.1 it throws on failure instead of aborting.
- Child processes are denied under `--permission` unless `--allow-child-process` is given.

***

#### Security

##### The permission model

The permission model is **stable** (Stability 2) in Node 26.[^15] It restricts what *the process* can do — it is defence in depth against compromised dependencies, not a sandbox for running hostile code (once native addons, `node:ffi` or child processes are allowed they can bypass it, and several 26.x security releases fixed bypasses).

```bash
node --permission \
  --allow-fs-read=/app --allow-fs-read=/etc/myapp/config.json \
  --allow-fs-write=/var/lib/myapp/ \
  --allow-net \
  app.js
```

| Flag | Grants | Since |
|---|---|---|
| `--permission` | enables enforcement; everything below is denied by default | stable (23.5 / 22.13) |
| `--allow-fs-read=path`, `--allow-fs-write=path` | file-system access (repeatable, `*` wildcard) | 20 |
| `--allow-child-process`, `--allow-worker`, `--allow-wasi`, `--allow-addons` | spawning, workers, WASI, native addons | 20–22 |
| `--allow-net` | outbound and inbound network (experimental warning) | **25.0** |
| `--allow-inspector` | inspector | 25.0 / 24.12 |
| `--allow-ffi`, `--allow-openssl-store`, `--allow-fs-vfs` | `node:ffi`, OpenSSL STORE key loading, VFS mounts | **26.1, 26.7, 26.9** |
| `--permission-audit` | check and report on `node:permission-model:*` diagnostics channels, never deny | 25.8 / 24.x |

```mjs
// Runtime checks and irreversible dropping (process.permission is undefined without --permission)
if (process.permission) {
  console.log(process.permission.has('fs.read', '/etc/passwd'));
  // Drop exactly what was granted: dropping a file inside a granted directory is a silent no-op
  // process.permission.drop('fs.read', '/etc/myapp');   // 26.3+ (and 24.22)
} else {
  console.log('permission model not enabled');
}
```

Verified behaviour on 26.12.1: without `--allow-net`, `fetch()` fails with `cause.code === 'ERR_ACCESS_DENIED'`; `process.permission.drop('fs.read', file)` for a file inside a granted directory leaves access unchanged (`has()` stays `true`), while dropping the granted directory itself revokes it. Dropping never closes already-open file descriptors or sockets.

##### Prototype pollution

```mjs
const payload = JSON.parse('{"__proto__": {"isAdmin": true}}');   // JSON.parse creates an OWN "__proto__" key

// BAD: recursive merge with for...in and plain-object targets
function naiveMerge(target, src) {
  for (const k in src) {
    if (src[k] && typeof src[k] === 'object') naiveMerge((target[k] ??= {}), src[k]);
    else target[k] = src[k];
  }
  return target;
}
naiveMerge({}, payload);
console.log(({}).isAdmin);                  // true — every object in the process is now "admin"
delete Object.prototype.isAdmin;

// Object.assign also uses [[Set]] and changes the prototype of the target
console.log(Object.assign({}, payload).isAdmin, { ...payload }.isAdmin); // true undefined

// GOOD: skip dangerous keys, use null-prototype objects or Map for dynamic keys
const BLOCKED = new Set(['__proto__', 'constructor', 'prototype']);
function safeMerge(target, src) {
  for (const k of Object.keys(src)) {
    if (BLOCKED.has(k)) continue;
    const v = src[k];
    if (v && typeof v === 'object' && !Array.isArray(v)) {
      target[k] = safeMerge(Object.hasOwn(target, k) ? target[k] : Object.create(null), v);
    } else {
      target[k] = v;
    }
  }
  return target;
}
safeMerge({}, payload);
console.log(({}).isAdmin);                  // undefined
```

- Validate request bodies with a schema (Zod, Ajv with `removeAdditional`, Valibot) before they reach merge/set-by-path helpers.
- Lookups on plain objects with user keys (`handlers[req.query.action]()`) can hit `constructor`, `toString`, `__proto__`. Use `Map` or `Object.hasOwn()`.
- `node --disable-proto=throw` (or `=delete`) removes the `__proto__` accessor process-wide (`ERR_PROTO_ACCESS`); worth enabling in services that do not need it.
- `Object.freeze(Object.prototype)` at startup is effective but breaks some libraries; test before adopting.

##### Injection and unsafe evaluation

- **SQL**: always parameterize (`?`/`:name` placeholders, tagged templates of a driver that parameterizes). String-built SQL is a blocker in review.
- **`eval`, `new Function`, `vm.runInNewContext` on untrusted input**: `node:vm` is **not** a security boundary — code can escape via `this.constructor.constructor('return process')()` (verified: returns the host `process`). Use a separate process with the permission model, or a real isolate.
- **Templates**: HTML escaping is the template engine's job; never build HTML with string concatenation of user data.
- **Deserialization**: `v8.deserialize()`, packages such as `node-serialize`, and YAML loaders with custom tags can execute code or create gadgets — only for trusted data.
- **Regular expressions (ReDoS)**: nested quantifiers (`(a+)+`, `(\w+\s?)*$`) on attacker-controlled strings can take exponential time and block the event loop. Cap input length, rewrite the pattern, or run with V8's linear-time engine (`--enable-experimental-regexp-engine` + the `l` flag; experimental, not for production). Use `RegExp.escape()` (ES2025) when embedding user text in a pattern — note it escapes a leading alphanumeric as `\x..`.
- **SSRF**: validate outbound URLs (scheme allowlist, resolve DNS and reject private ranges with `net.BlockList` — `BlockList.PRIVATE_RANGES` exists since 26.8 (also 24.22) — and re-check after redirects; use `redirect: 'manual'`).
- **Headers**: Node rejects CR/LF in header values (`ERR_INVALID_CHAR`), but `Location`, `Set-Cookie` and `Content-Disposition` values still need semantic validation.
- **Open redirects**: only redirect to relative paths or allowlisted hosts (`new URL(target, base).origin === base.origin`).

##### Secrets, dependencies and supply chain

- Secrets from the environment or a secret manager — never committed `.env`, never logged. `util.inspect` and error objects can leak headers: redact `authorization`, `cookie`, tokens before logging.
- `npm ci` with a lockfile, `npm audit` / `osv-scanner` in CI, `--ignore-scripts` for installs where possible, and review new dependencies' install scripts.
- Pin the Docker base image to a 26.x patch digest and rebuild on Node security releases (26.12.1 is the current one).
- Run as a non-root user; `NODE_ENV=production` changes framework behaviour, not Node's.
- `NODE_OPTIONS` from untrusted environments can inject `--require`/`--import` — do not let users set it.

***

#### Cryptography

```mjs
import crypto from 'node:crypto';
import { promisify } from 'node:util';

const scrypt = promisify(crypto.scrypt);

// Passwords: slow KDF with per-user salt; constant-time comparison
async function hashPassword(password) {
  const salt = crypto.randomBytes(16);
  const key = await scrypt(password.normalize('NFKC'), salt, 64, { N: 2 ** 15, r: 8, p: 1, maxmem: 64 * 1024 * 1024 });
  return `scrypt$${salt.toString('base64')}$${key.toString('base64')}`;
}
async function verifyPassword(password, stored) {
  const [, saltB64, keyB64] = stored.split('$');
  const expected = Buffer.from(keyB64, 'base64');
  const actual = await scrypt(password.normalize('NFKC'), Buffer.from(saltB64, 'base64'), expected.length, { N: 2 ** 15, r: 8, p: 1, maxmem: 64 * 1024 * 1024 });
  return crypto.timingSafeEqual(actual, expected);
}
const stored = await hashPassword('correct horse');
console.log(await verifyPassword('correct horse', stored), await verifyPassword('wrong', stored)); // true false

// Authenticated encryption: AES-256-GCM with a fresh 12-byte IV and explicit tag length
function encrypt(key, plaintext, aad) {
  const iv = crypto.randomBytes(12);
  const cipher = crypto.createCipheriv('aes-256-gcm', key, iv, { authTagLength: 16 });
  cipher.setAAD(aad);
  const ct = Buffer.concat([cipher.update(plaintext, 'utf8'), cipher.final()]);
  return { iv, ct, tag: cipher.getAuthTag() };
}
function decrypt(key, { iv, ct, tag }, aad) {
  const decipher = crypto.createDecipheriv('aes-256-gcm', key, iv, { authTagLength: 16 });
  decipher.setAAD(aad);
  decipher.setAuthTag(tag);
  return Buffer.concat([decipher.update(ct), decipher.final()]).toString('utf8');  // throws if tampered
}
const key = crypto.randomBytes(32);
const box = encrypt(key, 'secret', Buffer.from('user:42'));
console.log(decrypt(key, box, Buffer.from('user:42')));                    // secret

// Short tags without authTagLength are an error since 26.0 (DEP0182 end-of-life)
try {
  const d = crypto.createDecipheriv('aes-256-gcm', key, box.iv);
  d.setAuthTag(box.tag.subarray(0, 4));
} catch (e) {
  console.log(e.code);                                                     // ERR_CRYPTO_INVALID_AUTH_TAG
}

// IDs and hashing
console.log(crypto.randomUUID().length, crypto.randomUUIDv7().length);      // 36 36 (v7: 26.1+)
console.log(crypto.hash('sha256', 'abc'));                                  // one-shot digest (hex)
console.log(crypto.createHash('shake256', { outputLength: 32 }).update('x').digest('hex').length); // 64
```

- **Passwords**: `scrypt` (or Argon2 via a vetted package) with a per-user salt; never plain SHA-*/MD5, never `pbkdf2` with low iteration counts. Use the async APIs in servers.
- **Comparison of secrets/HMACs**: `crypto.timingSafeEqual()` with equal-length buffers (check lengths first — it throws otherwise).
- **Randomness**: `crypto.randomBytes`, `randomInt`, `randomUUID`, `getRandomValues` — never `Math.random()` for tokens.
- **UUIDv7** (`crypto.randomUUIDv7()`, 26.1+) embeds a millisecond timestamp — good for index locality, but it leaks creation time; use v4 for public opaque IDs.
- **GCM**: never reuse an IV with the same key; pass `authTagLength` on both sides.
- **XOF hashes**: SHAKE128/256 without `outputLength` are runtime-deprecated (DEP0198) and already end-of-life on the Node 27 development branch.
- **`CryptoKey` in `node:crypto` APIs** (`createHmac(alg, cryptoKey)`, `createCipheriv(..., cryptoKey, ...)`) is runtime-deprecated in 26 (DEP0203). Convert with `KeyObject.from(cryptoKey)` or stay in Web Crypto (`crypto.subtle`).
- **`hmac.digest()` twice** returns an empty buffer (and is deprecated, DEP0206); create a new `Hmac` per message.
- Node 26 adds `crypto.createMac()` (generic MAC API, 26.9), `crypto.parsePKCS12()` (26.10), SIV/GCM-SIV modes (26.8) and post-quantum ML-KEM/ML-DSA (PKCS#8 export defaults to seed-only since 26.0). OpenSSL is 3.5.x; legacy algorithms are unavailable by default (`createHash('md4')` throws `ERR_OSSL_EVP_UNSUPPORTED`) — do not re-enable them with `--openssl-legacy-provider`.
- TLS: keep `rejectUnauthorized` at its default `true`; never set `NODE_TLS_REJECT_UNAUTHORIZED=0`. Use `NODE_EXTRA_CA_CERTS` or `--use-system-ca` for private CAs.

***

#### Data: `node:sqlite`

`node:sqlite` is unflagged and a **release candidate** (Stability 1.2).[^16] In Node 26.11+ the classes are named `Database` and `Statement`; `DatabaseSync` and `StatementSync` are deprecated aliases (DEP0210/DEP0211) and the only names that exist on 24/25 and on 26.0–26.10.

```mjs
import { Database } from 'node:sqlite';    // 26.11+; use DatabaseSync for older versions

using db = new Database(':memory:');
db.exec(`CREATE TABLE users (
  id INTEGER PRIMARY KEY,
  email TEXT UNIQUE NOT NULL,
  nick TEXT
) STRICT`);

using insert = db.prepare('INSERT INTO users (email, nick) VALUES (?, ?)');
console.log(insert.run('a@example.com', 'a'));          // { changes: 1, lastInsertRowid: 1 }
console.log(insert.run('b@example.com', undefined));    // undefined binds NULL (26.10+; TypeError before)

// Parameters, never string interpolation
const byEmail = db.prepare('SELECT id, nick FROM users WHERE email = :email');
console.log(byEmail.get({ email: "x' OR '1'='1" }));     // undefined
console.log(byEmail.get({ email: 'a@example.com' }));    // [Object: null prototype] { id: 1, nick: 'a' }

// Transactions: explicit BEGIN/COMMIT/ROLLBACK
db.exec('BEGIN');
try {
  insert.run('c@example.com', 'c');
  insert.run('c@example.com', 'dup');                    // UNIQUE violation
  db.exec('COMMIT');
} catch (err) {
  db.exec('ROLLBACK');
  console.log('rolled back:', err.code, err.errstr);     // ERR_SQLITE_ERROR constraint failed
}
console.log(db.prepare('SELECT count(*) AS n FROM users').get().n); // 2

// Integers beyond 2^53 throw unless BigInt reading is enabled
const big = db.prepare('SELECT 9007199254740993 AS n');
try { big.get(); } catch (e) { console.log(e.code); }    // ERR_OUT_OF_RANGE
big.setReadBigInts(true);
console.log(big.get().n);                                // 9007199254740993n
```

- The API is **synchronous**: every query blocks the event loop. Fine for CLIs, tests, embedded config and small services; for heavy concurrent workloads use a worker thread or a client/server database.
- Rows are null-prototype objects (`console.log` shows `[Object: null prototype]`); `Object.hasOwn` works, `row.hasOwnProperty` does not.
- Defensive mode is on by default since 25.5/24.14. Loading SQLite extensions requires `allowExtension: true` and is refused entirely when the permission model is enabled (`ERR_LOAD_SQLITE_EXTENSION`, verified) — treat extensions as native code.
- Use `STRICT` tables to get type enforcement; SQLite is otherwise dynamically typed.
- `statement.close()` / `using` (26.8+) releases prepared statements deterministically.

***

#### Configuration and CLI Arguments

```mjs
import { parseArgs, styleText } from 'node:util';

const { values, positionals } = parseArgs({
  args: ['--port', '8080', '--verbose', 'serve'],
  options: {
    port: { type: 'string', short: 'p', default: '3000' },
    verbose: { type: 'boolean', short: 'v', default: false },
  },
  allowPositionals: true,
  strict: true,                    // unknown flags throw ERR_PARSE_ARGS_UNKNOWN_OPTION
});
const port = Number.parseInt(values.port, 10);
if (!Number.isInteger(port) || port < 1 || port > 65_535) throw new RangeError(`bad port ${values.port}`);
console.log(port, values.verbose, positionals);          // 8080 true [ 'serve' ]

// Colours respect NO_COLOR / FORCE_COLOR / TTY detection when writing to a stream
console.log(styleText(['bold', 'green'], 'ok', { stream: process.stdout }));
```

- Validate environment variables once at startup into a typed, frozen config object; fail fast with a clear message. `process.env` values are always strings (`'false'` is truthy).
- `util.parseArgs` covers most CLIs without dependencies; `util.styleText` replaces colour libraries (hex colours since 26.1).
- `--env-file` does not override variables that are already set in the environment.

***

#### Worker Threads and Parallelism

```mjs
import { Worker } from 'node:worker_threads';

// Offload CPU-bound work; the worker code can be a file URL or (here) inline source
const source = `
  const { parentPort, workerData } = require('node:worker_threads');
  let sum = 0;
  for (let i = 0; i < workerData.n; i++) sum += i;
  parentPort.postMessage(sum);
`;

function runWorker(n) {
  return new Promise((resolve, reject) => {
    const worker = new Worker(source, { eval: true, workerData: { n }, name: `sum-${n}` });
    worker.once('message', resolve);
    worker.once('error', reject);
    worker.once('exit', (code) => { if (code !== 0) reject(new Error(`worker exited with ${code}`)); });
  });
}

console.log(await Promise.all([runWorker(1e6), runWorker(2e6)])); // [ 499999500000, 1999999000000 ]

// Transfer large buffers instead of copying them
const { port1, port2 } = new MessageChannel();
const buf = new ArrayBuffer(1024);
port2.once('message', (b) => { console.log(b.byteLength); port2.close(); });  // 1024
port1.postMessage(buf, [buf]);
console.log(buf.byteLength);                                     // 0 — detached after transfer
port1.close();
```

- Workers are for **CPU-bound** JavaScript. I/O is already asynchronous; moving `fetch` or DB calls to workers adds overhead.
- Spawning a worker costs milliseconds and memory (a whole V8 isolate). Use a pool (e.g., `piscina`) for request-level work instead of one worker per request.
- Always handle `'error'` and non-zero `'exit'`; set `resourceLimits` for untrusted-size workloads.
- `SharedArrayBuffer` + `Atomics` for shared state; `Atomics.pause()` (ES2027 candidate, shipped) for spin-wait hints. Shared mutable state needs the same care as in any threaded language.
- Workers are denied under `--permission` without `--allow-worker`.
- The Web-compatible global `Worker` (26.9) requires `--experimental-web-worker`; production code uses `node:worker_threads`.
- `cluster` / multiple processes behind a load balancer remain the way to use all cores for an HTTP server; in containers, prefer one process per container and scale horizontally.

***

#### Performance

- **Measure first**: `node --cpu-prof`, `--heap-prof`, `--heapsnapshot-near-heap-limit=N` (stable since 25.4), `perf_hooks.performance.timerify()`, `monitorEventLoopDelay()`. `v8.startCpuProfile()` (25.0/24.12) and `v8.startHeapProfile()` (26.1) return disposable handles for in-process profiling.
- **Avoid megamorphic hot paths**: create objects with the same shape (initialize all fields in the constructor, same order); do not `delete` properties on hot objects (set to `undefined` instead).
- **Strings**: build large strings with arrays + `join()` or streams, not `+=` in a loop over megabytes; avoid `JSON.stringify` of huge objects on the request path.
- **Buffers**: `Buffer.allocUnsafe()` + fill is faster than `alloc()` only when every byte is overwritten; reuse buffers in hot loops; `fs.readFile()` into a caller-supplied buffer (26.4+) avoids allocations.
- **Regex**: precompile (`const RE = /.../` at module scope); `String#replaceAll` with a string avoids regex entirely.
- **`Array#includes` on large arrays inside loops** is O(n²) — use a `Set`.
- **Startup**: `module.enableCompileCache()`, lazy `import()` of rarely used modules, avoid synchronous work at module top level in libraries.
- **Memory**: the default old-generation limit is a V8 heuristic based on system memory; in containers set it explicitly with `--max-old-space-size=MiB` or `--max-old-space-size-percentage=N` (percentage of available memory, takes precedence) so the heap fits the cgroup limit with headroom for buffers and native memory. Leaks usually come from unbounded caches/maps, listeners never removed, closures captured by timers, and large buffers retained by small `subarray()` views.
- **Caches**: bound every in-process cache (LRU with max entries/TTL). `WeakMap`/`WeakRef`/`FinalizationRegistry` are not a cache eviction strategy — GC timing is unpredictable.
- **Benchmarks**: `node:bench` (26.9) is experimental and flagged; for now use `mitata`/`tinybench` or `process.hrtime.bigint()` with warm-up and enough iterations. Never benchmark with `console.log` inside the loop.

***

#### Testing with `node:test`

```mjs
import { describe, it, before, mock } from 'node:test';
import assert from 'node:assert/strict';

function makeRetry(fn, { retries = 2 } = {}) {
  return async (...args) => {
    let lastErr;
    for (let i = 0; i <= retries; i++) {
      try { return await fn(...args); } catch (err) { lastErr = err; }
    }
    throw lastErr;
  };
}

describe('makeRetry', () => {
  let flaky;
  before(() => {
    let calls = 0;
    flaky = mock.fn(async () => { if (++calls < 3) throw new Error('flaky'); return 'ok'; });
  });

  it('retries until success', async () => {
    assert.equal(await makeRetry(flaky)(), 'ok');
    assert.equal(flaky.mock.callCount(), 3);
  });

  it('rethrows the last error', async () => {
    const always = mock.fn(async () => { throw new Error('down'); });
    await assert.rejects(makeRetry(always, { retries: 1 })(), { message: 'down' });
    assert.equal(always.mock.callCount(), 2);
  });

  it('uses fake timers', (t) => {
    t.mock.timers.enable({ apis: ['setTimeout', 'Date'], now: 0 });
    const cb = t.mock.fn();
    setTimeout(cb, 1_000);
    t.mock.timers.tick(999);
    assert.equal(cb.mock.callCount(), 0);
    t.mock.timers.tick(1);
    assert.equal(cb.mock.callCount(), 1);
    assert.equal(Date.now(), 1_000);
  });

  it('partial deep equality', () => {
    assert.partialDeepStrictEqual({ id: 1, name: 'a', extra: true }, { id: 1, name: 'a' });
  });

  it.todo('handles jitter');
});
```

Run with `node --test`[^21] (globs `**/*.test.{js,mjs,cjs,ts,mts,cts}`, `**/test/**/*.js` and similar by default). Useful flags: `--test-reporter=spec|dot|junit|lcov`, `--test-concurrency`, `--test-isolation=process|none`, `--test-shard=1/3`, `--test-name-pattern`, `--test-only`, `--test-timeout`, `--test-force-exit`, `--test-update-snapshots`, `--experimental-test-coverage` with `--test-coverage-lines=80` thresholds, `--test-rerun-failures=state.json`. **New in 26:** `--test-randomize`/`--test-random-seed` (26.1), test tags with `--experimental-test-tag-filter` (26.2), `context.log()` and the `test:log` event (26.6), `--test-coverage-include-all` (26.7), `t.assert.called()`/`callCount()`/`lastCalledWith()`/`nthCalledWith()` (26.12).

Review rules:

- **Use `node:assert/strict`** (or `assert.strictEqual`) — the legacy loose `assert.equal` uses `==`.
- **`await` every async assertion**: `assert.rejects()` returns a promise; without `await` the test passes before the assertion runs.
- **Restore mocks/timers**: `t.mock.*` created through the test context are restored automatically; module-level `mock.fn()`/`mock.method()` need `mock.restoreAll()`.
- `mock.module()` needs `--experimental-test-module-mocks` and is early development; its options changed in 25.9 (`defaultExport`/`namedExports` were consolidated into `exports`) — do not rely on it for long-lived suites without pinning the minor.
- **Snapshot assertions** (`t.assert.snapshot()`, `t.assert.fileSnapshot()`) are stable; snapshot files must be reviewed like code. Node 26 prints proxies as `Proxy(...)` in `util.inspect` — expect snapshot churn when upgrading from 24.
- **`expectFailure: true`** (25.5, also 24.x) marks a known-failing test; prefer fixing or `todo` over long-lived expected failures.
- **Assertion messages** are printf-formatted in 26 (`assert.ok(x, 'got %s', x)`); a literal `%` in a message must be escaped as `%%` when extra arguments are passed.
- Flaky tests: no real network, no real clocks (`mock.timers`), unique temp dirs (`fs.mkdtemp()` per test), and no ordering dependencies (check with `--test-randomize`).
- Vitest 5 and Jest 30 support Node 26; `node:test` avoids the dependency for libraries and services.

***

#### Tooling

##### ESLint 10 flat config

ESLint 10 (February 2026; 10.13.0 is current) **removed eslintrc entirely**: `.eslintrc.*`, `.eslintignore`, `ESLINT_USE_FLAT_CONFIG`, `--env`, `--rulesdir`, `--no-eslintrc` and `--ignore-path` are gone, and the config is looked up from each linted file's directory. ESLint 9 reached end-of-life on 2026-08-06.[^12]

```js
// eslint.config.js (verified with eslint 10.13.0, @eslint/js 10, globals 17, eslint-plugin-n 18)
import js from '@eslint/js';
import globals from 'globals';
import n from 'eslint-plugin-n';
import { defineConfig, globalIgnores } from 'eslint/config';

export default defineConfig([
  globalIgnores(['dist/', 'coverage/']),
  {
    files: ['**/*.{js,mjs,cjs}'],
    extends: [js.configs.recommended, n.configs['flat/recommended']],
    languageOptions: {
      ecmaVersion: 'latest',            // parses `using`, import attributes, RegExp modifiers
      sourceType: 'module',
      globals: { ...globals.node },     // globals 17 includes Temporal
    },
    rules: {
      'no-unused-vars': ['error', { argsIgnorePattern: '^_' }],
      eqeqeq: ['error', 'always', { null: 'ignore' }],
      'no-throw-literal': 'error',
      'prefer-promise-reject-errors': 'error',
      'n/no-deprecated-api': 'error',
    },
  },
  { files: ['**/*.cjs'], languageOptions: { sourceType: 'commonjs' } },
]);
```

- `eslint-plugin-n` reads `engines.node` from `package.json` for `n/no-unsupported-features/*`; keep `engines` accurate so it flags APIs newer than the minimum.
- For TypeScript, `typescript-eslint` 8.x with `tseslint.configs.recommendedTypeChecked` enables `no-floating-promises`, `no-misused-promises` and `await-thenable` — the most valuable async rules.
- Prettier (3.x) or `eslint --fix` for formatting; do not hand-align.
- `npm pkg get engines`, `npx publint`/`@arethetypeswrong/cli` for library packaging checks.

##### TypeScript compiler

`typescript` 7.0 (the native Go port, `tsc` 7.0.2) is the current release; 6.0 is the last JavaScript-based line. Both understand `erasableSyntaxOnly`, `verbatimModuleSyntax`, `rewriteRelativeImportExtensions` and `module: nodenext`. Type-check in CI (`tsc --noEmit`) even when Node runs the `.ts` files directly.

***

#### Observability

##### Logging

- Use a structured logger (pino or similar) writing JSON to stdout; let the platform ship logs. `console.log` is synchronous for files/TTYs and asynchronous for pipes on POSIX — high-volume logging through `console` can block or buffer unboundedly.
- Log errors with their stack and `cause` (`util.inspect(err, { depth: 5 })` or the logger's error serializer); never `console.log(err.message)` alone.
- Include a request ID from `AsyncLocalStorage`; redact secrets.
- `process.on('warning')` receives deprecation/experimental warnings; `--disable-warning=DEP0040` (or `ExperimentalWarning`) silences specific codes instead of `--no-warnings`.
- `--trace-deprecation` / `--throw-deprecation` in CI catches use of the APIs Node 27 will remove.

##### diagnostics_channel and tracing

```mjs
import dc from 'node:diagnostics_channel';
import http from 'node:http';

dc.subscribe('http.server.request.start', ({ request }) => console.log('server', request.method, request.url));
dc.subscribe('undici:request:create', ({ request }) => console.log('fetch', request.method, request.path));

const server = http.createServer((req, res) => res.end('ok'));
await new Promise((resolve) => server.listen(0, '127.0.0.1', resolve));
await (await fetch(`http://127.0.0.1:${server.address().port}/x`)).text();
server.close();

// TracingChannel is stable since 26.8: wrap your own operations
const jobs = dc.tracingChannel('app.job');
jobs.subscribe({
  start: (ctx) => console.log('job start', ctx.id),
  asyncEnd: (ctx) => console.log('job end', ctx.id, ctx.result),
  error: (ctx) => console.log('job failed', ctx.error.message),
});
console.log(await jobs.tracePromise(async () => 7, { id: 1 }));
// fetch GET /x / server GET /x / job start 1 / job end 1 7 / 7
```

OpenTelemetry instrumentations build on these channels[^26]; prefer them over monkey-patching. Built-in channels exist for `http`, `http2`, `net`, `undici`, `worker_threads`, `process`, the module loader, Web Locks (25.9), SQLite (26.8), crypto (26.9) and the permission model (`node:permission-model:*` in audit mode).

##### Process lifecycle and diagnostics

```mjs
// Graceful shutdown skeleton
const shutdown = async (signal) => {
  console.log(`received ${signal}, draining`);
  // stop accepting work, close servers, flush logs, close DB pools...
  process.exitCode = 0;          // let the loop drain instead of process.exit()
};
process.once('SIGTERM', shutdown);
process.once('SIGINT', shutdown);
process.on('unhandledRejection', (reason) => {
  console.error('unhandled rejection', reason);
  process.exitCode = 1;
});
console.log('handlers installed', process.listenerCount('SIGTERM'));
process.removeAllListeners('SIGTERM');
process.removeAllListeners('SIGINT');
```

- Prefer `process.exitCode = n` over `process.exit(n)`; `exit()` drops pending writes to pipes (lost logs).
- In containers, Node as PID 1 does not get default signal handling for `SIGTERM` unless handlers are installed — install them or use `--init`/tini.
- `process.report.writeReport()` / `--report-on-fatalerror` produce JSON diagnostics; `--report-on-process-timeout` (26.11) with `--process-timeout`.
- `util.getCallSites()` gives structured stack frames without parsing `err.stack`.
- `process.ref(obj)`/`process.unref(obj)` (stable in 26.11) keep or release the event loop for refable handles.

***

#### Web Platform Globals

Node 26 exposes a large set of web-standard globals. Prefer them over userland equivalents where semantics match:

| Global | Notes for Node 26 |
|---|---|
| `fetch`, `Request`, `Response`, `Headers`, `FormData` | Undici 8; HTTP/2 negotiated for `https:` |
| `WebSocket`, `CloseEvent` | stable client |
| `URL`, `URLSearchParams`, `URL.parse()`, `URL.canParse()`, `URLPattern` | `URL.parse()` returns `null` instead of throwing |
| `AbortController`, `AbortSignal.timeout()/any()` | cancellation everywhere |
| `structuredClone`, `BroadcastChannel`, `MessageChannel` | |
| `crypto` (Web Crypto), `CryptoKey` | `crypto.subtle` for portable code |
| `ReadableStream` & co., `CompressionStream`, `DecompressionStream`, `TextEncoderStream` | |
| `Blob`, `File`, `blob.textStream()` (26.5) | |
| `navigator` (`hardwareConcurrency`, `language`, `platform`, `userAgent`) | `--no-experimental-global-navigator` disables it |
| `localStorage` | **`undefined` unless `--localstorage-file=path` is set** (26.0); `sessionStorage` is per-process memory |
| `ErrorEvent` (25.0), `QuotaExceededError` (26.0) | |
| `Temporal` (26.0) | |
| `Worker` | only with `--experimental-web-worker` (26.9) |
| `EventSource` | only with `--experimental-eventsource` |

```mjs
console.log(typeof localStorage);                                   // 'undefined' without --localstorage-file
const route = new URLPattern({ pathname: '/users/:id' });
console.log(route.exec('https://x.test/users/42')?.pathname.groups.id);   // 42
console.log(URL.parse('not a url'), URL.canParse('https://a.b'));         // null true

// URL setters normalize: dot segments are resolved, so this is not a traversal guard
const u = new URL('https://example.com/a');
u.pathname = '/../../etc';
console.log(u.href);                                                 // https://example.com/etc
```

Universal (browser + Node) libraries must feature-detect `localStorage` (`globalThis.localStorage?.getItem`) — on Node 26 the property exists but is `undefined` and emits an `ExperimentalWarning` when read.

***

#### Experimental Node 26 Subsystems (know them, gate them)

- **`node:ffi`** (26.1, enabled by default since 26.9, Stability 1): loads shared libraries and calls native symbols. It is **memory-unsafe by design** — invalid pointers or wrong signatures crash or corrupt the process — and is denied under `--permission` without `--allow-ffi`. Treat any `import 'node:ffi'` in a PR like a native addon: security review, pinned minor, tests on every platform.
- **`node:vfs`** (26.4, `--experimental-vfs`): virtual file systems that `fs`, `fs/promises` and (26.9) the module loaders see; single-executable applications can embed assets through it.
- **`zlib.ZipFile` / `ZipEntry` / `ZipBuffer`** (26.8, early development): native ZIP reading/writing. Archive extraction still needs path-traversal checks on entry names ("zip slip") and size limits (`zlib.setMaxZipContentSize()`).
- **`node:quic`** (early development), **DTLS** (`--experimental-dtls`, 26.9), **package maps** (`--experimental-package-map`, 26.4), **`node:bench`** (26.9), **`node:stream/iter`** (25.9, `--experimental-stream-iter`).
- **Single-executable applications**: `node --build-sea config.json` (25.5) builds a SEA directly; ESM entry points are supported since 25.7.

Code depending on any of these must state the exact minimum minor, expect breaking changes in minor releases, and keep the flag in the start command visible to reviewers.

***

#### Common Review Findings (BAD → GOOD)

```mjs
import { readFile } from 'node:fs/promises';
import { pipeline } from 'node:stream/promises';
import { Readable, PassThrough } from 'node:stream';

// 1. Floating promise
// BAD:  saveAudit(event);               // rejection → process crash, ordering lost
// GOOD: await saveAudit(event);         // or: saveAudit(event).catch(logError)

// 2. JSON.parse on untrusted input without try/catch or schema
function parseBody(text) {
  try {
    const data = JSON.parse(text);
    if (typeof data !== 'object' || data === null || Array.isArray(data)) throw new TypeError('object expected');
    return data;
  } catch (err) {
    throw new Error('invalid JSON body', { cause: err });
  }
}
console.log(parseBody('{"a":1}'));

// 3. Swallowed errors
// BAD:  try { await op(); } catch {}    // silent data loss
// GOOD: catch only what you expect, rethrow the rest
const cfg = await readFile('/definitely/missing.json', 'utf8').catch((err) => {
  if (err.code === 'ENOENT') return '{}';
  throw err;
});
console.log(cfg);

// 4. .pipe() without error handling → pipeline()
await pipeline(Readable.from(['x']), new PassThrough(), async function* (src) { for await (const c of src) yield c; });

// 5. Date math by hand → Temporal
// BAD:  new Date(Date.now() + 24 * 60 * 60 * 1000)   // wrong across DST for wall-clock "tomorrow"
const tomorrow = Temporal.Now.zonedDateTimeISO('Europe/Berlin').add({ days: 1 });
console.log(typeof tomorrow.toString());

// 6. has/get/set dance → upsert
const cache = new Map();
cache.getOrInsertComputed('k', () => ({ hits: 0 })).hits++;
console.log(cache.get('k').hits);                        // 1
```

Additional patterns to flag:

- `new Promise(async (resolve, reject) => { ... })` — an error thrown in the async executor rejects the executor's own (ignored) promise, which crashes the process as an unhandled rejection, while the outer promise never settles.
- `array.map(async ...)` without `Promise.all`; `for await` over a plain array of promises created all at once (no concurrency control).
- `setInterval(async () => ...)` where runs can overlap — use a loop with `await sleep()` or `timers/promises` `setInterval` async iterator.
- `process.exit()` inside libraries; `process.env.X === true` (always strings).
- `crypto.createHash('md5'|'sha1')` for anything security-relevant; `Math.random()` for IDs/tokens.
- `fs.existsSync()` + `fs.readFileSync()` in request handlers (blocking + TOCTOU).
- `Buffer.from(userString, 'base64')` accepting garbage — use `Uint8Array.fromBase64()` for strict decoding.
- Unbounded `Map` caches keyed by user input (memory DoS).
- `JSON.stringify` of objects containing `BigInt` (throws) or circular references (throws); logging whole request objects (leaks headers, huge output).
- Mixing `require()` and `import` of the same package (dual-package hazard: two module instances, broken `instanceof` and singletons).
- `instanceof Error` for values that may come from another `vm` realm — use `Error.isError()`.
- `res.writeHeader()`, `require('_stream_readable')`, `module.register()`, `--experimental-transform-types`, `sqlite.DatabaseSync` in Node-26-only code.

***

#### Removed / Changed in Node.js 26 — Migration Notes

1. **`readable.read()` returns one chunk.** Audit custom `'readable'` handlers; loop until `read()` returns `null`, pass an explicit size (`r.read(r.readableLength)`), or switch to `for await`.
2. **`fetch()` speaks HTTP/2.** Check servers/proxies that cannot handle h2 correctly, code depending on HTTP/1-only headers, and tests that assert `req.httpVersion`. Upgrade any explicitly installed `undici` package to 8.x to match the bundled dispatcher semantics.
3. **Temporal is global.** Remove unconditional polyfill installation (`globalThis.Temporal = ...`) in Node-26-only code; test polyfill-based libraries against the native implementation (older polyfill versions differ in parsing and edge cases).
4. **`Intl.Locale` getters are gone.** Replace `.weekInfo`, `.hourCycles`, `.timeZones`, `.calendars`, `.collations`, `.numberingSystems`, `.textInfo` with `getWeekInfo()`, `getHourCycles()`, `getTimeZones()`, `getCalendars()`, `getCollations()`, `getNumberingSystems()`, `getTextInfo()`.
5. **`localStorage`** is `undefined` without `--localstorage-file`; add the flag or stop relying on it on the server.
6. **HTTP upgrade handlers** that accept requests with bodies or call socket methods on the upgrade stream must read the body from `req` and use `req.socket`.
7. **`util.inspect` shows `Proxy(...)`**; update snapshots and log parsers.
8. **Assertion messages are printf-formatted**; messages containing `%` with extra arguments change.
9. **Removed APIs**: `writeHeader()` → `writeHead()`; `_stream_*` → `node:stream`; `--experimental-transform-types` → erasable TypeScript or a compiler; short GCM tags → `authTagLength`.
10. **Runtime deprecations to fix before Node 27**: `module.register()` → `registerHooks()`; `CryptoKey` in `node:crypto` → `KeyObject.from()`; `Duplex.toWeb({ type })` → `readableType`; SHAKE without `outputLength`; `zlib` classes without `new`; `url.parse()`; `punycode`; `util.isArray()`; `Buffer()`.
11. **Native addons**: rebuild for ABI 147 (or use Node-API); prebuilt binaries for 24 do not load on 26.
12. **Build from source**: GCC ≥ 13.2, Python ≥ 3.10.
13. **`Buffer.poolSize` is 64 KiB** (26.3; 25.x still uses 8 KiB): code that relied on small pooled slices sharing an 8 KiB slab sees different memory retention; never depend on the pool size.
14. **ML-KEM/ML-DSA PKCS#8 exports default to the seed-only format**: consumers that parse the expanded private-key encoding must handle seed-only keys.
15. **From 25 (if upgrading from 24)**: `SlowBuffer`, `fs.F_OK`…, `fs.rmdir({ recursive })`, `assert.CallTracker`, multi-argument `assert.fail()`, `process.on('multipleResolves')` are gone; `--allow-net` is required for network access under `--permission`.

***

#### Production / Review Checklist

##### Version and setup

- [ ] `engines.node`, `.nvmrc`, CI matrix and Docker image agree on Node 26 and a minimum minor ≥ the APIs used (≥ 26.12.1 for current security fixes)
- [ ] No Node 27 / unshipped ECMAScript APIs (`Math.sumPrecise`, `Iterator.zip`, decorators, `node:net/promises`, `fs.mkstemp`, 65 s keep-alive default assumed)
- [ ] Experimental APIs (`node:ffi`, `node:vfs`, `mock.module`, `zlib.ZipFile`, `--experimental-*`) are deliberate, flagged in start scripts, and pinned
- [ ] `"type"` set in every `package.json`; `exports` defined for libraries; built-ins imported with `node:`
- [ ] Native addons rebuilt for ABI 147 or Node-API based

##### Correctness

- [ ] Every promise awaited/returned/handled; no async `forEach`, no async Promise executors
- [ ] Errors are `Error` instances with `cause`; handling branches on `err.code`; `Error.isError()` across realms
- [ ] Streams use `pipeline()`; `read()` loops handle the Node 26 one-chunk behaviour; `write()` backpressure respected
- [ ] Temporal types chosen deliberately (Instant/ZonedDateTime/PlainDate); no `<`/`===` on Temporal objects; explicit time zones
- [ ] No `Intl.Locale` getter properties; `getWeekInfo()` & co. instead
- [ ] Numbers: radix on `parseInt`, `Number.isFinite` validation, `BigInt`/strings for 64-bit IDs, comparator for numeric `sort()`
- [ ] TypeScript run by Node uses only erasable syntax (`erasableSyntaxOnly`, `verbatimModuleSyntax`, `.ts` extensions)

##### Security

- [ ] No `exec`/`shell: true` with user input; `execFile`/`spawn` with arg arrays, timeouts, `maxBuffer`
- [ ] Paths resolved and containment-checked with `path.relative`; symlinks considered
- [ ] No prototype-pollution-prone merges; `Map`/null-prototype objects for user-keyed data; schema validation of input
- [ ] SQL parameterized (`node:sqlite` placeholders); no string-built queries
- [ ] Regexes on untrusted input reviewed for ReDoS; input length capped; `RegExp.escape()` for embedded user text
- [ ] Outbound requests: timeouts (`AbortSignal.timeout`), body consumed/cancelled, SSRF allowlists, `redirect: 'manual'` where needed
- [ ] HTTP servers: body size limits, `requestTimeout`/`headersTimeout`/`keepAliveTimeout` set, async handler errors caught
- [ ] Crypto: scrypt/Argon2 for passwords, `timingSafeEqual`, GCM with unique IVs and `authTagLength`, no `Math.random()` tokens, no `NODE_TLS_REJECT_UNAUTHORIZED=0`
- [ ] Permission model (`--permission` + minimal `--allow-*`) considered for services and CLIs that handle untrusted input; `node:vm` not used as a sandbox
- [ ] Lockfile committed, `npm ci`, dependency audit in CI, install scripts reviewed

##### Performance and operations

- [ ] No sync I/O or CPU-heavy work on request paths; workers/pools for CPU-bound tasks
- [ ] Bounded concurrency for fan-out; bounded caches
- [ ] Graceful shutdown on `SIGTERM` (`server.close()`, `closeIdleConnections()`, deadline); `process.exitCode` instead of `process.exit()`
- [ ] Heap limit set for containers (`--max-old-space-size[-percentage]`)
- [ ] Structured logging with request IDs (`AsyncLocalStorage`) and secret redaction
- [ ] Deprecation warnings surfaced in CI (`--trace-deprecation` or `--throw-deprecation`) to prepare for Node 27

##### Testing and tooling

- [ ] `node --test` (or Vitest/Jest) with `node:assert/strict`, awaited async assertions, fake timers instead of sleeps
- [ ] Tests pass with `--test-randomize`; coverage thresholds enforced
- [ ] ESLint 10 flat config (`eslint.config.js`) with `eslint-plugin-n` reading `engines`; typed linting (`no-floating-promises`) for TS
- [ ] `tsc --noEmit` in CI for TypeScript; `@types/node@26`

***

[^1]: Node.js 26 changelog (26.0.0 – 26.12.1, notable changes, semver-major commits, security releases). https://github.com/nodejs/node/blob/main/doc/changelogs/CHANGELOG_V26.md
[^2]: Node.js v26.12.1 release notes (latest patch, 2026-10-21). https://nodejs.org/en/blog/release/v26.12.1
[^3]: Node.js release schedule (schedule.json: 26 LTS 2026-10-28, maintenance 2027-10-20, EOL 2029-04-30; 27 alpha 2026-10-28, start 2027-04-22). https://github.com/nodejs/Release/blob/main/schedule.json
[^4]: Evolving the Node.js Release Schedule (one major per year from 27.x, Alpha/Current/LTS phases). https://nodejs.org/en/blog/announcements/evolving-the-nodejs-release-schedule
[^5]: Node.js 26.0.0 release announcement (Temporal, V8 14.6, Undici 8, deprecations and removals). https://nodejs.org/en/blog/release/v26.0.0
[^6]: TC39 finished proposals (ES2025/ES2026 features, Stage 4 proposals for ES2027). https://github.com/tc39/proposals/blob/main/finished-proposals.md
[^7]: TC39 proposals: Upsert and Iterator Sequencing. https://github.com/tc39/proposal-upsert , https://github.com/tc39/proposal-iterator-sequencing
[^8]: Node.js 26 errors reference (ERR_NO_TEMPORAL, ERR_UNSUPPORTED_TYPESCRIPT_SYNTAX, ERR_REQUIRE_ASYNC_MODULE). https://nodejs.org/docs/latest-v26.x/api/errors.html
[^9]: Undici v8.0.0 release (HTTP/2 enabled by default, removed legacy handler wrappers). https://github.com/nodejs/undici/releases/tag/v8.0.0
[^10]: Node.js 25 changelog (25.0.0 – 25.9.0, deprecations and removals). https://github.com/nodejs/node/blob/main/doc/changelogs/CHANGELOG_V25.md
[^11]: Node.js main branch API docs and deprecations (unreleased REPLACEME entries for 27.0.0). https://github.com/nodejs/node/blob/main/doc/api/deprecations.md
[^12]: ESLint v10.0.0 released (eslintrc removed, Node.js requirements). https://eslint.org/blog/2026/02/eslint-v10.0.0-released/
[^13]: Node.js 26 Modules: Loading ECMAScript modules using require(). https://nodejs.org/docs/latest-v26.x/api/modules.html#loading-ecmascript-modules-using-require
[^14]: Node.js 26 Modules: TypeScript (type stripping). https://nodejs.org/docs/latest-v26.x/api/typescript.html
[^15]: Node.js 26 Permissions (permission model, permission.drop, audit mode). https://nodejs.org/docs/latest-v26.x/api/permissions.html
[^16]: Node.js 26 SQLite (Database/Statement, DEP0210/DEP0211). https://nodejs.org/docs/latest-v26.x/api/sqlite.html
[^17]: nodejs/node#60441 — stream: readable read one buffer at a time. https://github.com/nodejs/node/pull/60441
[^18]: nodejs/node#60016 — http: fix handling of HTTP upgrades with bodies. https://github.com/nodejs/node/pull/60016
[^19]: Temporal documentation (TC39 proposal-temporal). https://tc39.es/proposal-temporal/docs/
[^20]: Node.js 26 util (debounce, throttle, markPromiseAsHandled, parseArgs, styleText). https://nodejs.org/docs/latest-v26.x/api/util.html
[^21]: Node.js 26 Test runner. https://nodejs.org/docs/latest-v26.x/api/test.html
[^22]: Node.js 26 Deprecated APIs (DEP0005 … DEP0211). https://nodejs.org/docs/latest-v26.x/api/deprecations.html
[^23]: Node.js April 2024 security releases (CVE-2024-27980, batch file argument injection on Windows). https://nodejs.org/en/blog/vulnerability/april-2024-security-releases-2
[^24]: MDN: Intl.Locale.prototype.getWeekInfo() (accessor replaced by method); TC39 Intl Locale Info proposal. https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Global_Objects/Intl/Locale/getWeekInfo , https://github.com/tc39/proposal-intl-locale-info
[^25]: TypeScript 5.8 release notes (--erasableSyntaxOnly). https://www.typescriptlang.org/docs/handbook/release-notes/typescript-5-8.html
[^26]: Node.js 26 Diagnostics Channel (built-in channels, TracingChannel). https://nodejs.org/docs/latest-v26.x/api/diagnostics_channel.html
[^27]: Node.js 24 changelog (24.x backports of 25/26 features, up to 24.22.1). https://github.com/nodejs/node/blob/main/doc/changelogs/CHANGELOG_V24.md
[^28]: Undici documentation bundled with Node 26 (headersTimeout/bodyTimeout defaults). https://github.com/nodejs/undici/blob/main/docs/docs/getting-started.md
