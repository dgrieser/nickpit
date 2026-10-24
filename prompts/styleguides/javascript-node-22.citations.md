### Citations: JavaScript on Node.js 22: The Complete Production Developer Guide

Sources for `javascript-node-22.md`. The guide carries no footnote markers; each entry lists where it was cited (section path, block, and the words just before the citation). Not sent to the model; kept for maintainers.

[^1]: Node.js 22 changelog (22.0.0 – 22.23.4). https://github.com/nodejs/node/blob/main/doc/changelogs/CHANGELOG_V22.md
    - JavaScript on Node.js 22: The Complete Production Developer Guide — quote block: "…— 22.0.0 released 2024-04-24; latest patch **22.23.4 (2026-10-21, security release)**."
    - Overview — paragraph 1: "…memory exhaustion, permission-model bypasses, `http.Agent` response-queue poisoning, DNS NUL-byte injection)."
    - Overview — paragraph 2: "…in **22.21.0**, and OpenSSL 3.5 replaced OpenSSL 3.0 in **22.20.0**."
    - What's New in Node.js 22 (vs. Node 20 / 21) › Runtime, per 22.x minor — bullet 1: "Maglev enabled (disabled again in 22.9)."
    - What's New in Node.js 22 (vs. Node 20 / 21) › Runtime, per 22.x minor — bullet 23: "…and a `TLSSocket` default `'error'` handler was added in 22.22.0."
    - Security › Keep the runtime patched — paragraph 1: "…non-zero-filled memory (CVE-2025-55131, 22.22.0) and a brotli decoder bug (22.23.4)."
[^2]: Node.js Release schedule (schedule.json: 22 "Jod" maintenance 2025-10-21, end 2027-04-30; 24, 26, 27 dates). https://github.com/nodejs/Release
    - JavaScript on Node.js 22: The Complete Production Developer Guide — quote block: "Only security and critical fixes land now."
    - Overview — paragraph 1: "…LTS on 2025-10-21**; it reaches **end of life on 2027-04-30**."
    - Overview — paragraph 1: "…open on 2026-10-28 and 27.0.0 is planned for April 2027."
[^3]: Node.js 22.23.4 (LTS) security release, 2026-10-21. https://nodejs.org/en/blog/release/v22.23.4
    - JavaScript on Node.js 22: The Complete Production Developer Guide — quote block: "…— 22.0.0 released 2024-04-24; latest patch **22.23.4 (2026-10-21, security release)**."
[^4]: Node.js 22 is now available! (release announcement). https://nodejs.org/en/blog/announcements/v22-release-announce
    - What's New in Node.js 22 (vs. Node 20 / 21) › Runtime, per 22.x minor — bullet 1: "Maglev enabled (disabled again in 22.9)."
[^5]: Node.js distribution index (release dates, bundled V8/npm/OpenSSL versions). https://nodejs.org/dist/index.json
    - Overview — paragraph 1: "…open on 2026-10-28 and 27.0.0 is planned for April 2027."
[^6]: Modules: CommonJS — Loading ECMAScript modules using `require()`. https://nodejs.org/docs/latest-v22.x/api/modules.html#loading-ecmascript-modules-using-require
    - Modules: ESM, CommonJS and Interop › New in 22: `require(esm)` (unflagged in 22.12.0) — paragraph 1: "Rules a reviewer must check:"
[^7]: Modules: Packages (`type`, `exports`, `imports`, `module-sync`, syntax detection). https://nodejs.org/docs/latest-v22.x/api/packages.html
    - Modules: ESM, CommonJS and Interop › How Node 22 decides the module system — paragraph 1: "Node 22 packages and ESM documentation."
[^8]: Modules: ECMAScript modules (import attributes, `import.meta`). https://nodejs.org/docs/latest-v22.x/api/esm.html
    - Modules: ESM, CommonJS and Interop › How Node 22 decides the module system — paragraph 1: "Node 22 packages and ESM documentation."
