### Citations: JavaScript on Node.js 24 (Krypton LTS): The Complete Production Developer Guide

Sources for `javascript-node-24.md`. The guide carries no footnote markers; each entry lists where it was cited (section path, block, and the words just before the citation). Not sent to the model; kept for maintainers.

[^1]: Node.js 24.0.0 release notes (V8 13.6, npm 11, AsyncContextFrame, URLPattern, permission flag, test runner, undici 7, deprecations, platform changes). https://nodejs.org/en/blog/release/v24.0.0
    - JavaScript on Node.js 24 (Krypton LTS): The Complete Production Developer Guide — quote block: "**24.22.1 (2026-10-21, security release)**."
    - What's new in Node.js 24 (vs. Node 22) › Language (V8 12.4 → 13.6) — paragraph 1: "…reviver `context.source` were already in Node 22 and remain available."
    - What's new in Node.js 24 (vs. Node 22) › Runtime and APIs — bullet 1: "`CloseEvent` is also global (since 23.0)."
    - New in 24: Explicit resource management — paragraph 1: "On Node 22 `using x = ...` is a `SyntaxError`."
    - Removed / changed in 24 — migration notes — bullet 14: "…(MSVC removed), 32-bit armv7 downgraded to experimental, 32-bit ppc/s390 removed."
[^2]: Node.js release schedule (schedule.json: 24 LTS 2025-10-28, maintenance 2026-10-20, end 2028-04-30; 22 end 2027-04-30; 25 end 2026-06-01; 26 LTS 2026-10-28; 27 alpha). https://github.com/nodejs/Release#release-schedule
    - JavaScript on Node.js 24 (Krypton LTS): The Complete Production Developer Guide — quote block: "**24.22.1 (2026-10-21, security release)**."
    - JavaScript on Node.js 24 (Krypton LTS): The Complete Production Developer Guide — quote block: "…alpha channel on 2026-10-28, and 27.0.0 is planned for 2027-04-22."
[^3]: Node.js 24.22.1 (LTS) security release, 2026-10-21. https://nodejs.org/en/blog/release/v24.22.1
    - JavaScript on Node.js 24 (Krypton LTS): The Complete Production Developer Guide — quote block: "**24.22.1 (2026-10-21, security release)**."
[^4]: Node.js 24 changelog (24.0.0–24.22.1, notable changes per minor). https://github.com/nodejs/node/blob/main/doc/changelogs/CHANGELOG_V24.md
    - What's new in Node.js 24 (vs. Node 22) › Language (V8 12.4 → 13.6) — paragraph 1: "…reviver `context.source` were already in Node 22 and remain available."
    - What's new in Node.js 24 (vs. Node 22) › Runtime and APIs — bullet 14: "…restored the documented (uninitialized) behaviour — never rely on either."
    - New in 24: Other ECMAScript additions › WebAssembly — bullet 3: "…and on 22.23.4; the integration still follows an evolving proposal."
[^5]: Node.js 25 changelog (25.0.0: V8 14.1, Uint8Array base64/hex, Web Storage, ErrorEvent, --allow-net, removals). https://github.com/nodejs/node/blob/main/doc/changelogs/CHANGELOG_V25.md
    - Removed / changed in 24 — migration notes — paragraph 2: "…longer bundled, `localStorage` behaves differently, and V8 14.6 adds `Temporal`."
[^6]: Node.js 26 changelog (26.0.0: Temporal, V8 14.6, upsert, Iterator.concat, undici 8, removals; 26.x additions). https://github.com/nodejs/node/blob/main/doc/changelogs/CHANGELOG_V26.md
    - Removed / changed in 24 — migration notes — paragraph 2: "…longer bundled, `localStorage` behaves differently, and V8 14.6 adds `Temporal`."
