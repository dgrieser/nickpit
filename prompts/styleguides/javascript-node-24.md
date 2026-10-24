### JavaScript on Node.js 24 (Krypton LTS): The Complete Production Developer Guide

> **Version**: Node.js 24.0.0 was released on 2025-05-06 and became LTS ("Krypton") with 24.11.0 on 2025-10-28. As of October 2026 it is in **Maintenance LTS** (since 2026-10-20) and is supported until **2028-04-30**. Latest patch: **24.22.1 (2026-10-21, security release)**.
> **Engine and bundled components (24.22.1)**: V8 13.6.233.17, npm 11.19.1, undici 7.30.1, libuv 1.52.1, OpenSSL 3.5.9, ICU 78.3 (Unicode 17).
> **Release landscape (2026-10-22)**: Node.js 26.12.1 is the newest release ("Current"; scheduled to enter LTS on 2026-10-28). Node.js 22 ("Jod") is in Maintenance LTS until 2027-04-30. Node.js 25 (odd, never LTS) reached end-of-life on 2026-06-01 with 25.9.0 as its last release. Node.js 27 has no release yet: per the release schedule it starts as an alpha channel on 2026-10-28, and 27.0.0 is planned for 2027-04-22.
> **Scope**: The JavaScript language as implemented by Node 24's V8 (ES2025 plus the shipped Stage 3/4 features listed below) and the Node.js 24 runtime APIs: modules, TypeScript type stripping, async patterns, errors, I/O and streams, HTTP/fetch, security, testing, performance, observability and tooling. Projects whose engine is pinned to 25.x are reviewed with this guide too (see "Version-aware review").
> **Note**: This document is self-contained. Everything from earlier Node/ECMAScript versions that is still correct is carried forward. Sections marked **New in 24** (or **New in 24.x** with the minor that introduced it) cover behavior that is not available on Node 22.

---

#### Version-aware review

##### Determine the runtime before judging an API

Review code against the Node version the project actually runs on. Look at, in this order: `engines.node` in `package.json`, `.nvmrc` / `.node-version` / `.tool-versions`, the `FROM node:<version>` line of Dockerfiles, `actions/setup-node` `node-version` in CI, `volta` pins, and `@types/node` (its major version should match the runtime major). If the project declares a range that includes Node 22 (for example `"engines": { "node": ">=22" }`), every **New in 24** feature in this guide is a compatibility bug for the 22 users.

- **Verify before claiming something is unavailable.** Node backports a lot of features to older LTS lines in minor releases. Node 22.23.4, for example, has `import.meta.main`, `process.execve()`, `module.registerHooks()`, `--permission`, type stripping enabled by default, `require(esm)` without a flag and `zstd` compression, even though several of these were announced with Node 23/24. Say "not available in 24" only when the API is absent from Node 24.22.1 (this guide lists the verified cases).
- **Minor versions matter inside the 24 line.** Many APIs arrived in 24.x minors (for example `crypto.randomUUIDv7()` in 24.16.0, `util.debounce()` in 24.22.0). Code that uses them needs `"engines": { "node": ">=24.16" }` (or later) rather than `>=24`. The **New in 24.x** markers give the minimum minor.
- **Node 25 is not a superset of Node 24.22.** Node 25 stopped at 25.9.0 (2026-04-01). Features backported to 24.x after that are missing from 25.9.0: verified absent on 25.9.0 are `util.debounce()`, `util.throttle()`, `util.markPromiseAsHandled()`, `MIMEType.parse()`, `crypto.randomUUIDv7()`, `crypto.parsePKCS12()`, `crypto.createMac()`, `res.writeInformation()`, `req.signal` on `IncomingMessage`, `Blob.prototype.textStream()`, `StatementSync.prototype.close()`, the 64 KiB `Buffer.poolSize` default, `--experimental-package-map`, `--experimental-import-text`, `--test-randomize`, `--experimental-test-tag-filter`, `process.permission.drop()`, `--allow-openssl-store`, `net.BlockList.PRIVATE_RANGES`/`addCIDRs()`, hex colours in `util.styleText()`, and the `aes-*-siv`/`aes-*-gcm-siv` ciphers. Conversely 25.x has features 24 lacks (see "Do not use or recommend"). Node 25 is end-of-life; a project pinned to 25.x should move to 24 LTS or 26.
- **Type definitions can lie.** A newer `@types/node` (for example 26.x) type-checks code against APIs that do not exist at runtime on 24. Pin `@types/node@24` for a Node 24 project.
- **Experimental status is part of the contract.** Check the stability index in the v24 docs. Stability 1.0/1.1 (early/active development) APIs can change in any minor; flag their use in libraries and production code paths, and require a comment pinning the expected behavior.

***

#### What's new in Node.js 24 (vs. Node 22)

All "Node 22" statements below were checked on 22.23.4, the last 22.x release available on 2026-10-22; "24" on 24.22.1.

##### Language (V8 12.4 → 13.6)

| Feature | Available since | On 22.23.4? |
|---|---|---|
| `using` / `await using` declarations, `DisposableStack`, `AsyncDisposableStack`, `SuppressedError`, `Iterator.prototype[Symbol.dispose]` | 24.0 | No (syntax error; 22 only defines the `Symbol.dispose`/`Symbol.asyncDispose` symbols and Node-object methods) |
| `Float16Array`, `Math.f16round()` | 24.0 | No |
| `RegExp.escape()` | 24.0 | No |
| `Error.isError()` | 24.0 | No |
| `Promise.try()` | 23.0 | No |
| RegExp pattern modifiers `(?i:...)`, `(?-i:...)` | 23.0 | No (syntax error) |
| Duplicate named capture groups in different alternatives | 23.0 | No (syntax error) |
| `Intl.DurationFormat` | 23.0 | No |
| `Intl.Locale.prototype.getWeekInfo()`, `getCalendars()`, `getTimeZones()`, `getHourCycles()`, ... (method forms) | 24.0 | No (only the deprecated accessors) |
| `Atomics.pause()` | 24.0 | No |
| WebAssembly Memory64 (`new WebAssembly.Memory({ initial: 1n, address: 'i64' })`) | 24.0 | No |
| WebAssembly JSPI (`WebAssembly.Suspending`, `WebAssembly.promising`) | 24.20.0 | No |

Iterator helpers, `Set` methods, `Array.fromAsync`, `Object.groupBy`/`Map.groupBy`, `Promise.withResolvers`, resizable/transferable `ArrayBuffer`, `String.prototype.isWellFormed`, `JSON.rawJSON` and the `JSON.parse` reviver `context.source` were already in Node 22 and remain available.

##### Runtime and APIs

- **`URLPattern` is a global** (24.0); it is not available on 22. `CloseEvent` is also global (since 23.0).
- **`AsyncLocalStorage` uses `AsyncContextFrame` by default** (24.0); `--no-async-context-frame` restores the async_hooks-based implementation. New constructor options `name` and `defaultValue` (24.0) and `asyncLocalStorage.withScope()` for `using` (24.20.0, experimental).
- **Permission model**: `--permission` is the only spelling (24 rejects `--experimental-permission` as a bad option, 22.23.4 still accepts it). New: the entry point is implicitly readable (24.2), `--allow-inspector` (24.12), `--permission-audit` (24.20), `process.permission.drop()` (24.20), `--allow-openssl-store` (24.21).
- **Module system**: `require(esm)` is stable (24.15.0) and `--no-require-module` exists; package `"imports"` may start with `#/` (24.14.0, rejected on 22); WebAssembly source-phase imports (`import source mod from './x.wasm'`) work (rejected on 22); the namespace of an imported CommonJS module has an extra `'module.exports'` key (since 23.0). `module.register()` is documentation-deprecated (DEP0205, 24.15.0) in favour of `module.registerHooks()`.
- **TypeScript type stripping is stable** (24.12.0) and no longer prints an experimental warning (24.3.0). It is also enabled by default since 22.18.0 (and runs without a warning on 22.23.4), but is still documented as experimental on the 22 line.
- **`node:sqlite`** loads without an experimental warning on 24 (22 warns), is a release candidate (24.15.0) and gained `Session`, `createTagStore()`, `setAuthorizer()`, `limits`, `Symbol.dispose` on databases and statements, and `StatementSync.prototype.close()` (24.22.0).
- **Test runner**: subtests are awaited automatically (24.0; on 22 an un-awaited `t.test()` is cancelled), the default reporter is always `spec` (22 prints TAP when stdout is not a TTY), `it.expectFailure`/`{ expectFailure }` (24.14.0), `--test-randomize`/`--test-random-seed` (24.16.0), `--test-rerun-failures` (24.7.0), `--test-global-setup` (24.0), test `tags` with `--experimental-test-tag-filter` (24.19.0).
- **undici 7** powers `fetch`, `WebSocket` and friends (undici 7.30.1 in 24.22.1).
- **Web Locks**: `navigator.locks` and `worker_threads.locks` (24.5.0).
- **Crypto**: OpenSSL 3.5 (24.5.0); `crypto.argon2()`/`argon2Sync()` (24.7.0); ML-KEM via `crypto.encapsulate()`/`decapsulate()` and ML-DSA/SLH-DSA signing (24.6–24.8); `crypto.randomUUIDv7()` (24.16.0); `crypto.hash()` `outputLength` for XOFs (24.4.0); `crypto.createMac()`/`getMacs()` and `crypto.parsePKCS12()` (24.22.0); Web Crypto gained AES-OCB, ChaCha20-Poly1305, ML-DSA, ML-KEM, SHA-3, SHAKE, Argon2, KMAC and `SubtleCrypto.supports()` (24.7–24.18).
- **HTTP**: `res.writeInformation()` for arbitrary 1xx responses (24.18.0); `req.signal` on `IncomingMessage` (24.16.0); `http.setGlobalProxyFromEnv()` (24.14.0); `shouldUpgradeCallback` server option (24.9.0); `optimizeEmptyRequests` server option (24.12.0); `httpValidation` header-validation option (24.19.0). HTTP/2 priority signaling was removed (24.2.0, DEP0194).
- **fs**: `fs.mkdtempDisposableSync()`/`fsPromises.mkdtempDisposable()` (24.4.0), `Dir` is (async-)disposable (24.1.0; also backported to 22.23.4), `fs.Utf8Stream` (24.6.0), `fs.watch` `ignore` (24.14.0), `stat` `throwIfNoEntry` for the promise/callback forms (24.15.0) and `signal` (24.16.0), caller-supplied `readFile()` buffers (24.19.0).
- **util**: `util.debounce()`/`util.throttle()`/`util.markPromiseAsHandled()` (24.22.0), `MIMEType.parse()` (24.21.0), `util.convertProcessSignalToExitCode()` (24.14.0), `util.deprecate()` options (24.12.0), hex colours in `styleText()` (24.16.0). `util.getCallSite()` was removed in 24.10.0 (use `util.getCallSites()`).
- **Buffers**: the default `Buffer.poolSize` grew from 8 KiB to 64 KiB (24.18.0). 24.0.0–24.11.0 had a bug where `Buffer.allocUnsafe()` returned zero-filled memory; 24.11.1 restored the documented (uninitialized) behaviour — never rely on either.
- **Streams**: `bytes()` in `node:stream/consumers` (24.14.0), `stream.compose()` stable (24.19.0), `Blob.prototype.textStream()` (24.19.0), `CompressionStream('brotli')` (24.7.0, also backported to 22).
- **Timers**: `clearImmediate()` now only clears immediates; passing a `Timeout` no longer cancels it (verified: 22 cancels, 24 does not).
- **Other**: `import.meta.main` (24.2.0), `--watch-kill-signal` (24.4.0), `NODE_USE_SYSTEM_CA=1` (24.6.0), `--env-file` stable (24.10.0), module compile cache stable (24.15.0), `--no-require-module` (24.15.0; the old `--no-experimental-require-module` spelling is legacy), built-in proxy support for `http.request()`/`https.request()` under `NODE_USE_ENV_PROXY` (24.5.0; `fetch` since 24.0), `--max-heap-size` (24.15.0), AES-SIV and AES-GCM-SIV in `createCipheriv()` (24.22.0; absent on 22 and 25.9.0). Several of these were backported to 22.x — check before flagging them on a 22-compatible project.

***

#### Do not use or recommend

Node 24 does not have the following. Never suggest them for a Node 24 project (all verified absent on 24.22.1).

##### Added in Node 25 (V8 14.1)

- `Uint8Array.fromBase64()`, `Uint8Array.fromHex()`, `Uint8Array.prototype.toBase64()`, `toHex()`, `setFromBase64()`, `setFromHex()` — use `Buffer.from(str, 'base64')`, `buf.toString('base64url')`, `Buffer.from(hex, 'hex')`.
- `ErrorEvent` global.
- Web Storage enabled by default (`localStorage`, `sessionStorage`, `Storage` globals, `--webstorage`). On 24 `localStorage` is `undefined` unless the process runs with `--experimental-webstorage --localstorage-file=<path>` (experimental, prints a warning).
- `--allow-net` (network permission scope). On 24 `--allow-net` is a "bad option", and `--permission` does **not** restrict network access at all.
- `process.addUncaughtExceptionCaptureCallback()`, `--build-sea`.
- Assumptions that `SlowBuffer`, `fs.F_OK`/`R_OK`/`W_OK`/`X_OK` or `assert.CallTracker` are gone: they still exist on 24 (runtime-deprecated) and were removed in 25. Do not "fix" code by relying on 25 behavior; migrate off them instead.

##### Added in Node 26 (V8 14.6)

- `Temporal` (global). Use `Date` plus `Intl.DateTimeFormat`, or a library such as a Temporal polyfill, on 24.
- `Map.prototype.getOrInsert()` / `getOrInsertComputed()` and the `WeakMap` versions (upsert proposal).
- `Iterator.concat()`.
- `QuotaExceededError` global, `Response.prototype.textStream()`/`Request.prototype.textStream()`.
- `node:ffi`, `node:vfs`/`--experimental-vfs`, Web Workers (`--experimental-web-worker`), `--bench`/`node:bench`, `--process-timeout`.
- `zlib.ZipFile`/`ZipEntry`/`ZipBuffer`, `fs.openAsBlobSync()`, `fs.Stats` `*Instant` properties, `buffer.isLatin1()`, `Buffer.stringLength()`, `http.isValidHeaderName()`/`isValidHeaderValue()`, `http.websocketMask()`, `util.isPartialDeepStrictEqual()`, `diagnostics_channel.boundedChannel()`.
- `node:sqlite` `Database`/`Statement` class names (26.11 renamed them); on 24 they are `DatabaseSync`/`StatementSync`.