[^9]: Modules: TypeScript (type stripping, unsupported syntax, tsconfig). https://nodejs.org/docs/latest-v22.x/api/typescript.html
    - TypeScript in Node 22: Type Stripping › Timeline (verify against the pinned minor) — bullet 4: "Stability: 1.2 (release candidate) in 22.20+."
[^10]: SQLite (`node:sqlite`). https://nodejs.org/docs/latest-v22.x/api/sqlite.html
    - `node:sqlite` — Built-in SQLite (New in 22) — paragraph 1: "…and small services, but each query blocks the event loop."
[^11]: Permissions (permission model, limitations). https://nodejs.org/docs/latest-v22.x/api/permissions.html
    - Security › New in 22: the permission model is stable (22.13) — bullet 4: "…Node threat model treats the code you run as trusted."
[^12]: Command-line API (`--run`, `--watch`, `--env-file`, `--max-old-space-size-percentage`, `--use-env-proxy`, `--trace-env`). https://nodejs.org/docs/latest-v22.x/api/cli.html
    - Tooling and Project Hygiene › Bundled tools and scripts — bullet 2: "**`node --run <script>`** (stable 22.10)"
[^13]: Test runner (`node:test`). https://nodejs.org/docs/latest-v22.x/api/test.html
    - Testing with `node:test` › Mocks, timers and snapshots — bullet 1: "Snapshot testing:"
[^14]: Deprecated APIs (DEP0005 … DEP0199). https://nodejs.org/docs/latest-v22.x/api/deprecations.html
    - Do Not Use or Recommend (Not Available in Node.js 22) › Older APIs: removed or deprecated in 22 — paragraph 1: "…print a `DeprecationWarning` with the DEP code on first use."
[^15]: Node.js 23 changelog. https://github.com/nodejs/node/blob/main/doc/changelogs/CHANGELOG_V23.md
    - Do Not Use or Recommend (Not Available in Node.js 22) › Node.js APIs missing from 22.x — bullet 4: "`sqlite` binding `undefined` as `NULL` (on 22 it throws `ERR_INVALID_ARG_TYPE`)."
[^16]: Node.js 24 changelog. https://github.com/nodejs/node/blob/main/doc/changelogs/CHANGELOG_V24.md
    - Do Not Use or Recommend (Not Available in Node.js 22) › Node.js APIs missing from 22.x — bullet 4: "`sqlite` binding `undefined` as `NULL` (on 22 it throws `ERR_INVALID_ARG_TYPE`)."
[^17]: Node.js 25 changelog. https://github.com/nodejs/node/blob/main/doc/changelogs/CHANGELOG_V25.md
    - Do Not Use or Recommend (Not Available in Node.js 22) › Node.js APIs missing from 22.x — bullet 4: "`sqlite` binding `undefined` as `NULL` (on 22 it throws `ERR_INVALID_ARG_TYPE`)."
[^18]: Node.js 26 changelog. https://github.com/nodejs/node/blob/main/doc/changelogs/CHANGELOG_V26.md
    - Do Not Use or Recommend (Not Available in Node.js 22) › Node.js APIs missing from 22.x — bullet 4: "`sqlite` binding `undefined` as `NULL` (on 22 it throws `ERR_INVALID_ARG_TYPE`)."
[^19]: undici v6 documentation (Client defaults; consuming response bodies). https://github.com/nodejs/undici/blob/v6.x/docs/docs/api/Client.md , https://github.com/nodejs/undici/blob/v6.x/README.md
    - HTTP, `fetch` and WebSocket › `fetch` (undici 6.x) — bullet 3: "…to connect and 300 s each for headers and body."
    - HTTP, `fetch` and WebSocket › `fetch` (undici 6.x) — bullet 4: "…promptly, which reduces connection reuse and can stall the pool."
[^20]: Modules: `node:module` API (`registerHooks`, `register`, `enableCompileCache`, `findPackageJSON`). https://nodejs.org/docs/latest-v22.x/api/module.html
    - Modules: ESM, CommonJS and Interop › Module customization hooks — bullet 1: "`module.registerHooks()` (22.15, Stability 1.1)"