[^7]: Node.js 23 changelog (require(esm) unflagged, Promise.try/V8 12.9, type stripping unflagged in 23.6, util.is* EOL). https://github.com/nodejs/node/blob/main/doc/changelogs/CHANGELOG_V23.md
    - What's new in Node.js 24 (vs. Node 22) › Language (V8 12.4 → 13.6) — paragraph 1: "…reviver `context.source` were already in Node 22 and remain available."
[^8]: Node.js v24 documentation — Deprecated APIs (DEP0005–DEP0209). https://nodejs.org/docs/latest-v24.x/api/deprecations.html
    - Deprecated and removed APIs — paragraph 1: "…several long-standing documentation deprecations into runtime warnings and removes others."
    - New in 24: Explicit resource management › Review guidance — bullet 6: "Keep `AbortSignal` for cancelling operations and timeouts."
    - HTTP, fetch and networking › `url` — bullet 1: "…caused host-confusion vulnerabilities and no CVEs are issued for it."
[^9]: TC39 proposal: Explicit Resource Management; V8 feature explainer. https://github.com/tc39/proposal-explicit-resource-management , https://v8.dev/features/explicit-resource-management
    - New in 24: Explicit resource management — paragraph 1: "On Node 22 `using x = ...` is a `SyntaxError`."
[^10]: TC39 proposal: RegExp.escape. https://github.com/tc39/proposal-regex-escaping
    - New in 24: Other ECMAScript additions › `RegExp.escape()` (24.0) — paragraph 1: "`RegExp.escape()`."
[^11]: TC39 proposal: Error.isError. https://github.com/tc39/proposal-is-error
    - New in 24: Other ECMAScript additions › `Error.isError()` (24.0) — paragraph 1: "`util.types.isNativeError()` is documentation-deprecated in its favour (DEP0197)."
[^12]: TC39 proposal: Float16 on TypedArrays, DataView, Math.f16round. https://github.com/tc39/proposal-float16array
    - New in 24: Other ECMAScript additions › `Float16Array` and `Math.f16round()` (24.0) — paragraph 1: "`util.types.isFloat16Array()` and `assert.deepStrictEqual` understand it."
[^13]: TC39 proposal: Promise.try. https://github.com/tc39/proposal-promise-try
    - New in 24: Other ECMAScript additions › `Promise.try()` (23.0) — paragraph 1: "…=> resolve(fn()))` and `Promise.resolve().then(fn)` (which delays `fn` by a microtask)."
[^14]: TC39 proposals: RegExp modifiers; duplicate named capturing groups. https://github.com/tc39/proposal-regexp-modifiers , https://github.com/tc39/proposal-duplicate-named-capturing-groups
    - New in 24: Other ECMAScript additions › RegExp modifiers and duplicate named groups (23.0) — paragraph 1: "Supported modifier flags are `i`, `m` and `s`."
[^15]: Node.js v24 documentation — Modules: CommonJS (require(esm)), node:module (registerHooks, register deprecation, compile cache). https://nodejs.org/docs/latest-v24.x/api/modules.html , https://nodejs.org/docs/latest-v24.x/api/module.html
    - What's new in Node.js 24 (vs. Node 22) › Runtime and APIs — bullet 4: "`module.register()` is documentation-deprecated (DEP0205, 24.15.0) in favour of `module.registerHooks()`."
    - Modules: ESM, CommonJS and interop › Module customization hooks — bullet 1: "…and runtime-deprecated in 26 — do not add new uses."