##### Not shipped in any Node release (as of 26.12.1)

Decorators (`@dec` is a syntax error; TypeScript `experimentalDecorators` code must be compiled), `import defer`, `Iterator.zip`, `Math.sumPrecise`, `ShadowRealm` (only behind `--experimental-shadow-realm`), `Array.prototype.group` (the shipped API is `Object.groupBy`/`Map.groupBy`).

##### Opt-in experimental features on 24 — not for production by default

`--experimental-strip-types` is default, but `--experimental-transform-types` (enums, namespaces, parameter properties) is experimental and removed in 26. Also experimental on 24: `--experimental-stream-iter` (`node:stream/iter`), `--experimental-package-map`, `--experimental-import-text`, `--experimental-test-module-mocks` (`mock.module()`), `--experimental-webstorage`, `--experimental-eventsource`, `--experimental-config-file`, `--experimental-test-coverage`, `--experimental-network-inspection`, `asyncLocalStorage.withScope()`.

***

#### Deprecated and removed APIs

Node 24 turns several long-standing documentation deprecations into runtime warnings and removes others. Flag any new use of these:

| API | Status on 24 | Replacement |
|---|---|---|
| `url.parse()` (and `url.format(string)`/`url.resolve()`, which call it) | Runtime warning DEP0169 for application code (not for code inside `node_modules`) | `new URL(input, base)`, `URL.canParse()`, `URL.parse()` |
| `SlowBuffer` | Runtime warning DEP0030 (removed in 25) | `Buffer.allocUnsafeSlow()` |
| `fs.F_OK`, `fs.R_OK`, `fs.W_OK`, `fs.X_OK` | Runtime warning DEP0176 (removed in 25) | `fs.constants.F_OK`, ... |
| zlib classes without `new` (`zlib.Gzip()`) | Runtime warning DEP0184 | `new zlib.Gzip()` or `zlib.createGzip()` |
| REPL classes without `new` | Runtime warning DEP0185 | `new repl.REPLServer()` |
| `fs.existsSync()` with invalid argument types | Runtime warning DEP0187 | pass a string/Buffer/URL |
| `args` array passed to `spawn`/`execFile` with `shell: true` | Runtime warning DEP0190 | no shell (`execFile(cmd, args)`), or one fully escaped command string |
| `tls.createSecurePair()`, `tls.SecurePair` | Removed (End-of-Life) | `tls.TLSSocket` |
| `OutgoingMessage.prototype._headers` / `_headerNames` | Removed | `getHeaders()`, `getHeaderNames()` |
| `fs.truncate(fd)` | Removed: now throws `ERR_INVALID_ARG_TYPE` | `fs.ftruncate(fd)` |
| `dirent.path` | Removed (`undefined`) | `dirent.parentPath` |
| `timers.enroll()`, `unenroll()`, `active()`, `_unrefActive()` | Removed | `setTimeout()`/`timeout.refresh()`/`unref()` |
| `net._setSimultaneousAccepts()`, `tls.Server.prototype.setOptions()` | Removed | — |
| `crypto.Cipher`/`crypto.Decipher` exports | Removed | `crypto.createCipheriv()`/`createDecipheriv()` |
| `util.isBoolean()`, `isBuffer()`, `isDate()`, `isError()`, `isFunction()`, `isNull()`, `isNullOrUndefined()`, `isNumber()`, `isObject()`, `isPrimitive()`, `isRegExp()`, `isString()`, `isSymbol()`, `isUndefined()`, `util.log()` (`util.isArray()` still exists) | Removed in 23 (calling them throws `TypeError: ... is not a function`) | `typeof`, `Array.isArray`, `Buffer.isBuffer`, `Error.isError`, `util.types.*` |
| `process.assert()` | Removed (23) | `node:assert` |
| `util.getCallSite()` | Removed (24.10.0) | `util.getCallSites()` |
| HTTP/2 priority signaling | Removed (24.2.0) | — |
| `--experimental-permission`, `--experimental-default-type` | Rejected as bad options | `--permission`; `"type"` in `package.json` |
| `module.register()` | Documentation-deprecated (DEP0205) | `module.registerHooks()` |
| `util.types.isNativeError()` | Documentation-deprecated (DEP0197) | `Error.isError()` |
| `http` classes without `new` | Documentation-deprecated (DEP0195) | `new http.Agent()` etc. |
| `options.shell === ''` in `child_process` | Documentation-deprecated (DEP0196) | `shell: false` or a shell path |
| `require('node:_http_*')`, `_stream_*`, `_tls_common`, `_tls_wrap` | Documentation-deprecated (DEP0199/0193/0192) | public modules |
| Letting `fs.Dir` close on GC | Documentation-deprecated (DEP0200) | `using`/`await using`, `dir.close()` |
| Passing a `CryptoKey` to `node:crypto` functions | Documentation-deprecated (DEP0203) | `KeyObject.from(cryptoKey)` |
| `Hmac#digest()` on a finalized instance | Documentation-deprecated (DEP0206, 24.18.0) | create a new Hmac |
| Using `AbortSignal` to dispose long-lived resources (`server.listen({ signal })`) | Documentation-deprecated (DEP0209, 24.22.0) | `using`/`await using`; keep `AbortSignal` for cancellation |

The runtime warnings were verified on 24.22.1 (none of them is printed on 22.23.4). Run CI with `--throw-deprecation` (or at least `--trace-deprecation`) so new uses fail fast; `--pending-deprecation` also surfaces documentation-only deprecations that support it (DEP0191 `repl.builtinModules`, DEP0198 SHAKE digests without `outputLength`).

***

#### Core language style and idioms

##### Declarations, equality, naming

```js
// const by default, let when reassigned, never var
const MAX_RETRIES = 3;            // SCREAMING_SNAKE_CASE for module-level constants
let attempt = 0;                  // camelCase for variables and functions
class RetryPolicy {}              // PascalCase for classes and constructors

// Strict equality everywhere; the only accepted loose comparison is `== null`
if (value === expected) {}
if (value == null) {}             // deliberately matches null and undefined

// Nullish operators: ?? keeps 0, '' and false; || does not
const port = options.port ?? 8080;
const name = user?.profile?.name ?? 'anonymous';
config.retries ??= 3;             // logical assignment (only assigns when nullish)
```

- Use object/array destructuring, default parameters, rest/spread, template literals, shorthand properties and methods, computed keys, `for...of` over arrays and iterables. Never use `for...in` over arrays (it yields string keys, includes inherited enumerable properties and does not guarantee order semantics a reviewer expects).
- Prefer named parameters via destructuring (`function createUser({ name, email, role = 'user' })`) once a function has more than two or three parameters or boolean flags.
- Do not mutate arguments; return new objects (`{ ...options, processed: true }`) or use the non-mutating array methods (`toSorted`, `toReversed`, `toSpliced`, `with`).
- Use `#private` fields and methods for encapsulation instead of `_underscore` conventions or closures; `static` blocks for complex static initialisation; `Object.hasOwn()` instead of `obj.hasOwnProperty()`.
- Use `.at(-1)` for the last element, `Array.prototype.findLast()`/`findLastIndex()` for reverse searches.
- Write strict-mode code. ES modules and class bodies are always strict; CommonJS files need `'use strict'` at the top if they rely on strict semantics.

##### Classes and objects

```js
class Account {
  static #count = 0;
  #balance = 0;                                   // truly private; not visible to JSON.stringify
  static { Account.currency = 'EUR'; }           // static initialisation block

  constructor(owner) { this.owner = owner; Account.#count++; }

  get balance() { return this.#balance; }

  deposit(amountInCents) {
    if (!Number.isInteger(amountInCents) || amountInCents <= 0) {
      throw new RangeError('amount must be a positive integer number of cents');
    }
    this.#balance += amountInCents;
    return this;
  }

  static isAccount(value) { return #balance in value; }  // ergonomic brand check
  static get count() { return Account.#count; }
}

const a = new Account('ann').deposit(500);
JSON.stringify(a);                 // '{"owner":"ann"}' -- private state is not serialised
Account.isAccount({ balance: 1 }); // false
```

- Methods lose `this` when detached (`const { onClick } = button; onClick()` → `this` is `undefined` in class code). Use arrow-function class fields (`onClick = () => this.label`) or `.bind()` for callbacks passed to event emitters/timers.
- Do not extend built-ins (`Array`, `Map`, `Promise`) for domain objects; compose instead. Extending `Error` and `EventEmitter` is idiomatic.
- Keep constructors cheap and synchronous; use a static async factory (`static async create()`) for I/O-dependent initialisation.
- `Object.freeze()` is shallow; use it for constant tables, not as a security boundary.

##### Numbers, strings, dates — classic pitfalls (verified on 24.22.1)

```js
[10, 9, 1].sort();                        // [1, 10, 9]  -- default sort compares strings
[10, 9, 1].toSorted((a, b) => a - b);     // [1, 9, 10]
parseInt(0.0000005);                      // 5  -- number coerced to "5e-7"
Number('12px');                           // NaN; Number('') is 0
0.1 + 0.2 === 0.3;                        // false -- compare with a tolerance or use integers (cents)
Number.MAX_SAFE_INTEGER + 2;              // 9007199254740992 -- use BigInt for 64-bit IDs
JSON.stringify({ n: 1n });                // TypeError: Do not know how to serialize a BigInt
new Date('2026-10-22');                   // UTC midnight (date-only ISO strings are UTC)
new Date('2026-10-22T00:00');             // LOCAL midnight (date-time without offset is local)
Array(3).map(() => 1);                    // [ <3 empty items> ] -- holes are skipped
Array.from({ length: 3 }, () => 1);       // [1, 1, 1]
structuredClone(new Money(1)) instanceof Money; // false -- class prototypes are not preserved
```

- Parse numbers with `Number()` plus `Number.isFinite()`/`Number.isInteger()` checks, or `Number.parseInt(str, 10)` with an explicit radix.
- Keep 64-bit database IDs and money in `BigInt`/strings/integers of minor units; never in floating point.
- `Date` has no time-zone-aware arithmetic. Format with `Intl.DateTimeFormat` and an explicit `timeZone`. `Temporal` does **not** exist on Node 24 (Node 26+).
- Always pass a `locale` to `toLocaleString()`/`Intl.*` in server code whose output is persisted or compared; the default locale comes from the environment.

##### Text and internationalisation (ICU 78, Unicode 17)

```js
const s = '👨‍👩‍👧' + 'e\u0301';   // family emoji (ZWJ sequence) + e + combining acute
s.length;                                                          // 10 UTF-16 code units
[...s].length;                                                     // 7 code points
[...new Intl.Segmenter('en', { granularity: 'grapheme' }).segment(s)].length; // 2 user-perceived characters

['b', 'a', 'ä', 'Z'].sort();                                       // ['Z', 'a', 'b', 'ä'] -- code-unit order
['b', 'a', 'ä', 'Z'].sort(new Intl.Collator('de').compare);        // ['a', 'ä', 'b', 'Z']

new Intl.NumberFormat('de-DE', { style: 'currency', currency: 'EUR' }).format(1234.5);   // '1.234,50 €'
new Intl.DateTimeFormat('en-GB', { timeZone: 'Europe/Berlin', dateStyle: 'medium', timeStyle: 'short' })
  .format(new Date('2026-10-22T12:00:00Z'));                       // '22 Oct 2026, 14:00'
'caf\u00e9' === 'cafe\u0301';                                    // false -- same text, different code points
'caf\u00e9'.normalize('NFC') === 'cafe\u0301'.normalize('NFC');  // true -- normalise before comparing
```

- Truncating or validating "characters" by `.length` breaks emoji and combining marks; use `Intl.Segmenter` for user-visible lengths and `Buffer.byteLength()` for storage limits.
- Case conversion is locale-sensitive and can change length (`'ß'.toUpperCase()` is `'SS'`); use `toLocaleUpperCase('tr')` etc. only when the locale is known, and compare identifiers with `localeCompare`/`Intl.Collator` `sensitivity` options or after `normalize('NFC')`.
- Official Node binaries ship full ICU; custom builds with `small-icu`/`without-intl` change `Intl` behaviour — check `process.versions.icu` if a deployment uses a custom build.

##### Collections

- `Map`/`Set` for dynamic keys and membership tests (no prototype keys, any key type, insertion order). Plain objects only for records with known keys; when an object is used as a dictionary of untrusted keys, create it with `Object.create(null)` or use a `Map` (see Security: prototype pollution).
- `Set` methods are available: `union`, `intersection`, `difference`, `symmetricDifference`, `isSubsetOf`, `isSupersetOf`, `isDisjointFrom`.
- `Object.groupBy()` returns a null-prototype object; `Map.groupBy()` returns a `Map`.
- `WeakMap`/`WeakRef`/`FinalizationRegistry` for caches keyed by object identity; never rely on finalizers for correctness (they may never run).

***

#### New in 24: Explicit resource management

Node 24 ships the TC39 explicit resource management proposal: `using` and `await using` declarations, `DisposableStack`, `AsyncDisposableStack` and `SuppressedError`. On Node 22 `using x = ...` is a `SyntaxError`.

##### Semantics

```js
class Lock {
  #name;
  constructor(name) { this.#name = name; console.log('acquire', name); }
  [Symbol.dispose]() { console.log('release', this.#name); }
}

{
  using a = new Lock('a');
  using b = new Lock('b');
  using none = null;        // null and undefined are allowed and skipped
  console.log('body');
}                           // prints: acquire a, acquire b, body, release b, release a
```

- Resources are disposed in reverse declaration order when the block exits — normally, via `return`/`break`, or by an exception.
- `using` requires the value to be `null`, `undefined`, or an object with a callable `[Symbol.dispose]`; anything else throws `TypeError` at the declaration (`using bad = {}` → `TypeError: Symbol(Symbol.dispose) is not a function`).
- `await using` calls `[Symbol.asyncDispose]` (falling back to `[Symbol.dispose]`) and awaits it; it is only allowed where `await` is (async functions, module top level).
- If the block throws **and** a disposer throws, the error is a `SuppressedError` whose `.error` is the disposal error and `.suppressed` is the original error:

```js
try {
  using x = { [Symbol.dispose]() { throw new Error('dispose failed'); } };
  throw new Error('body failed');
} catch (e) {
  console.log(e.constructor.name, e.error.message, '|', e.suppressed.message);
  // SuppressedError dispose failed | body failed
}
```

- Grammar limits (verified): `using` is not allowed at the top level of a classic script (`vm.runInThisContext('using z = null')` is a `SyntaxError`), nor directly in a `switch` `case`; it is allowed at the top level of CommonJS files (they are function bodies), modules, blocks and `for (using x of iterable)`. `using` is a contextual keyword: `let using = 1` is legal and `using [a] = []` is parsed as a member assignment to a variable named `using`, not a declaration — destructuring is not supported in `using` declarations.

##### Node APIs that are disposable on 24

| Object | Protocol |
|---|---|
| `fsPromises.open()` `FileHandle` | `Symbol.asyncDispose` (closes) |
| `fs.opendir()`/`fsPromises.opendir()` `Dir` (24.1.0) | `Symbol.dispose` and `Symbol.asyncDispose` |
| `fsPromises.mkdtempDisposable()` / `fs.mkdtempDisposableSync()` (24.4.0) | removes the temp directory |
| `http.Server`, `https.Server`, `net.Server` | `Symbol.asyncDispose` (closes the server) |
| `dgram.Socket` | `Symbol.asyncDispose` (closes the socket) |
| `ChildProcess` | `Symbol.dispose` (kills the child) |
| `worker_threads.Worker` (24.2.0) | `Symbol.asyncDispose` (terminates) |
| `stream.Readable`/`Writable` | `Symbol.asyncDispose` (destroys) |
| `node:sqlite` `DatabaseSync`, `StatementSync` (24.22.0), `Session` | `Symbol.dispose` |
| Event loop delay histogram, `v8.GCProfiler`, `AsyncLocalStorage` `RunScope` | `Symbol.dispose` |
| Iterators (`Iterator.prototype[Symbol.dispose]` calls `return()`) | `Symbol.dispose` |

```js
import { open, mkdtempDisposable } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';

{
  await using dir = await mkdtempDisposable(join(tmpdir(), 'job-'));
  await using fh = await open(join(dir.path, 'out.txt'), 'w');
  await fh.write('hello');
} // file closed first, then the temp directory is removed
```

##### `DisposableStack` for conditional or dynamic resources

```js
function openAll() {
  using stack = new DisposableStack();
  const lock = stack.use(new Lock('r1'));                         // disposable object
  const conn = stack.adopt({ id: 2 }, (c) => console.log('close', c.id)); // value + cleanup fn
  stack.defer(() => console.log('deferred cleanup'));
  // ... if anything above throws, everything acquired so far is released
  return stack.move();   // success: transfer ownership to the caller; nothing is disposed here
}

const resources = openAll();
resources.dispose();     // deferred cleanup, close 2, release r1 (reverse order)
```

Use `stack.move()` to build "acquire several, release all on failure" constructors; use `AsyncDisposableStack` (`await stack.disposeAsync()`) for async cleanup.

##### Review guidance

- **BAD**: `try { ... } finally { handle.close(); }` chains that forget one resource on an early `return`, or close in the wrong order. **GOOD**: `using`/`await using` per resource.
- **BAD**: `using` on an object whose `[Symbol.dispose]` is async (returns a promise): the promise is ignored and its rejection becomes an unhandled rejection. **GOOD**: `await using` for async cleanup.
- Disposers should be idempotent and should not throw for "already closed".
- Remove `Symbol.dispose`/`Symbol.asyncDispose` polyfills on 24: the well-known symbols are built in and non-writable (`Object.getOwnPropertyDescriptor(Symbol, 'dispose')` is `{ writable: false, configurable: false }`), so assignments are silently ignored in sloppy mode and throw in strict mode.
- Libraries that must also support Node 22 cannot use `using` syntax at all (it is a parse error there, so even an unexecuted branch breaks the module); they can still implement `[Symbol.dispose]` methods.
- Prefer `using` over `AbortSignal` for tearing down long-lived resources: passing a signal to `server.listen({ signal })` to close the server is documentation-deprecated in 24.22.0 (DEP0209). Keep `AbortSignal` for cancelling operations and timeouts.

***

#### New in 24: Other ECMAScript additions

##### `RegExp.escape()` (24.0)

```js
const userInput = 'a.b*c(1)';
const re = new RegExp(`^${RegExp.escape(userInput)}$`);
re.test('a.b*c(1)');      // true
re.test('axb*c(1)');      // false
RegExp.escape('1abc');    // '\\x31abc' -- a leading digit/letter is hex-escaped so it is safe after \1 or \c
```

**BAD**: hand-written `str.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')` helpers (they usually miss `-`, `/` or whitespace edge cases) or building a `RegExp` from raw user input (ReDoS and injection). **GOOD**: `RegExp.escape()`.

##### `Error.isError()` (24.0)

```js
import vm from 'node:vm';
const foreign = vm.runInNewContext('new TypeError("x")');
foreign instanceof Error;                       // false -- different realm
Error.isError(foreign);                         // true  -- brand check
Error.isError(Object.create(Error.prototype));  // false -- not a real error
Error.isError(new DOMException('x'));           // true
```

Use `Error.isError(value)` in `catch` blocks and error utilities that may receive values from other realms (`vm` contexts, some test sandboxes); `instanceof Error` is false for those. `util.types.isNativeError()` is documentation-deprecated in its favour (DEP0197).

##### `Float16Array` and `Math.f16round()` (24.0)

```js
const h = new Float16Array([1.1, 65504, 65520, 0.0001]);
[...h];                 // [1.099609375, 65504, Infinity, 0.00010001659393310547]
Math.f16round(1.1);     // 1.099609375
h.BYTES_PER_ELEMENT;    // 2
```

Half precision overflows above 65504 and has ~3 significant decimal digits: only use it for ML/graphics interchange formats, never for money or counters. `util.types.isFloat16Array()` and `assert.deepStrictEqual` understand it.

##### `Promise.try()` (23.0)

```js
function loadConfig(path) {
  // Synchronous throws (e.g. argument validation) become rejections, so
  // callers only need one error path.
  return Promise.try(() => {
    if (typeof path !== 'string') throw new TypeError('path must be a string');
    return readConfig(path); // may return a value or a promise
  });
}
await Promise.try((a, b) => a + b, 2, 3); // 5 -- extra arguments are forwarded
```

It replaces `new Promise((resolve) => resolve(fn()))` and `Promise.resolve().then(fn)` (which delays `fn` by a microtask).

##### RegExp modifiers and duplicate named groups (23.0)

```js
/^(?i:hello) World$/.test('HELLO World');   // true  -- case-insensitive only inside the group
/^(?i:hello) World$/.test('HELLO WORLD');   // false

const date = /(?<year>\d{4})-(?<m>\d\d)|(?<m>\d\d)\/(?<year>\d{4})/;
date.exec('2026-10').groups;  // { year: '2026', m: '10' }
date.exec('10/2026').groups;  // { year: '2026', m: '10' }
```

Both are syntax errors on Node 22. Supported modifier flags are `i`, `m` and `s`.

##### `Intl.DurationFormat` (23.0) and `Intl.Locale` info methods (24.0)

```js
new Intl.DurationFormat('en', { style: 'short' }).format({ hours: 1, minutes: 5 }); // '1 hr, 5 min'
new Intl.Locale('en-US').getWeekInfo();  // { firstDay: 7, weekend: [6, 7] }
```

Use the method forms (`getWeekInfo()`, `getCalendars()`, `getCollations()`, `getHourCycles()`, `getNumberingSystems()`, `getTextInfo()`, `getTimeZones()`). The older accessor properties (`locale.weekInfo`, `locale.timeZones`, ...) still exist on 24 but were removed in Node 26.

##### `Atomics.pause()` (24.0)

A CPU hint for spin-wait loops on `SharedArrayBuffer` (returns `undefined`). Use it inside short spin loops before falling back to `Atomics.wait()`; never as a sleep.

##### WebAssembly

- Memory64 (24.0): 64-bit memories use BigInt sizes: `new WebAssembly.Memory({ initial: 1n, address: 'i64' })`.
- JSPI (24.20.0): `WebAssembly.Suspending` and `WebAssembly.promising` let Wasm call async JS functions.
- ES module integration: `import { add } from './add.wasm'` (instance phase) and `import source mod from './add.wasm'` (source phase, yields a `WebAssembly.Module`; 24 only — a syntax error on 22). Instance-phase `.wasm` imports work without a flag on 24 (unflagged in 24.5.0) and on 22.23.4; the integration still follows an evolving proposal.

***

#### Modern built-ins that are safe to use

Available on Node 22 and 24 — recommend them freely in Node 24 code:

```js
// Iterator helpers: lazy, work on any iterator (generators, Map.prototype.entries(), ...)
function* naturals() { let i = 0; while (true) yield i++; }
naturals().filter((n) => n % 2).map((n) => n * n).take(3).toArray(); // [1, 9, 25]
Iterator.from(someIterable).drop(1).some((x) => x > 10);

// Sets
new Set([1, 2, 3]).intersection(new Set([2, 3, 4]));   // Set {2, 3}

// Grouping
Object.groupBy(orders, (o) => o.status);               // null-prototype object
Map.groupBy(orders, (o) => o.customer);                // Map keyed by object identity

// Non-mutating array methods
const sorted = arr.toSorted((a, b) => a - b);
const replaced = arr.with(0, 'first');

// Promises
const { promise, resolve, reject } = Promise.withResolvers();
const results = await Array.fromAsync(asyncIterable);

// Strings and binary data
'ab\uD800'.isWellFormed();                              // false (lone surrogate)
'ab\uD800'.toWellFormed();                              // 'ab\uFFFD'
const ab = new ArrayBuffer(4, { maxByteLength: 16 });   // resizable
ab.resize(8);
const moved = ab.transfer();                            // ab.detached === true

// JSON with exact big numbers
JSON.stringify({ big: JSON.rawJSON('12345678901234567890') }); // '{"big":12345678901234567890}'
JSON.parse('{"n": 12345678901234567890}', (k, v, ctx) => (k === 'n' ? BigInt(ctx.source) : v));

// Deep copy with structured clone semantics (Date, Map, Set, typed arrays, errors with cause)
const copy = structuredClone(value);
```

Also: `Object.hasOwn`, `Array.prototype.at`, `Error` `cause`, `AggregateError`, `Promise.any`/`allSettled`, `WeakRef`, class fields/private methods/static blocks, top-level `await` in ESM, `Array.prototype.findLast`, hashbang lines, `String.prototype.replaceAll`, the RegExp `v` flag (set operations and string properties in character classes), `Atomics.waitAsync`, import attributes (`with { type: 'json' }`).

***

#### Web platform globals on Node 24

Available without imports on 24.22.1 (verified): `fetch`, `Request`, `Response`, `Headers`, `FormData`, `WebSocket` (client), `URL`, `URLSearchParams`, `URLPattern` (**New in 24**), `AbortController`, `AbortSignal`, `Blob`, `File`, `BroadcastChannel`, `MessageChannel`, `structuredClone`, `TextEncoder`/`TextDecoder` (and stream variants), `CompressionStream`/`DecompressionStream`, `ReadableStream`/`WritableStream`/`TransformStream`, `EventTarget`, `Event`, `CustomEvent`, `CloseEvent`, `DOMException`, `crypto` (Web Crypto, `crypto.subtle`, `crypto.randomUUID()`), `performance`, `queueMicrotask`, `navigator` (`hardwareConcurrency`, `language`, `languages`, `platform`, `userAgent`, `locks`), `WebAssembly`, `Iterator`.

Not available as globals on 24: `EventSource` (needs `--experimental-eventsource`), `localStorage`/`sessionStorage` (need `--experimental-webstorage`), `Worker` (Web Workers are Node 26+; use `node:worker_threads`), `ErrorEvent` (25+), `Temporal` (26+).

##### Dependencies that Node 24 makes unnecessary

Flag new dependencies (and suggest removing old ones) when the built-in covers the use case:

| Package | Built-in on Node 24 |
|---|---|
| `node-fetch`, `cross-fetch`, `isomorphic-fetch`, `abort-controller`, `form-data` | global `fetch`, `AbortController`, `FormData` |
| `ws` (client side only) | global `WebSocket` (a WebSocket **server** still needs a library) |
| `uuid` (v4; v7 on 24.16+) | `crypto.randomUUID()`, `crypto.randomUUIDv7()` |
| `dotenv` (simple cases) | `--env-file`, `process.loadEnvFile()`, `util.parseEnv()` |
| `glob`, `fast-glob` (simple cases), `rimraf`, `mkdirp` | `fs.glob()`, `fs.rm({ recursive: true })`, `fs.mkdir({ recursive: true })` |
| `chalk`, `kleur` (basic colours) | `util.styleText()` |
| `minimist`, simple `yargs` use | `util.parseArgs()` |
| `escape-string-regexp` | `RegExp.escape()` |
| `lodash.debounce`/`throttle` (24.22+ only) | `util.debounce()`, `util.throttle()` |
| `ts-node`/`tsx` for erasable-only TypeScript | type stripping |
| `jest`/`mocha`/`sinon` for straightforward suites | `node:test`, `node:assert`, `mock`, `mock.timers`, snapshots |
| `core-js` polyfills for ES2025 features | native (`Set` methods, iterator helpers, `Promise.try`, ...) |
| `@ungap/structured-clone` | `structuredClone()` |

Do **not** drop a library when the project must support runtimes that lack the built-in, or when the library provides behaviour the built-in lacks (e.g. `dotenv-expand` interpolation, `glob` options, `undici` agents/interceptors).

***

#### Errors and error handling

##### Throw `Error` objects with context and `cause`