[^21]: HTTP (`server.keepAliveTimeoutBuffer`, `shouldUpgradeCallback`, built-in proxy support). https://nodejs.org/docs/latest-v22.x/api/http.html
    - HTTP, `fetch` and WebSocket › `node:http` server defaults and hardening — bullet 8: "`http.Agent({ proxyEnv: process.env })` (22.21)"
[^22]: File system (`fs.glob`, `fs.cp`, `Dir`). https://nodejs.org/docs/latest-v22.x/api/fs.html
    - File System › New in 22: `fs.glob()` — paragraph 1: "`fs.glob` replaces the `glob`/`fast-glob` packages for simple cases."
[^23]: Stream (default high water mark, `pipeline`, operators, Web Streams adapters). https://nodejs.org/docs/latest-v22.x/api/stream.html
    - Streams › New in 22: default `highWaterMark` is 64 KiB — paragraph 1: "…threshold of byte streams from 16 KiB to **64 KiB**"
[^24]: Global objects (`fetch`, `WebSocket`, `navigator`, `EventSource`, Web Storage). https://nodejs.org/docs/latest-v22.x/api/globals.html
    - Overview — bullet 3: "…— **new in 22** — `WebSocket` are available without imports."
    - HTTP, `fetch` and WebSocket › `fetch` (undici 6.x) — paragraph 1: "`fetch`, `Request`, `Response`, `Headers`, `FormData` and `WebSocket` are globals."
[^25]: Util (`parseArgs`, `styleText`, `getCallSites`, `parseEnv`, `diff`). https://nodejs.org/docs/latest-v22.x/api/util.html
    - Process Lifecycle and CLI Programs › Argument parsing without dependencies — paragraph 1: "`parseArgs` returns strings;"
[^26]: Asynchronous context tracking (`AsyncLocalStorage`). https://nodejs.org/docs/latest-v22.x/api/async_context.html
    - Asynchronous Programming › `AsyncLocalStorage` for request context — bullet 1: "The store propagates through `await`, timers and promise callbacks."
[^27]: Child process (shell options, DEP0190/DEP0196). https://nodejs.org/docs/latest-v22.x/api/child_process.html
    - Workers and Child Processes › `child_process` safety — bullet 1: "`exec()`/`execSync()` always use a shell."
[^28]: TLS (`getCACertificates`, `setDefaultCACertificates`, `--use-system-ca`). https://nodejs.org/docs/latest-v22.x/api/tls.html
    - Security › TLS and certificates — bullet 2: "`tls.getCACertificates('bundled' | 'system' | 'extra' | 'default')` (22.15)"
[^29]: Diagnostics Channel (built-in channels). https://nodejs.org/docs/latest-v22.x/api/diagnostics_channel.html
    - Observability and Diagnostics — bullet 2: "`diagnostics_channel` channels on 22 include"
[^30]: Process (`getBuiltinModule`, `features`, `constrainedMemory`, `availableMemory`, process I/O). https://nodejs.org/docs/latest-v22.x/api/process.html
    - Version-Aware Review › Verify before claiming something is unavailable — paragraph 2: "…without static imports, which is the idiomatic way to feature-detect."
[^31]: TC39 finished proposals (ES2024/ES2025 feature list). https://github.com/tc39/proposals/blob/main/finished-proposals.md
    - What's New in Node.js 22 (vs. Node 20 / 21) › Language (V8 11.3 → 12.4) — paragraph 1: "Feature-to-edition mapping follows the TC39 finished-proposals list."
[^32]: Node.js threat model (SECURITY.md). https://github.com/nodejs/node/blob/main/SECURITY.md
    - Security › New in 22: the permission model is stable (22.13) — bullet 4: "…Node threat model treats the code you run as trusted."
[^33]: TypeScript 5.8 release notes (`--erasableSyntaxOnly`). https://www.typescriptlang.org/docs/handbook/release-notes/typescript-5-8.html
    - TypeScript in Node 22: Type Stripping › What runs and what fails — paragraph 3: "`erasableSyntaxOnly` (TypeScript 5.8)"