[^16]: Node.js v24 documentation — ECMAScript modules and Packages (imports/exports, module-sync, #/ imports). https://nodejs.org/docs/latest-v24.x/api/esm.html , https://nodejs.org/docs/latest-v24.x/api/packages.html
    - What's new in Node.js 24 (vs. Node 22) › Runtime and APIs — bullet 4: "`module.register()` is documentation-deprecated (DEP0205, 24.15.0) in favour of `module.registerHooks()`."
[^17]: Node.js v24 documentation — Modules: TypeScript (type stripping). https://nodejs.org/docs/latest-v24.x/api/typescript.html
    - What's new in Node.js 24 (vs. Node 22) › Runtime and APIs — bullet 5: "…but is still documented as experimental on the 22 line."
    - TypeScript in Node 24 (type stripping) — paragraph 1: "…type checking at runtime — run `tsc --noEmit` in CI."
[^18]: Node.js v24 documentation — Permissions (permission model, audit mode, constraints). https://nodejs.org/docs/latest-v24.x/api/permissions.html
    - What's new in Node.js 24 (vs. Node 22) › Runtime and APIs — bullet 3: "…readable (24.2), `--allow-inspector` (24.12), `--permission-audit` (24.20), `process.permission.drop()` (24.20), `--allow-openssl-store` (24.21)."
    - Security › Permission model (`--permission`) — bullet 1: "Violations throw `ERR_ACCESS_DENIED` with `permission` and `resource` properties (verified)."
[^19]: Node.js v24 documentation — Asynchronous context tracking (AsyncLocalStorage, withScope, RunScope) and CLI `--no-async-context-frame`. https://nodejs.org/docs/latest-v24.x/api/async_context.html
    - What's new in Node.js 24 (vs. Node 22) › Runtime and APIs — bullet 2: "…`name` and `defaultValue` (24.0) and `asyncLocalStorage.withScope()` for `using` (24.20.0, experimental)."
    - Asynchronous code and concurrency › `AsyncLocalStorage` — request context (**changed in 24**) — bullet 1: "The docs ask users to report flow differences."
[^20]: Node.js v24 documentation — Test runner. https://nodejs.org/docs/latest-v24.x/api/test.html
    - What's new in Node.js 24 (vs. Node 22) › Runtime and APIs — bullet 7: "…(24.16.0), `--test-rerun-failures` (24.7.0), `--test-global-setup` (24.0), test `tags` with `--experimental-test-tag-filter` (24.19.0)."
    - Testing with `node:test` — paragraph 1: "…all six tests pass; on 22 `it.expectFailure` does not exist."
[^21]: Node.js v24 documentation — SQLite. https://nodejs.org/docs/latest-v24.x/api/sqlite.html
    - What's new in Node.js 24 (vs. Node 22) › Runtime and APIs — bullet 6: "…`setAuthorizer()`, `limits`, `Symbol.dispose` on databases and statements, and `StatementSync.prototype.close()` (24.22.0)."
    - `node:sqlite` — paragraph 1: "It exists only as `node:sqlite` (`require('sqlite')` fails)."
[^22]: Node.js v24 documentation — Command-line API. https://nodejs.org/docs/latest-v24.x/api/cli.html
    - Tooling: npm 11, scripts, configuration, linting › Running scripts — paragraph 1: "Flags below are documented in the v24 CLI reference."
[^23]: Node.js v24 documentation — util (debounce, throttle, markPromiseAsHandled, MIMEType.parse, styleText, getCallSites). https://nodejs.org/docs/latest-v24.x/api/util.html
    - What's new in Node.js 24 (vs. Node 22) › Runtime and APIs — bullet 13: "`util.getCallSite()` was removed in 24.10.0 (use `util.getCallSites()`)."
    - Asynchronous code and concurrency › Promise combinators — paragraph 2: "…keep existing libraries when the project must support earlier versions."
[^24]: Node.js v24 documentation — Crypto (argon2, randomUUIDv7, hash, encapsulate, createMac, parsePKCS12). https://nodejs.org/docs/latest-v24.x/api/crypto.html
    - What's new in Node.js 24 (vs. Node 22) › Runtime and APIs — bullet 10: "…ChaCha20-Poly1305, ML-DSA, ML-KEM, SHA-3, SHAKE, Argon2, KMAC and `SubtleCrypto.supports()` (24.7–24.18)."
[^25]: Node.js v24 documentation — HTTP (server timeouts, req.signal, writeInformation, setGlobalProxyFromEnv). https://nodejs.org/docs/latest-v24.x/api/http.html
    - What's new in Node.js 24 (vs. Node 22) › Runtime and APIs — bullet 11: "HTTP/2 priority signaling was removed (24.2.0, DEP0194)."
[^26]: Node.js v24 documentation — File system (glob, mkdtempDisposable, Utf8Stream, stat options). https://nodejs.org/docs/latest-v24.x/api/fs.html
    - What's new in Node.js 24 (vs. Node 22) › Runtime and APIs — bullet 12: "…promise/callback forms (24.15.0) and `signal` (24.16.0), caller-supplied `readFile()` buffers (24.19.0)."
[^27]: Node.js v24 documentation — Stream and Web Streams. https://nodejs.org/docs/latest-v24.x/api/stream.html , https://nodejs.org/docs/latest-v24.x/api/webstreams.html
    - What's new in Node.js 24 (vs. Node 22) › Runtime and APIs — bullet 15: "…stable (24.19.0), `Blob.prototype.textStream()` (24.19.0), `CompressionStream('brotli')` (24.7.0, also backported to 22)."
[^28]: Node.js v24 documentation — Globals (URLPattern, navigator, WebSocket, fetch). https://nodejs.org/docs/latest-v24.x/api/globals.html
    - Web platform globals on Node 24 — paragraph 1: "Available without imports on 24.22.1 (verified):"
[^29]: undici v7.0.0 release notes and README (body consumption). https://github.com/nodejs/undici/releases/tag/v7.0.0 , https://github.com/nodejs/undici#garbage-collection
    - What's new in Node.js 24 (vs. Node 22) › Runtime and APIs — bullet 8: "…7** powers `fetch`, `WebSocket` and friends (undici 7.30.1 in 24.22.1)."
    - HTTP, fetch and networking › `fetch` (undici 7) — bullet 4: "…res.text()`, `res.body?.cancel()`); unconsumed bodies hold the connection until garbage collection."
    - Removed / changed in 24 — migration notes — bullet 15: "…of the `undici` package must follow its 7.x migration notes."
[^30]: npm v11.0.0 release notes; npm config documentation (min-release-age, allow-git, allow-scripts, strict-allow-scripts) and npm-approve-scripts as bundled with npm 11.19.1. https://github.com/npm/cli/releases/tag/v11.0.0 , https://docs.npmjs.com/cli/v11/using-npm/config
    - Security › Dependencies and supply chain (npm 11) — bullet 2: "…but are reported, and `strict-allow-scripts=true` turns that into an error."
    - Tooling: npm 11, scripts, configuration, linting › npm 11 behaviour changes — bullet 4: "`npm init` asks for `"type"`."
[^31]: TypeScript 5.8 release notes (erasableSyntaxOnly) and 5.9 release notes (--module node20). https://www.typescriptlang.org/docs/handbook/release-notes/typescript-5-8.html , https://www.typescriptlang.org/docs/handbook/release-notes/typescript-5-9.html
    - TypeScript in Node 24 (type stripping) — bullet 4: "…executes directly (TypeScript 5.8+ for `erasableSyntaxOnly`; verified with `tsc` 5.9):"
[^32]: Node.js security policy and threat model. https://github.com/nodejs/node/blob/main/SECURITY.md
    - Security › Threat model — paragraph 1: "…`vm`, workers and SEA are **not** sandboxes for untrusted code."
[^33]: Node.js April 2024 security releases (CVE-2024-27980, Windows batch-file command injection). https://nodejs.org/en/blog/vulnerability/april-2024-security-releases-2
    - Security › Child processes — bullet 2: "…2024 fix for CVE-2024-27980); not verified here (Linux-only test environment)."
[^34]: Node.js 22 changelog (22.18.0: type stripping enabled by default; backports of 23/24 features to the 22 line). https://github.com/nodejs/node/blob/main/doc/changelogs/CHANGELOG_V22.md
    - What's new in Node.js 24 (vs. Node 22) › Runtime and APIs — bullet 5: "…but is still documented as experimental on the 22 line."