```js
class AppError extends Error {
  constructor(message, { code, statusCode = 500, cause } = {}) {
    super(message, { cause });          // standard `cause` (ES2022): preserves the original error
    this.name = new.target.name;        // 'AppError' / subclass name
    this.code = code;
    this.statusCode = statusCode;
  }
}

class NotFoundError extends AppError {
  constructor(resource, id, options) {
    super(`${resource} ${id} not found`, { code: 'NOT_FOUND', statusCode: 404, ...options });
  }
}

async function loadUser(id) {
  try {
    return await db.findUser(id);
  } catch (err) {
    throw new AppError(`loading user ${id} failed`, { code: 'DB_READ', cause: err });
  }
}
```

- **BAD**: `throw 'not found'`, `throw { code: 404 }`, `reject('x')` — no stack, fails `instanceof Error`/`Error.isError`. **GOOD**: always `Error` (sub)classes.
- **BAD**: `throw new Error('failed: ' + err.message)` (loses the original stack). **GOOD**: `{ cause: err }`. `util.inspect` and uncaught-exception output print the cause chain.
- Branch on stable properties (`err.code`, `err.name`, `instanceof`), never on `err.message` text — Node and V8 error messages and stack formats change between major versions.
- Node system errors carry `code` (`'ENOENT'`, `'ECONNREFUSED'`, `'ERR_INVALID_ARG_TYPE'`, `'ERR_ACCESS_DENIED'`...), `syscall`, `path`, `errno`. Compare `err.code === 'ENOENT'`.
- `Error.captureStackTrace(this, Ctor)` is unnecessary in subclasses (the `super()` call already captures the stack); use it only to hide helper frames.
- Use `AggregateError` (or `Promise.any` rejection) to report several failures; `SuppressedError` comes from resource disposal.
- `catch (err)` variables are `unknown` in spirit: guard with `Error.isError(err)` before reading `.message` when the thrower is not under your control.

##### Process-level handlers

- Unhandled promise rejections terminate the process with exit code 1 (default `--unhandled-rejections=throw`, verified). Do not change this to `warn` or `none` in production.
- `process.on('uncaughtException')` is for logging and a fast, clean shutdown — not for resuming. After an uncaught exception the process is in an undefined state.
- A floating promise (`doWork();` without `await`/`.catch()`) that rejects crashes the server. Lint with `@typescript-eslint/no-floating-promises` (or equivalent) and review every un-awaited call.
- `util.markPromiseAsHandled(promise)` (24.22.0) marks a promise as handled without attaching a handler; prefer explicit `.catch()` for readability and use this only in infrastructure code (and remember it is missing on 25.9.0 and on 24 < 24.22).

```js
process.on('unhandledRejection', (reason) => {
  logger.fatal({ err: reason }, 'unhandled rejection');
  // let the default behaviour (crash) happen; a supervisor restarts the process
});

process.on('uncaughtException', (err, origin) => {
  logger.fatal({ err, origin }, 'uncaught exception');
  process.exitCode = 1;
  shutdown().finally(() => process.exit());
});
```

##### `async` error-handling pitfalls

```js
// BAD: forEach ignores returned promises -- errors become unhandled rejections and
// the function returns before the work is done.
items.forEach(async (item) => { await save(item); });

// GOOD: sequential
for (const item of items) await save(item);

// GOOD: concurrent, fail fast
await Promise.all(items.map((item) => save(item)));

// GOOD: concurrent, collect all outcomes
const results = await Promise.allSettled(items.map(save));
const failures = results.filter((r) => r.status === 'rejected');

// BAD: `return promise` inside try without await -- the catch never sees the rejection
async function bad() { try { return fetchJson(); } catch { return null; } }
// GOOD
async function good() { try { return await fetchJson(); } catch { return null; } }
```

Verified: when `[1, 2, 3].forEach(async ...)` returns, none of the three async callbacks has completed.

***

#### Asynchronous code and concurrency

##### The event loop rules a reviewer must enforce

- Never block the event loop in request paths: no `fs.*Sync`, `child_process.*Sync`, `crypto.*Sync` (`scryptSync`, `pbkdf2Sync`, `argon2Sync`, `generateKeyPairSync`), `zlib.*Sync`, large `JSON.parse`/`JSON.stringify`, or CPU-heavy loops. Sync APIs are fine at startup, in CLIs and in tests.
- Move CPU-bound work to `worker_threads` (or a separate process); move blocking I/O to the async APIs.
- Microtasks (promise reactions, `queueMicrotask`) and `process.nextTick` callbacks run before the loop continues; recursive microtasks/nextTicks starve I/O.
- Ordering (verified): in a CommonJS entry point `process.nextTick` callbacks run before promise reactions; in an ES module entry point promise reactions queued during module evaluation run **before** `nextTick` callbacks. Code that depends on nextTick-vs-microtask ordering is fragile; the order of `setTimeout(fn, 0)` vs `setImmediate(fn)` from the main module is not guaranteed either.
- `setTimeout`/`setInterval` delays above 2³¹−1 ms (~24.8 days) overflow: Node prints `TimeoutOverflowWarning` and uses **1 ms** (verified). Clamp long delays or reschedule.
- Use `timeout.unref()` for timers that must not keep the process alive (e.g. periodic metrics flush).

##### Promise combinators

| Need | Use |
|---|---|
| All must succeed, fail fast | `Promise.all` |
| Every outcome, never rejects | `Promise.allSettled` |
| First success, ignore failures | `Promise.any` (rejects with `AggregateError`) |
| First settled (timeouts, races) | `Promise.race` — but prefer `AbortSignal.timeout()` so the loser is actually cancelled |
| Wrap sync-or-async callback | `Promise.try` (**New in 24**) |
| Resolve from outside | `Promise.withResolvers()` |

**BAD**: `await` in a loop when iterations are independent (serialises latency). **BAD**: `Promise.all(hugeArray.map(fetch))` (unbounded concurrency exhausts sockets, file descriptors or the remote API's rate limit). **GOOD**: bounded concurrency:

```js
async function mapLimit(items, limit, fn) {
  const results = new Array(items.length);
  let next = 0;
  async function worker() {
    while (next < items.length) {
      const i = next++;                 // safe: no await between read and increment
      results[i] = await fn(items[i], i);
    }
  }
  await Promise.all(Array.from({ length: Math.min(limit, items.length) }, worker));
  return results;
}
```

For rate limiting, Node 24.22.0 adds `util.throttle(fn, limit, interval, options)`, and `util.debounce(fn, wait, options)` for coalescing bursts. Both return promise-returning wrappers (verified: three rapid calls to a debounced function resolve to the result of the last call, and `fn` runs once). They are **New in 24.22** (not in 25.9.0 or older 24.x); keep existing libraries when the project must support earlier versions.

```js
import { debounce } from 'node:util';
const saveDraft = debounce(async (text) => api.save(text), 500, { signal: shutdown.signal });
await saveDraft('a'); // all callers in the window get the result of the last invocation
```

##### Subtle async bug patterns (all verified on 24.22.1)

```js
// BAD: for await over an array of already-started promises. If a later promise rejects
// before an earlier one settles, the rejection is "unhandled" first (process crash with the
// default --unhandled-rejections=throw), even though the loop's try/catch sees it later.
for await (const user of [fetchUser(1), fetchUser(2)]) { /* ... */ }
// GOOD
for (const user of await Promise.all([fetchUser(1), fetchUser(2)])) { /* ... */ }

// BAD: async executor -- a throw inside it never rejects the promise; the promise stays
// pending forever and the error becomes an unhandled rejection.
const p = new Promise(async (resolve) => { resolve(await load()); });
// GOOD
const p2 = load();                 // or Promise.try(load)

// BAD: async callback in setInterval -- invocations overlap when the work is slower than
// the interval (3 concurrent runs observed with a 10 ms interval and 25 ms of work).
setInterval(async () => { await syncWithRemote(); }, 10_000);
// GOOD: schedule the next run after the previous one finished
async function loop(signal) {
  while (!signal.aborted) {
    await syncWithRemote().catch(logError);
    await sleep(10_000, undefined, { signal }).catch(() => {});
  }
}

// BAD: shared reference
const grid = Array(3).fill([]);    // all rows are the same array
// GOOD
const grid2 = Array.from({ length: 3 }, () => []);
```

Other frequent review findings: `[NaN].indexOf(NaN)` is `-1` (use `includes`), `Date` months are 0-based, `delete arr[i]` leaves a hole (use `splice`/`toSpliced`), `JSON.parse` of external input needs `try/catch`, mixing callbacks and promises (`util.promisify` or the `node:*/promises` APIs instead), and attaching `.then()` without returning it from the enclosing function.

##### Cancellation with `AbortSignal`

```js
import { setTimeout as sleep } from 'node:timers/promises';

// Timeouts: AbortSignal.timeout(); combine with a caller signal via AbortSignal.any()
async function getJson(url, { signal } = {}) {
  const res = await fetch(url, { signal: AbortSignal.any([AbortSignal.timeout(5_000), signal].filter(Boolean)) });
  if (!res.ok) throw new Error(`GET ${url}: HTTP ${res.status}`);
  return res.json();
}

try {
  await getJson('https://api.example.com/items');
} catch (err) {
  if (err.name === 'TimeoutError') { /* fetch: timeout */ }
  else if (err.name === 'AbortError') { /* fetch: aborted by caller */ }
  else throw err;
}
```

- Error names differ by API (verified): `fetch` rejects with a `DOMException` named `TimeoutError` when an `AbortSignal.timeout()` fires and `AbortError` for manual aborts; `timers/promises` and most `node:` APIs reject with an `AbortError` (`code: 'ABORT_ERR'`) whose `cause` is the signal's reason (a `TimeoutError` DOMException for `AbortSignal.timeout()`). Check `err.name`, not `instanceof`.
- Every long operation (HTTP calls, DB queries, `events.once`, `stream.pipeline`, `fs.readFile`, child processes) accepts a `signal`. Pass the request's signal down: on 24.16+ `req.signal` on an incoming HTTP request aborts when the client disconnects.
- Remove listeners you add to long-lived signals (`signal.addEventListener('abort', fn, { once: true })` and `removeEventListener` when done) — otherwise you leak closures and get `MaxListenersExceededWarning`.

##### Events

```js
import { once, on, EventEmitter } from 'node:events';

const [value] = await once(emitter, 'ready', { signal });          // rejects on 'error'
for await (const [msg] of on(emitter, 'message', { signal })) {    // async iteration
  handle(msg);
}
```

- An `'error'` event with no listener throws and usually crashes the process: every `EventEmitter`, stream, socket, server, child process and worker needs an `'error'` handler (or must be wrapped by `pipeline`/`once`).
- `async` listeners on an `EventEmitter` produce unhandled rejections unless the emitter is created with `{ captureRejections: true }` (rejections are then routed to `'error'`).
- `events.listenerCount(emitterOrTarget, name)` accepts `EventTarget`s too (24.14.0).

##### `AsyncLocalStorage` — request context (**changed in 24**)

```js
import { AsyncLocalStorage } from 'node:async_hooks';

const requestContext = new AsyncLocalStorage({ name: 'request', defaultValue: { requestId: 'none' } });
const log = (msg) => console.log(`[${requestContext.getStore().requestId}] ${msg}`);

http.createServer((req, res) => {
  requestContext.run({ requestId: req.headers['x-request-id'] ?? randomUUID() }, () => {
    handle(req, res).catch((err) => { log(err.message); res.statusCode = 500; res.end(); });
  });
});
```

- Node 24 implements `AsyncLocalStorage` on top of V8's `AsyncContextFrame` by default (`--no-async-context-frame` restores the async_hooks implementation). The common patterns — `run()`, propagation across `await`, timers, promise reactions and event listeners — behaved identically on 22 and 24 in testing. The docs ask users to report flow differences.
- `defaultValue` and `name` options are **New in 24** (on 22 `getStore()` outside `run()` returns `undefined`).
- Prefer `run()`. `enterWith()` is experimental and leaks: calling it in an async function before the first `await` changes the **caller's** context (verified: the caller sees the new store after the call returns).
- `asyncLocalStorage.withScope(store)` (24.20.0, experimental) returns a `RunScope` for `using`; the same leak applies inside async functions before the first `await`. Use it only in synchronous code.
- Context is lost through custom thenables and callback APIs that queue work internally (connection pools). `EventEmitter` listeners run in the context of the code that calls `emit()`, not the code that registered them (verified: listener registered in context L, emitted in M, sees M). Use `AsyncResource.bind()`/`AsyncLocalStorage.bind()` to pin a callback to the registering context.

##### Worker threads

```js
import { Worker } from 'node:worker_threads';
import { once } from 'node:events';

{
  await using worker = new Worker(new URL('./hash-worker.mjs', import.meta.url));
  worker.on('error', (err) => logger.error(err));
  worker.postMessage({ id: 1, password, salt });
  const [msg] = await once(worker, 'message');
} // terminated by Symbol.asyncDispose (24.2.0+)
```

- Workers are for CPU-bound JavaScript; they do not speed up I/O. Reuse a pool (e.g. `piscina`) instead of spawning a worker per request (each worker has its own isolate, ~MBs of memory and startup time).
- Transfer large `ArrayBuffer`s (`postMessage(msg, [buf.buffer])`) instead of copying; share state only via `SharedArrayBuffer` + `Atomics`.
- Under `--permission`, creating a worker requires `--allow-worker`, and Node prints `SecurityWarning [PERM0006]: The flag --allow-worker must be used with extreme caution. It could invalidate the permission model.` In testing on 24.22.1 the file-system restrictions still applied inside the worker, but the docs list "does not inherit to a worker thread" as a constraint — do not rely on workers being sandboxed.
- Web Locks (`navigator.locks.request(name, async (lock) => {...})`, 24.5.0) coordinate across threads of one process; they are not distributed locks.

***

#### Modules: ESM, CommonJS and interop

##### Choosing the module system

- New code: ES modules with `"type": "module"` in `package.json` (or `.mjs`). Use `.cjs` for the few files that must stay CommonJS.
- A `.js` file without a `"type"` field that contains `import`/`export` is detected as ESM, but Node prints `MODULE_TYPELESS_PACKAGE_JSON` and parses it twice when a `package.json` without `"type"` exists (verified on 22 and 24). Always set `"type"`.
- `--experimental-default-type` is gone (24 rejects it); use `"type"`.
- Always import built-ins with the `node:` prefix (`node:fs`, `node:test`). Some modules (`node:sqlite`, `node:test`, `node:sea`) exist **only** with the prefix.

##### ESM essentials

```js
import data from './data.json' with { type: 'json' };            // import attributes
const mod = await import('./data.json', { with: { type: 'json' } });
console.log(import.meta.dirname, import.meta.filename, import.meta.url);
if (import.meta.main) await main();                              // entry point check (24.2.0; also 22.18+)
const resolved = import.meta.resolve('./worker.mjs');            // synchronous, returns a URL string
```

- The old `assert { type: 'json' }` syntax is a `SyntaxError` on 22 and 24; JSON imports without an attribute throw `ERR_IMPORT_ATTRIBUTE_MISSING` (verified).
- ESM has no `require`, `__dirname`, `__filename`, `module`, `exports`. Use `import.meta.dirname`/`filename`, `createRequire(import.meta.url)` when a CommonJS-only API is unavoidable, and `process.getBuiltinModule('node:fs')` for conditional access to built-ins from code that must run in both module systems.
- Relative specifiers need the file extension (`./util.js`, not `./util`). Directory imports (`./lib`) do not resolve to `index.js` in ESM.
- Top-level `await` blocks the importing modules; keep it to startup configuration.

##### Importing CommonJS from ESM

```js
import pkg from './legacy.cjs';          // always works: default export = module.exports
import { a } from './exports-a.cjs';     // works only if the CJS lexer detects `a`
```

- Named exports are detected statically (on 24.14.0+ by the `merve` lexer, earlier by `cjs-module-lexer`). Detected: `exports.a = ...`, `module.exports = { a, b: b }` with identifier values, re-exports via `module.exports = require(...)`. **Not detected** (verified): `module.exports = { d: 4 }` with literal values and `module.exports = someObject` — `import { d } from './c3.cjs'` fails with "Named export 'd' not found". Use the default import and destructure.
- **Changed since 22**: the namespace of an imported CommonJS module also has a `'module.exports'` key (verified: `Object.keys(ns)` is `['a', 'default', 'module.exports']` for `exports.a = 1` on 24, `['a', 'default']` on 22). Code that enumerates the namespace must expect it.

##### `require(esm)` (stable since 24.15.0)

```js
// main.cjs
const lib = require('./lib.mjs');     // works without flags on 24 (and 22.12+)
lib.default();                        // default export
lib.x;                                // named exports
// lib.__esModule === true; keys: ['__esModule', 'default', 'x']
```

- Only **synchronous** ES module graphs can be required: a graph with top-level `await` throws `ERR_REQUIRE_ASYNC_MODULE` (verified). `--experimental-print-required-tla` shows where.
- An ES module can control what `require()` returns by exporting a name `'module.exports'`: `export { value as 'module.exports' }` (verified: `require()` returns `value`).
- Packages can ship one ESM build to both worlds; the `"module-sync"` export condition selects an ESM file for both `import` and `require` when the runtime supports `require(esm)`.
- `process.features.require_module` is `true` on 24; `--no-require-module` (24.15.0) disables the feature.
- Libraries that still support Node < 20.19/22.12 consumers must keep dual CJS/ESM builds; for a Node 24-only codebase prefer ESM-only packages.

##### `package.json` fields

```json
{
  "name": "@acme/orders",
  "type": "module",
  "engines": { "node": ">=24.16" },
  "exports": {
    ".": { "types": "./dist/index.d.ts", "default": "./dist/index.js" },
    "./package.json": "./package.json"
  },
  "imports": {
    "#/*": "./src/*"
  }
}
```

- `"exports"` encapsulates the package: deep imports not listed are `ERR_PACKAGE_PATH_NOT_EXPORTED`. Put `"types"` first in each condition object, `"default"` last.
- `"imports"` subpaths that start with `#/` (as above) are **New in 24.14.0**; on 22 `import '#/lib/x.js'` throws `ERR_INVALID_MODULE_SPECIFIER` (verified). Use `"#src/*"` style names if 22 must be supported.
- Trailing-slash pattern keys (`"./features/"`) and double slashes in targets are deprecated (DEP0155, DEP0166).

##### Module customization hooks

- `module.registerHooks({ resolve, load })` (synchronous, in-thread, works for `require` and `import`; release candidate on 24) is the supported API. `module.register()` (async, off-thread hooks) is documentation-deprecated in 24.15.0 (DEP0205) and runtime-deprecated in 26 — do not add new uses.

```js
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
hooks.deregister();
```

##### Compile cache

`module.enableCompileCache()` / `NODE_COMPILE_CACHE=dir` (stable since 24.15.0) caches V8 code for faster startup of CLIs. `NODE_COMPILE_CACHE_PORTABLE=1` (24.12.0) makes the cache relocatable. Never point it at a world-writable shared directory.

***

#### TypeScript in Node 24 (type stripping)

Node 24 runs `.ts`, `.mts` and `.cts` files directly by **erasing** type annotations (stable since 24.12.0; no warning since 24.3.0). There is no type checking at runtime — run `tsc --noEmit` in CI.

```ts
// app.ts  (package.json has "type": "module")
import type { Server } from 'node:http';          // type-only import: required
import { helper } from './helper.ts';             // .ts extension: required
interface User { id: number; name: string }
function greet<T extends User>(x: T): string { return `hi ${x.name}`; }
let s: Server | undefined;
console.log(greet({ id: 1, name: 'a' }), helper(), s);
```

Verified rules on 24.22.1:

| Construct | Result |
|---|---|
| Annotations, interfaces, type aliases, generics, `as`, `satisfies`, `declare`, type-only namespaces | Stripped, runs |
| `enum`, `namespace` with values, parameter properties (`constructor(private x)`), `import x = require()` ("import equals") | `SyntaxError [ERR_UNSUPPORTED_TYPESCRIPT_SYNTAX]` (unless `--experimental-transform-types`, experimental, removed in 26) |
| Legacy angle-bracket assertions `<number>x` | `SyntaxError` |
| `import { User } from './types.ts'` where `User` is only a type | Runtime `SyntaxError: ... does not provide an export named 'User'` — use `import type` |
| `import './helper'` without extension | `ERR_MODULE_NOT_FOUND` |
| `tsconfig.json` `paths` aliases | Ignored (`ERR_MODULE_NOT_FOUND`) — use package `"imports"` |
| `.ts` files under `node_modules` | `ERR_UNSUPPORTED_NODE_MODULES_TYPE_STRIPPING` — publish compiled JS |
| `.tsx` | Unsupported |
| Decorators | Syntax error (not stripped, not supported) |

- Module format follows the same rules as JavaScript: `.ts` uses the nearest `package.json` `"type"`, `.mts` is ESM, `.cts` is CommonJS (and `require('./x.ts')` needs the extension too).
- `node -e`/`--eval` accepts TypeScript; `--input-type=module-typescript` selects ESM. The REPL and `node --check` do not support TypeScript.
- `--no-strip-types` disables the feature (then `.ts` is `ERR_UNKNOWN_FILE_EXTENSION`); `process.features.typescript` is `'strip'`.
- Recommended `tsconfig.json` for code that Node executes directly (TypeScript 5.8+ for `erasableSyntaxOnly`; verified with `tsc` 5.9):

```json
{
  "compilerOptions": {
    "target": "esnext",
    "module": "nodenext",
    "noEmit": true,
    "allowImportingTsExtensions": true,
    "rewriteRelativeImportExtensions": true,
    "erasableSyntaxOnly": true,
    "verbatimModuleSyntax": true,
    "strict": true,
    "types": ["node"]
  }
}
```

  `erasableSyntaxOnly` makes `tsc` reject enums/namespaces/parameter properties (`TS1294`), `verbatimModuleSyntax` forces `import type` (`TS1484`). Use `rewriteRelativeImportExtensions` (instead of `noEmit`) when you also emit `.js` for publishing. TypeScript 5.9 adds `"module": "node20"` (rejected by 5.8) as a stable alternative to `nodenext`.
- Install `@types/node@24`. Newer type packages describe APIs that 24 does not have.
- Type stripping replaces types with whitespace, so stack-trace line/column numbers match the source without source maps.

***

#### File system, paths, buffers and streams

##### `node:fs`

```js
import { readFile, writeFile, rename, rm, mkdir, glob, opendir } from 'node:fs/promises';
import { constants } from 'node:fs';

const text = await readFile(path, { encoding: 'utf8', signal });   // async, cancellable
await mkdir(dir, { recursive: true });                               // no EEXIST race
await rm(dir, { recursive: true, force: true });                     // not fs.rmdir({ recursive })

// Atomic replace: write a temp file in the same directory, then rename
await writeFile(`${target}.tmp`, data, { mode: 0o600 });
await rename(`${target}.tmp`, target);

for await (const file of glob('src/**/*.js', { exclude: ['**/node_modules/**'] })) { /* ... */ }

await using dir = await opendir(root);         // Dir is disposable on 24.1+
for await (const entry of dir) console.log(entry.parentPath, entry.name);
```

- **BAD**: `fs.existsSync(p)` then `fs.readFileSync(p)` (time-of-check/time-of-use race). **GOOD**: just open/read and handle `ENOENT`. On 24.15+ `fsPromises.stat(p, { throwIfNoEntry: false })` returns `undefined` instead of throwing (the sync form has had it for years).
- **BAD**: `fs.access(p, fs.F_OK)` — `fs.F_OK` is runtime-deprecated on 24 (removed in 25). **GOOD**: `fs.constants.F_OK`.
- **BAD**: `fs.rmdir(dir, { recursive: true })` (DEP0147 warning). **GOOD**: `fs.rm(dir, { recursive: true, force: true })`.
- `fs.truncate()` with a file descriptor throws on 24 — use `fs.ftruncate(fd)`. `dirent.path` is gone — use `dirent.parentPath`.
- `fs.glob()`/`fsPromises.glob()`/`fs.globSync()` are stable since 24.0 (experimental on 22). `exclude` accepts a function or glob patterns.
- Close what you open: a `FileHandle` closed by garbage collection prints `Warning: Closing file descriptor N on garbage collection` plus DEP0137 (verified; DEP0137 moved to End-of-Life in 25, where a `FileHandle` closed by GC throws and crashes the process — verified on 25.9.0), and letting a `Dir` close on GC is documentation-deprecated (DEP0200). Use `await using`.
- `fs.watch` is platform-dependent (event types and filenames differ between Linux, macOS and Windows, and events can be duplicated or coalesced); 24.14 adds an `ignore` option. Prefer a tested watcher library for production file watching.
- `fsPromises.mkdtempDisposable(prefix)` / `fs.mkdtempDisposableSync()` (24.4.0) return `{ path, remove(), [Symbol.asyncDispose] }` — ideal for tests.
- `fs.Utf8Stream` (24.6.0, port of SonicBoom) is a fast synchronous-or-async UTF-8 file writer intended for loggers.

##### Paths and URLs

```js
import path from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';

const here = import.meta.dirname;                        // ESM replacement for __dirname
const cfg = path.join(here, 'config', 'app.json');
const asUrl = pathToFileURL(cfg);                        // for import() of computed paths
const back = fileURLToPath(new URL('./data.json', import.meta.url));
```

- `import()` of an absolute Windows path (`C:\...`) fails because it is parsed as a URL with scheme `c:`; always convert with `pathToFileURL()` (Windows behaviour, not verified here).
- `path.join()` does not stop at `..` or absolute segments; never use it alone to confine user input (see Security).
- `path.matchesGlob(path, pattern)` is available for glob matching without the file system.

##### Buffers and binary data

- `Buffer.alloc(n)` is zero-filled; `Buffer.allocUnsafe(n)` returns **uninitialized** memory that can contain old secrets. Only use `allocUnsafe` when every byte is overwritten before use. (24.0.0–24.11.0 accidentally zero-filled; 24.11.1+ is back to uninitialized — do not rely on either behaviour.)
- `new Buffer()` / `Buffer()` is runtime-deprecated (DEP0005); `SlowBuffer` is runtime-deprecated on 24 (DEP0030) and removed in 25.
- `buf.slice()` is documentation-deprecated (DEP0158) because it differs from `TypedArray.prototype.slice()`; use `buf.subarray()` — and remember both **share memory** with the original (writing to the view mutates the source, verified).
- The default `Buffer.poolSize` is 64 KiB on 24.18+ (8 KiB before, and on 22/25). Small `Buffer.from()`/`allocUnsafe()` results are slices of a shared pool (verified: `Buffer.from('hi').buffer.byteLength` is 65536). Always pass `buf.byteOffset`/`buf.byteLength` when wrapping `buf.buffer` in another view, and copy before transferring: `postMessage(buf, [buf.buffer])` on a pooled buffer throws `DataCloneError` (verified).
- Encodings: `buf.toString('base64url')`, `Buffer.from(str, 'hex')`, `Buffer.byteLength(str, 'utf8')`. `Uint8Array.prototype.toBase64()`/`toHex()` and `Uint8Array.fromBase64()` do **not** exist on 24.
- `TextDecoder` with `{ fatal: true }` rejects invalid UTF-8 instead of inserting U+FFFD.

##### Streams

```js
import { pipeline } from 'node:stream/promises';
import { Readable, Transform } from 'node:stream';
import { createGzip } from 'node:zlib';
import { createWriteStream } from 'node:fs';

const upper = new Transform({
  transform(chunk, _encoding, callback) { callback(null, chunk.toString().toUpperCase()); },
});

await pipeline(
  Readable.from(['a\n', 'b\n']),
  upper,
  createGzip(),
  createWriteStream(outFile),
  { signal: AbortSignal.timeout(5_000) },   // rejects with AbortError and destroys every stage
);

// Async generator stages
await pipeline(
  source,
  async function* (chunks) { for await (const c of chunks) yield c.toString().replace(/\n/g, '|'); },
  async function (chunks) { for await (const c of chunks) console.log(c); },
);
```

- **BAD**: `a.pipe(b).pipe(c)` — errors are not forwarded, streams are not destroyed on failure (fd leaks), no completion signal. **GOOD**: `stream.pipeline()` from `node:stream/promises`; use `stream.finished()` to await one stream.
- Respect backpressure: when `writable.write()` returns `false`, wait for `'drain'` (or use `pipeline`, which does it).
- `for await (const chunk of readable)` is the simplest consumer; `node:stream/consumers` has `text()`, `json()`, `arrayBuffer()`, `blob()`, `buffer()` and (24.14.0) `bytes()`.
- Do not mix object-mode and byte streams by accident: `Readable.from(['a', 'b'])` creates an **object-mode** stream of strings. Converting it with `Readable.toWeb()` and passing it to `new Response()`/`fetch` fails with `TypeError: Received non-Uint8Array chunk` (verified). Yield `Buffer`/`Uint8Array` chunks for web consumers.
- Web streams interoperate: `Readable.toWeb()`, `Readable.fromWeb()`, `Writable.toWeb()`, `Duplex.toWeb()` (on 24.15+ its option is `readableType`; `type` is documentation-deprecated, DEP0201). `CompressionStream`/`DecompressionStream` support `'gzip'`, `'deflate'`, `'deflate-raw'` and `'brotli'` (24.7.0).
- `stream.compose()` is stable (24.19.0). `node:stream/iter` (24.20.0) needs `--experimental-stream-iter` — not for production.
- Default `highWaterMark` is 64 KiB for byte streams and 16 objects for object mode (same as 22).

***

#### HTTP, fetch and networking

##### `fetch` (undici 7)

```js
async function getJson(url, { signal } = {}) {
  const res = await fetch(url, {
    headers: { accept: 'application/json' },
    signal: AbortSignal.any([AbortSignal.timeout(10_000), ...(signal ? [signal] : [])]),
  });
  if (!res.ok) {
    await res.body?.cancel();                   // release the connection
    throw new Error(`GET ${url} failed: HTTP ${res.status}`);
  }
  return res.json();
}
```

- `fetch` does **not** reject on HTTP 4xx/5xx; check `res.ok`/`res.status`.
- `fetch` has **no default timeout** that fits a server: always pass a `signal` (typically `AbortSignal.timeout()`).
- Network failures reject with `TypeError: fetch failed`; the real reason is in `err.cause` (`err.cause.code === 'ECONNREFUSED'`, verified). Log the cause.
- Always consume or cancel the response body (`await res.text()`, `res.body?.cancel()`); unconsumed bodies hold the connection until garbage collection.
- Streaming request bodies need `duplex: 'half'` (verified: without it, `TypeError: RequestInit: duplex option is required when sending a body`).
- Redirects: `fetch` follows up to 20 by default; for requests carrying credentials to user-controlled URLs use `redirect: 'manual'` or `'error'`.
- Ports blocked by the Fetch spec (e.g. 9, 25) fail with `cause.message === 'bad port'`.
- Proxies: `fetch` and `http`/`https` honour `HTTP_PROXY`/`HTTPS_PROXY`/`NO_PROXY` only when `NODE_USE_ENV_PROXY=1` or `--use-env-proxy` is set; `http.setGlobalProxyFromEnv()` (24.14.0) enables it at runtime. Use only trusted proxies.
- `WebSocket` (client) is a stable global; there is no built-in WebSocket server.
- Undici 7 dropped some APIs used by people who imported the `undici` package directly (e.g. `throwOnError`, the old interceptors API); the bundled `fetch` is unaffected. If a project depends on the `undici` npm package, its major version — not Node's — decides the API.

##### `node:http` servers

```js
import http from 'node:http';

const server = http.createServer({
  requestTimeout: 30_000,     // default 300_000
  headersTimeout: 10_000,     // default 60_000
  keepAliveTimeout: 5_000,    // default 5_000
  maxHeaderSize: 16 * 1024,   // default 16 KiB
}, async (req, res) => {
  try {
    const result = await doWork({ signal: req.signal });   // req.signal: 24.16+
    res.writeHead(200, { 'content-type': 'application/json' });
    res.end(JSON.stringify(result));
  } catch (err) {
    if (err.name === 'AbortError') return;                  // client went away
    res.writeHead(500).end();
  }
});
```

- Defaults on 24.22.1 (verified): `requestTimeout` 300 s, `headersTimeout` 60 s, `keepAliveTimeout` 5 s, `maxHeaderSize` 16 KiB, `server.timeout` 0 (no socket inactivity timeout), and `http.globalAgent` uses keep-alive. Behind a load balancer, set `keepAliveTimeout` above the LB's idle timeout to avoid 502s on reused connections.
- Errors thrown inside an `async` request handler are **not** caught by the server — they become unhandled rejections (crash). Wrap the handler body in `try/catch` (or use a framework that does).
- Never write user input into headers without validation; Node rejects CR/LF in header values (`ERR_INVALID_CHAR`), but validate names/values with `http.validateHeaderName()`/`validateHeaderValue()` when building headers dynamically. (`http.isValidHeaderName()` is Node 26 only.)
- `res.writeInformation(statusCode, headers)` (24.18.0) sends arbitrary 1xx responses; `res.writeEarlyHints()` sends 103.
- `server.close()` stops accepting connections but waits for keep-alive sockets; `server.closeIdleConnections()`/`closeAllConnections()` speed up graceful shutdown. `await using server = http.createServer(...)` closes it at scope exit.
- `http.Server`/`http.Agent` constructors without `new` are documentation-deprecated (DEP0195).

##### Graceful shutdown

```js
import http from 'node:http';
import { once } from 'node:events';

const server = http.createServer(handler);
server.listen(port);

let shuttingDown = false;
async function shutdown(signal) {
  if (shuttingDown) return;
  shuttingDown = true;
  const forceExit = setTimeout(() => process.exit(1), 10_000).unref();  // hard deadline
  server.close();                    // stop accepting; in-flight requests complete
  server.closeIdleConnections();     // drop idle keep-alive sockets immediately
  await once(server, 'close');
  await db.close();                  // then pools, queues, telemetry exporters
  clearTimeout(forceExit);
  process.exitCode = 0;              // let the event loop drain; avoid process.exit() here
}

for (const sig of ['SIGTERM', 'SIGINT']) {
  process.once(sig, () => { shutdown(sig).catch((err) => { console.error(err); process.exit(1); }); });
}
```

Verified: after `server.close()` an in-flight request still completes and the `'close'` callback fires afterwards. `process.exit()` in the middle of work truncates pending asynchronous writes (including logs to pipes); set `process.exitCode` and let the process end naturally whenever possible. Container runtimes send `SIGTERM` to PID 1 — run Node behind an init (`docker run --init`, `tini`) or handle signals explicitly, because PID 1 has no default signal handlers.

##### `url`

- Use WHATWG `URL`: `new URL(input, base)`, `URL.canParse(input)`, `URL.parse(input)` (returns `null` instead of throwing). `url.parse()` emits DEP0169 on 24 for application code (code in `node_modules` is exempt, verified) — its non-standard parsing has caused host-confusion vulnerabilities and no CVEs are issued for it.
- `URLPattern` (global, **New in 24**) matches URLs against route patterns:

```js
const route = new URLPattern({ pathname: '/users/:id(\\d+)' });
route.test('https://x.dev/users/42');                       // true
route.exec('https://x.dev/users/42').pathname.groups;       // { id: '42' }
route.test('https://x.dev/users/abc');                      // false
new URLPattern('/api/*', 'https://x.dev').test('https://x.dev/api/v1/items'); // true
```

##### TLS, DNS and sockets

- Never ship `rejectUnauthorized: false` or `NODE_TLS_REJECT_UNAUTHORIZED=0`. To trust a private CA use `ca:`, `NODE_EXTRA_CA_CERTS`, `--use-system-ca` / `NODE_USE_SYSTEM_CA=1` (24.6.0), or `tls.setDefaultCACertificates()` (24.5.0, also on 22) — e.g. `tls.setDefaultCACertificates([...tls.getCACertificates('default'), ...tls.getCACertificates('system')])`.
- OpenSSL 3.5 (since 24.5.0) supports post-quantum key exchange groups; `tlsSocket.getEphemeralKeyInfo()` and the negotiated-group reporting (24.19.0) help diagnose handshakes.
- `dns.lookup()` (used by `http`, `net`, `fetch`) runs `getaddrinfo` on the libuv thread pool (4 threads by default, `UV_THREADPOOL_SIZE`); slow DNS starves `fs`, `crypto` and `zlib` work sharing the pool. Cache or use `dns.resolve*()` for bulk lookups.
- Happy Eyeballs (`autoSelectFamily`) is on by default with a 250 ms attempt timeout on 24 (500 ms on 25+).

***

#### Security

##### Threat model

Node.js trusts the code it runs and the environment it runs in. The permission model, `vm`, workers and SEA are **not** sandboxes for untrusted code.

##### Permission model (`--permission`)

```bash
node --permission \
  --allow-fs-read=/srv/app/ --allow-fs-read=/etc/app/config.json \
  --allow-fs-write=/var/lib/app/ \
  server.js
```

- With `--permission`, `fs`, child processes, worker threads, native addons, WASI, the inspector and OpenSSL STORE loaders are denied unless granted with `--allow-fs-read`, `--allow-fs-write`, `--allow-child-process`, `--allow-worker`, `--allow-addons`, `--allow-wasi`, `--allow-inspector` (24.12.0), `--allow-openssl-store` (24.21.0). Violations throw `ERR_ACCESS_DENIED` with `permission` and `resource` properties (verified).
- **The network is not restricted on 24**: `fetch`, `http`, `net` work under `--permission` (verified); `--allow-net` only exists in 25+.
- The entry point is readable implicitly (24.2.0). `process.permission.has(scope, ref)` checks at runtime; `process.permission.drop(scope, ref)` (24.20.0) irreversibly drops a grant after startup (drop the same reference that was granted).
- `--permission-audit` (24.20.0) only reports violations on `diagnostics_channel` (`node:permission-model:fs`, `...:child`, ...) without denying — use it to discover required grants, never in production as protection.
- Limits: symlinks inside granted paths are followed (even outside the grant); existing file descriptors bypass the checks; `--env-file`, `--openssl-config` and similar startup flags are read before the model is active; `process._debugProcess(pid)` is not gated. `--allow-child-process` lets the code run arbitrary programs outside the model (verified: a spawned `cat /etc/hostname` succeeds while a spawned `node` inherits the permission flags, 24.4.0+, and is still denied); Node prints `SecurityWarning [PERM0002]`/`[PERM0006]` for `--allow-child-process`/`--allow-worker`.
- Use it as defence-in-depth for your own code (limit blast radius of a compromised dependency), in addition to containers/OS users — not instead of them.

##### Child processes

```js
import { execFile } from 'node:child_process';
import { promisify } from 'node:util';
const execFileAsync = promisify(execFile);

// BAD: shell injection -- filename "x; rm -rf ~" runs a second command
exec(`convert ${filename} out.png`);
// BAD on 24: DEP0190 -- args are concatenated, not escaped, when shell: true
spawn('convert', [filename, 'out.png'], { shell: true });

// GOOD: no shell, argument vector, timeout, bounded output
const { stdout } = await execFileAsync('convert', ['--', filename, 'out.png'], {
  timeout: 30_000, maxBuffer: 10 * 1024 * 1024, signal,
});
```

- Pass `--` before user-controlled operands so they cannot be parsed as options.
- On Windows, `.bat`/`.cmd` files cannot be spawned without `shell: true` (Node throws `EINVAL` since the April 2024 fix for CVE-2024-27980); not verified here (Linux-only test environment).
- `shell: ''` is documentation-deprecated (DEP0196).

##### Paths from user input

```js
import path from 'node:path';
const ROOT = path.resolve('/srv/uploads');

// BAD: string-prefix check -- '../uploads-evil/x' joins to '/srv/uploads-evil/x', which starts with ROOT
function badResolve(name) { const p = path.join(ROOT, name); return p.startsWith(ROOT) ? p : null; }

// GOOD: resolve, then check the relative path
function safeResolve(name) {
  const p = path.resolve(ROOT, name);
  const rel = path.relative(ROOT, p);
  if (rel === '' || rel === '..' || rel.startsWith('..' + path.sep) || path.isAbsolute(rel)) return null;
  return p;
}
// verified: '../uploads-evil/x' -> bad: '/srv/uploads-evil/x', good: null; '..foo' -> allowed by both
```

Also resolve symlinks (`fs.realpath`) before the check when the directory can contain links, and reject NUL bytes.

##### Prototype pollution and untrusted objects

```js
const payload = JSON.parse('{"__proto__": {"isAdmin": true}}');   // own key "__proto__"
Object.assign({}, payload).isAdmin;   // true  -- [[Set]] of __proto__ replaced the target's prototype
({ ...payload }).isAdmin;             // undefined -- spread defines an own property instead
naiveDeepMerge({}, payload);          // recursive merges pollute Object.prototype: ({}).isAdmin === true
```

- Validate input with a schema (Zod, Ajv, Valibot, ...) and copy only known keys. Reject `__proto__`, `constructor`, `prototype` keys in merge/set-by-path utilities.
- Use `Map` or `Object.create(null)` for dictionaries keyed by user input.
- `--disable-proto=delete` (removes `Object.prototype.__proto__`) or `--disable-proto=throw` (`ERR_PROTO_ACCESS`) hardens applications (verified); `Object.freeze(Object.prototype)` is another option for apps whose dependencies tolerate it.

##### Injection, deserialization, ReDoS

- SQL: parameters only (`node:sqlite` binds `?`/`:name` parameters and tag-store template values). Never build SQL with string concatenation or `${}` in a plain template string.
- Never pass user input to `eval`, `new Function`, `vm.run*` (the `vm` module is not a security boundary), `require()`/`import()` specifiers, `child_process` commands, or `RegExp` constructors (escape with `RegExp.escape()`).
- Regular expressions run on untrusted input must not backtrack catastrophically (nested quantifiers like `(a+)+`, overlapping alternations); bound input length.
- `JSON.parse` is safe; `v8.deserialize()` and `node-serialize`-style libraries on untrusted data are not.
- Do not render untrusted strings into HTML without escaping; set `Content-Type` explicitly.

##### SSRF

```js
import net from 'node:net';
import http from 'node:http';

const blockList = new net.BlockList();
blockList.addCIDRs(net.BlockList.PRIVATE_RANGES);   // 24.21.0: RFC 1918, loopback, link-local, ULA
blockList.addSubnet('0.0.0.0', 8);                  // not part of PRIVATE_RANGES
blockList.addSubnet('100.64.0.0', 10);              // CGNAT, not part of PRIVATE_RANGES

http.get({ host: userHost, path: '/', blockList }, onResponse)
  .on('error', (err) => { /* err.code === 'ERR_IP_BLOCKED' */ });
```

Verified: the check runs at connect time against the resolved address (so DNS rebinding cannot bypass it), and IPv4-mapped IPv6 (`::ffff:127.0.0.1`) is matched. `fetch` has no `blockList` option. `BlockList.PRIVATE_RANGES`, `addCIDRs()` and the remove methods are **New in 24.21**; on older versions add the CIDRs with `addSubnet()`.

##### Cryptography

```js
import { argon2, randomBytes, timingSafeEqual, randomUUID, randomUUIDv7, hash } from 'node:crypto';
import { promisify } from 'node:util';

// Password hashing (24.7.0+): Argon2id with an explicit cost; async to keep the loop free
const argon2Async = promisify(argon2);
const nonce = randomBytes(16);
const tag = await argon2Async('argon2id', {
  message: password, nonce, parallelism: 4, tagLength: 32, memory: 65536, passes: 3,
});
// store nonce + parameters + tag; verify by recomputing and comparing in constant time
timingSafeEqual(tag, await argon2Async('argon2id', { message: attempt, nonce, parallelism: 4, tagLength: 32, memory: 65536, passes: 3 }));

randomUUID();                       // v4
randomUUIDv7();                     // 24.16.0+: time-ordered, good for DB keys (leaks creation time)
hash('sha256', data, 'base64url');  // one-shot digest (not for passwords)
```

- `crypto.argon2()` is **New in 24.7**; on older runtimes use `crypto.scrypt()` with a random 16-byte salt (or an Argon2 package). Never use plain SHA-x, MD5 or `pbkdf2` with low iteration counts for passwords.
- `timingSafeEqual()` throws `ERR_CRYPTO_TIMING_SAFE_EQUAL_LENGTH` on different lengths (verified); compare fixed-length digests (HMACs), not raw user strings.
- `Math.random()` is not cryptographic — use `crypto.randomInt()`, `randomBytes()`, `randomUUID()`, `crypto.getRandomValues()`.
- Prefer AEAD ciphers (`aes-256-gcm`, `chacha20-poly1305`) with a unique 12-byte IV per message; pass `authTagLength` for GCM (short tags without it are runtime-deprecated, DEP0182).
- SHAKE digests should specify `outputLength` (`hash('shake256', data, { outputEncoding: 'hex', outputLength: 32 })`); the implicit default is documentation-deprecated (DEP0198).
- Post-quantum: ML-KEM (`crypto.encapsulate()`/`decapsulate()`), ML-DSA and SLH-DSA signatures are available on 24.6–24.8+ with OpenSSL 3.5.
- Passing a Web Crypto `CryptoKey` to `node:crypto` functions is documentation-deprecated (DEP0203); convert with `KeyObject.from()`.

##### Secrets and configuration

- Load configuration from the environment (`process.env`, `--env-file=.env`, `--env-file-if-exists`, `process.loadEnvFile()`, `util.parseEnv()`; `--env-file` is stable since 24.10.0). Never commit `.env` files; never log `process.env`.
- `--env-file` values do not override variables already set in the environment.
- Do not put secrets on the command line (visible in `ps`); avoid `NODE_OPTIONS` from untrusted sources (it can `--require`/`--import` code).
- Scrub secrets from error messages and logs; `fetch`/proxy errors may contain URLs with credentials (24.17.0 redacts proxy credentials in tunnel errors — older code paths may not).

##### Dependencies and supply chain (npm 11)

- Commit `package-lock.json` and install with `npm ci` in CI/CD.
- npm 11.19.1 (bundled with 24.22.1) supports `min-release-age=<days>` (only install versions older than N days), `allow-git=none|root`, and an `allowScripts` policy in `package.json` maintained with `npm approve-scripts`; in the current npm 11 releases unreviewed install scripts still run but are reported, and `strict-allow-scripts=true` turns that into an error.
- `--ignore-scripts` now also skips `prepare` (npm 11.0.0).
- Run `npm audit` (npm 11 no longer falls back to the old audit endpoint when the bulk advisory request fails).

***

#### `node:sqlite`

`node:sqlite` is a synchronous, embedded SQLite binding. On 24 it needs no flag, prints no experimental warning (22 does), and is a **release candidate** since 24.15.0 — stable enough for tools, tests, CLIs and small services; keep an abstraction layer in larger applications. It exists only as `node:sqlite` (`require('sqlite')` fails).

```js
import { DatabaseSync } from 'node:sqlite';

using db = new DatabaseSync('app.db', { timeout: 5_000 });    // busy timeout: default is 0
db.exec('CREATE TABLE IF NOT EXISTS users (id INTEGER PRIMARY KEY, name TEXT NOT NULL) STRICT');

const insert = db.prepare('INSERT INTO users (name) VALUES (?)');
insert.run("Robert'); DROP TABLE users;--");                  // bound, not interpolated
const byName = db.prepare('SELECT id, name FROM users WHERE name = :name');
byName.get({ name: 'Robert' });                              // null-prototype row object or undefined

// Tagged-template store (24.9.0+): template values are bound parameters
const sql = db.createTagStore();
const rows = sql.all`SELECT * FROM users WHERE name = ${userInput}`;

// Transactions
db.exec('BEGIN');
try {
  insert.run('a');
  insert.run('b');
  db.exec('COMMIT');
} catch (err) {
  db.exec('ROLLBACK');
  throw err;
}
```

- It is **synchronous**: every query blocks the event loop. Keep queries small and indexed, or run the database in a worker thread for servers with concurrent traffic.
- Binding `undefined` throws `ERR_INVALID_ARG_TYPE` on 24 (verified; Node 26.10 binds it as `NULL`). Convert to `null` explicitly.
- Named parameters can be passed with or without the prefix (`{ name }` or `{ ':name': ... }`); unknown named parameters throw `ERR_INVALID_STATE` unless `allowUnknownNamedParameters` is set (verified).
- Integers outside the safe range (±2⁵³−1) need `readBigInts: true` (database option, 24.4.0) or `stmt.setReadBigInts(true)`; otherwise reading them throws `RangeError [ERR_OUT_OF_RANGE]: Value is too large to be represented as a JavaScript number` (verified) — a latent crash once IDs or counters grow.
- Defensive mode is on by default since 24.14.0 (prevents SQL from corrupting the database file via `PRAGMA writable_schema` etc.); foreign keys are enforced by default; double-quoted string literals are rejected by default.
- `db.setAuthorizer(callback)` (24.10.0) can deny statement classes (e.g. return `constants.SQLITE_DENY` for `SQLITE_DROP_TABLE`) when executing partly untrusted SQL; extensions are disabled unless `allowExtension: true` **and** `enableLoadExtension(true)`, and cannot be loaded under the permission model.
- Close statements and databases (`using`, `stmt.close()` on 24.22+, `db.close()`); `db.createSession()`/`applyChangeset()` provide change tracking; `sqlite.backup()` creates online backups.
- On Node 26.11+ the classes are named `Database`/`Statement`; on 24 use `DatabaseSync`/`StatementSync`.

***

#### Testing with `node:test`

```js
import { describe, it, mock, beforeEach } from 'node:test';
import assert from 'node:assert/strict';

describe('clock-dependent code', () => {
  beforeEach(() => mock.timers.reset());

  it('advances fake timers', async () => {
    mock.timers.enable({ apis: ['setTimeout', 'Date'], now: 0 });
    const p = new Promise((resolve) => setTimeout(() => resolve(Date.now()), 5_000));
    mock.timers.tick(5_000);
    assert.equal(await p, 5_000);
  });
});

describe('mocks', () => {
  it('spies on a method (restored automatically after the test)', (t) => {
    const obj = { add: (a, b) => a + b };
    const spy = t.mock.method(obj, 'add');
    obj.add(1, 2);
    assert.equal(spy.mock.callCount(), 1);
    assert.deepEqual(spy.mock.calls[0].arguments, [1, 2]);
  });

  it('mocks a property', (t) => {
    const cfg = { debug: false };
    t.mock.property(cfg, 'debug', true);
    assert.equal(cfg.debug, true);
  });

  it('snapshots', (t) => {
    t.assert.snapshot({ id: 1, tags: ['a'] });     // update with --test-update-snapshots
  });

  it.expectFailure('documents known bug #123', () => {   // 24.14.0+
    assert.equal(1 + 1, 3);
  });

  it('partial deep equality', () => {
    assert.partialDeepStrictEqual({ a: 1, b: { c: 2, d: 3 } }, { b: { c: 2 } });
  });
});
```

Verified on 24.22.1 (`node --test`): all six tests pass; on 22 `it.expectFailure` does not exist.

- Run with `node --test` (default patterns include `**/*.test.{js,mjs,cjs}`, `**/test/**/*.{js,...}` and — because type stripping is on — the `.ts`/`.mts`/`.cts` equivalents). Pass explicit globs in quotes.
- **Changed in 24**: subtests are awaited automatically. On 22, `t.test('child', async () => ...)` without `await` is cancelled when the parent finishes (`cancelledByParent`); on 24 it runs to completion (verified). Awaiting is still clearer.
- **Changed since 22**: the default reporter is always `spec`; 22 used TAP when stdout was not a TTY. CI scripts parsing TAP must pass `--test-reporter=tap`. Use `--test-reporter=junit --test-reporter-destination=report.xml` (multiple reporters allowed) for CI systems.
- Use `node:assert/strict` (or `t.assert`) — loose `assert.equal` uses `==`.
- `t.mock.*` mocks are restored after the test; top-level `mock.*` must be restored with `mock.restoreAll()`/`mock.reset()`.
- `mock.module()` requires `--experimental-test-module-mocks` (early development). On 24.15+ use the `exports` option (`{ exports: { default, named } }`); `defaultExport`/`namedExports` are deprecated.
- `mock.timers.enable({ apis })` accepts `'setTimeout'`, `'setInterval'`, `'setImmediate'` (the `node:timers/promises` forms are mocked too), `'Date'`, and on 24 also `'scheduler.wait'` and `'AbortSignal.timeout'` (verified on 24.22.1 although the API docs list only the first four; 22 rejects them with `ERR_INVALID_ARG_VALUE`; `AbortSignal.timeout` support is **New in 24.16**). `enable()` without `apis` mocks the default set, which on 24.22.1 includes `AbortSignal.timeout` (verified). Because `setImmediate` is mocked too, awaiting a real `setImmediate` inside a fake-timer test hangs.
- New in 24.x: `--test-randomize` / `--test-random-seed=<n>` (24.16.0) to find order-dependent tests; `--test-rerun-failures=<state file>` (24.7.0); `--test-global-setup=<module>` (24.0, early development); test `tags` + `--experimental-test-tag-filter` (24.19.0); `t.waitFor()`, `assert.register()` for custom assertions; `--test-isolation=process|none` (default `process`).
- Coverage: `--experimental-test-coverage` (still experimental) with `--test-coverage-lines/branches/functions=<threshold>` and `--test-coverage-include/exclude` globs.
- `--test-concurrency`, `--test-timeout`, `--test-name-pattern`, `--test-skip-pattern`, `--test-only` (`only: true` tests) and `--watch` are available. Never commit `it.only`/`describe.only`.
- `--test-force-exit` hides leaked handles; prefer fixing them (close servers, `unref()` timers, `await using`).

***

#### Performance

- Measure first: `node --cpu-prof` (`--cpu-prof-dir`, `--cpu-prof-name` supports `${pid}` on 24.5+), `node --heap-prof`, `--heapsnapshot-near-heap-limit=<n>`, `v8.writeHeapSnapshot()`, `v8.startCpuProfile()` (24.12.0, returns a disposable handle whose `stop()` returns the profile JSON), Chrome DevTools via `--inspect`, `perf_hooks.monitorEventLoopDelay()`, `performance.eventLoopUtilization()`.
- Event loop: offload CPU work to workers; break long synchronous loops; avoid sync APIs in request paths; avoid large `JSON.parse`/`stringify` on the main thread (stream or chunk instead).
- Memory: set `--max-old-space-size` (MiB) or `--max-old-space-size-percentage` to match the container limit; unbounded caches (`Map` without eviction), listeners added per request, closures captured by long-lived timers and global arrays are the usual leaks.
- Prefer `Map`/`Set` for lookups; avoid `delete` on hot objects and changing object shapes in hot paths (it deoptimises V8 inline caches); keep arrays monomorphic.
- String building: template literals or `array.join()`; `Buffer.concat()` once rather than repeated `+=` on buffers.
- Streams over buffering whole payloads (`fs.readFile` of a multi-GB file allocates it all).
- `--max-semi-space-size` can reduce scavenge frequency for allocation-heavy services; benchmark before changing.
- Startup: `module.enableCompileCache()` / `NODE_COMPILE_CACHE`, lazy `import()` of rarely used modules, single executable applications (`--experimental-sea-config`) for CLI distribution.
- Do not micro-optimise based on folklore; Maglev/TurboFan in V8 13.6 optimise idiomatic code (`for...of`, spread, array methods) well. Benchmark with realistic data (`node --test`-style benchmarks, `mitata`, `tinybench`).
- Keep the libuv thread pool in mind (`UV_THREADPOOL_SIZE`, default 4): `fs`, `dns.lookup`, `crypto.pbkdf2/scrypt/argon2`, `zlib` async work share it.

***

#### Observability and diagnostics

```js
import { monitorEventLoopDelay, performance } from 'node:perf_hooks';
import dc from 'node:diagnostics_channel';

{
  using h = monitorEventLoopDelay({ resolution: 10 });  // histogram is disposable (24.2.0)
  h.enable();
  // ... later: h.percentile(99) / 1e6 -> p99 loop delay in ms
}
const { utilization } = performance.eventLoopUtilization();

// Built-in channels: http.client.request.start, http.server.request.start, undici:request:create, ...
dc.subscribe('http.server.request.start', ({ request }) => metrics.increment('http.requests', { method: request.method }));
```

- Log structured JSON with a fast logger (pino, or `fs.Utf8Stream`-based writers); include a request ID from `AsyncLocalStorage`; never log secrets, tokens, full request bodies or `process.env`.
- `console.*` writes synchronously to files/TTYs and asynchronously to pipes on POSIX — do not use `console.log` as a high-volume production logger.
- `diagnostics_channel` (stable) is how APM/OpenTelemetry instrumentations observe `http`, `undici`/`fetch`, `net`, `dns`, `child_process`, `worker_threads` and the permission model; `tracingChannel()` (still experimental) wraps sync/async/promise operations. Prefer channels over monkey-patching.
- `util.styleText()` respects `NO_COLOR`/`FORCE_COLOR` and whether the stream is a TTY by default (verified: plain text when not a TTY or `NO_COLOR=1`); hex colours are supported on 24.16+.
- `process.report.writeReport()` / `--report-on-fatalerror` produce diagnostic JSON reports; `--trace-warnings`, `--trace-deprecation`, `--trace-uncaught`, `--trace-exit` help locate problems.
- `util.getCallSites()` returns structured call sites (with source-map support); `util.getCallSite()` was removed in 24.10.0.
- Name things: `AsyncLocalStorage({ name })`, worker `name` option, `diagnostics_channel` channel names prefixed with your package name.
- `--experimental-network-inspection` (experimental) shows network activity in Chrome DevTools' Network tab (HTTP/2 support added in 24.8.0) — development only.

***

#### Tooling: npm 11, scripts, configuration, linting

##### Runtime selection

- Declare the minimum minor you need: `"engines": { "node": ">=24.16" }` (or `"^24.16.0 || >=26"`); set `engine-strict=true` in `.npmrc` to make `npm install` fail with `EBADENGINE` on an unsupported runtime (verified for the root package with npm 11.19.1).
- Keep `.nvmrc`/`.node-version`, Docker base images (`node:24-*`) and CI matrices in sync; test on the oldest supported 24.x minor and the latest.
- Corepack is still bundled with Node 24 (it is **not** distributed with Node 25+). Pin the package manager with `"packageManager": "pnpm@<version>"` and `corepack enable`, or install the package manager explicitly in CI, so the build keeps working when moving to Node 26.

##### Running scripts

Flags below are documented in the v24 CLI reference.

- `node --run <script>` executes `package.json` scripts with `node_modules/.bin` on `PATH` and much less overhead than `npm run`, but intentionally does **not** run `pre`/`post` scripts. On 24.22.0+ `node --run` without a name lists scripts.
- `node --watch` (`--watch-path=<dir>`, `--watch-preserve-output`, `--watch-kill-signal`) restarts on changes — development only.
- `--env-file=.env` / `--env-file-if-exists=.env` load environment files (stable since 24.10.0; multi-line values supported).
- `--experimental-config-file=node.config.json` (`--experimental-default-config-file` for the default name) centralises flags with a JSON schema; still early development on 24 — never load an untrusted config file.

##### npm 11 behaviour changes

- npm 11 requires Node `^20.17.0 || >=22.9.0`.
- `npm publish` refuses to tag a lower version as `latest`; publishing a pre-release requires an explicit `--tag`.
- `--ignore-scripts` applies to all lifecycle scripts including `prepare`.
- `npm init` asks for `"type"`.

##### Linting and static analysis

- ESLint flat config (`eslint.config.js`) with `languageOptions: { ecmaVersion: 'latest', sourceType: 'module', globals: globals.node }`; `eslint-plugin-n`'s `n/no-unsupported-features/*` rules read `engines` and flag APIs newer than the declared range; `n/no-deprecated-api` flags deprecated core APIs.
- For TypeScript, `@typescript-eslint/no-floating-promises` and `no-misused-promises` catch the async bugs described above.
- Parsers must understand `using` declarations — older tooling (Babel/ESLint/Prettier/bundler versions from before 2025) may fail to parse them; check before adopting `using` in a shared codebase.
- `node --check file.js` performs a syntax-only check (JavaScript only).
- Format with Prettier (or the project's formatter); do not hand-align code.

##### Command-line programs

```js
#!/usr/bin/env node
import { parseArgs, styleText } from 'node:util';
import { text } from 'node:stream/consumers';

export async function main(argv = process.argv.slice(2)) {
  const { values, positionals } = parseArgs({
    args: argv,
    options: {
      output: { type: 'string', short: 'o' },
      verbose: { type: 'boolean', short: 'v', default: false },
      tag: { type: 'string', multiple: true },
    },
    allowPositionals: true,
    strict: true,                       // unknown options throw ERR_PARSE_ARGS_UNKNOWN_OPTION
  });
  const input = positionals[0] === '-' ? await text(process.stdin) : positionals[0];
  // ...
}

if (import.meta.main) {                 // importable for tests without side effects
  main().catch((err) => {
    console.error(styleText('red', `error: ${err.message}`));
    process.exitCode = err.code === 'ERR_PARSE_ARGS_UNKNOWN_OPTION' ? 2 : 1;
  });
}
```

- Set `process.exitCode` instead of calling `process.exit()` so buffered output is flushed.
- Writing to a closed pipe (`mycli | head -1`) with `process.stdout.write()` emits an `EPIPE` `'error'` on `process.stdout`; without a listener the process crashes with "Unhandled 'error' event" and exit code 1 (verified). `console.log()` swallows stream errors, so the same loop written with `console.log` exits 0 but keeps doing useless work. Add `process.stdout.on('error', (err) => { if (err.code === 'EPIPE') process.exit(0); else throw err; })` in CLIs that produce large output.
- Respect backpressure for large outputs (`if (!process.stdout.write(chunk)) await once(process.stdout, 'drain')`).
- Colours via `util.styleText()` are disabled automatically when output is not a TTY or `NO_COLOR` is set.

##### Single executable applications

`node --experimental-sea-config sea-config.json` plus `postject` builds a self-contained binary; 24.7.0 added `execArgv`/`execArgvExtension` to the config and `node:sea` provides `getAsset()`/`getAssetAsBlob()`/`isSea()`. `--build-sea` (one-step build) is Node 25+ only.

***

#### Removed / changed in 24 — migration notes

Upgrading from Node 22 (all items verified on 22.23.4 vs 24.22.1 unless sourced):

1. **V8 13.6**: native addons must be rebuilt (`NODE_MODULE_VERSION` 137); `using` becomes a contextual keyword (existing identifiers named `using` keep working).
2. **`--experimental-permission` → `--permission`**: start scripts with the old flag fail with "bad option".
3. **`url.parse()` warns** (DEP0169): replace with `new URL()`; `url.format(urlObject)` with a WHATWG `URL` is fine, `url.format(string)`/`url.resolve()` warn too.
4. **Runtime deprecation warnings** for `SlowBuffer`, `fs.F_OK`-style constants, zlib/REPL classes without `new`, invalid `fs.existsSync()` arguments, and `args` + `shell: true` in `spawn`/`execFile`. Several of them (`SlowBuffer`, the `fs.F_OK` family) are removed in 25 — fix them now.
5. **Removed**: `tls.createSecurePair()`, `OutgoingMessage#_headers`/`_headerNames`, `fs.truncate(fd)` (throws), `dirent.path`, `timers.enroll/unenroll/active/_unrefActive`, `net._setSimultaneousAccepts()`, `tls.Server#setOptions()`, `crypto.Cipher`/`Decipher` exports, `util.getCallSite()` (24.10), HTTP/2 priority signaling (24.2); `--experimental-default-type`.
6. **`clearImmediate(timeout)` no longer cancels a `setTimeout`**: use the matching clear function.
7. **Test runner**: subtests are auto-awaited; default reporter is `spec` even in CI (TAP consumers need `--test-reporter=tap`).
8. **`AsyncLocalStorage`** is backed by `AsyncContextFrame`; if an APM agent or custom context propagation misbehaves, test with `--no-async-context-frame` and report the difference.
9. **CommonJS namespace objects** contain a `'module.exports'` key (since 23).
10. **Type stripping** is stable and silent; `.ts` files run directly. Projects that intentionally ran `.ts` through a loader (`tsx`, `ts-node`) keep working, but erasable-only code can drop the loader.
11. **`node:sqlite`** no longer prints an experimental warning; defensive mode is on by default since 24.14.0 (code that writes `sqlite_schema` directly breaks).
12. **`Buffer.poolSize`** is 64 KiB on 24.18+; code that assumed small buffers do not share a large `ArrayBuffer` (e.g. transferring `buf.buffer`) must copy first.
13. **npm 11**: stricter `publish` tagging, `--ignore-scripts` covers `prepare`, `npm hook` removed.
14. **Platform support** (sourced): macOS 13.5+, Xcode 16.1+, ClangCL required on Windows (MSVC removed), 32-bit armv7 downgraded to experimental, 32-bit ppc/s390 removed.
15. **Undici 7** under `fetch`: the bundled behaviour is spec-compliant fetch; direct users of the `undici` package must follow its 7.x migration notes.

Upgrading from 24 to 26 later (do not apply on 24): `module.register()` becomes a runtime deprecation, `--experimental-transform-types` and the `_stream_*` modules are removed, `http.ServerResponse#writeHeader()` is removed, `SlowBuffer`/`fs.F_OK`/`assert.CallTracker` are already gone in 25, Corepack is no longer bundled, `localStorage` behaves differently, and V8 14.6 adds `Temporal`.

***

#### Production / review checklist

##### Version compatibility

- [ ] `engines.node` matches the APIs used, including the 24.x minor (e.g. `>=24.16` for `crypto.randomUUIDv7()`, `>=24.22` for `util.debounce`/`throttle`)
- [ ] No Node 25/26 APIs: `Uint8Array.fromBase64`/`toBase64`/`toHex`, `Temporal`, `Map#getOrInsert`, `Iterator.concat`, `ErrorEvent`, default `localStorage`, `--allow-net`, `node:ffi`, `zlib.ZipFile`, `fs.openAsBlobSync`, `http.isValidHeaderName`, sqlite `Database`/`Statement` names
- [ ] If Node 22 must still be supported: no `using`/`await using`, `Float16Array`, `RegExp.escape`, `Error.isError`, `Promise.try`, RegExp modifiers, duplicate named groups, `URLPattern` global, `#/` imports, `navigator.locks`, `it.expectFailure`, AsyncLocalStorage `defaultValue`
- [ ] `@types/node` major matches the runtime (24); TypeScript ≥ 5.8 with `erasableSyntaxOnly` when `.ts` runs directly
- [ ] CI runs with `--throw-deprecation` (or reviews warnings); no `url.parse`, `SlowBuffer`, `fs.F_OK`, `new Buffer()`, `util.is*`, `fs.rmdir({ recursive })`

##### Correctness

- [ ] Every promise is awaited, returned, or explicitly handled; no `forEach(async ...)`; `return await` inside `try`
- [ ] Errors are `Error` instances with `cause`; branching uses `code`/`name`/`instanceof`/`Error.isError`, not messages
- [ ] Every `EventEmitter`/stream/socket/child/worker has an `'error'` handler or is managed by `pipeline`/`once`
- [ ] Resources are released on all paths (`using`/`await using`, `try/finally`); servers, workers, file handles and `Dir`s are closed
- [ ] `fetch` checks `res.ok`, has a timeout signal, consumes/cancels bodies, logs `err.cause`
- [ ] Abort errors are identified by `err.name` (`AbortError` vs `TimeoutError`)
- [ ] Streams composed with `stream/promises` `pipeline`; backpressure respected; byte vs object mode correct
- [ ] `sort()` has a comparator for numbers; money/IDs are not floats; dates use explicit time zones
- [ ] ESM imports use extensions and `node:` prefixes; JSON imports use `with { type: 'json' }`; no reliance on undetectable CJS named exports
- [ ] No `require()` of ES modules with top-level `await`
- [ ] TypeScript files use `import type` for types and only erasable syntax

##### Security

- [ ] No `exec`/`shell: true` with user input; `execFile` with argument arrays, `--`, timeouts and `maxBuffer`
- [ ] User-supplied paths confined with `path.resolve` + `path.relative` (and `realpath` for symlinks)
- [ ] No `eval`/`new Function`/`vm` on untrusted input; `vm` not used as a sandbox
- [ ] Untrusted objects validated by schema; no recursive merge without `__proto__`/`constructor` guards; `Map`/`Object.create(null)` for user-keyed dictionaries
- [ ] SQL is parameterized (prepared statements or `createTagStore()`), never string-built
- [ ] Outbound requests to user-controlled hosts protected against SSRF (`net.BlockList` on `http`/`https`, redirect policy)
- [ ] Passwords hashed with Argon2id/scrypt; secrets compared with `timingSafeEqual` on equal-length digests; `crypto.random*` for tokens
- [ ] TLS verification never disabled; private CAs configured via `ca`/`NODE_EXTRA_CA_CERTS`/system CA
- [ ] `--permission` grants are minimal; `--allow-child-process`/`--allow-worker` justified; network is not assumed to be restricted
- [ ] Secrets not logged, not in argv, `.env` not committed; `npm ci` with a lockfile; install-script policy reviewed

##### Performance and operations

- [ ] No sync I/O or heavy CPU work on request paths; CPU work in a worker pool
- [ ] Concurrency bounded for fan-out (`Promise.all` over unbounded arrays)
- [ ] Timers that should not keep the process alive are `unref()`ed; long delays clamped below 2³¹−1 ms
- [ ] HTTP server timeouts set deliberately (`requestTimeout`, `headersTimeout`, `keepAliveTimeout` vs load balancer)
- [ ] `--max-old-space-size` (or percentage) aligned with container memory
- [ ] Graceful shutdown on `SIGTERM`: stop accepting, drain, close DB/pools, exit with `process.exitCode`
- [ ] Structured logs with request IDs (`AsyncLocalStorage`); event-loop delay and ELU monitored
- [ ] Unhandled rejections still crash the process (no `--unhandled-rejections=warn/none`)

##### Testing

- [ ] Tests run with `node --test` (or the project's runner) on the oldest supported 24.x minor
- [ ] No `.only`; snapshots reviewed; `--test-randomize` used periodically to catch order dependence
- [ ] Mocks restored (`t.mock` or `mock.restoreAll()`); fake timers reset
- [ ] Experimental flags (`--experimental-test-module-mocks`, coverage) pinned and documented
